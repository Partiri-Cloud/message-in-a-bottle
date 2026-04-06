package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/partiri-cloud/message-in-a-box/internal/model"
)

type TwilioProvider struct {
	creds model.TwilioCreds
}

func NewTwilioProvider(creds json.RawMessage, meta model.IntegrationMeta) (Provider, error) {
	var c model.TwilioCreds
	if err := json.Unmarshal(creds, &c); err != nil {
		return nil, fmt.Errorf("invalid twilio credentials: %w", err)
	}
	return &TwilioProvider{creds: c}, nil
}

func (p *TwilioProvider) ID() string      { return "twilio" }
func (p *TwilioProvider) Channel() string { return "sms" }

func (p *TwilioProvider) Send(ctx context.Context, opts SendOptions) (SendResult, error) {
	apiURL := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", p.creds.AccountSID)

	data := url.Values{}
	data.Set("To", opts.To)
	data.Set("From", p.creds.FromNumber)
	data.Set("Body", opts.Content)

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return SendResult{}, err
	}
	req.SetBasicAuth(p.creds.AccountSID, p.creds.AuthToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return SendResult{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return SendResult{}, fmt.Errorf("twilio error: status %d, body: %s", resp.StatusCode, string(body))
	}

	var result struct {
		SID string `json:"sid"`
	}
	json.Unmarshal(body, &result)

	return SendResult{ProviderMessageID: result.SID}, nil
}
