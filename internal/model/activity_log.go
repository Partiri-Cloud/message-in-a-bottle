package model

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type ActivityLog struct {
	ID             bson.ObjectID  `bson:"_id,omitempty"   json:"id"`
	EnvironmentID  bson.ObjectID  `bson:"environmentId"   json:"environmentId"`
	NotificationID bson.ObjectID  `bson:"notificationId"  json:"notificationId"`
	SubscriberID   bson.ObjectID  `bson:"subscriberId"    json:"subscriberId"`
	Channel        string         `bson:"channel,omitempty" json:"channel,omitempty"`
	Event          string         `bson:"event"             json:"event"`
	Detail         map[string]any `bson:"detail,omitempty"  json:"detail,omitempty"`
	CreatedAt      time.Time      `bson:"createdAt" json:"createdAt"`
	ExpireAt       time.Time      `bson:"expireAt"  json:"expireAt"`
}
