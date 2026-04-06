package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/hibiken/asynq"
	"github.com/partiri-cloud/message-in-a-box/internal/config"
	"github.com/partiri-cloud/message-in-a-box/internal/engine"
	"github.com/partiri-cloud/message-in-a-box/internal/model"
	"github.com/partiri-cloud/message-in-a-box/internal/provider"
	"github.com/partiri-cloud/message-in-a-box/internal/repository"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type DeliveryHandler struct {
	wfRepo       *repository.WorkflowRepository
	subRepo      *repository.SubscriberRepository
	intgRepo     *repository.IntegrationRepository
	notifRepo    *repository.NotificationRepository
	activityRepo *repository.ActivityRepository
	prefRepo     *repository.PreferenceRepository
	rlRepo       *repository.RateLimitRepository
	factory      *provider.ProviderFactory
	asynq        *asynq.Client
	rdb          *redis.Client
	encKey       []byte
	rlConfig     map[string]config.RateLimitChannelConfig
}

func NewDeliveryHandler(
	wfRepo *repository.WorkflowRepository,
	subRepo *repository.SubscriberRepository,
	intgRepo *repository.IntegrationRepository,
	notifRepo *repository.NotificationRepository,
	activityRepo *repository.ActivityRepository,
	prefRepo *repository.PreferenceRepository,
	rlRepo *repository.RateLimitRepository,
	factory *provider.ProviderFactory,
	asynqClient *asynq.Client,
	rdb *redis.Client,
	encKey []byte,
	rlConfig map[string]config.RateLimitChannelConfig,
) *DeliveryHandler {
	return &DeliveryHandler{
		wfRepo:       wfRepo,
		subRepo:      subRepo,
		intgRepo:     intgRepo,
		notifRepo:    notifRepo,
		activityRepo: activityRepo,
		prefRepo:     prefRepo,
		rlRepo:       rlRepo,
		factory:      factory,
		asynq:        asynqClient,
		rdb:          rdb,
		encKey:       encKey,
		rlConfig:     rlConfig,
	}
}

func (h *DeliveryHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var payload DeliveryPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal delivery payload: %w", err)
	}

	envID, _ := bson.ObjectIDFromHex(payload.EnvironmentID)
	subID, _ := bson.ObjectIDFromHex(payload.SubscriberID)
	notifID, _ := bson.ObjectIDFromHex(payload.NotificationID)

	// Load subscriber
	subscriber, err := h.subRepo.FindByID(ctx, subID)
	if err != nil {
		return fmt.Errorf("find subscriber: %w", err)
	}

	// Load notification
	notif, err := h.notifRepo.FindByID(ctx, notifID, envID)
	if err != nil {
		return fmt.Errorf("find notification: %w", err)
	}

	// Load workflow for preferences
	wf, err := h.wfRepo.FindByID(ctx, notif.WorkflowID)
	if err != nil {
		return fmt.Errorf("find workflow: %w", err)
	}

	// Check rate limit
	if rlCfg, ok := h.rlConfig[payload.Channel]; ok {
		windowStart := time.Now().Truncate(time.Duration(rlCfg.WindowMinutes) * time.Minute)
		exceeded, err := h.rlRepo.IncrementAndCheck(ctx, envID, subID, payload.Channel, windowStart, time.Duration(rlCfg.WindowMinutes)*time.Minute, rlCfg.MaxPerWindow)
		if err != nil {
			log.Printf("rate limit check error: %v", err)
		}
		if exceeded {
			h.logActivity(ctx, envID, notifID, subID, payload.Channel, "rate_limited", nil)
			h.notifRepo.UpdateChannelStatus(ctx, notifID, payload.Channel, "skipped", bson.M{"errorMessage": "rate limited"})
			return nil
		}
	}

	// Check preferences
	workflowPref, _ := h.prefRepo.FindBySubscriberAndWorkflow(ctx, envID, subID, &wf.ID)
	globalPref, _ := h.prefRepo.FindBySubscriberAndWorkflow(ctx, envID, subID, nil)
	if !engine.IsChannelEnabled(payload.Channel, workflowPref, globalPref, wf.PreferenceDefaults) {
		h.logActivity(ctx, envID, notifID, subID, payload.Channel, "step_skipped", map[string]any{"reason": "channel disabled by preference"})
		h.notifRepo.UpdateChannelStatus(ctx, notifID, payload.Channel, "skipped", bson.M{"errorMessage": "disabled by preference"})
		return nil
	}

	// For in_app channel, publish to Redis pub/sub instead of using a provider
	if payload.Channel == "in_app" {
		return h.deliverInApp(ctx, envID, notifID, subID, notif, subscriber, wf, payload)
	}

	// Load integration
	intg, err := h.intgRepo.FindPrimaryByChannel(ctx, envID, payload.Channel)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			h.logActivity(ctx, envID, notifID, subID, payload.Channel, "provider_error", map[string]any{"error": "no integration configured"})
			h.notifRepo.UpdateChannelStatus(ctx, notifID, payload.Channel, "failed", bson.M{"errorMessage": "no integration configured"})
			return nil
		}
		return fmt.Errorf("find integration: %w", err)
	}

	// Create provider
	prov, err := h.factory.Create(intg, h.encKey)
	if err != nil {
		return fmt.Errorf("create provider: %w", err)
	}

	// Render template
	step := wf.Steps[payload.StepIndex]
	sendOpts := h.buildSendOptions(subscriber, step, payload.Payload, subscriber.Locale)

	h.logActivity(ctx, envID, notifID, subID, payload.Channel, "provider_request", map[string]any{"providerId": intg.ProviderID})

	// Send
	result, err := prov.Send(ctx, sendOpts)
	if err != nil {
		h.logActivity(ctx, envID, notifID, subID, payload.Channel, "provider_error", map[string]any{"error": err.Error()})

		// Retry logic
		if payload.Attempt < MaxRetries {
			payload.Attempt++
			backoff := time.Duration(BackoffBaseMs) * time.Millisecond
			for i := 0; i < payload.Attempt; i++ {
				backoff *= time.Duration(BackoffMultiplier)
			}
			data, _ := json.Marshal(payload)
			task := asynq.NewTask(TaskTypeDelivery, data)
			h.asynq.Enqueue(task, asynq.ProcessIn(backoff))
			h.logActivity(ctx, envID, notifID, subID, payload.Channel, "retry_scheduled", map[string]any{"attempt": payload.Attempt})
			return nil
		}

		now := time.Now()
		h.notifRepo.UpdateChannelStatus(ctx, notifID, payload.Channel, "failed", bson.M{
			"errorMessage": err.Error(),
			"retryCount":   payload.Attempt,
			"failedAt":     now,
		})
		return nil
	}

	// Success
	now := time.Now()
	h.notifRepo.UpdateChannelStatus(ctx, notifID, payload.Channel, "sent", bson.M{
		"providerMessageId": result.ProviderMessageID,
		"providerId":        intg.ProviderID,
		"sentAt":            now,
	})
	h.logActivity(ctx, envID, notifID, subID, payload.Channel, "provider_success", map[string]any{
		"providerMessageId": result.ProviderMessageID,
	})

	return nil
}

