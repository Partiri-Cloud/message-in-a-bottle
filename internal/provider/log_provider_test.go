package provider

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogProvider_ID(t *testing.T) {
	p := &LogProvider{}
	assert.Equal(t, "log", p.ID())
}

func TestLogProvider_Channel(t *testing.T) {
	p := &LogProvider{}
	assert.Equal(t, "log", p.Channel())
}

func TestLogProvider_Send(t *testing.T) {
	p := &LogProvider{}
	result, err := p.Send(context.Background(), SendOptions{
		To:      "test@example.com",
		Subject: "Test",
		Content: "Hello",
	})
	require.NoError(t, err)
	assert.Contains(t, result.ProviderMessageID, "log-")
}
