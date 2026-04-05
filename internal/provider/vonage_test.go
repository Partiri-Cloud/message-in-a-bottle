package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/partiri/message-in-a-bottle/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sendVonage replicates VonageProvider.Send with a configurable API URL
func sendVonage(ctx context.Context, creds model.VonageCreds, apiURL string, opts SendOptions) (SendResult, error) {
	data := url.Values{}
	data.Set("api_key", creds.APIKey)
	data.Set("api_secret", creds.APISecret)
	data.Set("from", creds.FromNumber)
	data.Set("to", opts.To)
	data.Set("text", opts.Content)

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return SendResult{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return SendResult{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return SendResult{}, fmt.Errorf("vonage error: status %d, body: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Messages []struct {
			MessageID string `json:"message-id"`
			Status    string `json:"status"`
		} `json:"messages"`
	}
	json.Unmarshal(body, &result)

	if len(result.Messages) > 0 && result.Messages[0].Status != "0" {
		return SendResult{}, fmt.Errorf("vonage sms failed: status %s", result.Messages[0].Status)
	}

	var msgID string
	if len(result.Messages) > 0 {
		msgID = result.Messages[0].MessageID
	}
	return SendResult{ProviderMessageID: msgID}, nil
}

func TestVonageProvider_SendSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"messages": []map[string]string{
				{"message-id": "MSG123", "status": "0"},
			},
		})
	}))
	defer server.Close()

	creds := model.VonageCreds{APIKey: "key", APISecret: "secret", FromNumber: "+1234567890"}
	result, err := sendVonage(context.Background(), creds, server.URL, SendOptions{
		To:      "+0987654321",
		Content: "Test SMS",
	})
	require.NoError(t, err)
	assert.Equal(t, "MSG123", result.ProviderMessageID)
}

func TestVonageProvider_SendFailedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"messages": []map[string]string{
				{"message-id": "", "status": "4"},
			},
		})
	}))
	defer server.Close()

	creds := model.VonageCreds{APIKey: "key", APISecret: "secret", FromNumber: "+1234567890"}
	_, err := sendVonage(context.Background(), creds, server.URL, SendOptions{
		To:      "+0987654321",
		Content: "Test",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status 4")
}

func TestVonageProvider_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer server.Close()

	creds := model.VonageCreds{APIKey: "key", APISecret: "secret", FromNumber: "+1234567890"}
	_, err := sendVonage(context.Background(), creds, server.URL, SendOptions{
		To:      "+0987654321",
		Content: "Test",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}
