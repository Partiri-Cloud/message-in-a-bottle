package engine

import (
	"testing"

	"github.com/partiri-cloud/message-in-a-bottle/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestIsChannelEnabled_WorkflowPrefTakesPriority(t *testing.T) {
	wfPref := &model.SubscriberPreference{Channels: model.ChannelPrefs{Email: false}}
	globalPref := &model.SubscriberPreference{Channels: model.ChannelPrefs{Email: true}}
	defaults := model.ChannelPrefs{Email: true}

	assert.False(t, IsChannelEnabled("email", wfPref, globalPref, defaults))
}

// The global preference is an opt-out mask over the workflow defaults: it can
// silence a channel the defaults had on, but never enable one they had off.
func TestIsChannelEnabled_GlobalOptOutSilencesADefault(t *testing.T) {
	globalPref := &model.SubscriberPreference{Channels: model.ChannelPrefs{SMS: false, Email: true}}
	defaults := model.ChannelPrefs{SMS: true, Email: true}

	assert.False(t, IsChannelEnabled("sms", nil, globalPref, defaults))
	assert.True(t, IsChannelEnabled("email", nil, globalPref, defaults))
}

func TestIsChannelEnabled_GlobalPrefCannotEnableADisabledDefault(t *testing.T) {
	globalPref := &model.SubscriberPreference{Channels: model.ChannelPrefs{SMS: true}}
	defaults := model.ChannelPrefs{SMS: false}

	assert.False(t, IsChannelEnabled("sms", nil, globalPref, defaults))
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
	defaults := model.ChannelPrefs{Slack: true}

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

// ResolveChannelPrefs is what IsChannelEnabled resolves through, and what the
// preferences read path reports. A workflow row wins outright; without one the
// defaults apply, ANDed with the global opt-out mask.
func TestResolveChannelPrefs_Precedence(t *testing.T) {
	defaults := model.ChannelPrefs{InApp: true, Email: true}
	// Mask allows email, silences everything else (including in-app).
	globalPref := &model.SubscriberPreference{Channels: model.ChannelPrefs{Email: true}}
	wfPref := &model.SubscriberPreference{Channels: model.ChannelPrefs{Push: true}}

	assert.Equal(t, defaults, ResolveChannelPrefs(nil, nil, defaults))
	assert.Equal(t, model.ChannelPrefs{Email: true}, ResolveChannelPrefs(nil, globalPref, defaults),
		"defaults filtered through the mask: in-app silenced, email passes, nothing off gets enabled")
	assert.Equal(t, wfPref.Channels, ResolveChannelPrefs(wfPref, globalPref, defaults),
		"an explicit workflow row wins outright")
}
