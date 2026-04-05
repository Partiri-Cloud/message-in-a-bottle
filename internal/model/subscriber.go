package model

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Subscriber struct {
	ID            bson.ObjectID      `bson:"_id,omitempty"`
	EnvironmentID bson.ObjectID      `bson:"environmentId"`
	SubscriberID  string             `bson:"subscriberId"`
	Email         string             `bson:"email,omitempty"`
	Phone         string             `bson:"phone,omitempty"`
	FirstName     string             `bson:"firstName,omitempty"`
	LastName      string             `bson:"lastName,omitempty"`
	Avatar        string             `bson:"avatar,omitempty"`
	Locale        string             `bson:"locale"`
	Timezone      string             `bson:"timezone,omitempty"`
	Data          map[string]any     `bson:"data,omitempty"`
	Channels      SubscriberChannels `bson:"channels"`
	IsOnline      bool               `bson:"isOnline"`
	LastOnlineAt  *time.Time         `bson:"lastOnlineAt,omitempty"`
	CreatedAt     time.Time          `bson:"createdAt"`
	UpdatedAt     time.Time          `bson:"updatedAt"`
}

type SubscriberChannels struct {
	Push    PushChannelConfig `bson:"push,omitempty"`
	Slack   SlackConfig       `bson:"slack,omitempty"`
	MSTeams MSTeamsConfig     `bson:"msTeams,omitempty"`
}

type PushChannelConfig struct {
	FCMTokens  []string `bson:"fcmTokens,omitempty"`
	APNSTokens []string `bson:"apnsTokens,omitempty"`
}

type SlackConfig struct {
	WebhookURL string `bson:"webhookUrl,omitempty"`
}

type MSTeamsConfig struct {
	WebhookURL string `bson:"webhookUrl,omitempty"`
}
