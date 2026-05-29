package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/partiri-cloud/message-in-a-bottle/internal/repository"
)

type DigestHandler struct {
	notifRepo *repository.NotificationRepository
	wfRepo    *repository.WorkflowRepository
	asynq     *asynq.Client
	rdb       *redis.Client
}

func NewDigestHandler(
	notifRepo *repository.NotificationRepository,
	wfRepo *repository.WorkflowRepository,
	asynqClient *asynq.Client,
	rdb *redis.Client,
) *DigestHandler {
	return &DigestHandler{
		notifRepo: notifRepo,
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
	// Key format must match the one used in trigger_handler.go enqueueDigest.
	digestKey := fmt.Sprintf("digest:%s:%s:%d:%s:%s", payload.EnvironmentID, payload.WorkflowID, payload.StepIndex, payload.SubscriberID, payload.DigestKey)
	notifIDs, err := h.rdb.LRange(ctx, digestKey, 0, -1).Result()
	if err != nil {
		return fmt.Errorf("lrange digest key: %w", err)
	}
	h.rdb.Del(ctx, digestKey)

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
	wf, err := h.wfRepo.FindByID(ctx, envID, wfID)
	if err != nil {
		return fmt.Errorf("find workflow: %w", err)
	}

	// Collect digested notification object IDs
	var digestedOIDs []bson.ObjectID
	for _, idStr := range notifIDs {
		oid, err := bson.ObjectIDFromHex(idStr)
		if err == nil {
			digestedOIDs = append(digestedOIDs, oid)
		}
	}

	// Enqueue delivery for steps after the digest step
	for i := payload.StepIndex + 1; i < len(wf.Steps); i++ {
		step := wf.Steps[i]
		if step.Type == "delay" || step.Type == "digest" {
			break
		}

		// Use first notification as the representative
		dp := DeliveryPayload{
			EnvironmentID:  payload.EnvironmentID,
			NotificationID: notifIDs[0],
			SubscriberID:   payload.SubscriberID,
			Channel:        step.Type,
			StepIndex:      i,
			Payload:        map[string]any{"digestCount": len(notifIDs), "digestedIds": notifIDs},
			Attempt:        0,
		}
		data, merr := json.Marshal(dp)
		if merr != nil {
			log.Printf("failed to marshal post-digest delivery step %d: %v", i, merr)
			continue
		}
		task := asynq.NewTask(TaskTypeDelivery, data)
		if _, err := h.asynq.Enqueue(task); err != nil {
			log.Printf("failed to enqueue post-digest delivery step %d: %v", i, err)
		}
	}

	return nil
}
