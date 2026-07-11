package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Handlers serialize these models straight onto the wire. They carry BSON tags,
// and without matching JSON tags Go falls back to the Go field names — the API
// then answers {"WorkflowID":...,"Channels":...} while every client (the SDK,
// the dashboard) reads workflowId/channels and silently sees undefined.
//
// These tests pin the key casing. They are about the keys, not the values.

func keysOf(t *testing.T, v any) map[string]json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	var out map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(b, &out))
	return out
}

func TestNotificationSerializesCamelCase(t *testing.T) {
	keys := keysOf(t, Notification{})

	for _, k := range []string{"id", "environmentId", "subscriberId", "workflowId", "transactionId", "payload", "channels", "seen", "read", "createdAt"} {
		assert.Contains(t, keys, k)
	}
	for _, k := range []string{"ID", "WorkflowID", "Channels", "CreatedAt"} {
		assert.NotContains(t, keys, k)
	}
}

func TestSubscriberPreferenceSerializesCamelCase(t *testing.T) {
	keys := keysOf(t, SubscriberPreference{})

	for _, k := range []string{"id", "environmentId", "subscriberId", "workflowId", "channels", "updatedAt"} {
		assert.Contains(t, keys, k)
	}
	assert.NotContains(t, keys, "WorkflowID")
}

func TestSubscriberSerializesCamelCase(t *testing.T) {
	keys := keysOf(t, Subscriber{})

	for _, k := range []string{"id", "environmentId", "subscriberId", "locale", "channels", "isOnline", "createdAt"} {
		assert.Contains(t, keys, k)
	}
	assert.NotContains(t, keys, "SubscriberID")
}

func TestChannelPrefsSerializesCamelCase(t *testing.T) {
	keys := keysOf(t, ChannelPrefs{})

	for _, k := range []string{"email", "sms", "push", "inApp", "slack", "msTeams"} {
		assert.Contains(t, keys, k)
	}
}

// Credential material and key hashes must never reach a response body, even if
// a future handler returns the model directly instead of a redacted view.
func TestSecretsAreNeverSerialized(t *testing.T) {
	assert.NotContains(t, keysOf(t, Integration{}), "credentials")
	assert.NotContains(t, keysOf(t, Environment{}), "apiKeys")
	assert.NotContains(t, keysOf(t, APIKey{}), "keyHash")
}
