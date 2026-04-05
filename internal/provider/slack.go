package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/partiri/message-in-a-bottle/internal/model"
)

type SlackProvider struct{}

func NewSlackProvider(creds json.RawMessage, meta model.IntegrationMeta) (Provider, error) {
	return &SlackProvider{}, nil
}

func (p *SlackProvider) ID() string      { return "slack_webhook" }
func (p *SlackProvider) Channel() string { return "slack" }

func (p *SlackProvider) Send(ctx context.Context, opts SendOptions) (SendResult, error) {
	body, _ := json.Marshal(map[string]any{"text": opts.Content})
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
		return SendResult{}, fmt.Errorf("slack webhook error: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	return SendResult{}, nil
}
