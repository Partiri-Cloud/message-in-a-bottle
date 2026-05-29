package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/partiri-cloud/message-in-a-bottle/internal/model"
)

type SlackProvider struct{}

func NewSlackProvider(creds json.RawMessage, meta model.IntegrationMeta) (Provider, error) {
	return &SlackProvider{}, nil
}

func (p *SlackProvider) ID() string      { return "slack_webhook" }
func (p *SlackProvider) Channel() string { return "slack" }

func (p *SlackProvider) Send(ctx context.Context, opts SendOptions) (SendResult, error) {
	if err := ValidateWebhookURL(opts.To); err != nil {
		return SendResult{}, err
	}
	return p.post(ctx, opts.To, opts.Content, WebhookClient)
}

// post performs the actual HTTP POST to the Slack webhook. It is separate from
// Send to allow unit tests to inject a test server client without bypassing
// URL validation.
func (p *SlackProvider) post(ctx context.Context, webhookURL, content string, client *http.Client) (SendResult, error) {
	body, _ := json.Marshal(map[string]any{"text": content})
	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewReader(body))
	if err != nil {
		return SendResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return SendResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return SendResult{}, fmt.Errorf("slack webhook error: status %d", resp.StatusCode)
	}

	return SendResult{}, nil
}
