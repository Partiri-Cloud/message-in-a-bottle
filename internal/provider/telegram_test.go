package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/partiri-cloud/message-in-a-bottle/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testBotToken = "123456:ABC-secret-token"

func newTestTelegramProvider(server *httptest.Server) *TelegramProvider {
	return &TelegramProvider{token: testBotToken, baseURL: server.URL, client: server.Client()}
}

func TestTelegramProvider_SendSuccess(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotBody)
		w.Write([]byte(`{"ok":true,"result":{"message_id":42}}`))
	}))
	defer server.Close()

	res, err := newTestTelegramProvider(server).Send(context.Background(), SendOptions{To: "-1001234567890", Subject: "ignored", Content: "Hello Telegram"})
	require.NoError(t, err)
	assert.Equal(t, "42", res.ProviderMessageID)
	assert.Equal(t, "/bot"+testBotToken+"/sendMessage", gotPath)
	assert.Equal(t, "-1001234567890", gotBody["chat_id"])
	assert.Equal(t, "Hello Telegram", gotBody["text"])
	assert.NotContains(t, gotBody, "parse_mode", "v1 sends plain text only")
}

func TestTelegramProvider_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"ok":false,"error_code":403,"description":"Forbidden: bot was blocked by the user"}`))
	}))
	defer server.Close()

	_, err := newTestTelegramProvider(server).Send(context.Background(), SendOptions{To: "123", Content: "hi"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "403")
	assert.Contains(t, err.Error(), "bot was blocked by the user")
}

func TestTelegramProvider_NonJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte("<html>bad gateway</html>"))
	}))
	defer server.Close()

	_, err := newTestTelegramProvider(server).Send(context.Background(), SendOptions{To: "123", Content: "hi"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "502")
}

func TestTelegramProvider_InvalidChatIDSendsNothing(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer server.Close()

	_, err := newTestTelegramProvider(server).Send(context.Background(), SendOptions{To: "", Content: "hi"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no telegram chat id configured", "an unset chat id is not reported as malformed")

	_, err = newTestTelegramProvider(server).Send(context.Background(), SendOptions{To: "not a chat", Content: "hi"})
	require.Error(t, err)
	assert.False(t, called)
}

func TestTelegramProvider_OKFalseWith200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ok":false,"error_code":400,"description":"Bad Request: chat not found"}`))
	}))
	defer server.Close()

	_, err := newTestTelegramProvider(server).Send(context.Background(), SendOptions{To: "123", Content: "hi"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chat not found")
}

// A token straddling the description cut must not leave a prefix behind.
func TestTelegramProvider_TokenAcrossDescriptionCut(t *testing.T) {
	desc := strings.Repeat("x", telegramMaxDescription-5) + testBotToken
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{"ok": false, "error_code": 400, "description": desc})
	}))
	defer server.Close()

	_, err := newTestTelegramProvider(server).Send(context.Background(), SendOptions{To: "123", Content: "hi"})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), testBotToken[:5])
}

// The token is part of the request URL, and transport errors quote the URL.
// Provider errors are stored in the activity log, so the token must not appear.
func TestTelegramProvider_TokenNeverInError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	p := newTestTelegramProvider(server)
	server.Close() // connection refused

	_, err := p.Send(context.Background(), SendOptions{To: "123", Content: "hi"})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), testBotToken)

	echo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"ok":false,"error_code":400,"description":"bad token ` + testBotToken + `"}`))
	}))
	defer echo.Close()

	_, err = newTestTelegramProvider(echo).Send(context.Background(), SendOptions{To: "123", Content: "hi"})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), testBotToken)
	assert.Contains(t, err.Error(), "<redacted>")
}

func TestNewTelegramProvider_RequiresToken(t *testing.T) {
	_, err := NewTelegramProvider(json.RawMessage(`{}`), model.IntegrationMeta{})
	assert.Error(t, err)

	for _, bad := range []string{"no-colon", "123:abc/../getMe", "123:abc?x=1", ":abc"} {
		_, err := NewTelegramProvider(json.RawMessage(`{"botToken":"`+bad+`"}`), model.IntegrationMeta{})
		assert.Error(t, err, bad)
	}

	p, err := NewTelegramProvider(json.RawMessage(`{"botToken":"`+testBotToken+`"}`), model.IntegrationMeta{})
	require.NoError(t, err)
	assert.Equal(t, "telegram_bot", p.ID())
	assert.Equal(t, "telegram", p.Channel())
}

func TestValidateTelegramChatID(t *testing.T) {
	for _, ok := range []string{"123456789", "-1001234567890", "@my_channel"} {
		assert.NoError(t, ValidateTelegramChatID(ok), ok)
	}
	for _, bad := range []string{"", "abc", "12 34", "@abc", "@1channel", "https://t.me/x", "123456789012345678901"} {
		assert.Error(t, ValidateTelegramChatID(bad), bad)
	}
}

func utf16Len(s string) int { return len(utf16.Encode([]rune(s))) }

func TestTruncateTelegramText(t *testing.T) {
	short := strings.Repeat("a", telegramMaxTextUnits)
	assert.Equal(t, short, truncateTelegramText(short), "text at the limit is untouched")

	long := truncateTelegramText(strings.Repeat("a", telegramMaxTextUnits+10))
	assert.Equal(t, telegramMaxTextUnits, utf16Len(long))
	assert.True(t, strings.HasSuffix(long, "…"))

	// Emoji are two UTF-16 units; the cut must not split one.
	emoji := truncateTelegramText(strings.Repeat("😀", telegramMaxTextUnits))
	assert.LessOrEqual(t, utf16Len(emoji), telegramMaxTextUnits)
	assert.True(t, strings.HasSuffix(emoji, "😀…"))
}
