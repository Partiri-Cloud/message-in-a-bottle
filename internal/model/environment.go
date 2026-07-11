package model

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Environment struct {
	ID         bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Name       string        `bson:"name"       json:"name"`
	Identifier string        `bson:"identifier" json:"identifier"`
	// Never serialized: handlers that expose keys build their own redacted view
	// (see AdminHandler.ListEnvironments). Tagging it out keeps a future handler
	// from leaking key hashes by returning the model directly.
	APIKeys   []APIKey  `bson:"apiKeys"   json:"-"`
	CreatedAt time.Time `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `bson:"updatedAt" json:"updatedAt"`
}

type APIKey struct {
	KeyHash     string     `bson:"keyHash"     json:"-"`
	Name        string     `bson:"name"        json:"name"`
	Permissions []string   `bson:"permissions" json:"permissions"`
	CreatedAt   time.Time  `bson:"createdAt"   json:"createdAt"`
	ExpiresAt   *time.Time `bson:"expiresAt,omitempty"  json:"expiresAt,omitempty"`
	LastUsedAt  *time.Time `bson:"lastUsedAt,omitempty" json:"lastUsedAt,omitempty"`
	IsActive    bool       `bson:"isActive" json:"isActive"`
}
