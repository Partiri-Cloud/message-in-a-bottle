package provider

import (
	"context"
	"log"
)

type LogProvider struct{}

func (p *LogProvider) ID() string      { return "log" }
func (p *LogProvider) Channel() string { return "log" }

func (p *LogProvider) Send(ctx context.Context, opts SendOptions) (SendResult, error) {
	log.Printf("[LOG PROVIDER] to=%s subject=%q content_len=%d", opts.To, opts.Subject, len(opts.Content))
	return SendResult{ProviderMessageID: "log-" + opts.To}, nil
}
