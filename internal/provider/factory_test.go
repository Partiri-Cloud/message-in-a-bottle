package provider

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/partiri-cloud/message-in-a-bottle/internal/crypto"
	"github.com/partiri-cloud/message-in-a-bottle/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderFactory_CreateLog(t *testing.T) {
	f := NewProviderFactory()
	intg := &model.Integration{ProviderID: "log"}
	p, err := f.Create(intg, nil)
	require.NoError(t, err)
	assert.Equal(t, "log", p.ID())
}

func TestProviderFactory_UnknownProvider(t *testing.T) {
	f := NewProviderFactory()
	intg := &model.Integration{ProviderID: "nonexistent"}
	_, err := f.Create(intg, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown provider")
}

func TestProviderFactory_DecryptsCredentials(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	creds := map[string]string{"apiKey": "test-key"}
	credsJSON, _ := json.Marshal(creds)

	encrypted, err := crypto.Encrypt(credsJSON, key)
	require.NoError(t, err)

	var receivedCreds json.RawMessage
	f := NewProviderFactory()
	f.Register("test-provider", func(c json.RawMessage, meta model.IntegrationMeta) (Provider, error) {
		receivedCreds = c
		return &LogProvider{}, nil
	})

	intg := &model.Integration{ProviderID: "test-provider", Credentials: encrypted}
	_, err = f.Create(intg, key)
	require.NoError(t, err)

	var parsed map[string]string
	require.NoError(t, json.Unmarshal(receivedCreds, &parsed))
	assert.Equal(t, "test-key", parsed["apiKey"])
}

func TestProviderFactory_DecryptionError(t *testing.T) {
	key := make([]byte, 32)
	f := NewProviderFactory()
	intg := &model.Integration{ProviderID: "log", Credentials: []byte("not-encrypted-data")}
	_, err := f.Create(intg, key)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decrypt")
}

func TestProviderFactory_NoEncryptionKey(t *testing.T) {
	f := NewProviderFactory()
	intg := &model.Integration{ProviderID: "log", Credentials: []byte("raw-data")}
	p, err := f.Create(intg, nil)
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestProviderFactory_RegisterCustom(t *testing.T) {
	f := NewProviderFactory()
	f.Register("custom", func(c json.RawMessage, meta model.IntegrationMeta) (Provider, error) {
		return &customTestProvider{}, nil
	})
	intg := &model.Integration{ProviderID: "custom"}
	p, err := f.Create(intg, nil)
	require.NoError(t, err)
	assert.Equal(t, "custom", p.ID())
}

func TestProviderFactory_AllDefaultsRegistered(t *testing.T) {
	f := NewProviderFactory()
	expected := []string{"log", "sendgrid", "ses", "smtp", "twilio", "vonage", "fcm", "apns", "slack_webhook", "ms_teams_webhook"}
	for _, id := range expected {
		_, ok := f.builders[id]
		assert.True(t, ok, "provider %q should be registered", id)
	}
}

type customTestProvider struct{}

func (p *customTestProvider) ID() string      { return "custom" }
func (p *customTestProvider) Channel() string { return "custom" }
func (p *customTestProvider) Send(ctx context.Context, opts SendOptions) (SendResult, error) {
	return SendResult{}, nil
}
