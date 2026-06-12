package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/partiri-cloud/message-in-a-bottle/internal/engine"
	"github.com/redis/go-redis/v9"
)

// stepRef identifies the workflow execution a step belongs to. All IDs are hex strings.
type stepRef struct {
	EnvironmentID  string
	WorkflowID     string
	NotificationID string
	SubscriberID   string
	Payload        map[string]any
	Overrides      map[string]any
}

func deliveryTaskID(notifID string, stepIndex, attempt int) string {
	return fmt.Sprintf("delivery:%s:%d:%d", notifID, stepIndex, attempt)
}

func delayTaskID(notifID string, stepIndex int) string {
	return fmt.Sprintf("delay:%s:%d", notifID, stepIndex)
}

// scheduleSteps enqueues the planned steps of a workflow starting at fromIdx.
// Deliveries are enqueued until the first non-skipped control step (delay or
// digest), which is scheduled and then owns the rest of the chain: its handler
// re-evaluates the workflow when it fires and calls scheduleSteps for the
// remaining steps. A skipped control step does not interrupt the chain.
//
// Enqueues are idempotent via deterministic task IDs, so callers can safely
// return the error to asynq and have the whole task retried.
func scheduleSteps(
	ctx context.Context,
	client *asynq.Client,
	rdb *redis.Client,
	planned []engine.PlannedStep,
	fromIdx int,
	ref stepRef,
	onSkip func(ps engine.PlannedStep),
) error {
	for _, ps := range planned {
		if ps.StepIndex < fromIdx {
			continue
		}
		if ps.Skipped {
			if onSkip != nil {
				onSkip(ps)
			}
			continue
		}

		switch ps.Step.Type {
		case "delay":
			if err := scheduleDelayStep(client, ps, ref); err != nil {
				return fmt.Errorf("schedule delay step %d: %w", ps.StepIndex, err)
			}
			return nil // the delay handler owns the subsequent steps
		case "digest":
			if err := scheduleDigestStep(ctx, client, rdb, ps, ref); err != nil {
				return fmt.Errorf("schedule digest step %d: %w", ps.StepIndex, err)
			}
			return nil // the digest handler owns the subsequent steps
		default:
			if err := scheduleDeliveryStep(client, ps, ref); err != nil {
				return fmt.Errorf("schedule delivery step %d: %w", ps.StepIndex, err)
			}
		}
	}
	return nil
}

func scheduleDeliveryStep(client *asynq.Client, ps engine.PlannedStep, ref stepRef) error {
	dp := DeliveryPayload{
		EnvironmentID:  ref.EnvironmentID,
		NotificationID: ref.NotificationID,
		SubscriberID:   ref.SubscriberID,
		Channel:        ps.Step.Type,
		StepIndex:      ps.StepIndex,
		Payload:        ref.Payload,
		Overrides:      ref.Overrides,
		Attempt:        0,
	}
	data, err := json.Marshal(dp)
	if err != nil {
		return fmt.Errorf("marshal delivery payload: %w", err)
	}
	task := asynq.NewTask(TaskTypeDelivery, data)
	_, err = client.Enqueue(task, asynq.TaskID(deliveryTaskID(ref.NotificationID, ps.StepIndex, 0)))
	if errors.Is(err, asynq.ErrTaskIDConflict) {
		return nil // already enqueued by a previous run of this task
	}
	return err
}

func scheduleDelayStep(client *asynq.Client, ps engine.PlannedStep, ref stepRef) error {
	if ps.Step.DelayConfig == nil {
		return nil
	}
	duration := ParseDuration(ps.Step.DelayConfig.Amount, ps.Step.DelayConfig.Unit)

	dp := DelayPayload{
		EnvironmentID:  ref.EnvironmentID,
		NotificationID: ref.NotificationID,
		SubscriberID:   ref.SubscriberID,
		WorkflowID:     ref.WorkflowID,
		StepIndex:      ps.StepIndex,
		Payload:        ref.Payload,
		Overrides:      ref.Overrides,
	}
	data, err := json.Marshal(dp)
	if err != nil {
		return fmt.Errorf("marshal delay payload: %w", err)
	}
	task := asynq.NewTask(TaskTypeDelay, data)
	_, err = client.Enqueue(task, asynq.ProcessIn(duration), asynq.TaskID(delayTaskID(ref.NotificationID, ps.StepIndex)))
	if errors.Is(err, asynq.ErrTaskIDConflict) {
		return nil // already scheduled by a previous run of this task
	}
	return err
}

func scheduleDigestStep(ctx context.Context, client *asynq.Client, rdb *redis.Client, ps engine.PlannedStep, ref stepRef) error {
	if ps.Step.DigestConfig == nil {
		return nil
	}

	// Include step index to prevent key collision when multiple digest steps share the same DigestKey string.
	digestKey := fmt.Sprintf("digest:%s:%s:%d:%s:%s",
		ref.EnvironmentID,
		ref.WorkflowID,
		ps.StepIndex,
		ref.SubscriberID,
		ps.Step.DigestConfig.DigestKey,
	)

	// Accumulate this notification's ID in the Redis list.
	// RPUSH returns the list length after the push.
	count, err := rdb.RPush(ctx, digestKey, ref.NotificationID).Result()
	if err != nil {
		return fmt.Errorf("rpush digest key: %w", err)
	}

	duration := ParseDuration(ps.Step.DigestConfig.Amount, ps.Step.DigestConfig.Unit)

	if count == 1 {
		// First notification in this window: set TTL and schedule the digest task.
		// Add a small buffer so the key outlives the task.
		rdb.Expire(ctx, digestKey, duration+5*time.Minute)

		dp := DigestPayload{
			EnvironmentID: ref.EnvironmentID,
			WorkflowID:    ref.WorkflowID,
			SubscriberID:  ref.SubscriberID,
			Channel:       ps.Step.Type,
			DigestKey:     ps.Step.DigestConfig.DigestKey,
			StepIndex:     ps.StepIndex,
			Overrides:     ref.Overrides,
		}
		data, err := json.Marshal(dp)
		if err != nil {
			return fmt.Errorf("marshal digest payload: %w", err)
		}
		task := asynq.NewTask(TaskTypeDigest, data)
		_, err = client.Enqueue(task, asynq.ProcessIn(duration))
		return err
	}

	// count > 1: a digest task is already scheduled for this window; nothing more to do.
	return nil
}
