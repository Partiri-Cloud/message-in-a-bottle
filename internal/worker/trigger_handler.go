package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/hibiken/asynq"
	"github.com/partiri/message-in-a-bottle/internal/engine"
	"github.com/partiri/message-in-a-bottle/internal/model"
	"github.com/partiri/message-in-a-bottle/internal/repository"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type TriggerHandler struct {
	wfRepo       *repository.WorkflowRepository
	subRepo      *repository.SubscriberRepository
	notifRepo    *repository.NotificationRepository
	activityRepo *repository.ActivityRepository
	asynq        *asynq.Client
}

func NewTriggerHandler(
	wfRepo *repository.WorkflowRepository,
	subRepo *repository.SubscriberRepository,
	notifRepo *repository.NotificationRepository,
	activityRepo *repository.ActivityRepository,
	asynqClient *asynq.Client,
) *TriggerHandler {
	return &TriggerHandler{
		wfRepo:       wfRepo,
		subRepo:      subRepo,
		notifRepo:    notifRepo,
		activityRepo: activityRepo,
		asynq:        asynqClient,
	}
}

func (h *TriggerHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var payload TriggerPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal trigger payload: %w", err)
	}

	envID, _ := bson.ObjectIDFromHex(payload.EnvironmentID)
	wfID, _ := bson.ObjectIDFromHex(payload.WorkflowID)

	wf, err := h.wfRepo.FindByID(ctx, wfID)
	if err != nil {
		return fmt.Errorf("find workflow: %w", err)
	}

	if !wf.IsActive {
		log.Printf("workflow %s is inactive, skipping", wf.Identifier)
		return nil
	}

	for _, subIDStr := range payload.SubscriberIDs {
		subID, _ := bson.ObjectIDFromHex(subIDStr)

		subscriber, err := h.subRepo.FindByID(ctx, subID)
		if err != nil {
			log.Printf("subscriber %s not found, skipping", subIDStr)
			continue
		}

		// Find notification for this subscriber + transaction
		notif, err := h.notifRepo.FindByTransactionID(ctx, envID, payload.TransactionID)
		if err != nil {
			log.Printf("notification not found for tx %s, skipping", payload.TransactionID)
			continue
		}

		// Log workflow started
		h.activityRepo.Create(ctx, &model.ActivityLog{
			EnvironmentID:  envID,
			NotificationID: notif.ID,
			SubscriberID:   subID,
			Event:          "workflow_started",
			Detail:         map[string]any{"workflowId": wf.Identifier},
			ExpireAt:       time.Now().Add(30 * 24 * time.Hour),
		})

		// Evaluate workflow steps
		planned := engine.EvaluateWorkflow(wf, subscriber, payload.Payload, notif)

		for _, ps := range planned {
			if ps.Skipped {
				h.activityRepo.Create(ctx, &model.ActivityLog{
					EnvironmentID:  envID,
					NotificationID: notif.ID,
					SubscriberID:   subID,
					Channel:        ps.Step.Type,
					Event:          "step_skipped",
					Detail:         map[string]any{"reason": ps.Reason, "stepIndex": ps.StepIndex},
					ExpireAt:       time.Now().Add(30 * 24 * time.Hour),
				})
				continue
			}

			switch ps.Step.Type {
			case "delay":
				if err := h.enqueueDelay(ps, notif, subscriber, wf, payload); err != nil {
					log.Printf("failed to enqueue delay: %v", err)
				}
			case "digest":
				if err := h.enqueueDigest(ps, notif, subscriber, wf, payload); err != nil {
					log.Printf("failed to enqueue digest: %v", err)
				}
			default:
				// Channel delivery (email, sms, push, in_app, slack, ms_teams)
				if err := h.enqueueDelivery(ps, notif, subscriber, payload); err != nil {
					log.Printf("failed to enqueue delivery: %v", err)
				}
			}
		}
	}

	return nil
}

func (h *TriggerHandler) enqueueDelivery(ps engine.PlannedStep, notif *model.Notification, sub *model.Subscriber, payload TriggerPayload) error {
	dp := DeliveryPayload{
		EnvironmentID:  payload.EnvironmentID,
		NotificationID: notif.ID.Hex(),
		SubscriberID:   sub.ID.Hex(),
		Channel:        ps.Step.Type,
		StepIndex:      ps.StepIndex,
		Payload:        payload.Payload,
		Overrides:      payload.Overrides,
		Attempt:        0,
	}
	data, _ := json.Marshal(dp)
	task := asynq.NewTask(TaskTypeDelivery, data)
	_, err := h.asynq.Enqueue(task)
	return err
}

func (h *TriggerHandler) enqueueDelay(ps engine.PlannedStep, notif *model.Notification, sub *model.Subscriber, wf *model.Workflow, payload TriggerPayload) error {
	if ps.Step.DelayConfig == nil {
		return nil
	}
	duration := ParseDuration(ps.Step.DelayConfig.Amount, ps.Step.DelayConfig.Unit)

	dp := DelayPayload{
		EnvironmentID:  payload.EnvironmentID,
		NotificationID: notif.ID.Hex(),
		SubscriberID:   sub.ID.Hex(),
		WorkflowID:     wf.ID.Hex(),
		StepIndex:      ps.StepIndex,
		Payload:        payload.Payload,
		Overrides:      payload.Overrides,
	}
	data, _ := json.Marshal(dp)
	task := asynq.NewTask(TaskTypeDelay, data)
	_, err := h.asynq.Enqueue(task, asynq.ProcessIn(duration))
	return err
}

func (h *TriggerHandler) enqueueDigest(ps engine.PlannedStep, notif *model.Notification, sub *model.Subscriber, wf *model.Workflow, payload TriggerPayload) error {
	if ps.Step.DigestConfig == nil {
		return nil
	}

	dp := DigestPayload{
		EnvironmentID: payload.EnvironmentID,
		WorkflowID:    wf.ID.Hex(),
		SubscriberID:  sub.ID.Hex(),
		Channel:       ps.Step.Type,
		DigestKey:     ps.Step.DigestConfig.DigestKey,
		StepIndex:     ps.StepIndex,
	}
	data, _ := json.Marshal(dp)
	duration := ParseDuration(ps.Step.DigestConfig.Amount, ps.Step.DigestConfig.Unit)
	task := asynq.NewTask(TaskTypeDigest, data)
	_, err := h.asynq.Enqueue(task, asynq.ProcessIn(duration))
	return err
}

func ParseDuration(amount int, unit string) time.Duration {
	switch unit {
	case "minutes":
		return time.Duration(amount) * time.Minute
	case "hours":
		return time.Duration(amount) * time.Hour
	case "days":
		return time.Duration(amount) * 24 * time.Hour
	default:
		return time.Duration(amount) * time.Minute
	}
}
