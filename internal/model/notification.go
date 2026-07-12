package model

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Notification struct {
	ID            bson.ObjectID  `bson:"_id,omitempty" json:"id"`
	EnvironmentID bson.ObjectID  `bson:"environmentId" json:"environmentId"`
	SubscriberID  bson.ObjectID  `bson:"subscriberId"  json:"subscriberId"`
	WorkflowID    bson.ObjectID  `bson:"workflowId"    json:"workflowId"`
	TransactionID string         `bson:"transactionId" json:"transactionId"`
	Payload       map[string]any `bson:"payload"       json:"payload"`
	// Subject and Content hold the in-app step's rendered template. They are
	// written when the in_app channel is delivered. Without them the feed can
	// only return an id and a timestamp — a live WebSocket push carries the
	// rendered text, but any client loading history has nothing to display.
	Subject                 string            `bson:"subject,omitempty" json:"subject,omitempty"`
	Content                 string            `bson:"content,omitempty" json:"content,omitempty"`
	Channels                []ChannelDelivery `bson:"channels"      json:"channels"`
	Seen                    bool              `bson:"seen"          json:"seen"`
	Read                    bool              `bson:"read"          json:"read"`
	SeenAt                  *time.Time        `bson:"seenAt,omitempty"     json:"seenAt,omitempty"`
	ReadAt                  *time.Time        `bson:"readAt,omitempty"     json:"readAt,omitempty"`
	ArchivedAt              *time.Time        `bson:"archivedAt,omitempty" json:"archivedAt,omitempty"`
	DigestedNotificationIDs []bson.ObjectID   `bson:"digestedNotificationIds,omitempty" json:"digestedNotificationIds,omitempty"`
	CreatedAt               time.Time         `bson:"createdAt" json:"createdAt"`
	UpdatedAt               time.Time         `bson:"updatedAt" json:"updatedAt"`
	ExpireAt                time.Time         `bson:"expireAt"  json:"expireAt"`
}

type ChannelDelivery struct {
	Channel           string     `bson:"channel" json:"channel"`
	Status            string     `bson:"status"  json:"status"` // pending|queued|sent|delivered|failed|skipped
	ProviderID        string     `bson:"providerId" json:"providerId"`
	ProviderMessageID string     `bson:"providerMessageId,omitempty" json:"providerMessageId,omitempty"`
	ErrorMessage      string     `bson:"errorMessage,omitempty"      json:"errorMessage,omitempty"`
	RetryCount        int        `bson:"retryCount" json:"retryCount"`
	SentAt            *time.Time `bson:"sentAt,omitempty"      json:"sentAt,omitempty"`
	DeliveredAt       *time.Time `bson:"deliveredAt,omitempty" json:"deliveredAt,omitempty"`
	FailedAt          *time.Time `bson:"failedAt,omitempty"    json:"failedAt,omitempty"`
}
