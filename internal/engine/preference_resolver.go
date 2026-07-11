package engine

import "github.com/partiri-cloud/message-in-a-bottle/internal/model"

// ResolveChannelPrefs returns the channel set that governs delivery for one
// subscriber on one workflow.
//
// A workflow-specific preference row is the subscriber's most specific intent
// and wins outright. ChannelPrefs is plain bools, so an unset channel is
// indistinguishable from one explicitly set to false and there is nothing to
// fall through on per channel within a workflow row.
//
// Without one, the workflow's declared defaults apply, filtered through the
// subscriber's global preference: a per-channel opt-out mask that can silence
// a channel the defaults had on, but never enable one they had off. Enabling a
// channel a workflow chose not to send on is a workflow-level decision, made
// by writing a workflow row.
//
// This is the single source of that precedence: delivery (IsChannelEnabled) and
// the preferences read path both go through here, so what a subscriber is shown
// cannot drift from what is used to route their notifications.
func ResolveChannelPrefs(workflowPref, globalPref *model.SubscriberPreference, workflowDefaults model.ChannelPrefs) model.ChannelPrefs {
	if workflowPref != nil {
		return workflowPref.Channels
	}
	if globalPref != nil {
		return workflowDefaults.And(globalPref.Channels)
	}
	return workflowDefaults
}

// IsChannelEnabled reports whether a single channel is on for this subscriber
// and workflow. channel is the delivery-side name ("in_app", "ms_teams", …); an
// unknown name is not a channel and is never enabled.
func IsChannelEnabled(channel string, workflowPref *model.SubscriberPreference, globalPref *model.SubscriberPreference, workflowDefaults model.ChannelPrefs) bool {
	v, _ := ResolveChannelPrefs(workflowPref, globalPref, workflowDefaults).Get(channel)
	return v
}
