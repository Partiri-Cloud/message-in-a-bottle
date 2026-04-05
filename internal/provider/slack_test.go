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

func TestSlackProvider_SendSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	p := &SlackProvider{}
	_, err := p.Send(context.Background(), SendOptions{
		To:      server.URL,
		Content: "Hello Slack!",
	})
	require.NoError(t, err)
}

func TestSlackProvider_SendError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid_payload"))
	}))
	defer server.Close()

	p := &SlackProvider{}
	_, err := p.Send(context.Background(), SendOptions{
		To:      server.URL,
		Content: "Hello",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "400")
}

func TestSlackProvider_PayloadFormat(t *testing.T) {
	var receivedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := &SlackProvider{}
	p.Send(context.Background(), SendOptions{
		To:      server.URL,
		Content: "Test message",
	})
	assert.Equal(t, "Test message", receivedBody["text"])
}
