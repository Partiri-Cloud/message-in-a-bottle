package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/partiri-cloud/message-in-a-box/internal/repository"
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

	// Collect digested notification IDs from Redis list
	digestKey := fmt.Sprintf("digest:%s:%s:%s:%s", payload.EnvironmentID, payload.WorkflowID, payload.SubscriberID, payload.DigestKey)
	notifIDs, err := h.rdb.LRange(ctx, digestKey, 0, -1).Result()
	if err != nil {
		return fmt.Errorf("lrange digest key: %w", err)
	}
	h.rdb.Del(ctx, digestKey)

	if len(notifIDs) == 0 {
		return nil
	}

	// Load workflow for subsequent steps
	wfID, _ := bson.ObjectIDFromHex(payload.WorkflowID)
	wf, err := h.wfRepo.FindByID(ctx, wfID)
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
		data, _ := json.Marshal(dp)
		task := asynq.NewTask(TaskTypeDelivery, data)
		if _, err := h.asynq.Enqueue(task); err != nil {
			log.Printf("failed to enqueue post-digest delivery step %d: %v", i, err)
		}
	}

	return nil
}
