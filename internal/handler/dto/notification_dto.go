package dto

type BulkActionRequest struct {
	Action          string   `json:"action" binding:"required"` // "read" | "seen"
	NotificationIDs []string `json:"notificationIds" binding:"required"`
}
