package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/hibiken/asynq"
	"github.com/partiri-cloud/message-in-a-bottle/internal/config"
	"github.com/partiri-cloud/message-in-a-bottle/internal/engine"
	"github.com/partiri-cloud/message-in-a-bottle/internal/model"
	"github.com/partiri-cloud/message-in-a-bottle/internal/provider"
	"github.com/partiri-cloud/message-in-a-bottle/internal/repository"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type DeliveryHandler struct {
	wfRepo        *repository.WorkflowRepository
	subRepo       *repository.SubscriberRepository
	intgRepo      *repository.IntegrationRepository
	notifRepo     *repository.NotificationRepository
	activityRepo  *repository.ActivityRepository
	prefRepo      *repository.PreferenceRepository
	rlRepo        *repository.RateLimitRepository
	factory       *provider.ProviderFactory
	asynq         *asynq.Client
	rdb           *redis.Client
	encKey        []byte
	rlConfig      map[string]config.RateLimitChannelConfig
	retentionDays int
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
	retentionDays int,
) *DeliveryHandler {
	return &DeliveryHandler{
		wfRepo:        wfRepo,
		subRepo:       subRepo,
		intgRepo:      intgRepo,
		notifRepo:     notifRepo,
		activityRepo:  activityRepo,
		prefRepo:      prefRepo,
		rlRepo:        rlRepo,
		factory:       factory,
		asynq:         asynqClient,
		rdb:           rdb,
		encKey:        encKey,
		rlConfig:      rlConfig,
		retentionDays: retentionDays,
	}
}

