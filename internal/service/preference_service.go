package service

import (
	"context"

	"github.com/partiri-cloud/message-in-a-bottle/internal/engine"
	"github.com/partiri-cloud/message-in-a-bottle/internal/model"
	"github.com/partiri-cloud/message-in-a-bottle/internal/repository"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type PreferenceService struct {
	prefRepo *repository.PreferenceRepository
	wfRepo   *repository.WorkflowRepository
}

func NewPreferenceService(prefRepo *repository.PreferenceRepository, wfRepo *repository.WorkflowRepository) *PreferenceService {
	return &PreferenceService{prefRepo: prefRepo, wfRepo: wfRepo}
}

func (s *PreferenceService) IsChannelEnabled(ctx context.Context, envID, subscriberID bson.ObjectID, workflowID bson.ObjectID, channel string) bool {
	wf, err := s.wfRepo.FindByID(ctx, envID, workflowID)
	if err != nil {
		return true // default to enabled if workflow not found
	}

	workflowPref, _ := s.prefRepo.FindBySubscriberAndWorkflow(ctx, envID, subscriberID, &workflowID)
	globalPref, _ := s.prefRepo.FindBySubscriberAndWorkflow(ctx, envID, subscriberID, nil)

	return engine.IsChannelEnabled(channel, workflowPref, globalPref, wf.PreferenceDefaults)
}

func (s *PreferenceService) GetResolvedPreferences(ctx context.Context, envID, subscriberID bson.ObjectID) ([]ResolvedPreference, error) {
	prefs, err := s.prefRepo.FindBySubscriber(ctx, envID, subscriberID)
	if err != nil {
		return nil, err
	}

	// Build a map for quick lookup
	var global *model.SubscriberPreference
	perWorkflow := make(map[bson.ObjectID]*model.SubscriberPreference)
	for i := range prefs {
		if prefs[i].WorkflowID == nil {
			global = &prefs[i]
		} else {
			perWorkflow[*prefs[i].WorkflowID] = &prefs[i]
		}
	}

	// Get all workflows for this environment
	workflows, _, err := s.wfRepo.FindMany(ctx, envID, 1, 1000)
	if err != nil && err != mongo.ErrNoDocuments {
		return nil, err
	}

	var resolved []ResolvedPreference
	for _, wf := range workflows {
		wfPref := perWorkflow[wf.ID]
		rp := ResolvedPreference{
			WorkflowID:   wf.ID,
			WorkflowName: wf.Name,
			Channels: map[string]bool{
				"email":    engine.IsChannelEnabled("email", wfPref, global, wf.PreferenceDefaults),
				"sms":      engine.IsChannelEnabled("sms", wfPref, global, wf.PreferenceDefaults),
				"push":     engine.IsChannelEnabled("push", wfPref, global, wf.PreferenceDefaults),
				"in_app":   engine.IsChannelEnabled("in_app", wfPref, global, wf.PreferenceDefaults),
				"slack":    engine.IsChannelEnabled("slack", wfPref, global, wf.PreferenceDefaults),
				"ms_teams": engine.IsChannelEnabled("ms_teams", wfPref, global, wf.PreferenceDefaults),
			},
		}
		resolved = append(resolved, rp)
	}

	return resolved, nil
}

type ResolvedPreference struct {
	WorkflowID   bson.ObjectID   `json:"workflowId"`
	WorkflowName string          `json:"workflowName"`
	Channels     map[string]bool `json:"channels"`
}
