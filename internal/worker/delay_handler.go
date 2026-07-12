package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/partiri-cloud/message-in-a-bottle/internal/repository"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type DelayHandler struct {
	wfRepo *repository.WorkflowRepository
	asynq  *asynq.Client
}

func NewDelayHandler(wfRepo *repository.WorkflowRepository, asynqClient *asynq.Client) *DelayHandler {
	return &DelayHandler{wfRepo: wfRepo, asynq: asynqClient}
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
	wf, err := h.wfRepo.FindByID(ctx, envID, wfID)
	if err != nil {
		return fmt.Errorf("find workflow: %w", err)
	}

	// Enqueue subsequent steps after the delay step
	enqueueFollowingDeliveries(h.asynq, wf, payload.StepIndex, "post-delay", DeliveryPayload{
		EnvironmentID:  payload.EnvironmentID,
		NotificationID: payload.NotificationID,
		SubscriberID:   payload.SubscriberID,
		Payload:        payload.Payload,
		Overrides:      payload.Overrides,
		Attempt:        0,
	})

	return nil
}
