package provider

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/partiri/message-in-a-bottle/internal/crypto"
	"github.com/partiri/message-in-a-bottle/internal/model"
)

type ProviderFactory struct {
	mu       sync.RWMutex
	builders map[string]BuilderFunc
}

type BuilderFunc func(credentials json.RawMessage, meta model.IntegrationMeta) (Provider, error)

func NewProviderFactory() *ProviderFactory {
	f := &ProviderFactory{
		builders: make(map[string]BuilderFunc),
	}
	f.registerDefaults()
	return f
}

func (f *ProviderFactory) Register(providerID string, builder BuilderFunc) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.builders[providerID] = builder
}

func (f *ProviderFactory) Create(integration *model.Integration, encryptionKey []byte) (Provider, error) {
	f.mu.RLock()
	builder, ok := f.builders[integration.ProviderID]
	f.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", integration.ProviderID)
	}

	var credJSON json.RawMessage
	if len(integration.Credentials) > 0 && len(encryptionKey) > 0 {
		decrypted, err := crypto.Decrypt(integration.Credentials, encryptionKey)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt credentials: %w", err)
		}
		credJSON = decrypted
	}

	return builder(credJSON, integration.Metadata)
}

func (f *ProviderFactory) registerDefaults() {
	f.Register("log", func(creds json.RawMessage, meta model.IntegrationMeta) (Provider, error) {
		return &LogProvider{}, nil
	})
	f.Register("sendgrid", NewSendGridProvider)
	f.Register("ses", NewSESProvider)
	f.Register("smtp", NewSMTPProvider)
	f.Register("twilio", NewTwilioProvider)
	f.Register("vonage", NewVonageProvider)
	f.Register("fcm", NewFCMProvider)
	f.Register("apns", NewAPNSProvider)
	f.Register("slack_webhook", NewSlackProvider)
	f.Register("ms_teams_webhook", NewMSTeamsProvider)
}
