package worker

import (
	"context"
	"encoding/json"
	"errors"
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

	wf, err := h.wfRepo.FindByID(ctx, envID, wfID)
	if err != nil {
		return fmt.Errorf("find workflow: %w", err)
	}

	if !wf.IsActive {
		log.Printf("workflow %s is inactive, skipping", wf.Identifier)
		return nil
	}

	var errs []error
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
		notif, err := h.notifRepo.FindByID(ctx, envID, notifID)
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

		ref := stepRef{
			EnvironmentID:  payload.EnvironmentID,
			WorkflowID:     wf.ID.Hex(),
			NotificationID: notif.ID.Hex(),
			SubscriberID:   subscriber.ID.Hex(),
			Payload:        payload.Payload,
			Overrides:      payload.Overrides,
		}
		onSkip := func(ps engine.PlannedStep) {
			h.activityRepo.Create(ctx, &model.ActivityLog{
				EnvironmentID:  envID,
				NotificationID: notif.ID,
				SubscriberID:   subID,
				Channel:        ps.Step.Type,
				Event:          "step_skipped",
				Detail:         map[string]any{"reason": ps.Reason, "stepIndex": ps.StepIndex},
				ExpireAt:       time.Now().Add(time.Duration(h.retentionDays) * 24 * time.Hour),
			})
		}

		if err := scheduleSteps(ctx, h.asynq, h.rdb, planned, 0, ref, onSkip); err != nil {
			errs = append(errs, fmt.Errorf("subscriber %s: %w", subIDStr, err))
		}
	}

	// Returning an error makes asynq retry the whole trigger task; deterministic
	// task IDs plus the delivery handler's status guard make the re-run safe.
	return errors.Join(errs...)
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
