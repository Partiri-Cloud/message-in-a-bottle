package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/hibiken/asynq"
	"github.com/partiri-cloud/message-in-a-bottle/internal/engine"
	"github.com/partiri-cloud/message-in-a-bottle/internal/model"
	"github.com/partiri-cloud/message-in-a-bottle/internal/repository"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type TriggerHandler struct {
	wfRepo        *repository.WorkflowRepository
	subRepo       *repository.SubscriberRepository
	notifRepo     *repository.NotificationRepository
	activityRepo  *repository.ActivityRepository
	asynq         *asynq.Client
	rdb           *redis.Client
	retentionDays int
}

func NewTriggerHandler(
	wfRepo *repository.WorkflowRepository,
	subRepo *repository.SubscriberRepository,
	notifRepo *repository.NotificationRepository,
	activityRepo *repository.ActivityRepository,
	asynqClient *asynq.Client,
	rdb *redis.Client,
	retentionDays int,
) *TriggerHandler {
	return &TriggerHandler{
		wfRepo:        wfRepo,
		subRepo:       subRepo,
		notifRepo:     notifRepo,
		activityRepo:  activityRepo,
		asynq:         asynqClient,
		rdb:           rdb,
		retentionDays: retentionDays,
	}
}

func (h *TriggerHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var payload TriggerPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal trigger payload: %w", err)
	}

	envID, err := bson.ObjectIDFromHex(payload.EnvironmentID)
	if err != nil {
		return fmt.Errorf("invalid environmentId %q: %w", payload.EnvironmentID, err)
	}
	wfID, err := bson.ObjectIDFromHex(payload.WorkflowID)
	if err != nil {
		return fmt.Errorf("invalid workflowId %q: %w", payload.WorkflowID, err)
	}

	wf, err := h.wfRepo.FindByID(ctx, wfID)
	if err != nil {
		return fmt.Errorf("find workflow: %w", err)
	}

	if !wf.IsActive {
		log.Printf("workflow %s is inactive, skipping", wf.Identifier)
		return nil
	}

	for subIDStr, notifIDStr := range payload.SubscriberIDs {
		subID, err := bson.ObjectIDFromHex(subIDStr)
		if err != nil {
			log.Printf("invalid subscriberId %q, skipping: %v", subIDStr, err)
			continue
		}

		subscriber, err := h.subRepo.FindByID(ctx, subID)
		if err != nil {
			log.Printf("subscriber %s not found, skipping", subIDStr)
			continue
		}

		notifID, err := bson.ObjectIDFromHex(notifIDStr)
		if err != nil {
			log.Printf("invalid notificationId %q for subscriber %s, skipping: %v", notifIDStr, subIDStr, err)
			continue
		}
		notif, err := h.notifRepo.FindByID(ctx, notifID)
		if err != nil {
			log.Printf("notification %s not found for subscriber %s, skipping", notifIDStr, subIDStr)
			continue
		}

		h.activityRepo.Create(ctx, &model.ActivityLog{
			EnvironmentID:  envID,
			NotificationID: notif.ID,
			SubscriberID:   subID,
			Event:          "workflow_started",
			Detail:         map[string]any{"workflowId": wf.Identifier},
			ExpireAt:       time.Now().Add(time.Duration(h.retentionDays) * 24 * time.Hour),
		})

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
					ExpireAt:       time.Now().Add(time.Duration(h.retentionDays) * 24 * time.Hour),
				})
				continue
			}

			switch ps.Step.Type {
			case "delay":
				if err := h.enqueueDelay(ps, notif, subscriber, wf, payload); err != nil {
					log.Printf("failed to enqueue delay: %v", err)
				}
			case "digest":
				if err := h.enqueueDigest(ctx, ps, notif, subscriber, wf, payload); err != nil {
					log.Printf("failed to enqueue digest: %v", err)
				}
			default:
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
	data, err := json.Marshal(dp)
	if err != nil {
		return fmt.Errorf("marshal delivery payload: %w", err)
	}
	task := asynq.NewTask(TaskTypeDelivery, data)
	_, err = h.asynq.Enqueue(task)
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
	data, err := json.Marshal(dp)
	if err != nil {
		return fmt.Errorf("marshal delay payload: %w", err)
	}
	task := asynq.NewTask(TaskTypeDelay, data)
	_, err = h.asynq.Enqueue(task, asynq.ProcessIn(duration))
	return err
}

func (h *TriggerHandler) enqueueDigest(ctx context.Context, ps engine.PlannedStep, notif *model.Notification, sub *model.Subscriber, wf *model.Workflow, payload TriggerPayload) error {
	if ps.Step.DigestConfig == nil {
		return nil
	}

	// Include step index to prevent key collision when multiple digest steps share the same DigestKey string.
	digestKey := fmt.Sprintf("digest:%s:%s:%d:%s:%s",
		payload.EnvironmentID,
		wf.ID.Hex(),
		ps.StepIndex,
		sub.ID.Hex(),
		ps.Step.DigestConfig.DigestKey,
	)

	// Accumulate this notification's ID in the Redis list.
	// RPUSH returns the list length after the push.
	count, err := h.rdb.RPush(ctx, digestKey, notif.ID.Hex()).Result()
	if err != nil {
		return fmt.Errorf("rpush digest key: %w", err)
	}

	duration := ParseDuration(ps.Step.DigestConfig.Amount, ps.Step.DigestConfig.Unit)

	if count == 1 {
		// First notification in this window: set TTL and schedule the digest task.
		// Add a small buffer so the key outlives the task.
		h.rdb.Expire(ctx, digestKey, duration+5*time.Minute)

		dp := DigestPayload{
			EnvironmentID: payload.EnvironmentID,
			WorkflowID:    wf.ID.Hex(),
			SubscriberID:  sub.ID.Hex(),
			Channel:       ps.Step.Type,
			DigestKey:     ps.Step.DigestConfig.DigestKey,
			StepIndex:     ps.StepIndex,
		}
		data, err := json.Marshal(dp)
		if err != nil {
			return fmt.Errorf("marshal digest payload: %w", err)
		}
		task := asynq.NewTask(TaskTypeDigest, data)
		_, err = h.asynq.Enqueue(task, asynq.ProcessIn(duration))
		return err
	}

	// count > 1: a digest task is already scheduled for this window; nothing more to do.
	return nil
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
