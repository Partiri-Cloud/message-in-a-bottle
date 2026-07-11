package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/partiri-cloud/message-in-a-bottle/internal/engine"
	"github.com/partiri-cloud/message-in-a-bottle/internal/handler/dto"
	"github.com/partiri-cloud/message-in-a-bottle/internal/middleware"
	"github.com/partiri-cloud/message-in-a-bottle/internal/model"
	"github.com/partiri-cloud/message-in-a-bottle/internal/repository"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type preferenceRepository interface {
	UpdateChannels(ctx context.Context, envID, subscriberID bson.ObjectID, workflowID *bson.ObjectID, overrides map[string]bool, seed model.ChannelPrefs) (*model.SubscriberPreference, error)
	FindBySubscriber(ctx context.Context, envID, subscriberID bson.ObjectID) ([]model.SubscriberPreference, error)
	FindForWorkflow(ctx context.Context, envID, subscriberID, workflowID bson.ObjectID) (workflowPref, globalPref *model.SubscriberPreference, err error)
}

type workflowLookupRepository interface {
	FindByID(ctx context.Context, envID, id bson.ObjectID) (*model.Workflow, error)
	FindByIdentifier(ctx context.Context, envID bson.ObjectID, identifier string) (*model.Workflow, error)
	FindAllActive(ctx context.Context, envID bson.ObjectID) ([]model.Workflow, error)
}

type PreferenceHandler struct {
	prefRepo preferenceRepository
	subRepo  subscriberLookupRepository
	wfRepo   workflowLookupRepository
}

func NewPreferenceHandler(prefRepo *repository.PreferenceRepository, subRepo *repository.SubscriberRepository, wfRepo *repository.WorkflowRepository) *PreferenceHandler {
	return &PreferenceHandler{prefRepo: prefRepo, subRepo: subRepo, wfRepo: wfRepo}
}

// storedPrefs is a subscriber's saved preference rows, split the way the
// resolver consumes them.
type storedPrefs struct {
	global     *model.SubscriberPreference
	byWorkflow map[bson.ObjectID]*model.SubscriberPreference
}

func (s storedPrefs) workflow(id bson.ObjectID) *model.SubscriberPreference {
	return s.byWorkflow[id]
}

func (h *PreferenceHandler) loadStored(ctx context.Context, envID, subID bson.ObjectID) (storedPrefs, error) {
	rows, err := h.prefRepo.FindBySubscriber(ctx, envID, subID)
	if err != nil {
		return storedPrefs{}, err
	}

	out := storedPrefs{byWorkflow: make(map[bson.ObjectID]*model.SubscriberPreference, len(rows))}
	for i := range rows {
		row := &rows[i]
		if row.WorkflowID == nil {
			out.global = row
			continue
		}
		out.byWorkflow[*row.WorkflowID] = row
	}
	return out, nil
}

// GetAll returns the subscriber's effective settings: one row per active
// workflow plus one for their global preference.
//
// Returning only the stored rows would be useless to a client. A workflow with
// no stored preference still has defaults that govern delivery, and the client
// cannot see those — so it would have to guess, and a guess of "enabled" is
// wrong for any workflow whose defaults disable a channel. The settings page
// would then show a channel as on while the server has it off. Resolving here
// means the client renders the truth.
func (h *PreferenceHandler) GetAll(c *gin.Context) {
	envID := middleware.GetEnvironmentID(c)

	sub, err := h.subRepo.FindBySubscriberID(c.Request.Context(), envID, c.Param("subscriberId"))
	if err != nil {
		h.respondSubscriberErr(c, err)
		return
	}

	stored, err := h.loadStored(c.Request.Context(), envID, sub.ID)
	if err != nil {
		internalError(c, err)
		return
	}

	workflows, err := h.wfRepo.FindAllActive(c.Request.Context(), envID)
	if err != nil {
		internalError(c, err)
		return
	}

	out := make([]dto.PreferenceResponse, 0, len(workflows)+1)

	// The global row. With nothing stored, "no global opt-out" means everything
	// is allowed through to the per-workflow defaults.
	out = append(out, globalPrefResponse(stored.global))

	for i := range workflows {
		wf := &workflows[i]
		wfPref := stored.workflow(wf.ID)
		out = append(out, workflowPrefResponse(wf,
			engine.ResolveChannelPrefs(wfPref, stored.global, wf.PreferenceDefaults), wfPref))
	}

	c.JSON(http.StatusOK, gin.H{"data": out})
}