func (h *DeliveryHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var payload DeliveryPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal delivery payload: %w", err)
	}

	envID, err := bson.ObjectIDFromHex(payload.EnvironmentID)
	if err != nil {
		return fmt.Errorf("invalid environmentId %q: %w", payload.EnvironmentID, err)
	}
	subID, err := bson.ObjectIDFromHex(payload.SubscriberID)
	if err != nil {
		return fmt.Errorf("invalid subscriberId %q: %w", payload.SubscriberID, err)
	}

	// Load subscriber
	subscriber, err := h.subRepo.FindByID(ctx, subID)
	if err != nil {
		return fmt.Errorf("find subscriber: %w", err)
	}

	// Transactional (template) sends carry pre-rendered content and have no
	// notification or workflow record to load — route them before those lookups.
	if isTransactional(payload) {
		return h.processTransactional(ctx, envID, subID, subscriber, payload)
	}

	notifID, err := bson.ObjectIDFromHex(payload.NotificationID)
	if err != nil {
		return fmt.Errorf("invalid notificationId %q: %w", payload.NotificationID, err)
	}

	// Load notification
	notif, err := h.notifRepo.FindByID(ctx, envID, notifID)
	if err != nil {
		return fmt.Errorf("find notification: %w", err)
	}

	// Load workflow for preferences
	wf, err := h.wfRepo.FindByID(ctx, envID, notif.WorkflowID)
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

	// Push requires special handling: one or two providers (FCM + APNS), multiple tokens each
	if payload.Channel == "push" {
		return h.deliverPush(ctx, envID, notifID, subID, subscriber, wf, payload)
	}

	// All other channels: single primary integration
	intg, err := h.intgRepo.FindPrimaryByChannel(ctx, envID, payload.Channel)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			h.logActivity(ctx, envID, notifID, subID, payload.Channel, "provider_error", map[string]any{"error": "no integration configured"})
			h.notifRepo.UpdateChannelStatus(ctx, notifID, payload.Channel, "failed", bson.M{"errorMessage": "no integration configured"})
			return nil
		}
		return fmt.Errorf("find integration: %w", err)
	}

	prov, err := h.factory.Create(intg, h.encKey)
	if err != nil {
		return fmt.Errorf("create provider: %w", err)
	}

	step, err := stepAt(wf, payload.StepIndex)
	if err != nil {
		log.Printf("step index error: %v", err)
		return err
	}
	sendOpts := h.buildSendOptions(subscriber, step, payload.Payload, subscriber.Locale)

	h.logActivity(ctx, envID, notifID, subID, payload.Channel, "provider_request", map[string]any{"providerId": intg.ProviderID})

	result, err := prov.Send(ctx, sendOpts)
	if err != nil {
		h.logActivity(ctx, envID, notifID, subID, payload.Channel, "provider_error", map[string]any{"error": err.Error()})

		if h.scheduleRetry(&payload) {
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

// isTransactional reports whether a delivery payload originated from a direct
// template send (TemplateService.Send) rather than a workflow-triggered
// notification. The discriminator lives on a dedicated top-level field set only
// by TemplateService.Send — never on payload.Payload, which for the workflow
// path is the tenant's raw trigger event data and must not be trusted to gate
// this routing decision (a trigger payload containing "__transactional": true
// must never be treated as transactional).
func isTransactional(payload DeliveryPayload) bool {
	return payload.Transactional
}

// recipientForChannel maps a subscriber's contact info to the destination address
// used for transactional (template) sends. Channels with no subscriber-addressable
// recipient are not supported for transactional delivery.
func recipientForChannel(sub *model.Subscriber, channel string) (string, error) {
	switch channel {
	case "email":
		if sub.Email == "" {
			return "", fmt.Errorf("subscriber has no email address configured")
		}
		return sub.Email, nil
	case "sms":
		if sub.Phone == "" {
			return "", fmt.Errorf("subscriber has no phone number configured")
		}
		return sub.Phone, nil
	case "slack":
		if sub.Channels.Slack.WebhookURL == "" {
			return "", fmt.Errorf("subscriber has no slack webhook configured")
		}
		return sub.Channels.Slack.WebhookURL, nil
	case "ms_teams":
		if sub.Channels.MSTeams.WebhookURL == "" {
			return "", fmt.Errorf("subscriber has no ms_teams webhook configured")
		}
		return sub.Channels.MSTeams.WebhookURL, nil
	default:
		return "", fmt.Errorf("unsupported transactional channel %q", channel)
	}
}

// buildTransactionalSendOptions builds provider send options from the pre-rendered
// subject/body a transactional delivery payload carries (no template step to
// render against).
func buildTransactionalSendOptions(to, renderedSubject, renderedBody string) provider.SendOptions {
	return provider.SendOptions{
		To:       to,
		Subject:  renderedSubject,
		Content:  renderedBody,
		Metadata: make(map[string]any),
	}
}

// skipRetryError wraps err so asynq dead-letters the task instead of retrying —
// used for permanent transactional-delivery failures that have no notification
// document to record a failed status against.
func skipRetryError(err error) error {
	return fmt.Errorf("%w: %w", err, asynq.SkipRetry)
}

// stepAt safely resolves wf.Steps[idx], guarding against a payload.StepIndex
// that has gone stale relative to the current workflow (e.g. a delay step
// holds a task for days while the workflow is edited to fewer steps). An
// out-of-range index is a permanent condition — retrying the same payload
// will never make the index valid again — so the error dead-letters the task
// via asynq.SkipRetry instead of looping through retries.
func stepAt(wf *model.Workflow, idx int) (model.WorkflowStep, error) {
	if idx < 0 || idx >= len(wf.Steps) {
		return model.WorkflowStep{}, skipRetryError(fmt.Errorf("step index %d out of range for workflow %s (%d steps)", idx, wf.ID.Hex(), len(wf.Steps)))
	}
	return wf.Steps[idx], nil
}

// nextRetryDelay reports whether another retry attempt is available for the given
// current attempt count and, if so, the incremented attempt number and the backoff
// duration to schedule it after. Pulled out of scheduleRetry so the attempt/backoff
// arithmetic is unit-testable without a live asynq client.
func nextRetryDelay(attempt int) (nextAttempt int, backoff time.Duration, ok bool) {
	if attempt >= MaxRetries {
		return attempt, 0, false
	}
	nextAttempt = attempt + 1
	backoff = time.Duration(BackoffBaseMs) * time.Millisecond
	for i := 0; i < nextAttempt; i++ {
		backoff *= time.Duration(BackoffMultiplier)
	}
	return nextAttempt, backoff, true
}

// scheduleRetry re-enqueues the payload with backoff if attempts remain, mutating
// payload.Attempt in place. It reports whether a retry was scheduled: false means
// either attempts are exhausted or the retry payload could not be marshaled, and
// callers must fall back to recording a permanent failure (workflow path: mark the
// channel status "failed"; transactional path: dead-letter via asynq.SkipRetry).
//
// This is a deliberate change from the previous inline retry logic, which on a
// marshal error returned nil straight out of ProcessTask — silently acking the
// task with no failure recorded and no retry. Marshal failures here are expected
// to be effectively unreachable (DeliveryPayload only holds JSON-safe types), but
// if one ever occurs it should surface as a recorded failure rather than vanish.
func (h *DeliveryHandler) scheduleRetry(payload *DeliveryPayload) bool {
	nextAttempt, backoff, ok := nextRetryDelay(payload.Attempt)
	if !ok {
		return false
	}
	payload.Attempt = nextAttempt

	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("marshal retry payload: %v", err)
		return false
	}
	task := asynq.NewTask(TaskTypeDelivery, data)
	h.asynq.Enqueue(task, asynq.ProcessIn(backoff))
	return true
}

// processTransactional delivers a template send directly: no Notification or
// Workflow document exists, so it resolves the primary integration for the
// channel, builds SendOptions from the pre-rendered payload, and sends. Unlike
// the workflow path there is no notification document to record status on, so
// permanent failures dead-letter the task via asynq.SkipRetry instead of being
// written to a status field.
func (h *DeliveryHandler) processTransactional(ctx context.Context, envID, subID bson.ObjectID, sub *model.Subscriber, payload DeliveryPayload) error {
	to, err := recipientForChannel(sub, payload.Channel)
	if err != nil {
		h.logActivity(ctx, envID, bson.NilObjectID, subID, payload.Channel, "provider_error", map[string]any{"error": err.Error()})
		return skipRetryError(err)
	}

	intg, err := h.intgRepo.FindPrimaryByChannel(ctx, envID, payload.Channel)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			h.logActivity(ctx, envID, bson.NilObjectID, subID, payload.Channel, "provider_error", map[string]any{"error": "no integration configured"})
			return skipRetryError(fmt.Errorf("no integration configured for channel %q", payload.Channel))
		}
		return fmt.Errorf("find integration: %w", err)
	}

	prov, err := h.factory.Create(intg, h.encKey)
	if err != nil {
		return fmt.Errorf("create provider: %w", err)
	}

	sendOpts := buildTransactionalSendOptions(to, payload.RenderedSubject, payload.RenderedBody)

	h.logActivity(ctx, envID, bson.NilObjectID, subID, payload.Channel, "provider_request", map[string]any{"providerId": intg.ProviderID})

	result, err := prov.Send(ctx, sendOpts)
	if err != nil {
		h.logActivity(ctx, envID, bson.NilObjectID, subID, payload.Channel, "provider_error", map[string]any{"error": err.Error()})

		if h.scheduleRetry(&payload) {
			h.logActivity(ctx, envID, bson.NilObjectID, subID, payload.Channel, "retry_scheduled", map[string]any{"attempt": payload.Attempt})
			return nil
		}

		return skipRetryError(fmt.Errorf("transactional send failed after %d attempts: %w", payload.Attempt, err))
	}

	h.logActivity(ctx, envID, bson.NilObjectID, subID, payload.Channel, "provider_success", map[string]any{
		"providerMessageId": result.ProviderMessageID,
	})
	return nil
}

