package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/partiri-cloud/message-in-a-bottle/internal/config"
	"github.com/partiri-cloud/message-in-a-bottle/internal/handler"
	"github.com/partiri-cloud/message-in-a-bottle/internal/logging"
	"github.com/partiri-cloud/message-in-a-bottle/internal/middleware"
	"github.com/partiri-cloud/message-in-a-bottle/internal/repository"
	"github.com/partiri-cloud/message-in-a-bottle/internal/service"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

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

	// Ensure indexes
	if err := repository.EnsureIndexes(ctx, db); err != nil {
		logger.Warn("failed to ensure indexes", slog.Any("error", err))
	}

	// Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
	})
	defer rdb.Close()

	// Asynq client
	asynqClient := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
	})
	defer asynqClient.Close()

	// Repositories
	envRepo := repository.NewEnvironmentRepository(db)
	subRepo := repository.NewSubscriberRepository(db)
	topicRepo := repository.NewTopicRepository(db)
	tsRepo := repository.NewTopicSubscriberRepository(db)
	wfRepo := repository.NewWorkflowRepository(db)
	intgRepo := repository.NewIntegrationRepository(db)
	tmplRepo := repository.NewTemplateRepository(db)
	prefRepo := repository.NewPreferenceRepository(db)
	notifRepo := repository.NewNotificationRepository(db)
	activityRepo := repository.NewActivityRepository(db)

	// Services
	triggerSvc := service.NewTriggerService(wfRepo, subRepo, tsRepo, topicRepo, notifRepo, asynqClient, cfg.NotificationRetentionDays)
	tmplSvc := service.NewTemplateService(tmplRepo, subRepo, asynqClient)

	// Handlers
	handlers := &handler.Handlers{
		Subscriber:   handler.NewSubscriberHandler(subRepo, tsRepo),
		Topic:        handler.NewTopicHandler(topicRepo, tsRepo, subRepo),
		Workflow:     handler.NewWorkflowHandler(wfRepo),
		Integration:  handler.NewIntegrationHandler(intgRepo, cfg.CredentialsEncryptionKeyBytes),
		Template:     handler.NewTemplateHandler(tmplRepo, tmplSvc),
		Preference:   handler.NewPreferenceHandler(prefRepo, subRepo, wfRepo),
		Notification: handler.NewNotificationHandler(notifRepo, activityRepo, subRepo),
		Event:        handler.NewEventHandler(triggerSvc),
		Admin:        handler.NewAdminHandler(envRepo),
		Announcement: handler.NewAnnouncementHandler(rdb),
	}

	// Gin router. Release mode + structured request logging for production;
	// GIN_MODE=debug can still override locally.
	if gin.Mode() == gin.DebugMode && os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(middleware.RequestID(), middleware.Logging(logger), gin.Recovery())
	router.Use(middleware.CORS(cfg.CORSAllowedOrigins))
	router.MaxMultipartMemory = cfg.MaxRequestBodyBytes
	router.Use(func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, cfg.MaxRequestBodyBytes)
		c.Next()
	})
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	handler.RegisterRoutes(router, handlers, envRepo, cfg.AdminSecret, cfg.SubscriberHMACSecret)

	srv := &http.Server{
		Addr:    ":" + cfg.APIPort,
		Handler: router,
	}

	go func() {
		logger.Info("API server starting", slog.String("port", cfg.APIPort))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("api server error", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down API server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("api server forced shutdown", slog.Any("error", err))
		os.Exit(1)
	}
	logger.Info("API server stopped")
}