func (h *PreferenceHandler) UpdateGlobal(c *gin.Context) {
	var req dto.UpdatePreferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	overrides, err := channelOverrides(req.Channels)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	envID := middleware.GetEnvironmentID(c)
	sub, err := h.subRepo.FindBySubscriberID(c.Request.Context(), envID, c.Param("subscriberId"))
	if err != nil {
		h.respondSubscriberErr(c, err)
		return
	}

	// Only the channels the caller named are written, each as its own path, so a
	// concurrent update to a different channel cannot be lost. The channels they
	// did not name are seeded — on insert only — from the all-allowed identity.
	//
	// The global row is a per-channel opt-out mask (engine.ResolveChannelPrefs):
	// true lets the workflow defaults through, false silences the channel wherever
	// the subscriber has no explicit workflow row. All-true is the mask that
	// changes nothing, so a first write of {"sms": false} stores exactly that one
	// opt-out — it cannot enable anything, and it cannot accidentally opt the
	// subscriber out of everything.
	pref, err := h.prefRepo.UpdateChannels(c.Request.Context(), envID, sub.ID, nil,
		overrides, model.AllChannelsEnabled())
	if err != nil {
		internalError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": globalPrefResponse(pref)})
}

func (h *PreferenceHandler) UpdateWorkflow(c *gin.Context) {
	var req dto.UpdatePreferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	overrides, err := channelOverrides(req.Channels)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	envID := middleware.GetEnvironmentID(c)

	wf, err := h.resolveWorkflow(c.Request.Context(), envID, c.Param("workflowId"))
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "workflow not found"}})
			return
		}
		internalError(c, err)
		return
	}

	sub, err := h.subRepo.FindBySubscriberID(c.Request.Context(), envID, c.Param("subscriberId"))
	if err != nil {
		h.respondSubscriberErr(c, err)
		return
	}

	// Only the two rows the seed needs, not every row the subscriber has.
	wfPref, globalPref, err := h.prefRepo.FindForWorkflow(c.Request.Context(), envID, sub.ID, wf.ID)
	if err != nil {
		internalError(c, err)
		return
	}

	// Only the channels the caller named are written, so a concurrent update to a
	// different channel of the same workflow cannot be lost.
	//
	// The channels they did not name are seeded — on insert only — from the state
	// that governs this workflow right now, resolved through exactly the precedence
	// delivery uses. That makes a no-op toggle a genuine no-op: writing a workflow
	// row promotes the subscriber from "whatever applied before" to "this row", and
	// since a row wins outright, seeding it from anything else would silently
	// discard their global opt-out. Once the row exists the seed is ignored and the
	// unnamed channels keep whatever is stored.
	seed := engine.ResolveChannelPrefs(wfPref, globalPref, wf.PreferenceDefaults)

	pref, err := h.prefRepo.UpdateChannels(c.Request.Context(), envID, sub.ID, &wf.ID,
		overrides, seed)
	if err != nil {
		internalError(c, err)
		return
	}

	// A workflow row wins outright at resolution time (the global mask only filters
	// workflow *defaults*), so the row we stored IS the effective state — exactly
	// what GetAll will report for this workflow.
	c.JSON(http.StatusOK, gin.H{"data": workflowPrefResponse(wf, pref.Channels, pref)})
}

// resolveWorkflow accepts either a workflow identifier ("deploy-started") or an
// ObjectID hex. Identifier is tried first: it is what every trigger carries and
// what clients actually hold. The hex fallback keeps older callers working.
//
// Inactive workflows are treated as absent. GetAll lists only active workflows,
// so accepting a write here would store a preference no read can ever show the
// client — invisible, unrevertable, and silently governing delivery again the
// moment the workflow is reactivated.
func (h *PreferenceHandler) resolveWorkflow(ctx context.Context, envID bson.ObjectID, param string) (*model.Workflow, error) {
	wf, err := h.lookupWorkflow(ctx, envID, param)
	if err != nil {
		return nil, err
	}
	if !wf.IsActive {
		return nil, mongo.ErrNoDocuments
	}
	return wf, nil
}

func (h *PreferenceHandler) lookupWorkflow(ctx context.Context, envID bson.ObjectID, param string) (*model.Workflow, error) {
	wf, err := h.wfRepo.FindByIdentifier(ctx, envID, param)
	if err == nil {
		return wf, nil
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}

	id, hexErr := bson.ObjectIDFromHex(param)
	if hexErr != nil {
		return nil, mongo.ErrNoDocuments
	}
	return h.wfRepo.FindByID(ctx, envID, id)
}

func (h *PreferenceHandler) respondSubscriberErr(c *gin.Context, err error) {
	if errors.Is(err, mongo.ErrNoDocuments) {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "subscriber not found"}})
		return
	}
	internalError(c, err)
}

// channelOverrides reduces a partial channel payload to the channels the caller
// actually named, keyed by delivery-side name. A channel absent from the payload
// is absent from the map, which is what lets the repository leave it alone.
//
// Every key is resolved through model.ChannelByField, so this cannot drift from
// the model the way a hand-written field list would: an unknown channel is an
// error rather than a silently ignored key, and a channel added to the model
// needs no change here.
//
// An empty result is an error too. The payload named nothing, so there is
// nothing to write — but the upsert would still create a row, promoting a
// subscriber who was inheriting into one with an explicit choice they never
// made, which then shadows their global opt-out at delivery time.
func channelOverrides(channels map[string]*bool) (map[string]bool, error) {
	out := make(map[string]bool, len(channels))
	for field, v := range channels {
		name, ok := model.ChannelByField(field)
		if !ok {
			return nil, fmt.Errorf("unknown channel %q", field)
		}
		if v != nil { // null means "leave it alone", same as omitting it
			out[name] = *v
		}
	}
	if len(out) == 0 {
		return nil, errors.New("channels must name at least one channel to change")
	}
	return out, nil
}

// globalPrefResponse renders the row describing the subscriber's global opt-out
// mask. A nil row means they have never set one, which allows everything through.
func globalPrefResponse(pref *model.SubscriberPreference) dto.PreferenceResponse {
	row := dto.PreferenceResponse{Channels: model.AllChannelsEnabled()}
	if pref != nil {
		row.Channels = pref.Channels
		row.Explicit = true
		row.UpdatedAt = &pref.UpdatedAt
	}
	return row
}

// workflowPrefResponse renders one workflow's row. channels are the effective
// values; pref is the stored row, or nil when the subscriber is inheriting.
func workflowPrefResponse(wf *model.Workflow, channels model.ChannelPrefs, pref *model.SubscriberPreference) dto.PreferenceResponse {
	hexID := wf.ID.Hex()
	identifier := wf.Identifier
	row := dto.PreferenceResponse{
		WorkflowID:         &hexID,
		WorkflowIdentifier: &identifier,
		Channels:           channels,
		Explicit:           pref != nil,
	}
	if pref != nil {
		row.UpdatedAt = &pref.UpdatedAt
	}
	return row
}
