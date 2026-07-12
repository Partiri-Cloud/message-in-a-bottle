package ws

import "encoding/json"

// Server → client events
const (
	EventNotificationNew = "notification:new"
	EventUnseenCount     = "notification:unseen_count"
)

// Client → server actions
const (
	ActionSeen    = "notification:seen"
	ActionRead    = "notification:read"
	ActionArchive = "notification:archive"
	ActionFetch   = "feed:fetch"
)

type WSMessage struct {
	Room  string `json:"room"`
	Event string `json:"event"`
	Data  any    `json:"data"`
}

type ClientMessage struct {
	Action  string          `json:"action"`
	Payload json.RawMessage `json:"payload"`
}

type SeenPayload struct {
	NotificationID string `json:"notificationId"`
}

type ReadPayload struct {
	NotificationID string `json:"notificationId"`
}

type ArchivePayload struct {
	NotificationID string `json:"notificationId"`
}

type FetchPayload struct {
	Page  int   `json:"page"`
	Limit int   `json:"limit"`
	Read  *bool `json:"read,omitempty"`
	Seen  *bool `json:"seen,omitempty"`
}
