package model

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Workflow struct {
	ID                 bson.ObjectID  `bson:"_id,omitempty" json:"id"`
	EnvironmentID      bson.ObjectID  `bson:"environmentId" json:"environmentId"`
	Identifier         string         `bson:"identifier"    json:"identifier"`
	Name               string         `bson:"name"          json:"name"`
	Description        string         `bson:"description,omitempty" json:"description,omitempty"`
	Tags               []string       `bson:"tags,omitempty"        json:"tags,omitempty"`
	Steps              []WorkflowStep `bson:"steps"              json:"steps"`
	PreferenceDefaults ChannelPrefs   `bson:"preferenceDefaults" json:"preferenceDefaults"`
	IsActive           bool           `bson:"isActive"  json:"isActive"`
	CreatedAt          time.Time      `bson:"createdAt" json:"createdAt"`
	UpdatedAt          time.Time      `bson:"updatedAt" json:"updatedAt"`
}

type WorkflowStep struct {
	ID             bson.ObjectID   `bson:"_id,omitempty" json:"id"`
	Type           string          `bson:"type"  json:"type"`
	Order          int             `bson:"order" json:"order"`
	Template       *StepTemplate   `bson:"template,omitempty"     json:"template,omitempty"`
	DigestConfig   *DigestConfig   `bson:"digestConfig,omitempty" json:"digestConfig,omitempty"`
	DelayConfig    *DelayConfig    `bson:"delayConfig,omitempty"  json:"delayConfig,omitempty"`
	Conditions     []StepCondition `bson:"conditions,omitempty"   json:"conditions,omitempty"`
	DefaultEnabled bool            `bson:"defaultEnabled" json:"defaultEnabled"`
}

type StepTemplate struct {
	Subject map[string]string `bson:"subject,omitempty" json:"subject,omitempty"`
	Body    map[string]string `bson:"body,omitempty"    json:"body,omitempty"`
	Content map[string]string `bson:"content,omitempty" json:"content,omitempty"`
}

type DigestConfig struct {
	Amount    int    `bson:"amount" json:"amount"`
	Unit      string `bson:"unit"   json:"unit"`
	DigestKey string `bson:"digestKey,omitempty" json:"digestKey,omitempty"`
}

type DelayConfig struct {
	Amount int    `bson:"amount" json:"amount"`
	Unit   string `bson:"unit"   json:"unit"`
}

type StepCondition struct {
	Field    string `bson:"field"    json:"field"`
	Operator string `bson:"operator" json:"operator"`
	Value    any    `bson:"value"    json:"value"`
}

type ChannelPrefs struct {
	Email   bool `bson:"email"   json:"email"`
	SMS     bool `bson:"sms"     json:"sms"`
	Push    bool `bson:"push"    json:"push"`
	InApp   bool `bson:"inApp"   json:"inApp"`
	Slack   bool `bson:"slack"   json:"slack"`
	MSTeams bool `bson:"msTeams" json:"msTeams"`
}
