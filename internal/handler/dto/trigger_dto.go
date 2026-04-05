package dto

type TriggerRequest struct {
	WorkflowIdentifier string         `json:"workflowIdentifier" binding:"required"`
	To                 []TriggerTo    `json:"to" binding:"required,min=1"`
	Payload            map[string]any `json:"payload"`
	TransactionID      string         `json:"transactionId"`
	Overrides          map[string]any `json:"overrides"`
}

type TriggerTo struct {
	Type         string `json:"type"`         // "Topic" or empty for subscriber
	TopicKey     string `json:"topicKey"`     // when type is "Topic"
	SubscriberID string `json:"subscriberId"` // when direct
}

type BulkTriggerRequest struct {
	Events []TriggerRequest `json:"events" binding:"required,max=100"`
}

type BroadcastRequest struct {
	WorkflowIdentifier string         `json:"workflowIdentifier" binding:"required"`
	Payload            map[string]any `json:"payload"`
	TransactionID      string         `json:"transactionId"`
	Overrides          map[string]any `json:"overrides"`
}
