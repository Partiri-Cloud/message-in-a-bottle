package dto

type CreateTopicRequest struct {
	Key  string `json:"key" binding:"required"`
	Name string `json:"name" binding:"required"`
}

type UpdateTopicRequest struct {
	Name string `json:"name" binding:"required"`
}

type TopicSubscribersRequest struct {
	SubscriberIDs []string `json:"subscriberIds" binding:"required,min=1"`
}
