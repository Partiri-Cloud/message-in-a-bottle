package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/partiri-cloud/message-in-a-box/internal/handler/dto"
	"github.com/partiri-cloud/message-in-a-box/internal/model"
	"github.com/partiri-cloud/message-in-a-box/internal/repository"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

var (
	ErrDuplicateTransaction = errors.New("duplicate transactionId")
	ErrWorkflowNotFound     = errors.New("workflow not found")
)

const TaskTypeTrigger = "task:trigger"

type TriggerPayload struct {
	EnvironmentID string         `json:"environmentId"`
	WorkflowID    string         `json:"workflowId"`
	SubscriberIDs []string       `json:"subscriberIds"`
	Payload       map[string]any `json:"payload"`
	TransactionID string         `json:"transactionId"`
	Overrides     map[string]any `json:"overrides,omitempty"`
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

	// Check idempotency
	_, err = s.notifRepo.FindByTransactionID(ctx, envID, txID)
	if err == nil {
		return nil, ErrDuplicateTransaction
	}
	if err != mongo.ErrNoDocuments {
		return nil, err
	}

	// Resolve subscriber IDs
	subscriberIDs, err := s.resolveRecipients(ctx, envID, req.To)
	if err != nil {
		return nil, err
	}

	// Create notifications and enqueue
	var notifIDs []string
	var allSubIDStrings []string
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
		allSubIDStrings = append(allSubIDStrings, subID.Hex())
	}

	// Enqueue trigger task
	payload := TriggerPayload{
		EnvironmentID: envID.Hex(),
		WorkflowID:    wf.ID.Hex(),
		SubscriberIDs: allSubIDStrings,
		Payload:       req.Payload,
		TransactionID: txID,
		Overrides:     req.Overrides,
	}

	data, _ := json.Marshal(payload)
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
	triggerReq := &dto.TriggerRequest{
		WorkflowIdentifier: req.WorkflowIdentifier,
		Payload:            req.Payload,
		TransactionID:      req.TransactionID,
		Overrides:          req.Overrides,
	}

	// Get all subscribers (paginated fetch all)
	page := 1
	var allSubIDs []dto.TriggerTo
	for {
		subs, _, err := s.subRepo.FindMany(ctx, envID, page, 100)
		if err != nil {
			return nil, err
		}
		if len(subs) == 0 {
			break
		}
		for _, sub := range subs {
			allSubIDs = append(allSubIDs, dto.TriggerTo{SubscriberID: sub.SubscriberID})
		}
		page++
	}

	triggerReq.To = allSubIDs
	return s.Trigger(ctx, envID, triggerReq)
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