// deliverPush sends to all FCM and APNS tokens registered on the subscriber.
// It loads push integrations by provider ID so both can coexist in the same environment.
func (h *DeliveryHandler) deliverPush(ctx context.Context, envID, notifID, subID bson.ObjectID, sub *model.Subscriber, wf *model.Workflow, payload DeliveryPayload) error {
	integrations, err := h.intgRepo.FindAllActiveByChannel(ctx, envID, "push")
	if err != nil {
		return fmt.Errorf("find push integrations: %w", err)
	}

	var fcmIntg, apnsIntg *model.Integration
	for i := range integrations {
		switch integrations[i].ProviderID {
		case "fcm":
			fcmIntg = &integrations[i]
		case "apns":
			apnsIntg = &integrations[i]
		}
	}

	if fcmIntg == nil && apnsIntg == nil {
		h.logActivity(ctx, envID, notifID, subID, "push", "provider_error", map[string]any{"error": "no push integration configured"})
		h.notifRepo.UpdateChannelStatus(ctx, notifID, "push", "failed", bson.M{"errorMessage": "no push integration configured"})
		return nil
	}

	step, err := stepAt(wf, payload.StepIndex)
	if err != nil {
		log.Printf("step index error: %v", err)
		return err
	}
	baseOpts := h.buildPushOptions(sub, step, payload.Payload, sub.Locale)

	anySent := false

	// Send to FCM tokens
	if fcmIntg != nil && len(sub.Channels.Push.FCMTokens) > 0 {
		fcmProv, err := h.factory.Create(fcmIntg, h.encKey)
		if err != nil {
			log.Printf("create FCM provider: %v", err)
		} else {
			for _, token := range sub.Channels.Push.FCMTokens {
				opts := baseOpts
				opts.To = token
				h.logActivity(ctx, envID, notifID, subID, "push", "provider_request", map[string]any{"providerId": "fcm", "token": token})
				result, err := fcmProv.Send(ctx, opts)
				if err != nil {
					h.logActivity(ctx, envID, notifID, subID, "push", "provider_error", map[string]any{"providerId": "fcm", "token": token, "error": err.Error()})
				} else {
					h.logActivity(ctx, envID, notifID, subID, "push", "provider_success", map[string]any{"providerId": "fcm", "providerMessageId": result.ProviderMessageID})
					anySent = true
				}
			}
		}
	}

	// Send to APNS tokens
	if apnsIntg != nil && len(sub.Channels.Push.APNSTokens) > 0 {
		apnsProv, err := h.factory.Create(apnsIntg, h.encKey)
		if err != nil {
			log.Printf("create APNS provider: %v", err)
		} else {
			for _, token := range sub.Channels.Push.APNSTokens {
				opts := baseOpts
				opts.To = token
				h.logActivity(ctx, envID, notifID, subID, "push", "provider_request", map[string]any{"providerId": "apns", "token": token})
				result, err := apnsProv.Send(ctx, opts)
				if err != nil {
					h.logActivity(ctx, envID, notifID, subID, "push", "provider_error", map[string]any{"providerId": "apns", "token": token, "error": err.Error()})
				} else {
					h.logActivity(ctx, envID, notifID, subID, "push", "provider_success", map[string]any{"providerId": "apns", "providerMessageId": result.ProviderMessageID})
					anySent = true
				}
			}
		}
	}

	now := time.Now()
	if anySent {
		h.notifRepo.UpdateChannelStatus(ctx, notifID, "push", "sent", bson.M{"sentAt": now})
	} else {
		h.notifRepo.UpdateChannelStatus(ctx, notifID, "push", "failed", bson.M{"errorMessage": "all push deliveries failed", "failedAt": now})
	}
	return nil
}

