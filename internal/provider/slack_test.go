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

// The post() helper is used in HTTP delivery tests to avoid requiring an
// allowlisted URL — URL validation is covered separately via TestValidateWebhookURL.

func TestSlackProvider_SendSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	p := &SlackProvider{}
	_, err := p.post(context.Background(), server.URL, "Hello Slack!", server.Client())
	require.NoError(t, err)
}

func TestSlackProvider_SendError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid_payload"))
	}))
	defer server.Close()

	p := &SlackProvider{}
	_, err := p.post(context.Background(), server.URL, "Hello", server.Client())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "400")
}

func TestSlackProvider_SendError_NoBodyReflection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("secret-internal-error-detail"))
	}))
	defer server.Close()

	p := &SlackProvider{}
	_, err := p.post(context.Background(), server.URL, "Hello", server.Client())
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "secret-internal-error-detail",
		"response body must not be reflected in the error")
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
	p.post(context.Background(), server.URL, "Test message", server.Client())
	assert.Equal(t, "Test message", receivedBody["text"])
}

func TestSlackProvider_Send_RejectsInvalidURL(t *testing.T) {
	p := &SlackProvider{}
	_, err := p.Send(context.Background(), SendOptions{
		To:      "http://hooks.slack.com/services/x",
		Content: "test",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "https")
}

func TestSlackProvider_Send_RejectsNonAllowlisted(t *testing.T) {
	p := &SlackProvider{}
	_, err := p.Send(context.Background(), SendOptions{
		To:      "https://attacker.example.com/exfil",
		Content: "test",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not in the allowed list")
}
