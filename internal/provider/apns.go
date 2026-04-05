package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/partiri/message-in-a-bottle/internal/model"
	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/payload"
	"github.com/sideshow/apns2/token"
)

type APNSProvider struct {
	creds model.APNSCreds
}

func NewAPNSProvider(creds json.RawMessage, meta model.IntegrationMeta) (Provider, error) {
	var c model.APNSCreds
	if err := json.Unmarshal(creds, &c); err != nil {
		return nil, fmt.Errorf("invalid apns credentials: %w", err)
	}
	return &APNSProvider{creds: c}, nil
}

func (p *APNSProvider) ID() string      { return "apns" }
func (p *APNSProvider) Channel() string { return "push" }

func (p *APNSProvider) Send(ctx context.Context, opts SendOptions) (SendResult, error) {
	authKey, err := token.AuthKeyFromBytes([]byte(p.creds.PrivateKey))
	if err != nil {
		return SendResult{}, fmt.Errorf("auth key parse: %w", err)
	}

	tok := &token.Token{
		AuthKey: authKey,
		KeyID:   p.creds.KeyID,
		TeamID:  p.creds.TeamID,
	}

	client := apns2.NewTokenClient(tok).Production()

	pl := payload.NewPayload().
		AlertTitle(opts.Subject).
		AlertBody(opts.Content)

	if opts.Metadata != nil {
		pl.Custom("data", opts.Metadata)
	}

	notification := &apns2.Notification{
		DeviceToken: opts.To,
		Topic:       p.creds.BundleID,
		Payload:     pl,
	}

	resp, err := client.PushWithContext(ctx, notification)
	if err != nil {
		return SendResult{}, err
	}

	if !resp.Sent() {
		return SendResult{}, fmt.Errorf("apns error: %d %s", resp.StatusCode, resp.Reason)
	}

	return SendResult{ProviderMessageID: resp.ApnsID}, nil
}
