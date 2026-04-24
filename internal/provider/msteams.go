package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	card := map[string]any{
		"@type":    "MessageCard",
		"@context": "http://schema.org/extensions",
		"summary":  opts.Subject,
		"sections": []map[string]any{{
			"activityTitle": opts.Subject,
			"text":          opts.Content,
		}},
	}
	body, _ := json.Marshal(card)

	req, err := http.NewRequestWithContext(ctx, "POST", opts.To, bytes.NewReader(body))
	if err != nil {
		return SendResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return SendResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return SendResult{}, fmt.Errorf("ms teams webhook error: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	return SendResult{}, nil
}
