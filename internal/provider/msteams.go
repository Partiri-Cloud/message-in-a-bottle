package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/partiri-cloud/message-in-a-bottle/internal/model"
)

type MSTeamsProvider struct{}

func NewMSTeamsProvider(creds json.RawMessage, meta model.IntegrationMeta) (Provider, error) {
	return &MSTeamsProvider{}, nil
}

func (p *MSTeamsProvider) ID() string      { return "ms_teams_webhook" }
func (p *MSTeamsProvider) Channel() string { return "ms_teams" }

func (p *MSTeamsProvider) Send(ctx context.Context, opts SendOptions) (SendResult, error) {
	if err := ValidateWebhookURL(opts.To); err != nil {
		return SendResult{}, err
	}
	return p.post(ctx, opts.To, opts.Subject, opts.Content, WebhookClient)
}

// post performs the actual HTTP POST to the MS Teams webhook. It is separate from
// Send to allow unit tests to inject a test server client without bypassing
// URL validation.
func (p *MSTeamsProvider) post(ctx context.Context, webhookURL, subject, content string, client *http.Client) (SendResult, error) {
	card := map[string]any{
		"@type":    "MessageCard",
		"@context": "http://schema.org/extensions",
		"summary":  subject,
		"sections": []map[string]any{{
			"activityTitle": subject,
			"text":          content,
		}},
	}
	body, _ := json.Marshal(card)

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
		return SendResult{}, fmt.Errorf("ms teams webhook error: status %d", resp.StatusCode)
	}

	return SendResult{}, nil
}
