package provider

import (
	"context"
	"log/slog"
)

type LogProvider struct{}

func (p *LogProvider) ID() string      { return "log" }
func (p *LogProvider) Channel() string { return "log" }

func (p *LogProvider) Send(ctx context.Context, opts SendOptions) (SendResult, error) {
	slog.Info("log provider delivery",
		"to", opts.To,
		"subject", opts.Subject,
		"contentLen", len(opts.Content),
	)
	return SendResult{ProviderMessageID: "log-" + opts.To}, nil
}
