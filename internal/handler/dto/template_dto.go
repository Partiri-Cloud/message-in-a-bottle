package dto

type CreateTemplateRequest struct {
	Identifier    string            `json:"identifier" binding:"required"`
	Name          string            `json:"name" binding:"required"`
	Channel       string            `json:"channel" binding:"required"`
	Subject       map[string]string `json:"subject" binding:"required"`
	Body          map[string]string `json:"body" binding:"required"`
	DefaultLocale string            `json:"defaultLocale"`
	Variables     []string          `json:"variables"`
}

type UpdateTemplateRequest struct {
	Name          string            `json:"name"`
	Subject       map[string]string `json:"subject"`
	Body          map[string]string `json:"body"`
	DefaultLocale string            `json:"defaultLocale"`
	Variables     []string          `json:"variables"`
	IsActive      *bool             `json:"isActive"`
}

type SendTemplateRequest struct {
	To      TriggerTo      `json:"to" binding:"required"`
	Payload map[string]any `json:"payload"`
	Locale  string         `json:"locale"`
}
