package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/partiri/message-in-a-bottle/internal/model"
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

type SendGridProvider struct {
	creds model.SendGridCreds
	meta  model.IntegrationMeta
}

func NewSendGridProvider(creds json.RawMessage, meta model.IntegrationMeta) (Provider, error) {
	var c model.SendGridCreds
	if err := json.Unmarshal(creds, &c); err != nil {
		return nil, fmt.Errorf("invalid sendgrid credentials: %w", err)
	}
	return &SendGridProvider{creds: c, meta: meta}, nil
}

func (p *SendGridProvider) ID() string      { return "sendgrid" }
func (p *SendGridProvider) Channel() string { return "email" }

func (p *SendGridProvider) Send(ctx context.Context, opts SendOptions) (SendResult, error) {
	from := mail.NewEmail(p.meta.SenderName, p.meta.SenderEmail)
	to := mail.NewEmail("", opts.To)
	message := mail.NewSingleEmail(from, opts.Subject, to, "", opts.Content)
	client := sendgrid.NewSendClient(p.creds.APIKey)
	resp, err := client.SendWithContext(ctx, message)
	if err != nil {
		return SendResult{}, err
	}
	if resp.StatusCode >= 400 {
		return SendResult{}, fmt.Errorf("sendgrid error: status %d, body: %s", resp.StatusCode, resp.Body)
	}
	return SendResult{ProviderMessageID: resp.Headers["X-Message-Id"][0]}, nil
}
