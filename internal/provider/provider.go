package provider

import "context"

type Provider interface {
	ID() string
	Channel() string
	Send(ctx context.Context, opts SendOptions) (SendResult, error)
}

type SendOptions struct {
	To       string         // email, phone, device token, webhook URL
	Subject  string         // email, push
	Content  string         // rendered body
	Metadata map[string]any // channel-specific extra data
}

type SendResult struct {
	ProviderMessageID string
}
