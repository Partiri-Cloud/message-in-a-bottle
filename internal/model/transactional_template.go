package model

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type TransactionalTemplate struct {
	ID            bson.ObjectID     `bson:"_id,omitempty"`
	EnvironmentID bson.ObjectID     `bson:"environmentId"`
	Identifier    string            `bson:"identifier"`
	Name          string            `bson:"name"`
	Channel       string            `bson:"channel"`
	Subject       map[string]string `bson:"subject"`
	Body          map[string]string `bson:"body"`
	DefaultLocale string            `bson:"defaultLocale"`
	Variables     []string          `bson:"variables"`
	IsActive      bool              `bson:"isActive"`
	CreatedAt     time.Time         `bson:"createdAt"`
	UpdatedAt     time.Time         `bson:"updatedAt"`
}
