package model

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Subscriber struct {
	ID            bson.ObjectID      `bson:"_id,omitempty" json:"id"`
	EnvironmentID bson.ObjectID      `bson:"environmentId" json:"environmentId"`
	SubscriberID  string             `bson:"subscriberId"  json:"subscriberId"`
	Email         string             `bson:"email,omitempty"     json:"email,omitempty"`
	Phone         string             `bson:"phone,omitempty"     json:"phone,omitempty"`
	FirstName     string             `bson:"firstName,omitempty" json:"firstName,omitempty"`
	LastName      string             `bson:"lastName,omitempty"  json:"lastName,omitempty"`
	Avatar        string             `bson:"avatar,omitempty"    json:"avatar,omitempty"`
	Locale        string             `bson:"locale"              json:"locale"`
	Timezone      string             `bson:"timezone,omitempty"  json:"timezone,omitempty"`
	Data          map[string]any     `bson:"data,omitempty"      json:"data,omitempty"`
	Channels      SubscriberChannels `bson:"channels"     json:"channels"`
	IsOnline      bool               `bson:"isOnline"     json:"isOnline"`
	LastOnlineAt  *time.Time         `bson:"lastOnlineAt,omitempty" json:"lastOnlineAt,omitempty"`
	CreatedAt     time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt     time.Time          `bson:"updatedAt" json:"updatedAt"`
}

type SubscriberChannels struct {
	Push    PushChannelConfig `bson:"push,omitempty"    json:"push"`
	Slack   SlackConfig       `bson:"slack,omitempty"   json:"slack"`
	MSTeams MSTeamsConfig     `bson:"msTeams,omitempty" json:"msTeams"`
}

type PushChannelConfig struct {
	FCMTokens  []string `bson:"fcmTokens,omitempty"  json:"fcmTokens,omitempty"`
	APNSTokens []string `bson:"apnsTokens,omitempty" json:"apnsTokens,omitempty"`
}

type SlackConfig struct {
	WebhookURL string `bson:"webhookUrl,omitempty" json:"webhookUrl,omitempty"`
}

type MSTeamsConfig struct {
	WebhookURL string `bson:"webhookUrl,omitempty" json:"webhookUrl,omitempty"`
}
