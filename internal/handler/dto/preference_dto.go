package dto

type UpdatePreferenceRequest struct {
	Channels ChannelPrefsDTO `json:"channels" binding:"required"`
}
