package dto

type CreateIntegrationRequest struct {
	Channel     string             `json:"channel" binding:"required"`
	ProviderID  string             `json:"providerId" binding:"required"`
	Name        string             `json:"name" binding:"required"`
	Credentials map[string]any     `json:"credentials" binding:"required"`
	IsPrimary   bool               `json:"isPrimary"`
	Metadata    IntegrationMetaDTO `json:"metadata"`
}

type UpdateIntegrationRequest struct {
	Name        string             `json:"name"`
	Credentials map[string]any     `json:"credentials"`
	IsActive    *bool              `json:"isActive"`
	Metadata    IntegrationMetaDTO `json:"metadata"`
}

type IntegrationMetaDTO struct {
	SenderName  string `json:"senderName"`
	SenderEmail string `json:"senderEmail"`
}
