package model

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Workflow struct {
	ID                 bson.ObjectID  `bson:"_id,omitempty"`
	EnvironmentID      bson.ObjectID  `bson:"environmentId"`
	Identifier         string         `bson:"identifier"`
	Name               string         `bson:"name"`
	Description        string         `bson:"description,omitempty"`
	Tags               []string       `bson:"tags,omitempty"`
	Steps              []WorkflowStep `bson:"steps"`
	PreferenceDefaults ChannelPrefs   `bson:"preferenceDefaults"`
	IsActive           bool           `bson:"isActive"`
	CreatedAt          time.Time      `bson:"createdAt"`
	UpdatedAt          time.Time      `bson:"updatedAt"`
}

type WorkflowStep struct {
	ID             bson.ObjectID   `bson:"_id,omitempty"`
	Type           string          `bson:"type"`
	Order          int             `bson:"order"`
	Template       *StepTemplate   `bson:"template,omitempty"`
	DigestConfig   *DigestConfig   `bson:"digestConfig,omitempty"`
	DelayConfig    *DelayConfig    `bson:"delayConfig,omitempty"`
	Conditions     []StepCondition `bson:"conditions,omitempty"`
	DefaultEnabled bool            `bson:"defaultEnabled"`
}

type StepTemplate struct {
	Subject map[string]string `bson:"subject,omitempty"`
	Body    map[string]string `bson:"body,omitempty"`
	Content map[string]string `bson:"content,omitempty"`
}

type DigestConfig struct {
	Amount    int    `bson:"amount"`
	Unit      string `bson:"unit"`
	DigestKey string `bson:"digestKey,omitempty"`
}

type DelayConfig struct {
	Amount int    `bson:"amount"`
	Unit   string `bson:"unit"`
}

type StepCondition struct {
	Field    string `bson:"field"`
	Operator string `bson:"operator"`
	Value    any    `bson:"value"`
}

type ChannelPrefs struct {
	Email   bool `bson:"email"   json:"email"`
	SMS     bool `bson:"sms"     json:"sms"`
	Push    bool `bson:"push"    json:"push"`
	InApp   bool `bson:"inApp"   json:"inApp"`
	Slack   bool `bson:"slack"   json:"slack"`
	MSTeams bool `bson:"msTeams" json:"msTeams"`
}
