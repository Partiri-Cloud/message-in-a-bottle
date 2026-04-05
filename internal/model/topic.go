package model

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Topic struct {
	ID            bson.ObjectID `bson:"_id,omitempty"`
	EnvironmentID bson.ObjectID `bson:"environmentId"`
	Key           string        `bson:"key"`
	Name          string        `bson:"name"`
	CreatedAt     time.Time     `bson:"createdAt"`
	UpdatedAt     time.Time     `bson:"updatedAt"`
}
