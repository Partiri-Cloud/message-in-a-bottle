package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMSTeamsProvider_SendSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := &MSTeamsProvider{}
	_, err := p.Send(context.Background(), SendOptions{
		To:      server.URL,
		Subject: "Alert",
		Content: "CPU high",
	})
	require.NoError(t, err)
}

func TestMSTeamsProvider_SendError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer server.Close()

	p := &MSTeamsProvider{}
	_, err := p.Send(context.Background(), SendOptions{
		To:      server.URL,
		Subject: "Alert",
		Content: "test",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestMSTeamsProvider_PayloadFormat(t *testing.T) {
	var receivedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := &MSTeamsProvider{}
	p.Send(context.Background(), SendOptions{
		To:      server.URL,
		Subject: "Deploy Alert",
		Content: "Service deployed",
	})

	assert.Equal(t, "MessageCard", receivedBody["@type"])
	assert.Equal(t, "http://schema.org/extensions", receivedBody["@context"])
	assert.Equal(t, "Deploy Alert", receivedBody["summary"])

	sections := receivedBody["sections"].([]any)
	section := sections[0].(map[string]any)
	assert.Equal(t, "Deploy Alert", section["activityTitle"])
	assert.Equal(t, "Service deployed", section["text"])
}
