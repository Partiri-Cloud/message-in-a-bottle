package model

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Environment struct {
	ID         bson.ObjectID `bson:"_id,omitempty"`
	Name       string        `bson:"name"`
	Identifier string        `bson:"identifier"`
	APIKeys    []APIKey      `bson:"apiKeys"`
	CreatedAt  time.Time     `bson:"createdAt"`
	UpdatedAt  time.Time     `bson:"updatedAt"`
}

type APIKey struct {
	KeyHash     string     `bson:"keyHash"`
	Name        string     `bson:"name"`
	Permissions []string   `bson:"permissions"`
	CreatedAt   time.Time  `bson:"createdAt"`
	ExpiresAt   *time.Time `bson:"expiresAt,omitempty"`
	LastUsedAt  *time.Time `bson:"lastUsedAt,omitempty"`
	IsActive    bool       `bson:"isActive"`
}
