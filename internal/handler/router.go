package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/partiri-cloud/message-in-a-box/internal/middleware"
	"github.com/partiri-cloud/message-in-a-box/internal/repository"
)

type Handlers struct {
	Subscriber   *SubscriberHandler
	Topic        *TopicHandler
	Workflow     *WorkflowHandler
	Integration  *IntegrationHandler
	Template     *TemplateHandler
	Preference   *PreferenceHandler
	Notification *NotificationHandler
	Event        *EventHandler
	Admin        *AdminHandler
}

func RegisterRoutes(router *gin.Engine, h *Handlers, envRepo *repository.EnvironmentRepository, adminSecret, subscriberHMACSecret string) {
	// Admin routes (protected by admin secret, no API key required)
	admin := router.Group("/admin")
	admin.Use(middleware.AdminSecretAuth(adminSecret))
	{
		admin.POST("/environments", h.Admin.CreateEnvironment)
		admin.GET("/environments", h.Admin.ListEnvironments)
		admin.POST("/environments/:identifier/keys", h.Admin.AddAPIKey)
	}

	api := router.Group("/api/v1")
	api.Use(middleware.AuthMiddleware(envRepo))

	// Subscribers (server-only CRUD — no subscriber token required)
	sub := api.Group("/subscribers")
	{
		sub.POST("", middleware.RequirePermission("subscribers:write"), h.Subscriber.Create)
		sub.POST("/bulk", middleware.RequirePermission("subscribers:write"), h.Subscriber.BulkCreate)
		sub.GET("/:subscriberId", middleware.RequirePermission("subscribers:read"), h.Subscriber.Get)
		sub.PATCH("/:subscriberId", middleware.RequirePermission("subscribers:write"), h.Subscriber.Update)
		sub.DELETE("/:subscriberId", middleware.RequirePermission("subscribers:write"), h.Subscriber.Delete)
	}

	// Subscriber-scoped routes (require X-Subscriber-Token HMAC validation)
	scoped := sub.Group("")
	if subscriberHMACSecret != "" {
		scoped.Use(middleware.SubscriberScope(subscriberHMACSecret))
	}
	{
		// Subscriber preferences
		scoped.GET("/:subscriberId/preferences", middleware.RequirePermission("preferences:read"), h.Preference.GetAll)
		scoped.PATCH("/:subscriberId/preferences", middleware.RequirePermission("preferences:write"), h.Preference.UpdateGlobal)
		scoped.PATCH("/:subscriberId/preferences/:workflowId", middleware.RequirePermission("preferences:write"), h.Preference.UpdateWorkflow)

		// Subscriber feed
		scoped.GET("/:subscriberId/feed", middleware.RequirePermission("notifications:read"), h.Notification.Feed)
		scoped.GET("/:subscriberId/feed/unseen-count", middleware.RequirePermission("notifications:read"), h.Notification.UnseenCount)
		scoped.POST("/:subscriberId/feed/:notifId/seen", middleware.RequirePermission("notifications:read"), h.Notification.MarkSeen)
		scoped.POST("/:subscriberId/feed/:notifId/read", middleware.RequirePermission("notifications:read"), h.Notification.MarkRead)
		scoped.POST("/:subscriberId/feed/:notifId/archive", middleware.RequirePermission("notifications:read"), h.Notification.Archive)
		scoped.POST("/:subscriberId/feed/bulk-action", middleware.RequirePermission("notifications:read"), h.Notification.BulkAction)
	}

	// Topics
	topic := api.Group("/topics")
	{
		topic.POST("", middleware.RequirePermission("topics:write"), h.Topic.Create)
		topic.GET("", middleware.RequirePermission("topics:read"), h.Topic.List)
		topic.GET("/:topicKey", middleware.RequirePermission("topics:read"), h.Topic.Get)
		topic.PATCH("/:topicKey", middleware.RequirePermission("topics:write"), h.Topic.Update)
		topic.DELETE("/:topicKey", middleware.RequirePermission("topics:write"), h.Topic.Delete)
		topic.POST("/:topicKey/subscribers", middleware.RequirePermission("topics:write"), h.Topic.AddSubscribers)
		topic.DELETE("/:topicKey/subscribers", middleware.RequirePermission("topics:write"), h.Topic.RemoveSubscribers)
		topic.GET("/:topicKey/subscribers", middleware.RequirePermission("topics:read"), h.Topic.ListSubscribers)
	}

	// Workflows
	wf := api.Group("/workflows")
	{
		wf.POST("", middleware.RequirePermission("workflows:write"), h.Workflow.Create)
		wf.GET("", middleware.RequirePermission("workflows:read"), h.Workflow.List)
		wf.GET("/:workflowId", middleware.RequirePermission("workflows:read"), h.Workflow.Get)
		wf.PUT("/:workflowId", middleware.RequirePermission("workflows:write"), h.Workflow.Update)
		wf.PATCH("/:workflowId/status", middleware.RequirePermission("workflows:write"), h.Workflow.SetStatus)
		wf.DELETE("/:workflowId", middleware.RequirePermission("workflows:write"), h.Workflow.Delete)
	}

	// Integrations
	intg := api.Group("/integrations")
	{
		intg.POST("", middleware.RequirePermission("integrations:write"), h.Integration.Create)
		intg.GET("", middleware.RequirePermission("integrations:read"), h.Integration.List)
		intg.PUT("/:id", middleware.RequirePermission("integrations:write"), h.Integration.Update)
		intg.DELETE("/:id", middleware.RequirePermission("integrations:write"), h.Integration.Delete)
		intg.PATCH("/:id/primary", middleware.RequirePermission("integrations:write"), h.Integration.SetPrimary)
	}

	// Templates
	tmpl := api.Group("/templates")
	{
		tmpl.POST("", middleware.RequirePermission("templates:write"), h.Template.Create)
		tmpl.GET("", middleware.RequirePermission("templates:read"), h.Template.List)
		tmpl.GET("/:identifier", middleware.RequirePermission("templates:read"), h.Template.Get)
		tmpl.PUT("/:identifier", middleware.RequirePermission("templates:write"), h.Template.Update)
		tmpl.DELETE("/:identifier", middleware.RequirePermission("templates:write"), h.Template.Delete)
		tmpl.POST("/:identifier/send", middleware.RequirePermission("templates:send"), h.Template.Send)
	}

	// Events (triggers)
	events := api.Group("/events")
	{
		events.POST("/trigger", middleware.RequirePermission("notifications:trigger"), h.Event.Trigger)
		events.POST("/trigger/bulk", middleware.RequirePermission("notifications:trigger"), h.Event.BulkTrigger)
		events.POST("/trigger/broadcast", middleware.RequirePermission("notifications:trigger"), h.Event.Broadcast)
	}

	// Notifications & Activity
	notif := api.Group("/notifications")
	{
		notif.GET("", middleware.RequirePermission("notifications:read"), h.Notification.List)
		notif.GET("/:id", middleware.RequirePermission("notifications:read"), h.Notification.Get)
	}

	api.GET("/activity", middleware.RequirePermission("activity:read"), h.Notification.Activity)
}
