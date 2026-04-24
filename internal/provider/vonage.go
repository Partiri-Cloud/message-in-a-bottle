package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/partiri-cloud/message-in-a-bottle/internal/model"
)

type VonageProvider struct {
	creds model.VonageCreds
}

func NewVonageProvider(creds json.RawMessage, meta model.IntegrationMeta) (Provider, error) {
	var c model.VonageCreds
	if err := json.Unmarshal(creds, &c); err != nil {
		return nil, fmt.Errorf("invalid vonage credentials: %w", err)
	}
	return &VonageProvider{creds: c}, nil
}

func (p *VonageProvider) ID() string      { return "vonage" }
func (p *VonageProvider) Channel() string { return "sms" }

func (p *VonageProvider) Send(ctx context.Context, opts SendOptions) (SendResult, error) {
	data := url.Values{}
	data.Set("api_key", p.creds.APIKey)
	data.Set("api_secret", p.creds.APISecret)
	data.Set("from", p.creds.FromNumber)
	data.Set("to", opts.To)
	data.Set("text", opts.Content)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://rest.nexmo.com/sms/json", strings.NewReader(data.Encode()))
	if err != nil {
		return SendResult{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return SendResult{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return SendResult{}, fmt.Errorf("vonage error: status %d, body: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Messages []struct {
			MessageID string `json:"message-id"`
			Status    string `json:"status"`
		} `json:"messages"`
	}
	json.Unmarshal(body, &result)

	if len(result.Messages) > 0 && result.Messages[0].Status != "0" {
		return SendResult{}, fmt.Errorf("vonage sms failed: status %s", result.Messages[0].Status)
	}

	var msgID string
	if len(result.Messages) > 0 {
		msgID = result.Messages[0].MessageID
	}
	return SendResult{ProviderMessageID: msgID}, nil
}
