package provider

import (
	"context"
	"encoding/json"
	"fmt"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
	"github.com/partiri-cloud/message-in-a-box/internal/model"
)

type SESProvider struct {
	creds model.SESCreds
	meta  model.IntegrationMeta
}

func NewSESProvider(creds json.RawMessage, meta model.IntegrationMeta) (Provider, error) {
	var c model.SESCreds
	if err := json.Unmarshal(creds, &c); err != nil {
		return nil, fmt.Errorf("invalid ses credentials: %w", err)
	}
	return &SESProvider{creds: c, meta: meta}, nil
}

func (p *SESProvider) ID() string      { return "ses" }
func (p *SESProvider) Channel() string { return "email" }

func (p *SESProvider) Send(ctx context.Context, opts SendOptions) (SendResult, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(p.creds.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			p.creds.AccessKeyID,
			p.creds.SecretAccessKey,
			"",
		)),
	)
	if err != nil {
		return SendResult{}, fmt.Errorf("aws config: %w", err)
	}

	client := ses.NewFromConfig(cfg)

	input := &ses.SendEmailInput{
		Source: &p.meta.SenderEmail,
		Destination: &types.Destination{
			ToAddresses: []string{opts.To},
		},
		Message: &types.Message{
			Subject: &types.Content{Data: &opts.Subject},
			Body: &types.Body{
				Html: &types.Content{Data: &opts.Content},
			},
		},
	}

	result, err := client.SendEmail(ctx, input)
	if err != nil {
		return SendResult{}, err
	}

	return SendResult{ProviderMessageID: *result.MessageId}, nil
}
