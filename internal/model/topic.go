package model

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Topic struct {
	ID            bson.ObjectID `bson:"_id,omitempty" json:"id"`
	EnvironmentID bson.ObjectID `bson:"environmentId" json:"environmentId"`
	Key           string        `bson:"key"       json:"key"`
	Name          string        `bson:"name"      json:"name"`
	CreatedAt     time.Time     `bson:"createdAt" json:"createdAt"`
	UpdatedAt     time.Time     `bson:"updatedAt" json:"updatedAt"`
}
