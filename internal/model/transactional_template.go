package model

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type TransactionalTemplate struct {
	ID            bson.ObjectID     `bson:"_id,omitempty" json:"id"`
	EnvironmentID bson.ObjectID     `bson:"environmentId" json:"environmentId"`
	Identifier    string            `bson:"identifier"    json:"identifier"`
	Name          string            `bson:"name"          json:"name"`
	Channel       string            `bson:"channel"       json:"channel"`
	Subject       map[string]string `bson:"subject"       json:"subject"`
	Body          map[string]string `bson:"body"          json:"body"`
	DefaultLocale string            `bson:"defaultLocale" json:"defaultLocale"`
	Variables     []string          `bson:"variables"     json:"variables"`
	IsActive      bool              `bson:"isActive"      json:"isActive"`
	CreatedAt     time.Time         `bson:"createdAt"     json:"createdAt"`
	UpdatedAt     time.Time         `bson:"updatedAt"     json:"updatedAt"`
}
