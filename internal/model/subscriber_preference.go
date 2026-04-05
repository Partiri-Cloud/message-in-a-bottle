package model

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type SubscriberPreference struct {
	ID            bson.ObjectID  `bson:"_id,omitempty"`
	EnvironmentID bson.ObjectID  `bson:"environmentId"`
	SubscriberID  bson.ObjectID  `bson:"subscriberId"`
	WorkflowID    *bson.ObjectID `bson:"workflowId,omitempty"`
	Channels      ChannelPrefs   `bson:"channels"`
	UpdatedAt     time.Time      `bson:"updatedAt"`
}
