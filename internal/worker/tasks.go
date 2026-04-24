package worker

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
