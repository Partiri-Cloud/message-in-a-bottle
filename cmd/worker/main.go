package main

import (
	"context"
	"encoding/hex"
	"log"
	"os/signal"
	"syscall"

	"github.com/hibiken/asynq"
	"github.com/partiri-cloud/message-in-a-bottle/internal/config"
	"github.com/partiri-cloud/message-in-a-bottle/internal/provider"
	"github.com/partiri-cloud/message-in-a-bottle/internal/repository"
	"github.com/partiri-cloud/message-in-a-bottle/internal/worker"
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

	// Asynq client (for re-enqueueing)
	asynqClient := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
	})
	defer asynqClient.Close()

	// Encryption key
	var encKey []byte
	if cfg.CredentialsEncryptionKey != "" {
		encKey, err = hex.DecodeString(cfg.CredentialsEncryptionKey)
		if err != nil {
			log.Fatalf("invalid CREDENTIALS_ENCRYPTION_KEY: %v", err)
		}
	}

	// Repositories
	wfRepo := repository.NewWorkflowRepository(db)
	subRepo := repository.NewSubscriberRepository(db)
	intgRepo := repository.NewIntegrationRepository(db)
	notifRepo := repository.NewNotificationRepository(db)
	activityRepo := repository.NewActivityRepository(db)
	prefRepo := repository.NewPreferenceRepository(db)
	rlRepo := repository.NewRateLimitRepository(db)

	// Provider factory
	factory := provider.NewProviderFactory()

	// Task handlers
	triggerHandler := worker.NewTriggerHandler(wfRepo, subRepo, notifRepo, activityRepo, asynqClient, rdb, cfg.ActivityLogRetentionDays)
	deliveryHandler := worker.NewDeliveryHandler(wfRepo, subRepo, intgRepo, notifRepo, activityRepo, prefRepo, rlRepo, factory, asynqClient, rdb, encKey, cfg.RateLimitConfig, cfg.ActivityLogRetentionDays)
	delayHandler := worker.NewDelayHandler(wfRepo, asynqClient)
	digestHandler := worker.NewDigestHandler(notifRepo, wfRepo, asynqClient, rdb)
	broadcastHandler := worker.NewBroadcastHandler(wfRepo, subRepo, notifRepo, asynqClient)

	// Asynq worker
	srv := asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword,
		},
		asynq.Config{
			Concurrency: 10,
			Queues: map[string]int{
				"critical": 6,
				"default":  3,
				"low":      1,
			},
		},
	)

	mux := asynq.NewServeMux()
	mux.HandleFunc(worker.TaskTypeTrigger, triggerHandler.ProcessTask)
	mux.HandleFunc(worker.TaskTypeDelivery, deliveryHandler.ProcessTask)
	mux.HandleFunc(worker.TaskTypeDelay, delayHandler.ProcessTask)
	mux.HandleFunc(worker.TaskTypeDigest, digestHandler.ProcessTask)
	mux.HandleFunc(worker.TaskTypeBroadcast, broadcastHandler.ProcessTask)

	go func() {
		log.Println("Asynq worker started")
		if err := srv.Run(mux); err != nil {
			log.Fatalf("asynq worker error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down worker...")
	srv.Shutdown()
	log.Println("worker stopped")
}
