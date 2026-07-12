package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/coder/websocket"
	"github.com/partiri-cloud/message-in-a-bottle/internal/config"
	"github.com/partiri-cloud/message-in-a-bottle/internal/logging"
	"github.com/partiri-cloud/message-in-a-bottle/internal/middleware"
	"github.com/partiri-cloud/message-in-a-bottle/internal/repository"
	wslib "github.com/partiri-cloud/message-in-a-bottle/internal/ws"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type wsAuthMessage struct {
	ApiKey          string `json:"apiKey"`
	SubscriberToken string `json:"subscriberToken"`
	SubscriberID    string `json:"subscriberId"`
}

func main() {
	logger := logging.Init()

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", slog.Any("error", err))
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// MongoDB
	mongoClient, err := mongo.Connect(options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		logger.Error("failed to connect to mongodb", slog.Any("error", err))
		os.Exit(1)
	}
	defer func() {
		if err := mongoClient.Disconnect(context.Background()); err != nil {
			logger.Error("mongodb disconnect error", slog.Any("error", err))
		}
	}()
	db := mongoClient.Database(cfg.MongoDB)

	// Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
	})
	defer rdb.Close()

	// Repositories
	envRepo := repository.NewEnvironmentRepository(db)
	subRepo := repository.NewSubscriberRepository(db)
	notifRepo := repository.NewNotificationRepository(db)

	// WebSocket hub
	hub := wslib.NewHub(rdb)
	hub.SubscribeRedis(ctx)

	presence := wslib.NewPresenceTracker(subRepo)

	// WebSocket origin check — fail closed by default.
	// Set WS_INSECURE_ALLOW_ALL_ORIGINS=true only in controlled dev environments.
	acceptOpts := &websocket.AcceptOptions{}
	if os.Getenv("WS_INSECURE_ALLOW_ALL_ORIGINS") == "true" {
		logger.Warn("WS_INSECURE_ALLOW_ALL_ORIGINS=true: accepting all WebSocket origins — NOT safe for production")
		acceptOpts.InsecureSkipVerify = true
	} else if len(cfg.WSAllowedOrigins) > 0 {
		acceptOpts.OriginPatterns = cfg.WSAllowedOrigins
	} else {
		logger.Error("WS_ALLOWED_ORIGINS is required; set it or use WS_INSECURE_ALLOW_ALL_ORIGINS=true for dev only")
		os.Exit(1)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, acceptOpts)
		if err != nil {
			logger.Error("websocket accept error", slog.Any("error", err))
			return
		}

		// Require authentication via first message within 10 seconds
		authCtx, authCancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer authCancel()

		_, msgBytes, err := conn.Read(authCtx)
		if err != nil {
			conn.Close(websocket.StatusPolicyViolation, "authentication timeout")
			return
		}

		var authMsg wsAuthMessage
		if err := json.Unmarshal(msgBytes, &authMsg); err != nil {
			conn.Close(websocket.StatusPolicyViolation, "invalid auth message")
			return
		}

		// Validate API key
		if authMsg.ApiKey == "" {
			conn.Close(websocket.StatusPolicyViolation, "missing apiKey")
			return
		}
		hash := sha256.Sum256([]byte(authMsg.ApiKey))
		keyHash := hex.EncodeToString(hash[:])

		env, ak, err := envRepo.FindByAPIKeyHash(r.Context(), keyHash)
		if err != nil || !ak.IsActive {
			conn.Close(websocket.StatusPolicyViolation, "invalid API key")
			return
		}
		if ak.ExpiresAt != nil && ak.ExpiresAt.Before(time.Now()) {
			conn.Close(websocket.StatusPolicyViolation, "API key expired")
			return
		}

		// Validate subscriber token (HMAC)
		if authMsg.SubscriberToken == "" {
			conn.Close(websocket.StatusPolicyViolation, "missing subscriberToken")
			return
		}
		tokenPayload, err := middleware.ValidateSubscriberToken(authMsg.SubscriberToken, cfg.SubscriberHMACSecret)
		if err != nil {
			conn.Close(websocket.StatusPolicyViolation, "invalid subscriber token")
			return
		}

		// Ensure token subscriber matches requested subscriber
		if authMsg.SubscriberID == "" {
			conn.Close(websocket.StatusPolicyViolation, "missing subscriberId")
			return
		}
		if tokenPayload.SubscriberID != authMsg.SubscriberID {
			conn.Close(websocket.StatusPolicyViolation, "subscriber token mismatch")
			return
		}

		sub, err := subRepo.FindBySubscriberID(r.Context(), env.ID, authMsg.SubscriberID)
		if err != nil {
			conn.Close(websocket.StatusPolicyViolation, "subscriber not found")
			return
		}

		// Auth succeeded, send confirmation
		ackMsg, _ := json.Marshal(map[string]string{"event": "authenticated"})
		conn.Write(r.Context(), websocket.MessageText, ackMsg)

		room := wslib.RoomKey(env.ID, sub.ID)
		client := wslib.NewClient(conn, hub, room, env.ID, sub.ID, subRepo, notifRepo)

		hub.Register(room, client)
		presence.SetOnline(r.Context(), sub.ID)

		client.Run(ctx)
	})

	srv := &http.Server{
		Addr:    ":" + cfg.WSPort,
		Handler: mux,
	}

	go func() {
		logger.Info("WebSocket server starting", slog.String("port", cfg.WSPort))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("ws server error", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down WebSocket server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("ws server forced shutdown", slog.Any("error", err))
		os.Exit(1)
	}
	logger.Info("WebSocket server stopped")
}
