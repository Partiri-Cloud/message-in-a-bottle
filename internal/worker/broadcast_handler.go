package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/hibiken/asynq"
	"github.com/partiri-cloud/message-in-a-bottle/internal/model"
	"github.com/partiri-cloud/message-in-a-bottle/internal/repository"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type BroadcastHandler struct {
	wfRepo    *repository.WorkflowRepository
	subRepo   *repository.SubscriberRepository
	notifRepo *repository.NotificationRepository
	asynq     *asynq.Client
}

func NewBroadcastHandler(
	wfRepo *repository.WorkflowRepository,
	subRepo *repository.SubscriberRepository,
	notifRepo *repository.NotificationRepository,
	asynqClient *asynq.Client,
) *BroadcastHandler {
	return &BroadcastHandler{
		wfRepo:    wfRepo,
		subRepo:   subRepo,
		notifRepo: notifRepo,
		asynq:     asynqClient,
	}
}

func (h *BroadcastHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var payload BroadcastTaskPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal broadcast payload: %w", err)
	}

	envID, err := bson.ObjectIDFromHex(payload.EnvironmentID)
	if err != nil {
		return fmt.Errorf("invalid environmentId %q: %w", payload.EnvironmentID, err)
	}

	wf, err := h.wfRepo.FindByIdentifier(ctx, envID, payload.WorkflowIdentifier)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			log.Printf("broadcast: workflow %q not found, skipping", payload.WorkflowIdentifier)
			return nil
		}
		return fmt.Errorf("find workflow: %w", err)
	}

	if !wf.IsActive {
		log.Printf("broadcast: workflow %s is inactive, skipping", wf.Identifier)
		return nil
	}

	expireAt := time.Now().Add(time.Duration(payload.RetentionDays) * 24 * time.Hour)

	page := 1
	for {
		subs, _, err := h.subRepo.FindMany(ctx, envID, page, 100)
		if err != nil {
			return fmt.Errorf("paginate subscribers page %d: %w", page, err)
		}
		if len(subs) == 0 {
			break
		}

		notifIDMap := make(map[string]string, len(subs))
		for _, sub := range subs {
			channels := buildChannelDeliveries(wf.Steps)
			notif := &model.Notification{
				EnvironmentID: envID,
				SubscriberID:  sub.ID,
				WorkflowID:    wf.ID,
				TransactionID: payload.TransactionID,
				Payload:       payload.Payload,
				Channels:      channels,
				ExpireAt:      expireAt,
			}
			if err := h.notifRepo.Create(ctx, notif); err != nil {
				if mongo.IsDuplicateKeyError(err) {
					// Already processed — skip this subscriber (idempotent re-run).
					continue
				}
				log.Printf("broadcast: create notification for subscriber %s: %v", sub.ID.Hex(), err)
				continue
			}
			notifIDMap[sub.ID.Hex()] = notif.ID.Hex()
		}

		if len(notifIDMap) == 0 {
			page++
			continue
		}

		tp := TriggerPayload{
			EnvironmentID: payload.EnvironmentID,
			WorkflowID:    wf.ID.Hex(),
			SubscriberIDs: notifIDMap,
			Payload:       payload.Payload,
			TransactionID: payload.TransactionID,
			Overrides:     payload.Overrides,
		}
		data, err := json.Marshal(tp)
		if err != nil {
			return fmt.Errorf("marshal trigger payload for broadcast batch: %w", err)
		}
		task := asynq.NewTask(TaskTypeTrigger, data)
		if _, err := h.asynq.Enqueue(task); err != nil {
			return fmt.Errorf("enqueue trigger task for broadcast batch: %w", err)
		}

		page++
	}

	return nil
}

func buildChannelDeliveries(steps []model.WorkflowStep) []model.ChannelDelivery {
	channels := make([]model.ChannelDelivery, 0, len(steps))
	for _, step := range steps {
		if step.Type == "delay" || step.Type == "digest" {
			continue
		}
		channels = append(channels, model.ChannelDelivery{
			Channel: step.Type,
			Status:  "pending",
		})
	}
	return channels
}
