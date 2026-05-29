package dto

type CreateSubscriberRequest struct {
	SubscriberID string         `json:"subscriberId" binding:"required"`
	Email        string         `json:"email"`
	Phone        string         `json:"phone"`
	FirstName    string         `json:"firstName"`
	LastName     string         `json:"lastName"`
	Avatar       string         `json:"avatar"`
	Locale       string         `json:"locale"`
	Timezone     string         `json:"timezone"`
	Data         map[string]any `json:"data"`
	Channels     *ChannelsDTO   `json:"channels"`
}

type ChannelsDTO struct {
	Push    *PushDTO    `json:"push"`
	Slack   *SlackDTO   `json:"slack"`
	MSTeams *MSTeamsDTO `json:"msTeams"`
}

type PushDTO struct {
	FCMTokens  []string `json:"fcmTokens"`
	APNSTokens []string `json:"apnsTokens"`
}

// SlackDTO carries the Slack incoming-webhook URL for a subscriber channel.
// Non-empty values are validated against the SSRF allowlist in subscriber_handler.go
// before being stored. Omitting the field (empty string) disables the channel.
type SlackDTO struct {
	WebhookURL string `json:"webhookUrl"`
}

// MSTeamsDTO carries the MS Teams incoming-webhook URL for a subscriber channel.
// Non-empty values are validated against the SSRF allowlist in subscriber_handler.go
// before being stored. Omitting the field (empty string) disables the channel.
type MSTeamsDTO struct {
	WebhookURL string `json:"webhookUrl"`
}

type UpdateSubscriberRequest struct {
	Email     *string        `json:"email"`
	Phone     *string        `json:"phone"`
	FirstName *string        `json:"firstName"`
	LastName  *string        `json:"lastName"`
	Avatar    *string        `json:"avatar"`
	Locale    *string        `json:"locale"`
	Timezone  *string        `json:"timezone"`
	Data      map[string]any `json:"data"`
	Channels  *ChannelsDTO   `json:"channels"`
}

type BulkSubscribersRequest struct {
	Subscribers []CreateSubscriberRequest `json:"subscribers" binding:"required,max=500"`
}
