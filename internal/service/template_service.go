package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/partiri-cloud/message-in-a-bottle/internal/engine"
	"github.com/partiri-cloud/message-in-a-bottle/internal/repository"
	"github.com/partiri-cloud/message-in-a-bottle/internal/worker"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// Sentinel errors Send returns for conditions the caller caused, so the
// handler can map them to typed API errors instead of a generic 500 that
// echoes internal detail.
var (
	ErrTemplateNotFound   = errors.New("template not found")
	ErrTemplateInactive   = errors.New("template is inactive")
	ErrSubscriberNotFound = errors.New("subscriber not found")
	ErrTemplateRender     = errors.New("template failed to render")
)

type TemplateService struct {
	tmplRepo *repository.TemplateRepository
	subRepo  *repository.SubscriberRepository
	asynq    *asynq.Client
}

func NewTemplateService(tmplRepo *repository.TemplateRepository, subRepo *repository.SubscriberRepository, asynqClient *asynq.Client) *TemplateService {
	return &TemplateService{tmplRepo: tmplRepo, subRepo: subRepo, asynq: asynqClient}
}

func (s *TemplateService) Send(ctx context.Context, envID bson.ObjectID, identifier string, subscriberID string, payload map[string]any, locale string) error {
	tmpl, err := s.tmplRepo.FindByIdentifier(ctx, envID, identifier)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return ErrTemplateNotFound
		}
		return fmt.Errorf("find template: %w", err)
	}

	if !tmpl.IsActive {
		return ErrTemplateInactive
	}

	sub, err := s.subRepo.FindBySubscriberID(ctx, envID, subscriberID)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return ErrSubscriberNotFound
		}
		return fmt.Errorf("find subscriber: %w", err)
	}

	// Resolve locale: request → subscriber → template default
	effectiveLocale := locale
	if effectiveLocale == "" {
		effectiveLocale = sub.Locale
	}
	if effectiveLocale == "" {
		effectiveLocale = tmpl.DefaultLocale
	}

	// Resolve template strings
	subjectStr := engine.ResolveLocale(tmpl.Subject, effectiveLocale, tmpl.DefaultLocale)
	bodyStr := engine.ResolveLocale(tmpl.Body, effectiveLocale, tmpl.DefaultLocale)

	data := engine.TemplateData{
		Subscriber: engine.TemplateSubscriber{
			FirstName: sub.FirstName,
			LastName:  sub.LastName,
			Email:     sub.Email,
		},
		Payload: payload,
	}

	subject, err := engine.RenderTemplate(subjectStr, data)
	if err != nil {
		return fmt.Errorf("%w: subject: %v", ErrTemplateRender, err)
	}

	body, err := engine.RenderTemplate(bodyStr, data)
	if err != nil {
		return fmt.Errorf("%w: body: %v", ErrTemplateRender, err)
	}

	// Enqueue delivery task directly (bypass workflow engine). Transactional,
	// RenderedSubject, and RenderedBody are dedicated DeliveryPayload fields, kept
	// out of Payload (the caller-supplied template variables) so a workflow trigger
	// payload can never spoof the transactional discriminator or inject content.
	dp := worker.DeliveryPayload{
		EnvironmentID:   envID.Hex(),
		NotificationID:  "", // no parent notification for transactional
		SubscriberID:    sub.ID.Hex(),
		Channel:         tmpl.Channel,
		StepIndex:       0,
		Payload:         payload,
		Attempt:         0,
		Transactional:   true,
		RenderedSubject: subject,
		RenderedBody:    body,
	}

	taskData, _ := json.Marshal(dp)
	task := asynq.NewTask(worker.TaskTypeDelivery, taskData)
	_, err = s.asynq.Enqueue(task)
	return err
}