func (h *DeliveryHandler) deliverInApp(ctx context.Context, envID, notifID, subID bson.ObjectID, notif *model.Notification, sub *model.Subscriber, wf *model.Workflow, payload DeliveryPayload) error {
	// Render content
	step := wf.Steps[payload.StepIndex]
	data := engine.TemplateData{
		Subscriber: engine.TemplateSubscriber{
			FirstName: sub.FirstName,
			LastName:  sub.LastName,
			Email:     sub.Email,
		},
		Payload: payload.Payload,
	}

	var content string
	if step.Template != nil && step.Template.Content != nil {
		tmplStr := engine.ResolveLocale(step.Template.Content, sub.Locale, "en")
		rendered, err := engine.RenderTemplate(tmplStr, data)
		if err != nil {
			log.Printf("template render error: %v", err)
			content = tmplStr
		} else {
			content = rendered
		}
	}

	// Update notification status
	now := time.Now()
	h.notifRepo.UpdateChannelStatus(ctx, notifID, "in_app", "sent", bson.M{"sentAt": now})

	// Publish to Redis for WS delivery
	wsMsg, _ := json.Marshal(map[string]any{
		"room":  fmt.Sprintf("env:%s:sub:%s", envID.Hex(), subID.Hex()),
		"event": "notification:new",
		"data": map[string]any{
			"id":        notifID.Hex(),
			"content":   content,
			"payload":   payload.Payload,
			"createdAt": notif.CreatedAt,
		},
	})
	h.rdb.Publish(ctx, "ws:notifications", wsMsg)

	h.logActivity(ctx, envID, notifID, subID, "in_app", "provider_success", nil)
	return nil
}

func (h *DeliveryHandler) buildSendOptions(sub *model.Subscriber, step model.WorkflowStep, payload map[string]any, locale string) provider.SendOptions {
	data := engine.TemplateData{
		Subscriber: engine.TemplateSubscriber{
			FirstName: sub.FirstName,
			LastName:  sub.LastName,
			Email:     sub.Email,
		},
		Payload: payload,
	}

	opts := provider.SendOptions{
		Metadata: make(map[string]any),
	}

	if step.Template != nil {
		if step.Template.Subject != nil {
			tmplStr := engine.ResolveLocale(step.Template.Subject, locale, "en")
			rendered, err := engine.RenderTemplate(tmplStr, data)
			if err == nil {
				opts.Subject = rendered
			} else {
				opts.Subject = tmplStr
			}
		}
		if step.Template.Body != nil {
			tmplStr := engine.ResolveLocale(step.Template.Body, locale, "en")
			rendered, err := engine.RenderTemplate(tmplStr, data)
			if err == nil {
				opts.Content = rendered
			} else {
				opts.Content = tmplStr
			}
		}
		if step.Template.Content != nil {
			tmplStr := engine.ResolveLocale(step.Template.Content, locale, "en")
			rendered, err := engine.RenderTemplate(tmplStr, data)
			if err == nil {
				opts.Content = rendered
			} else {
				opts.Content = tmplStr
			}
		}
	}

	// Resolve "to" based on channel
	switch step.Type {
	case "email":
		opts.To = sub.Email
	case "sms":
		opts.To = sub.Phone
	case "push":
		// Use first FCM token
		if len(sub.Channels.Push.FCMTokens) > 0 {
			opts.To = sub.Channels.Push.FCMTokens[0]
		}
	case "slack":
		opts.To = sub.Channels.Slack.WebhookURL
	case "ms_teams":
		opts.To = sub.Channels.MSTeams.WebhookURL
	}

	return opts
}

func (h *DeliveryHandler) logActivity(ctx context.Context, envID, notifID, subID bson.ObjectID, channel, event string, detail map[string]any) {
	h.activityRepo.Create(ctx, &model.ActivityLog{
		EnvironmentID:  envID,
		NotificationID: notifID,
		SubscriberID:   subID,
		Channel:        channel,
		Event:          event,
		Detail:         detail,
		ExpireAt:       time.Now().Add(30 * 24 * time.Hour),
	})
}
