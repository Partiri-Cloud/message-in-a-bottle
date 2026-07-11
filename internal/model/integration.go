package model

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Integration struct {
	ID            bson.ObjectID `bson:"_id,omitempty" json:"id"`
	EnvironmentID bson.ObjectID `bson:"environmentId" json:"environmentId"`
	Channel       string        `bson:"channel"    json:"channel"`
	ProviderID    string        `bson:"providerId" json:"providerId"`
	Name          string        `bson:"name"       json:"name"`
	// AES-256-GCM encrypted JSON. Never serialized — handlers nil this out
	// before responding, and the tag makes that belt-and-braces.
	Credentials []byte          `bson:"credentials" json:"-"`
	IsPrimary   bool            `bson:"isPrimary" json:"isPrimary"`
	IsActive    bool            `bson:"isActive"  json:"isActive"`
	Metadata    IntegrationMeta `bson:"metadata"  json:"metadata"`
	CreatedAt   time.Time       `bson:"createdAt" json:"createdAt"`
	UpdatedAt   time.Time       `bson:"updatedAt" json:"updatedAt"`
}

type IntegrationMeta struct {
	SenderName  string `bson:"senderName,omitempty"  json:"senderName,omitempty"`
	SenderEmail string `bson:"senderEmail,omitempty" json:"senderEmail,omitempty"`
}

// Provider-specific credential shapes (stored as JSON inside the encrypted blob)

type SendGridCreds struct {
	APIKey string `json:"apiKey"`
}

type SESCreds struct {
	AccessKeyID     string `json:"accessKeyId"`
	SecretAccessKey string `json:"secretAccessKey"`
	Region          string `json:"region"`
}

type SMTPCreds struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Secure   bool   `json:"secure"`
}

type TwilioCreds struct {
	AccountSID string `json:"accountSid"`
	AuthToken  string `json:"authToken"`
	FromNumber string `json:"fromNumber"`
}

type VonageCreds struct {
	APIKey     string `json:"apiKey"`
	APISecret  string `json:"apiSecret"`
	FromNumber string `json:"fromNumber"`
}

type FCMCreds struct {
	ServiceAccountJSON string `json:"serviceAccountJson"`
}

type APNSCreds struct {
	KeyID      string `json:"keyId"`
	TeamID     string `json:"teamId"`
	PrivateKey string `json:"privateKey"`
	BundleID   string `json:"bundleId"`
}
