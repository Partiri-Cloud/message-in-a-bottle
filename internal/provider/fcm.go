package provider

import (
	"context"
	"encoding/json"
	"fmt"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"github.com/partiri/message-in-a-bottle/internal/model"
	"google.golang.org/api/option"
)

type FCMProvider struct {
	creds model.FCMCreds
}

func NewFCMProvider(creds json.RawMessage, meta model.IntegrationMeta) (Provider, error) {
	var c model.FCMCreds
	if err := json.Unmarshal(creds, &c); err != nil {
		return nil, fmt.Errorf("invalid fcm credentials: %w", err)
	}
	return &FCMProvider{creds: c}, nil
}

func (p *FCMProvider) ID() string      { return "fcm" }
func (p *FCMProvider) Channel() string { return "push" }

func (p *FCMProvider) Send(ctx context.Context, opts SendOptions) (SendResult, error) {
	app, err := firebase.NewApp(ctx, nil, option.WithCredentialsJSON([]byte(p.creds.ServiceAccountJSON)))
	if err != nil {
		return SendResult{}, fmt.Errorf("firebase init: %w", err)
	}

	client, err := app.Messaging(ctx)
	if err != nil {
		return SendResult{}, fmt.Errorf("messaging client: %w", err)
	}

	msg := &messaging.Message{
		Token: opts.To,
		Notification: &messaging.Notification{
			Title: opts.Subject,
			Body:  opts.Content,
		},
	}

	if opts.Metadata != nil {
		data := make(map[string]string)
		for k, v := range opts.Metadata {
			data[k] = fmt.Sprintf("%v", v)
		}
		msg.Data = data
	}

	resp, err := client.Send(ctx, msg)
	if err != nil {
		return SendResult{}, err
	}

	return SendResult{ProviderMessageID: resp}, nil
}
