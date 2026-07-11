package dto

import (
	"time"

	"github.com/partiri-cloud/message-in-a-bottle/internal/model"
)

// UpdatePreferenceRequest carries a partial channel update.
//
// Channels is a field map rather than a struct: a channel absent from the map
// means "leave it as it is", so a payload like {"email": true} cannot silently
// disable in-app delivery. Keeping it a map is also what stops the request
// shape drifting from model.ChannelPrefs — a struct here would need a field per
// channel, and a channel added to the model but forgotten here would be
// accepted and silently ignored. The handler resolves every key through
// model.ChannelByField and rejects one that is not a channel.
//
// A null value is treated as absent, so {"email": null} is a no-op rather than
// a disable.
type UpdatePreferenceRequest struct {
	Channels map[string]*bool `json:"channels" binding:"required"`
}

// PreferenceResponse is one row of a subscriber's notification settings.
//
// Channels are the *effective* values — what will actually govern delivery,
// after the subscriber's workflow-specific preference, their global preference,
// and the workflow's declared defaults have been resolved. A client can render
// this directly and be right; it never has to guess at a default it cannot see.
//
// Workflows are addressed by their identifier everywhere else in the system
// (that is what a trigger carries), so the identifier is what clients hold. The
// ObjectID is included too, for callers that already key on it. Both are null on
// the row describing the subscriber's global preference.
//
// Explicit says whether the subscriber has actually stored a choice here, or is
// simply inheriting. UpdatedAt is null when they have not.
//
// Channels is model.ChannelPrefs itself, not a copy of its shape: the two carry
// identical JSON tags, and a hand-written conversion between them is one more
// six-channel list to forget to update — a channel missing from it would report
// false forever while delivery had it on.
type PreferenceResponse struct {
	WorkflowID         *string            `json:"workflowId"`
	WorkflowIdentifier *string            `json:"workflowIdentifier"`
	Channels           model.ChannelPrefs `json:"channels"`
	Explicit           bool               `json:"explicit"`
	UpdatedAt          *time.Time         `json:"updatedAt"`
}
