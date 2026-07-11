package model

// channelFields is the canonical channel table.
//
// Every enumeration of the channels — masking, DTO conversion, partial $set
// updates, delivery lookups — is driven by this one table, so adding a channel
// means adding a field to ChannelPrefs and a row here, and nothing else. The
// previous arrangement hand-listed the six channels in half a dozen places, all
// of which compiled cleanly while silently leaving a new channel false.
//
// name is the delivery-side name a notification step carries ("in_app"),
// bsonField is the field it lives in on ChannelPrefs ("inApp"). They differ, so
// both are recorded here rather than derived.
var channelFields = []struct {
	name      string
	bsonField string
	ptr       func(*ChannelPrefs) *bool
}{
	{"email", "email", func(p *ChannelPrefs) *bool { return &p.Email }},
	{"sms", "sms", func(p *ChannelPrefs) *bool { return &p.SMS }},
	{"push", "push", func(p *ChannelPrefs) *bool { return &p.Push }},
	{"in_app", "inApp", func(p *ChannelPrefs) *bool { return &p.InApp }},
	{"slack", "slack", func(p *ChannelPrefs) *bool { return &p.Slack }},
	{"ms_teams", "msTeams", func(p *ChannelPrefs) *bool { return &p.MSTeams }},
}

// ChannelNames returns every delivery-side channel name, in declaration order.
func ChannelNames() []string {
	out := make([]string, 0, len(channelFields))
	for _, f := range channelFields {
		out = append(out, f.name)
	}
	return out
}

// ChannelBSONField maps a delivery-side channel name to the field it occupies
// on a stored ChannelPrefs, for callers building dotted update paths. ok is
// false for a name that is not a channel.
func ChannelBSONField(channel string) (field string, ok bool) {
	for _, f := range channelFields {
		if f.name == channel {
			return f.bsonField, true
		}
	}
	return "", false
}

// ChannelByField is the inverse: it maps the field name a client uses on the
// wire ("inApp" — the BSON and JSON names are the same) to the delivery-side
// channel name. ok is false for a field that is not a channel, which is how the
// API rejects an unknown channel instead of silently ignoring it.
func ChannelByField(field string) (channel string, ok bool) {
	for _, f := range channelFields {
		if f.bsonField == field {
			return f.name, true
		}
	}
	return "", false
}

// Get reports whether one channel is enabled. ok is false for an unknown name.
func (p ChannelPrefs) Get(channel string) (value, ok bool) {
	for _, f := range channelFields {
		if f.name == channel {
			return *f.ptr(&p), true
		}
	}
	return false, false
}

// Set writes one channel. An unknown name is ignored and reports false.
func (p *ChannelPrefs) Set(channel string, value bool) bool {
	for _, f := range channelFields {
		if f.name == channel {
			*f.ptr(p) = value
			return true
		}
	}
	return false
}

// And returns the channels enabled in both p and mask. This is how a global
// opt-out mask is applied to a workflow's declared defaults: the mask can
// silence a channel the defaults had on, never enable one they had off.
func (p ChannelPrefs) And(mask ChannelPrefs) ChannelPrefs {
	var out ChannelPrefs
	for _, f := range channelFields {
		*f.ptr(&out) = *f.ptr(&p) && *f.ptr(&mask)
	}
	return out
}

// AllChannelsEnabled is the identity for And: the mask that silences nothing.
func AllChannelsEnabled() ChannelPrefs {
	var out ChannelPrefs
	for _, f := range channelFields {
		*f.ptr(&out) = true
	}
	return out
}
