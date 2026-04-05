package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/hibiken/asynq"
	"github.com/partiri/message-in-a-bottle/internal/repository"
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

	wfID, _ := bson.ObjectIDFromHex(payload.WorkflowID)
	wf, err := h.wfRepo.FindByID(ctx, wfID)
	if err != nil {
		return fmt.Errorf("find workflow: %w", err)
	}

	// Enqueue subsequent steps after the delay step
	for i := payload.StepIndex + 1; i < len(wf.Steps); i++ {
		step := wf.Steps[i]
		if step.Type == "delay" || step.Type == "digest" {
			break // Stop at next control step
		}

		dp := DeliveryPayload{
			EnvironmentID:  payload.EnvironmentID,
			NotificationID: payload.NotificationID,
			SubscriberID:   payload.SubscriberID,
			Channel:        step.Type,
			StepIndex:      i,
			Payload:        payload.Payload,
			Overrides:      payload.Overrides,
			Attempt:        0,
		}
		data, _ := json.Marshal(dp)
		task := asynq.NewTask(TaskTypeDelivery, data)
		if _, err := h.asynq.Enqueue(task); err != nil {
			log.Printf("failed to enqueue post-delay delivery step %d: %v", i, err)
		}
	}

	return nil
}
