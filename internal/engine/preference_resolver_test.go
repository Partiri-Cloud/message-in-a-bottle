package engine

import (
	"testing"

	"github.com/partiri-cloud/message-in-a-box/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestIsChannelEnabled_WorkflowPrefTakesPriority(t *testing.T) {
	wfPref := &model.SubscriberPreference{Channels: model.ChannelPrefs{Email: false}}
	globalPref := &model.SubscriberPreference{Channels: model.ChannelPrefs{Email: true}}
	defaults := model.ChannelPrefs{Email: true}

	assert.False(t, IsChannelEnabled("email", wfPref, globalPref, defaults))
}

func TestIsChannelEnabled_GlobalPrefOverridesDefaults(t *testing.T) {
	globalPref := &model.SubscriberPreference{Channels: model.ChannelPrefs{SMS: true}}
	defaults := model.ChannelPrefs{SMS: false}

	assert.True(t, IsChannelEnabled("sms", nil, globalPref, defaults))
}

func TestIsChannelEnabled_DefaultsUsedWhenNoPrefs(t *testing.T) {
	defaults := model.ChannelPrefs{Push: true}
	assert.True(t, IsChannelEnabled("push", nil, nil, defaults))

	defaults2 := model.ChannelPrefs{Push: false}
	assert.False(t, IsChannelEnabled("push", nil, nil, defaults2))
}

func TestIsChannelEnabled_AllChannels(t *testing.T) {
	defaults := model.ChannelPrefs{
		Email: true, SMS: true, Push: true,
		InApp: true, Slack: true, MSTeams: true,
	}
	for _, ch := range []string{"email", "sms", "push", "in_app", "slack", "ms_teams"} {
		assert.True(t, IsChannelEnabled(ch, nil, nil, defaults), "channel %s should be enabled", ch)
	}
}

func TestIsChannelEnabled_NilWorkflowPref(t *testing.T) {
	globalPref := &model.SubscriberPreference{Channels: model.ChannelPrefs{Slack: true}}
	defaults := model.ChannelPrefs{Slack: false}

	assert.True(t, IsChannelEnabled("slack", nil, globalPref, defaults))
}

func TestIsChannelEnabled_NilBothPrefs(t *testing.T) {
	defaults := model.ChannelPrefs{MSTeams: false}
	assert.False(t, IsChannelEnabled("ms_teams", nil, nil, defaults))
}

func TestIsChannelEnabled_UnknownChannel(t *testing.T) {
	defaults := model.ChannelPrefs{Email: true}
	assert.False(t, IsChannelEnabled("carrier_pigeon", nil, nil, defaults))
}
