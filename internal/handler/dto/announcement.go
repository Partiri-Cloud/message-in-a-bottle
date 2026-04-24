package dto

import "time"

type CreateAnnouncementRequest struct {
	Severity  string     `json:"severity" binding:"required,oneof=info warning critical"`
	Title     string     `json:"title" binding:"required"`
	Message   string     `json:"message" binding:"required"`
	StartsAt  *time.Time `json:"startsAt"`
	ExpiresAt time.Time  `json:"expiresAt" binding:"required"`
}

type UpdateAnnouncementRequest struct {
	Severity  *string    `json:"severity" binding:"omitempty,oneof=info warning critical"`
	Title     *string    `json:"title"`
	Message   *string    `json:"message"`
	StartsAt  *time.Time `json:"startsAt"`
	ExpiresAt *time.Time `json:"expiresAt"`
}

type Announcement struct {
	ID        string     `json:"id"`
	Severity  string     `json:"severity"`
	Title     string     `json:"title"`
	Message   string     `json:"message"`
	StartsAt  *time.Time `json:"startsAt,omitempty"`
	ExpiresAt time.Time  `json:"expiresAt"`
	CreatedAt time.Time  `json:"createdAt"`
}
