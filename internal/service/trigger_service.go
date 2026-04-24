package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/partiri-cloud/message-in-a-bottle/internal/handler/dto"
	"github.com/partiri-cloud/message-in-a-bottle/internal/model"
	"github.com/partiri-cloud/message-in-a-bottle/internal/repository"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

var (
	ErrDuplicateTransaction = errors.New("duplicate transactionId")
	ErrWorkflowNotFound     = errors.New("workflow not found")
)

const (
	TaskTypeTrigger   = "task:trigger"
	TaskTypeBroadcast = "task:broadcast"
)

type TriggerPayload struct {
	EnvironmentID string            `json:"environmentId"`
	WorkflowID    string            `json:"workflowId"`
	SubscriberIDs map[string]string `json:"subscriberIds"` // subscriberID hex → notificationID hex
	Payload       map[string]any    `json:"payload"`
	TransactionID string            `json:"transactionId"`
	Overrides     map[string]any    `json:"overrides,omitempty"`
}

type BroadcastTaskPayload struct {
	EnvironmentID      string         `json:"environmentId"`
	WorkflowIdentifier string         `json:"workflowIdentifier"`
	Payload            map[string]any `json:"payload"`
	TransactionID      string         `json:"transactionId"`
	Overrides          map[string]any `json:"overrides,omitempty"`
	RetentionDays      int            `json:"retentionDays"`
}

type TriggerResult struct {
	TransactionID   string   `json:"transactionId"`
	NotificationIDs []string `json:"notificationIds"`
}

type TriggerService struct {
	wfRepo    *repository.WorkflowRepository
	subRepo   *repository.SubscriberRepository
	tsRepo    *repository.TopicSubscriberRepository
	topicRepo *repository.TopicRepository
	notifRepo *repository.NotificationRepository
	asynq     *asynq.Client
	retention int
}

func NewTriggerService(
	wfRepo *repository.WorkflowRepository,
	subRepo *repository.SubscriberRepository,
	tsRepo *repository.TopicSubscriberRepository,
	topicRepo *repository.TopicRepository,
	notifRepo *repository.NotificationRepository,
	asynqClient *asynq.Client,
	retentionDays int,
) *TriggerService {
	return &TriggerService{
		wfRepo:    wfRepo,
		subRepo:   subRepo,
		tsRepo:    tsRepo,
		topicRepo: topicRepo,
		notifRepo: notifRepo,
		asynq:     asynqClient,
		retention: retentionDays,
	}
}

func (s *TriggerService) Trigger(ctx context.Context, envID bson.ObjectID, req *dto.TriggerRequest) (*TriggerResult, error) {
	// Resolve workflow
	wf, err := s.wfRepo.FindByIdentifier(ctx, envID, req.WorkflowIdentifier)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrWorkflowNotFound
		}
		return nil, err
	}

	// Transaction ID
	txID := req.TransactionID
	if txID == "" {
		txID = uuid.New().String()
	}

	// Resolve subscriber IDs
	subscriberIDs, err := s.resolveRecipients(ctx, envID, req.To)
	if err != nil {
		return nil, err
	}

	// Create notifications and enqueue. The unique index on {environmentId, transactionId, subscriberId}
	// is the authoritative idempotency guard — no separate pre-check needed.
	var notifIDs []string
	notifIDMap := make(map[string]string, len(subscriberIDs))
	for _, subID := range subscriberIDs {
		channels := make([]model.ChannelDelivery, 0)
		for _, step := range wf.Steps {
			if step.Type == "delay" || step.Type == "digest" {
				continue
			}
			channels = append(channels, model.ChannelDelivery{
				Channel: step.Type,
				Status:  "pending",
			})
		}

		notif := &model.Notification{
			EnvironmentID: envID,
			SubscriberID:  subID,
			WorkflowID:    wf.ID,
			TransactionID: txID,
			Payload:       req.Payload,
			Channels:      channels,
			ExpireAt:      time.Now().Add(time.Duration(s.retention) * 24 * time.Hour),
		}

		if err := s.notifRepo.Create(ctx, notif); err != nil {
			if mongo.IsDuplicateKeyError(err) {
				return nil, ErrDuplicateTransaction
			}
			return nil, err
		}

		notifIDs = append(notifIDs, notif.ID.Hex())
		notifIDMap[subID.Hex()] = notif.ID.Hex()
	}

	// Enqueue trigger task
	payload := TriggerPayload{
		EnvironmentID: envID.Hex(),
		WorkflowID:    wf.ID.Hex(),
		SubscriberIDs: notifIDMap,
		Payload:       req.Payload,
		TransactionID: txID,
		Overrides:     req.Overrides,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal trigger payload: %w", err)
	}
	task := asynq.NewTask(TaskTypeTrigger, data)
	if _, err := s.asynq.Enqueue(task); err != nil {
		return nil, err
	}

	return &TriggerResult{
		TransactionID:   txID,
		NotificationIDs: notifIDs,
	}, nil
}

func (s *TriggerService) Broadcast(ctx context.Context, envID bson.ObjectID, req *dto.BroadcastRequest) (*TriggerResult, error) {
	txID := req.TransactionID
	if txID == "" {
		txID = uuid.New().String()
	}

	bp := BroadcastTaskPayload{
		EnvironmentID:      envID.Hex(),
		WorkflowIdentifier: req.WorkflowIdentifier,
		Payload:            req.Payload,
		TransactionID:      txID,
		Overrides:          req.Overrides,
		RetentionDays:      s.retention,
	}
	data, err := json.Marshal(bp)
	if err != nil {
		return nil, fmt.Errorf("marshal broadcast payload: %w", err)
	}
	task := asynq.NewTask(TaskTypeBroadcast, data)
	if _, err := s.asynq.Enqueue(task); err != nil {
		return nil, err
	}

	return &TriggerResult{TransactionID: txID}, nil
}

func (s *TriggerService) resolveRecipients(ctx context.Context, envID bson.ObjectID, to []dto.TriggerTo) ([]bson.ObjectID, error) {
	seen := make(map[bson.ObjectID]bool)
	var result []bson.ObjectID

	for _, target := range to {
		if target.Type == "Topic" && target.TopicKey != "" {
			topic, err := s.topicRepo.FindByKey(ctx, envID, target.TopicKey)
			if err != nil {
				if err == mongo.ErrNoDocuments {
					continue
				}
				return nil, err
			}
			subIDs, err := s.tsRepo.FindSubscriberIDsByTopic(ctx, topic.ID)
			if err != nil {
				return nil, err
			}
			for _, id := range subIDs {
				if !seen[id] {
					seen[id] = true
					result = append(result, id)
				}
			}
		} else if target.SubscriberID != "" {
			sub, err := s.subRepo.FindBySubscriberID(ctx, envID, target.SubscriberID)
			if err != nil {
				if err == mongo.ErrNoDocuments {
					continue
				}
				return nil, err
			}
			if !seen[sub.ID] {
				seen[sub.ID] = true
				result = append(result, sub.ID)
			}
		}
	}

	return result, nil
}
