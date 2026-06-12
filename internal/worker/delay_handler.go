package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/hibiken/asynq"
	"github.com/partiri-cloud/message-in-a-bottle/internal/engine"
	"github.com/partiri-cloud/message-in-a-bottle/internal/repository"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type DelayHandler struct {
	wfRepo    *repository.WorkflowRepository
	subRepo   *repository.SubscriberRepository
	notifRepo *repository.NotificationRepository
	asynq     *asynq.Client
	rdb       *redis.Client
}

func NewDelayHandler(
	wfRepo *repository.WorkflowRepository,
	subRepo *repository.SubscriberRepository,
	notifRepo *repository.NotificationRepository,
	asynqClient *asynq.Client,
	rdb *redis.Client,
) *DelayHandler {
	return &DelayHandler{
		wfRepo:    wfRepo,
		subRepo:   subRepo,
		notifRepo: notifRepo,
		asynq:     asynqClient,
		rdb:       rdb,
	}
}

func (h *DelayHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var payload DelayPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal delay payload: %w", err)
	}

	wfID, err := bson.ObjectIDFromHex(payload.WorkflowID)
	if err != nil {
		return fmt.Errorf("invalid workflowId %q: %w", payload.WorkflowID, err)
	}
	envID, err := bson.ObjectIDFromHex(payload.EnvironmentID)
	if err != nil {
		return fmt.Errorf("invalid environmentId %q: %w", payload.EnvironmentID, err)
	}
	subID, err := bson.ObjectIDFromHex(payload.SubscriberID)
	if err != nil {
		return fmt.Errorf("invalid subscriberId %q: %w", payload.SubscriberID, err)
	}
	notifID, err := bson.ObjectIDFromHex(payload.NotificationID)
	if err != nil {
		return fmt.Errorf("invalid notificationId %q: %w", payload.NotificationID, err)
	}

	wf, err := h.wfRepo.FindByID(ctx, envID, wfID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			log.Printf("delay: workflow %s no longer exists, dropping continuation", payload.WorkflowID)
			return nil
		}
		return fmt.Errorf("find workflow: %w", err)
	}

	subscriber, err := h.subRepo.FindByID(ctx, subID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			log.Printf("delay: subscriber %s no longer exists, dropping continuation", payload.SubscriberID)
			return nil
		}
		return fmt.Errorf("find subscriber: %w", err)
	}

	notif, err := h.notifRepo.FindByID(ctx, envID, notifID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			log.Printf("delay: notification %s no longer exists, dropping continuation", payload.NotificationID)
			return nil
		}
		return fmt.Errorf("find notification: %w", err)
	}

	// Re-evaluate the workflow at fire time so step conditions (e.g. "only if
	// the notification is still unseen") reflect the current state, then
	// schedule the steps after the delay. scheduleSteps stops at the next
	// control step, which owns the rest of the chain.
	planned := engine.EvaluateWorkflow(wf, subscriber, payload.Payload, notif)

	ref := stepRef{
		EnvironmentID:  payload.EnvironmentID,
		WorkflowID:     payload.WorkflowID,
		NotificationID: payload.NotificationID,
		SubscriberID:   payload.SubscriberID,
		Payload:        payload.Payload,
		Overrides:      payload.Overrides,
	}
	return scheduleSteps(ctx, h.asynq, h.rdb, planned, payload.StepIndex+1, ref, nil)
}