func (h *DeliveryHandler) deliverInApp(ctx context.Context, envID, notifID, subID bson.ObjectID, notif *model.Notification, sub *model.Subscriber, wf *model.Workflow, payload DeliveryPayload) error {
	step, err := stepAt(wf, payload.StepIndex)
	if err != nil {
		log.Printf("step index error: %v", err)
		return err
	}
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

	now := time.Now()
	h.notifRepo.UpdateChannelStatus(ctx, notifID, "in_app", "sent", bson.M{"sentAt": now})

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

// buildPushOptions renders the template fields for push without setting To (set per-token by the caller).
func (h *DeliveryHandler) buildPushOptions(sub *model.Subscriber, step model.WorkflowStep, payload map[string]any, locale string) provider.SendOptions {
	data := engine.TemplateData{
		Subscriber: engine.TemplateSubscriber{
			FirstName: sub.FirstName,
			LastName:  sub.LastName,
			Email:     sub.Email,
		},
		Payload: payload,
	}
	opts := provider.SendOptions{Metadata: make(map[string]any)}
	if step.Template != nil {
		if step.Template.Subject != nil {
			tmplStr := engine.ResolveLocale(step.Template.Subject, locale, "en")
			if rendered, err := engine.RenderTemplate(tmplStr, data); err == nil {
				opts.Subject = rendered
			} else {
				opts.Subject = tmplStr
			}
		}
		if step.Template.Body != nil {
			tmplStr := engine.ResolveLocale(step.Template.Body, locale, "en")
			if rendered, err := engine.RenderTemplate(tmplStr, data); err == nil {
				opts.Content = rendered
			} else {
				opts.Content = tmplStr
			}
		}
	}
	return opts
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

	// Email uses HTML-safe rendering; all other channels use plain text.
	render := engine.RenderTemplate
	if step.Type == "email" {
		render = engine.RenderHTMLTemplate
	}

	opts := provider.SendOptions{
		Metadata: make(map[string]any),
	}

	if step.Template != nil {
		if step.Template.Subject != nil {
			tmplStr := engine.ResolveLocale(step.Template.Subject, locale, "en")
			rendered, err := render(tmplStr, data)
			if err == nil {
				opts.Subject = rendered
			} else {
				opts.Subject = tmplStr
			}
		}
		if step.Template.Body != nil {
			tmplStr := engine.ResolveLocale(step.Template.Body, locale, "en")
			rendered, err := render(tmplStr, data)
			if err == nil {
				opts.Content = rendered
			} else {
				opts.Content = tmplStr
			}
		}
		if step.Template.Content != nil {
			tmplStr := engine.ResolveLocale(step.Template.Content, locale, "en")
			rendered, err := render(tmplStr, data)
			if err == nil {
				opts.Content = rendered
			} else {
				opts.Content = tmplStr
			}
		}
	}

	switch step.Type {
	case "email":
		opts.To = sub.Email
	case "sms":
		opts.To = sub.Phone
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
		ExpireAt:       time.Now().Add(time.Duration(h.retentionDays) * 24 * time.Hour),
	})
}
