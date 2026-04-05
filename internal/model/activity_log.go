package model

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type ActivityLog struct {
	ID             bson.ObjectID  `bson:"_id,omitempty"`
	EnvironmentID  bson.ObjectID  `bson:"environmentId"`
	NotificationID bson.ObjectID  `bson:"notificationId"`
	SubscriberID   bson.ObjectID  `bson:"subscriberId"`
	Channel        string         `bson:"channel,omitempty"`
	Event          string         `bson:"event"`
	Detail         map[string]any `bson:"detail,omitempty"`
	CreatedAt      time.Time      `bson:"createdAt"`
	ExpireAt       time.Time      `bson:"expireAt"`
}
