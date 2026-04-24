package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/partiri-cloud/message-in-a-bottle/internal/engine"
	"github.com/partiri-cloud/message-in-a-bottle/internal/repository"
	"go.mongodb.org/mongo-driver/v2/bson"
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
		return fmt.Errorf("template not found: %w", err)
	}

	if !tmpl.IsActive {
		return fmt.Errorf("template %s is inactive", identifier)
	}

	sub, err := s.subRepo.FindBySubscriberID(ctx, envID, subscriberID)
	if err != nil {
		return fmt.Errorf("subscriber not found: %w", err)
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
		return fmt.Errorf("render subject: %w", err)
	}

	body, err := engine.RenderTemplate(bodyStr, data)
	if err != nil {
		return fmt.Errorf("render body: %w", err)
	}

	// Enqueue delivery task directly (bypass workflow engine)
	dp := map[string]any{
		"environmentId":  envID.Hex(),
		"notificationId": "", // no parent notification for transactional
		"subscriberId":   sub.ID.Hex(),
		"channel":        tmpl.Channel,
		"stepIndex":      0,
		"payload": map[string]any{
			"__rendered_subject": subject,
			"__rendered_body":    body,
			"__transactional":    true,
		},
		"attempt": 0,
	}

	taskData, _ := json.Marshal(dp)
	task := asynq.NewTask("task:delivery", taskData)
	_, err = s.asynq.Enqueue(task)
	return err
}
