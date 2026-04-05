package model

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Notification struct {
	ID                      bson.ObjectID     `bson:"_id,omitempty"`
	EnvironmentID           bson.ObjectID     `bson:"environmentId"`
	SubscriberID            bson.ObjectID     `bson:"subscriberId"`
	WorkflowID              bson.ObjectID     `bson:"workflowId"`
	TransactionID           string            `bson:"transactionId"`
	Payload                 map[string]any    `bson:"payload"`
	Channels                []ChannelDelivery `bson:"channels"`
	Seen                    bool              `bson:"seen"`
	Read                    bool              `bson:"read"`
	SeenAt                  *time.Time        `bson:"seenAt,omitempty"`
	ReadAt                  *time.Time        `bson:"readAt,omitempty"`
	ArchivedAt              *time.Time        `bson:"archivedAt,omitempty"`
	DigestedNotificationIDs []bson.ObjectID   `bson:"digestedNotificationIds,omitempty"`
	CreatedAt               time.Time         `bson:"createdAt"`
	UpdatedAt               time.Time         `bson:"updatedAt"`
	ExpireAt                time.Time         `bson:"expireAt"`
}

type ChannelDelivery struct {
	Channel           string     `bson:"channel"`
	Status            string     `bson:"status"` // pending|queued|sent|delivered|failed|skipped
	ProviderID        string     `bson:"providerId"`
	ProviderMessageID string     `bson:"providerMessageId,omitempty"`
	ErrorMessage      string     `bson:"errorMessage,omitempty"`
	RetryCount        int        `bson:"retryCount"`
	SentAt            *time.Time `bson:"sentAt,omitempty"`
	DeliveredAt       *time.Time `bson:"deliveredAt,omitempty"`
	FailedAt          *time.Time `bson:"failedAt,omitempty"`
}
