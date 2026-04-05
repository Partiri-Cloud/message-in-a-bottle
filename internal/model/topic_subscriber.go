package model

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type TopicSubscriber struct {
	ID                   bson.ObjectID `bson:"_id,omitempty"`
	EnvironmentID        bson.ObjectID `bson:"environmentId"`
	TopicID              bson.ObjectID `bson:"topicId"`
	SubscriberID         bson.ObjectID `bson:"subscriberId"`
	ExternalSubscriberID string        `bson:"externalSubscriberId"`
	CreatedAt            time.Time     `bson:"createdAt"`
}
