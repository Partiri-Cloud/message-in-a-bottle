package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/hibiken/asynq"
	"github.com/partiri-cloud/message-in-a-bottle/internal/config"
	"github.com/partiri-cloud/message-in-a-bottle/internal/logging"
	"github.com/partiri-cloud/message-in-a-bottle/internal/provider"
	"github.com/partiri-cloud/message-in-a-bottle/internal/repository"
	"github.com/partiri-cloud/message-in-a-bottle/internal/worker"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func main() {
	logging.Init()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// MongoDB
	mongoClient, err := mongo.Connect(options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		slog.Error("failed to connect to mongodb", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := mongoClient.Disconnect(context.Background()); err != nil {
			slog.Error("mongodb disconnect error", "error", err)
		}
	}()
	db := mongoClient.Database(cfg.MongoDB)

	// Ensure indexes
	if err := repository.EnsureIndexes(ctx, db); err != nil {
		slog.Warn("failed to ensure indexes", "error", err)
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
	deliveryHandler := worker.NewDeliveryHandler(wfRepo, subRepo, intgRepo, notifRepo, activityRepo, prefRepo, rlRepo, factory, asynqClient, rdb, cfg.CredentialsEncryptionKeyBytes, cfg.RateLimitConfig, cfg.ActivityLogRetentionDays)
	delayHandler := worker.NewDelayHandler(wfRepo, subRepo, notifRepo, asynqClient, rdb)
	digestHandler := worker.NewDigestHandler(notifRepo, subRepo, wfRepo, asynqClient, rdb)
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
		slog.Info("Asynq worker started")
		if err := srv.Run(mux); err != nil {
			slog.Error("asynq worker error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down worker...")
	srv.Shutdown()
	slog.Info("worker stopped")
}
