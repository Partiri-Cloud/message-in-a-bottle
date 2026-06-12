package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/partiri-cloud/message-in-a-bottle/internal/engine"
	"github.com/partiri-cloud/message-in-a-bottle/internal/model"
	"github.com/partiri-cloud/message-in-a-bottle/internal/repository"
)

type DigestHandler struct {
	notifRepo *repository.NotificationRepository
	subRepo   *repository.SubscriberRepository
	wfRepo    *repository.WorkflowRepository
	asynq     *asynq.Client
	rdb       *redis.Client
}

func NewDigestHandler(
	notifRepo *repository.NotificationRepository,
	subRepo *repository.SubscriberRepository,
	wfRepo *repository.WorkflowRepository,
	asynqClient *asynq.Client,
	rdb *redis.Client,
) *DigestHandler {
	return &DigestHandler{
		notifRepo: notifRepo,
		subRepo:   subRepo,
		wfRepo:    wfRepo,
		asynq:     asynqClient,
		rdb:       rdb,
	}
}

func (h *DigestHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var payload DigestPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal digest payload: %w", err)
	}

	// Collect digested notification IDs from Redis list.
	// Key format must match the one used in continuation.go scheduleDigestStep.
	digestKey := fmt.Sprintf("digest:%s:%s:%d:%s:%s", payload.EnvironmentID, payload.WorkflowID, payload.StepIndex, payload.SubscriberID, payload.DigestKey)
	rawIDs, err := h.rdb.LRange(ctx, digestKey, 0, -1).Result()
	if err != nil {
		return fmt.Errorf("lrange digest key: %w", err)
	}
	h.rdb.Del(ctx, digestKey)

	// A retried trigger task may have pushed the same notification twice.
	seen := make(map[string]struct{}, len(rawIDs))
	notifIDs := make([]string, 0, len(rawIDs))
	for _, id := range rawIDs {
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		notifIDs = append(notifIDs, id)
	}

	if len(notifIDs) == 0 {
		return nil
	}

	// Load workflow for subsequent steps
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

	wf, err := h.wfRepo.FindByID(ctx, envID, wfID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			log.Printf("digest: workflow %s no longer exists, dropping continuation", payload.WorkflowID)
			return nil
		}
		return fmt.Errorf("find workflow: %w", err)
	}

	subscriber, err := h.subRepo.FindByID(ctx, subID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			log.Printf("digest: subscriber %s no longer exists, dropping continuation", payload.SubscriberID)
			return nil
		}
		return fmt.Errorf("find subscriber: %w", err)
	}

	// Use the first notification as the representative for the post-digest
	// steps. It may have expired; conditions on it then resolve to nil.
	var notif *model.Notification
	if repID, err := bson.ObjectIDFromHex(notifIDs[0]); err == nil {
		notif, _ = h.notifRepo.FindByID(ctx, envID, repID)
	}

	digestData := map[string]any{"digestCount": len(notifIDs), "digestedIds": notifIDs}
	planned := engine.EvaluateWorkflow(wf, subscriber, digestData, notif)

	ref := stepRef{
		EnvironmentID:  payload.EnvironmentID,
		WorkflowID:     payload.WorkflowID,
		NotificationID: notifIDs[0],
		SubscriberID:   payload.SubscriberID,
		Payload:        digestData,
		Overrides:      payload.Overrides,
	}
	return scheduleSteps(ctx, h.asynq, h.rdb, planned, payload.StepIndex+1, ref, nil)
}
