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

func TestMSTeamsProvider_SendSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := &MSTeamsProvider{}
	_, err := p.post(context.Background(), server.URL, "Alert", "CPU high", server.Client())
	require.NoError(t, err)
}

func TestMSTeamsProvider_SendError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer server.Close()

	p := &MSTeamsProvider{}
	_, err := p.post(context.Background(), server.URL, "Alert", "test", server.Client())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestMSTeamsProvider_SendError_NoBodyReflection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("secret-internal-error-detail"))
	}))
	defer server.Close()

	p := &MSTeamsProvider{}
	_, err := p.post(context.Background(), server.URL, "Alert", "test", server.Client())
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "secret-internal-error-detail",
		"response body must not be reflected in the error")
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
	p.post(context.Background(), server.URL, "Deploy Alert", "Service deployed", server.Client())

	assert.Equal(t, "MessageCard", receivedBody["@type"])
	assert.Equal(t, "http://schema.org/extensions", receivedBody["@context"])
	assert.Equal(t, "Deploy Alert", receivedBody["summary"])

	sections := receivedBody["sections"].([]any)
	section := sections[0].(map[string]any)
	assert.Equal(t, "Deploy Alert", section["activityTitle"])
	assert.Equal(t, "Service deployed", section["text"])
}

func TestMSTeamsProvider_Send_RejectsInvalidURL(t *testing.T) {
	p := &MSTeamsProvider{}
	_, err := p.Send(context.Background(), SendOptions{
		To:      "http://myorg.webhook.office.com/x",
		Subject: "Alert",
		Content: "test",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "https")
}

func TestMSTeamsProvider_Send_RejectsNonAllowlisted(t *testing.T) {
	p := &MSTeamsProvider{}
	_, err := p.Send(context.Background(), SendOptions{
		To:      "https://attacker.example.com/exfil",
		Subject: "Alert",
		Content: "test",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not in the allowed list")
}
