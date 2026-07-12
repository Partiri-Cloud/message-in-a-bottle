package worker

import (
	"context"
	"encoding/json"
	"fmt"

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

	// Enqueue delivery for steps after the digest step, with the first
	// notification as the representative.
	enqueueFollowingDeliveries(h.asynq, wf, payload.StepIndex, "post-digest", DeliveryPayload{
		EnvironmentID:  payload.EnvironmentID,
		NotificationID: notifIDs[0],
		SubscriberID:   payload.SubscriberID,
		Payload:        map[string]any{"digestCount": len(notifIDs), "digestedIds": notifIDs},
		Attempt:        0,
	})

	return nil
}
