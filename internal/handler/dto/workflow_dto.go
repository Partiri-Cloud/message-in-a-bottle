package dto

type CreateWorkflowRequest struct {
	Identifier         string            `json:"identifier" binding:"required"`
	Name               string            `json:"name" binding:"required"`
	Description        string            `json:"description"`
	Tags               []string          `json:"tags"`
	Steps              []WorkflowStepDTO `json:"steps" binding:"required,min=1"`
	PreferenceDefaults ChannelPrefsDTO   `json:"preferenceDefaults"`
}

type WorkflowStepDTO struct {
	Type           string             `json:"type" binding:"required"`
	Order          int                `json:"order"`
	Template       *StepTemplateDTO   `json:"template"`
	DigestConfig   *DigestConfigDTO   `json:"digestConfig"`
	DelayConfig    *DelayConfigDTO    `json:"delayConfig"`
	Conditions     []StepConditionDTO `json:"conditions"`
	DefaultEnabled bool               `json:"defaultEnabled"`
}

type StepTemplateDTO struct {
	Subject map[string]string `json:"subject"`
	Body    map[string]string `json:"body"`
	Content map[string]string `json:"content"`
}

type DigestConfigDTO struct {
	Amount    int    `json:"amount"`
	Unit      string `json:"unit"`
	DigestKey string `json:"digestKey"`
}

type DelayConfigDTO struct {
	Amount int    `json:"amount"`
	Unit   string `json:"unit"`
}

type StepConditionDTO struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    any    `json:"value"`
}

type ChannelPrefsDTO struct {
	Email   bool `json:"email"`
	SMS     bool `json:"sms"`
	Push    bool `json:"push"`
	InApp   bool `json:"inApp"`
	Slack   bool `json:"slack"`
	MSTeams bool `json:"msTeams"`
}

type WorkflowStatusRequest struct {
	IsActive bool `json:"isActive"`
}
