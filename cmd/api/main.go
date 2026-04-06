package main

import (
	"context"
	"encoding/hex"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/partiri-cloud/message-in-a-box/internal/config"
	"github.com/partiri-cloud/message-in-a-box/internal/handler"
	"github.com/partiri-cloud/message-in-a-box/internal/repository"
	"github.com/partiri-cloud/message-in-a-box/internal/service"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// MongoDB
	mongoClient, err := mongo.Connect(options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		log.Fatalf("failed to connect to mongodb: %v", err)
	}
	defer func() {
		if err := mongoClient.Disconnect(context.Background()); err != nil {
			log.Printf("mongodb disconnect error: %v", err)
		}
	}()
	db := mongoClient.Database(cfg.MongoDB)

	// Ensure indexes
	if err := repository.EnsureIndexes(ctx, db); err != nil {
		log.Printf("warning: failed to ensure indexes: %v", err)
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

	// Encryption key
	var encryptionKey []byte
	if cfg.CredentialsEncryptionKey != "" {
		encryptionKey, err = hex.DecodeString(cfg.CredentialsEncryptionKey)
		if err != nil {
			log.Fatalf("invalid CREDENTIALS_ENCRYPTION_KEY: %v", err)
		}
	}

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
		Integration:  handler.NewIntegrationHandler(intgRepo, encryptionKey),
		Template:     handler.NewTemplateHandler(tmplRepo, tmplSvc),
		Preference:   handler.NewPreferenceHandler(prefRepo, subRepo),
		Notification: handler.NewNotificationHandler(notifRepo, activityRepo, subRepo),
		Event:        handler.NewEventHandler(triggerSvc),
		Admin:        handler.NewAdminHandler(envRepo),
	}

	// Gin router
	router := gin.Default()
	router.MaxMultipartMemory = cfg.MaxRequestBodyBytes
	router.Use(func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, cfg.MaxRequestBodyBytes)
		c.Next()
	})
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	handler.RegisterRoutes(router, handlers, envRepo, cfg.AdminSecret, cfg.SubscriberHMACSecret)

	_ = rdb

	srv := &http.Server{
		Addr:    ":" + cfg.APIPort,
		Handler: router,
	}

	go func() {
		log.Printf("API server starting on :%s", cfg.APIPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("api server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down API server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("api server forced shutdown: %v", err)
	}
	log.Println("API server stopped")
}
