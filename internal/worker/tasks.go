package worker

import (
	"encoding/json"
	"log"

	"github.com/hibiken/asynq"

	"github.com/partiri-cloud/message-in-a-bottle/internal/model"
)

const (
	TaskTypeTrigger   = "task:trigger"
	TaskTypeDelivery  = "task:delivery"
	TaskTypeDigest    = "task:digest"
	TaskTypeDelay     = "task:delay"
	TaskTypeBroadcast = "task:broadcast"

	MaxRetries        = 3
	BackoffBaseMs     = 30_000
	BackoffMultiplier = 4
)

// TriggerPayload carries a subscriber→notification mapping so the trigger handler
// looks up each subscriber's own notification record instead of using a shared transactionId query.
type TriggerPayload struct {
	EnvironmentID string            `json:"environmentId"`
	WorkflowID    string            `json:"workflowId"`
	SubscriberIDs map[string]string `json:"subscriberIds"` // subscriberID hex → notificationID hex
	Payload       map[string]any    `json:"payload"`
	TransactionID string            `json:"transactionId"`
	Overrides     map[string]any    `json:"overrides,omitempty"`
}

type BroadcastTaskPayload struct {
	EnvironmentID      string         `json:"environmentId"`
	WorkflowIdentifier string         `json:"workflowIdentifier"`
	Payload            map[string]any `json:"payload"`
	TransactionID      string         `json:"transactionId"`
	Overrides          map[string]any `json:"overrides,omitempty"`
	RetentionDays      int            `json:"retentionDays"`
}

type DeliveryPayload struct {
	EnvironmentID  string         `json:"environmentId"`
	NotificationID string         `json:"notificationId"`
	SubscriberID   string         `json:"subscriberId"`
	Channel        string         `json:"channel"`
	StepIndex      int            `json:"stepIndex"`
	Payload        map[string]any `json:"payload"`
	Overrides      map[string]any `json:"overrides,omitempty"`
	Attempt        int            `json:"attempt"`

	// Transactional, RenderedSubject, and RenderedBody are set only by
	// TemplateService.Send for direct template sends. They live outside Payload
	// (the tenant's raw trigger event data for workflow-triggered notifications)
	// so a workflow trigger payload can never spoof the transactional discriminator
	// or inject rendered content.
	Transactional   bool   `json:"transactional,omitempty"`
	RenderedSubject string `json:"renderedSubject,omitempty"`
	RenderedBody    string `json:"renderedBody,omitempty"`
}

type DigestPayload struct {
	EnvironmentID string `json:"environmentId"`
	WorkflowID    string `json:"workflowId"`
	SubscriberID  string `json:"subscriberId"`
	Channel       string `json:"channel"`
	DigestKey     string `json:"digestKey"`
	StepIndex     int    `json:"stepIndex"`
}

type DelayPayload struct {
	EnvironmentID  string         `json:"environmentId"`
	NotificationID string         `json:"notificationId"`
	SubscriberID   string         `json:"subscriberId"`
	WorkflowID     string         `json:"workflowId"`
	StepIndex      int            `json:"stepIndex"`
	Payload        map[string]any `json:"payload"`
	Overrides      map[string]any `json:"overrides,omitempty"`
}

// IsControlStep reports whether a workflow step gates the flow of the steps
// after it (delay/digest) rather than delivering on a channel.
func IsControlStep(stepType string) bool {
	return stepType == "delay" || stepType == "digest"
}

// BuildChannelDeliveries returns a pending delivery-status row for every
// channel step in the workflow, skipping control steps.
func BuildChannelDeliveries(steps []model.WorkflowStep) []model.ChannelDelivery {
	channels := make([]model.ChannelDelivery, 0, len(steps))
	for _, step := range steps {
		if IsControlStep(step.Type) {
			continue
		}
		channels = append(channels, model.ChannelDelivery{
			Channel: step.Type,
			Status:  "pending",
		})
	}
	return channels
}

// enqueueFollowingDeliveries resumes a workflow after a control step fires:
// it enqueues a delivery task for each channel step after fromIndex, using
// base for everything but Channel and StepIndex. The trigger handler stops
// planning at the first control step, so these steps were never enqueued.
//
// A second control step ends the run: neither the delay nor the digest
// handler can re-schedule one (chained control steps are not supported), so
// it is logged loudly instead of silently dropped.
func enqueueFollowingDeliveries(client *asynq.Client, wf *model.Workflow, fromIndex int, origin string, base DeliveryPayload) {
	for i := fromIndex + 1; i < len(wf.Steps); i++ {
		step := wf.Steps[i]
		if IsControlStep(step.Type) {
			log.Printf("%s: workflow %s chains another control step (%s) at index %d; chained control steps are not supported, steps beyond it will not run", origin, wf.Identifier, step.Type, i)
			break
		}

		dp := base
		dp.Channel = step.Type
		dp.StepIndex = i
		data, err := json.Marshal(dp)
		if err != nil {
			log.Printf("failed to marshal %s delivery step %d: %v", origin, i, err)
			continue
		}
		if _, err := client.Enqueue(asynq.NewTask(TaskTypeDelivery, data)); err != nil {
			log.Printf("failed to enqueue %s delivery step %d: %v", origin, i, err)
		}
	}
}
