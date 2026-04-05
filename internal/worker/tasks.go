package worker

const (
	TaskTypeTrigger  = "task:trigger"
	TaskTypeDelivery = "task:delivery"
	TaskTypeDigest   = "task:digest"
	TaskTypeDelay    = "task:delay"

	MaxRetries        = 3
	BackoffBaseMs     = 30_000
	BackoffMultiplier = 4
)

type TriggerPayload struct {
	EnvironmentID string         `json:"environmentId"`
	WorkflowID    string         `json:"workflowId"`
	SubscriberIDs []string       `json:"subscriberIds"`
	Payload       map[string]any `json:"payload"`
	TransactionID string         `json:"transactionId"`
	Overrides     map[string]any `json:"overrides,omitempty"`
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
