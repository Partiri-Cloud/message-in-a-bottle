package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/smtp"
	"strings"

	"github.com/partiri-cloud/message-in-a-box/internal/model"
)

type SMTPProvider struct {
	creds model.SMTPCreds
	meta  model.IntegrationMeta
}

func NewSMTPProvider(creds json.RawMessage, meta model.IntegrationMeta) (Provider, error) {
	var c model.SMTPCreds
	if err := json.Unmarshal(creds, &c); err != nil {
		return nil, fmt.Errorf("invalid smtp credentials: %w", err)
	}
	return &SMTPProvider{creds: c, meta: meta}, nil
}

func (p *SMTPProvider) ID() string      { return "smtp" }
func (p *SMTPProvider) Channel() string { return "email" }

func (p *SMTPProvider) Send(ctx context.Context, opts SendOptions) (SendResult, error) {
	addr := fmt.Sprintf("%s:%d", p.creds.Host, p.creds.Port)
	auth := smtp.PlainAuth("", p.creds.User, p.creds.Password, p.creds.Host)

	msg := strings.Join([]string{
		"From: " + p.meta.SenderEmail,
		"To: " + opts.To,
		"Subject: " + opts.Subject,
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=UTF-8",
		"",
		opts.Content,
	}, "\r\n")

	err := smtp.SendMail(addr, auth, p.meta.SenderEmail, []string{opts.To}, []byte(msg))
	return SendResult{}, err
}
