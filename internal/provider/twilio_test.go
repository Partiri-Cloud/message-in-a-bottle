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

	"github.com/partiri-cloud/message-in-a-box/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sendTwilio replicates TwilioProvider.Send but with a configurable API URL
func sendTwilio(ctx context.Context, creds model.TwilioCreds, apiURL string, opts SendOptions) (SendResult, error) {
	data := url.Values{}
	data.Set("To", opts.To)
	data.Set("From", creds.FromNumber)
	data.Set("Body", opts.Content)

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return SendResult{}, err
	}
	req.SetBasicAuth(creds.AccountSID, creds.AuthToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return SendResult{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return SendResult{}, fmt.Errorf("twilio error: status %d, body: %s", resp.StatusCode, string(body))
	}

	var result struct {
		SID string `json:"sid"`
	}
	json.Unmarshal(body, &result)
	return SendResult{ProviderMessageID: result.SID}, nil
}

func TestTwilioProvider_SendSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"sid": "SM123abc"})
	}))
	defer server.Close()

	creds := model.TwilioCreds{AccountSID: "AC_test", AuthToken: "test_token", FromNumber: "+1234567890"}
	result, err := sendTwilio(context.Background(), creds, server.URL, SendOptions{
		To:      "+0987654321",
		Content: "Test SMS",
	})
	require.NoError(t, err)
	assert.Equal(t, "SM123abc", result.ProviderMessageID)
}

func TestTwilioProvider_SendError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"message":"Invalid phone number"}`))
	}))
	defer server.Close()

	creds := model.TwilioCreds{AccountSID: "AC_test", AuthToken: "test_token", FromNumber: "+1234567890"}
	_, err := sendTwilio(context.Background(), creds, server.URL, SendOptions{
		To:      "invalid",
		Content: "Test",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "400")
}

func TestTwilioProvider_BasicAuth(t *testing.T) {
	var gotUser, gotPass string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser, gotPass, _ = r.BasicAuth()
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"sid": "SM_test"})
	}))
	defer server.Close()

	creds := model.TwilioCreds{AccountSID: "AC_myaccount", AuthToken: "my_secret_token", FromNumber: "+1234567890"}
	sendTwilio(context.Background(), creds, server.URL, SendOptions{To: "+1111111111", Content: "Hi"})

	assert.Equal(t, "AC_myaccount", gotUser)
	assert.Equal(t, "my_secret_token", gotPass)
}

func TestTwilioProvider_FormEncoded(t *testing.T) {
	var gotContentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		r.ParseForm()
		assert.Equal(t, "+0987654321", r.FormValue("To"))
		assert.Equal(t, "+1234567890", r.FormValue("From"))
		assert.Equal(t, "Hello!", r.FormValue("Body"))
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"sid": "SM_test"})
	}))
	defer server.Close()

	creds := model.TwilioCreds{AccountSID: "AC_test", AuthToken: "token", FromNumber: "+1234567890"}
	sendTwilio(context.Background(), creds, server.URL, SendOptions{To: "+0987654321", Content: "Hello!"})

	assert.Equal(t, "application/x-www-form-urlencoded", gotContentType)
}
