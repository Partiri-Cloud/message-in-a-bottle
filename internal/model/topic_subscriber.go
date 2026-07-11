package model

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type TopicSubscriber struct {
	ID                   bson.ObjectID `bson:"_id,omitempty" json:"id"`
	EnvironmentID        bson.ObjectID `bson:"environmentId" json:"environmentId"`
	TopicID              bson.ObjectID `bson:"topicId"       json:"topicId"`
	SubscriberID         bson.ObjectID `bson:"subscriberId"  json:"subscriberId"`
	ExternalSubscriberID string        `bson:"externalSubscriberId" json:"externalSubscriberId"`
	CreatedAt            time.Time     `bson:"createdAt" json:"createdAt"`
}
