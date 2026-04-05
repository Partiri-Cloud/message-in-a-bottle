package engine

import "github.com/partiri/message-in-a-bottle/internal/model"

func IsChannelEnabled(channel string, workflowPref *model.SubscriberPreference, globalPref *model.SubscriberPreference, workflowDefaults model.ChannelPrefs) bool {
	// Priority: subscriber workflow-specific → subscriber global → workflow defaults
	if workflowPref != nil {
		if v, ok := getChannelPref(channel, workflowPref.Channels); ok {
			return v
		}
	}
	if globalPref != nil {
		if v, ok := getChannelPref(channel, globalPref.Channels); ok {
			return v
		}
	}
	v, _ := getChannelPref(channel, workflowDefaults)
	return v
}

func getChannelPref(channel string, prefs model.ChannelPrefs) (bool, bool) {
	switch channel {
	case "email":
		return prefs.Email, true
	case "sms":
		return prefs.SMS, true
	case "push":
		return prefs.Push, true
	case "in_app":
		return prefs.InApp, true
	case "slack":
		return prefs.Slack, true
	case "ms_teams":
		return prefs.MSTeams, true
	}
	return false, false
}
