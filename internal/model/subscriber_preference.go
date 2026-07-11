package model

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type SubscriberPreference struct {
	ID            bson.ObjectID  `bson:"_id,omitempty" json:"id"`
	EnvironmentID bson.ObjectID  `bson:"environmentId" json:"environmentId"`
	SubscriberID  bson.ObjectID  `bson:"subscriberId"  json:"subscriberId"`
	WorkflowID    *bson.ObjectID `bson:"workflowId,omitempty" json:"workflowId"`
	Channels      ChannelPrefs   `bson:"channels"      json:"channels"`
	UpdatedAt     time.Time      `bson:"updatedAt"     json:"updatedAt"`
}
