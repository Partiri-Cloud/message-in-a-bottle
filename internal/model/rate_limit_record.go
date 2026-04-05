package model

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type RateLimitRecord struct {
	ID            bson.ObjectID `bson:"_id,omitempty"`
	EnvironmentID bson.ObjectID `bson:"environmentId"`
	SubscriberID  bson.ObjectID `bson:"subscriberId"`
	Channel       string        `bson:"channel"`
	WindowStart   time.Time     `bson:"windowStart"`
	Count         int           `bson:"count"`
	CreatedAt     time.Time     `bson:"createdAt"`
	ExpireAt      time.Time     `bson:"expireAt"`
}
