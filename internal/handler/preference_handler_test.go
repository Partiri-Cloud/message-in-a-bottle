package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/partiri-cloud/message-in-a-bottle/internal/middleware"
	"github.com/partiri-cloud/message-in-a-bottle/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// PreferenceHandler is constructed against the narrow preferenceRepository /
// workflowLookupRepository / subscriberLookupRepository interfaces, so these
// tests drive the real handler with stubs. subRepoStub is shared with
// notification_handler_test.go.

type prefRepoStub struct {
	envID bson.ObjectID
	subID bson.ObjectID
	// stored is keyed by workflow hex, "" for the global preference.
	stored   map[string]model.SubscriberPreference
	upserted *model.SubscriberPreference
}

func newPrefRepoStub(envID, subID bson.ObjectID) *prefRepoStub {
	return &prefRepoStub{envID: envID, subID: subID, stored: map[string]model.SubscriberPreference{}}
}

func prefKey(workflowID *bson.ObjectID) string {
	if workflowID == nil {
		return ""
	}
	return workflowID.Hex()
}

// UpdateChannels mirrors the repository: the named channels are written onto the
// stored row, and the seed applies only when the row does not exist yet. Writing
// only the named channels is what makes concurrent updates to different channels
// safe, so the stub must not merge the seed into an existing row.
func (s *prefRepoStub) UpdateChannels(_ context.Context, envID, subID bson.ObjectID, workflowID *bson.ObjectID, overrides map[string]bool, seed model.ChannelPrefs) (*model.SubscriberPreference, error) {
	key := prefKey(workflowID)

	channels := seed
	if existing, ok := s.stored[key]; ok {
		channels = existing.Channels
	}
	for name, v := range overrides {
		channels.Set(name, v)
	}

	pref := model.SubscriberPreference{
		EnvironmentID: envID,
		SubscriberID:  subID,
		WorkflowID:    workflowID,
		Channels:      channels,
		UpdatedAt:     time.Now(),
	}
	s.stored[key] = pref
	s.upserted = &pref
	return &pref, nil
}

func (s *prefRepoStub) FindForWorkflow(_ context.Context, envID, subID, wfID bson.ObjectID) (*model.SubscriberPreference, *model.SubscriberPreference, error) {
	if envID != s.envID || subID != s.subID {
		return nil, nil, nil
	}
	var wfPref, globalPref *model.SubscriberPreference
	if row, ok := s.stored[wfID.Hex()]; ok {
		wfPref = &row
	}
	if row, ok := s.stored[""]; ok {
		globalPref = &row
	}
	return wfPref, globalPref, nil
}

func (s *prefRepoStub) FindBySubscriber(_ context.Context, envID, subID bson.ObjectID) ([]model.SubscriberPreference, error) {
	if envID != s.envID || subID != s.subID {
		return nil, nil
	}
	out := make([]model.SubscriberPreference, 0, len(s.stored))
	for _, p := range s.stored {
		out = append(out, p)
	}
	return out, nil
}

type wfRepoStub struct {
	envID bson.ObjectID
	wfs   []model.Workflow
}

func (s *wfRepoStub) FindByID(_ context.Context, envID, id bson.ObjectID) (*model.Workflow, error) {
	for i := range s.wfs {
		if s.wfs[i].ID == id && envID == s.envID {
			return &s.wfs[i], nil
		}
	}
	return nil, mongo.ErrNoDocuments
}

func (s *wfRepoStub) FindByIdentifier(_ context.Context, envID bson.ObjectID, identifier string) (*model.Workflow, error) {
	for i := range s.wfs {
		if s.wfs[i].Identifier == identifier && envID == s.envID {
			return &s.wfs[i], nil
		}
	}
	return nil, mongo.ErrNoDocuments
}

func (s *wfRepoStub) FindAllActive(_ context.Context, envID bson.ObjectID) ([]model.Workflow, error) {
	if envID != s.envID {
		return nil, nil
	}
	var out []model.Workflow
	for _, wf := range s.wfs {
		if wf.IsActive {
			out = append(out, wf)
		}
	}
	return out, nil
}

func setupPreferenceRouter(pref *prefRepoStub, sub *subRepoStub, wf *wfRepoStub, callerEnvID bson.ObjectID) *gin.Engine {
	h := &PreferenceHandler{prefRepo: pref, subRepo: sub, wfRepo: wf}
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(middleware.ContextKeyEnvironmentID, callerEnvID)
		c.Next()
	})
	r.GET("/subscribers/:subscriberId/preferences", h.GetAll)
	r.PATCH("/subscribers/:subscriberId/preferences", h.UpdateGlobal)
	r.PATCH("/subscribers/:subscriberId/preferences/:workflowId", h.UpdateWorkflow)
	return r
}

type prefEnv struct {
	envID    bson.ObjectID
	deployID bson.ObjectID // "deploy-started": in-app only, mirroring production
	silentID bson.ObjectID // "silent-workflow": no preferenceDefaults at all
	pref     *prefRepoStub
	router   *gin.Engine
}

// prefFixture mirrors production: every live workflow was bootstrapped with
// `preferenceDefaults: {inApp: true}`, so with plain-bool ChannelPrefs the other
// five channels are stored as false. "silent-workflow" is the degenerate case —
// created with no defaults key at all, so every channel is false.
func prefFixture() prefEnv {
	envID := bson.NewObjectID()
	subOID := bson.NewObjectID()
	deployID := bson.NewObjectID()
	silentID := bson.NewObjectID()

	pref := newPrefRepoStub(envID, subOID)
	sub := &subRepoStub{
		envID:        envID,
		subscriberID: "user-1",
		sub:          &model.Subscriber{ID: subOID, EnvironmentID: envID, SubscriberID: "user-1"},
	}
	wf := &wfRepoStub{envID: envID, wfs: []model.Workflow{
		{
			ID:                 deployID,
			EnvironmentID:      envID,
			Identifier:         "deploy-started",
			PreferenceDefaults: model.ChannelPrefs{InApp: true},
			IsActive:           true,
		},
		{
			ID:                 silentID,
			EnvironmentID:      envID,
			Identifier:         "silent-workflow",
			PreferenceDefaults: model.ChannelPrefs{},
			IsActive:           true,
		},
		{
			ID:                 bson.NewObjectID(),
			EnvironmentID:      envID,
			Identifier:         "retired-workflow",
			PreferenceDefaults: model.ChannelPrefs{InApp: true},
			IsActive:           false,
		},
	}}

	return prefEnv{
		envID:    envID,
		deployID: deployID,
		silentID: silentID,
		pref:     pref,
		router:   setupPreferenceRouter(pref, sub, wf, envID),
	}
}

func boolJSON(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func patchPrefs(t *testing.T, router *gin.Engine, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPatch, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

type prefRow struct {
	WorkflowID         *string `json:"workflowId"`
	WorkflowIdentifier *string `json:"workflowIdentifier"`
	Explicit           bool    `json:"explicit"`
	Channels           struct {
		Email   bool `json:"email"`
		SMS     bool `json:"sms"`
		Push    bool `json:"push"`
		InApp   bool `json:"inApp"`
		Slack   bool `json:"slack"`
		MSTeams bool `json:"msTeams"`
	} `json:"channels"`
}

func getPrefs(t *testing.T, router *gin.Engine) map[string]prefRow {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/subscribers/user-1/preferences", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Data []prefRow `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	// Key by identifier; the global row (null identifier) lands under "".
	out := make(map[string]prefRow, len(body.Data))
	for _, row := range body.Data {
		key := ""
		if row.WorkflowIdentifier != nil {
			key = *row.WorkflowIdentifier
		}
		out[key] = row
	}
	return out
}

// ── Wire format ──────────────────────────────────────────────────────────────

// The Go models carry BSON tags; without JSON tags they serialize as
// "WorkflowID"/"Channels" and every camelCase client reads undefined.
func TestPreferenceGetAll_SerializesCamelCase(t *testing.T) {
	env := prefFixture()

	req := httptest.NewRequest(http.MethodGet, "/subscribers/user-1/preferences", nil)
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Data []map[string]json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.NotEmpty(t, body.Data)

	record := body.Data[0]
	for _, k := range []string{"workflowId", "workflowIdentifier", "channels", "explicit", "updatedAt"} {
		assert.Contains(t, record, k)
	}
	assert.NotContains(t, record, "WorkflowID")
	assert.NotContains(t, record, "Channels")
}

// ── Effective preferences ────────────────────────────────────────────────────

// The whole point of resolving server-side: a client cannot see a workflow's
// defaults, so if the API returned only stored rows the client would have to
// guess — and a guess of "enabled" is wrong for every production workflow, whose
// defaults are in-app only. The settings page would show Email as on while the
// server has it off.
func TestPreferenceGetAll_UnsetWorkflowReportsWorkflowDefaults(t *testing.T) {
	env := prefFixture()

	rows := getPrefs(t, env.router)

	deploy := rows["deploy-started"]
	assert.False(t, deploy.Explicit, "subscriber has stored nothing for this workflow")
	assert.True(t, deploy.Channels.InApp, "in-app is on by default")
	assert.False(t, deploy.Channels.Email, "email is NOT on by default — the client must not assume it is")
	assert.False(t, deploy.Channels.SMS)
}

func TestPreferenceGetAll_IncludesEveryActiveWorkflowPlusGlobal(t *testing.T) {
	env := prefFixture()

	rows := getPrefs(t, env.router)

	require.Len(t, rows, 3, "global + both workflows")
	global := rows[""]
	assert.Nil(t, global.WorkflowID)
	assert.Nil(t, global.WorkflowIdentifier)
	assert.False(t, global.Explicit)
	assert.True(t, global.Channels.Email, "with no global opt-out stored, nothing is blocked globally")

	assert.Contains(t, rows, "deploy-started")
	assert.Contains(t, rows, "silent-workflow")
}

// An explicit workflow row wins outright; without one the workflow's defaults
// apply, filtered through the global opt-out mask. GetAll must report exactly
// what delivery (engine.IsChannelEnabled) will do — the two share
// engine.ResolveChannelPrefs.
func TestPreferenceGetAll_ReportsTheSamePrecedenceAsDelivery(t *testing.T) {
	env := prefFixture()
	env.pref.stored[""] = model.SubscriberPreference{
		WorkflowID: nil,
		Channels:   model.ChannelPrefs{Email: false, InApp: false},
	}
	env.pref.stored[env.deployID.Hex()] = model.SubscriberPreference{
		WorkflowID: &env.deployID,
		Channels:   model.ChannelPrefs{Email: true, InApp: true},
	}

	rows := getPrefs(t, env.router)

	// deploy-started has its own row, so that row wins outright.
	assert.True(t, rows["deploy-started"].Channels.InApp)
	assert.True(t, rows["deploy-started"].Channels.Email)
	assert.True(t, rows["deploy-started"].Explicit)

	// silent-workflow has no row of its own: its defaults (all off), filtered
	// through the global mask, stay off.
	assert.False(t, rows["silent-workflow"].Channels.InApp)
	assert.False(t, rows["silent-workflow"].Explicit)
}

// ── Workflow addressing ──────────────────────────────────────────────────────

// Clients hold workflow identifiers, not ObjectIDs. The route used to run the
// path param through bson.ObjectIDFromHex, so every real save 400'd.
func TestPreferenceUpdateWorkflow_AcceptsIdentifier(t *testing.T) {
	env := prefFixture()

	w := patchPrefs(t, env.router, "/subscribers/user-1/preferences/deploy-started", `{"channels":{"email":false}}`)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, env.pref.upserted)
	require.NotNil(t, env.pref.upserted.WorkflowID)
	assert.Equal(t, env.deployID, *env.pref.upserted.WorkflowID)
}

func TestPreferenceUpdateWorkflow_AcceptsObjectIDHex(t *testing.T) {
	env := prefFixture()

	w := patchPrefs(t, env.router, "/subscribers/user-1/preferences/"+env.deployID.Hex(), `{"channels":{"email":false}}`)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, env.pref.upserted)
	assert.Equal(t, env.deployID, *env.pref.upserted.WorkflowID)
}

func TestPreferenceUpdateWorkflow_UnknownWorkflowReturns404(t *testing.T) {
	env := prefFixture()

	w := patchPrefs(t, env.router, "/subscribers/user-1/preferences/no-such-workflow", `{"channels":{"email":false}}`)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// GetAll lists only active workflows. Accepting a write on an inactive one would
// store a preference no read can ever surface — the client could neither see it
// nor revert it, and it would silently govern delivery again on reactivation.
func TestPreferenceUpdateWorkflow_InactiveWorkflowReturns404(t *testing.T) {
	env := prefFixture()

	w := patchPrefs(t, env.router, "/subscribers/user-1/preferences/retired-workflow", `{"channels":{"email":false}}`)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Nil(t, env.pref.upserted, "nothing may be written for a workflow the read path cannot show")

	assert.NotContains(t, getPrefs(t, env.router), "retired-workflow", "and GET does not list it either")
}

// The request channel map is resolved through model.ChannelByField, so every
// channel the model declares is accepted on the wire. This is what stops the API
// shape drifting from the model: a channel added to model.ChannelPrefs is
// accepted here with no handler change, and this test fails if it is not.
func TestPreferenceUpdate_AcceptsEveryChannelTheModelDeclares(t *testing.T) {
	for _, channel := range model.ChannelNames() {
		field, ok := model.ChannelBSONField(channel)
		require.True(t, ok)

		env := prefFixture()
		w := patchPrefs(t, env.router, "/subscribers/user-1/preferences/deploy-started",
			`{"channels":{"`+field+`":true}}`)

		require.Equal(t, http.StatusOK, w.Code, "channel %q must be accepted", field)
		require.NotNil(t, env.pref.upserted)
		v, _ := env.pref.upserted.Channels.Get(channel)
		assert.True(t, v, "channel %q must actually be written", channel)
	}
}

// An unknown channel is a client mistake, not a key to ignore silently — a typo
// like "emial" would otherwise return 200 having changed nothing.
func TestPreferenceUpdate_UnknownChannelReturns400(t *testing.T) {
	env := prefFixture()

	w := patchPrefs(t, env.router, "/subscribers/user-1/preferences/deploy-started", `{"channels":{"emial":true}}`)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Nil(t, env.pref.upserted)
}

// A payload that names no channel has nothing to write, but the upsert would
// still create a row — promoting a subscriber who was inheriting into one with an
// explicit choice they never made, which then shadows their global opt-out.
func TestPreferenceUpdate_EmptyChannelPayloadReturns400(t *testing.T) {
	for _, body := range []string{`{"channels":{}}`, `{"channels":{"email":null}}`} {
		env := prefFixture()

		w := patchPrefs(t, env.router, "/subscribers/user-1/preferences/deploy-started", body)

		assert.Equal(t, http.StatusBadRequest, w.Code, "body %s", body)
		assert.Nil(t, env.pref.upserted, "nothing may be written for %s", body)
	}
}

func TestPreferenceUpdate_UnknownSubscriberReturns404(t *testing.T) {
	env := prefFixture()

	w := patchPrefs(t, env.router, "/subscribers/nobody/preferences/deploy-started", `{"channels":{"email":true}}`)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── Merge semantics ──────────────────────────────────────────────────────────

// The bug this pins: channel bools were non-pointers, so {"email":true} bound
// inApp/sms/push to false and the upsert wrote them. Turning email on silently
// turned in-app off.
func TestPreferenceUpdateWorkflow_PartialUpdatePreservesOtherChannels(t *testing.T) {
	env := prefFixture()
	env.pref.stored[env.deployID.Hex()] = model.SubscriberPreference{
		WorkflowID: &env.deployID,
		Channels:   model.ChannelPrefs{Email: false, InApp: true, Push: true},
	}

	w := patchPrefs(t, env.router, "/subscribers/user-1/preferences/deploy-started", `{"channels":{"email":true}}`)
	require.Equal(t, http.StatusOK, w.Code)

	require.NotNil(t, env.pref.upserted)
	assert.True(t, env.pref.upserted.Channels.Email, "email should be turned on")
	assert.True(t, env.pref.upserted.Channels.InApp, "in-app must survive an email-only update")
	assert.True(t, env.pref.upserted.Channels.Push, "push must survive an email-only update")
}

// A workflow update writes only the workflow row. It must not touch the global
// row, so a global opt-out on some other channel stays exactly as it was.
func TestPreferenceUpdateWorkflow_LeavesTheGlobalRowAlone(t *testing.T) {
	env := prefFixture()
	global := model.SubscriberPreference{
		WorkflowID: nil,
		Channels:   model.ChannelPrefs{Email: true, SMS: true, Push: true, InApp: false, Slack: true, MSTeams: true},
	}
	env.pref.stored[""] = global

	require.Equal(t, http.StatusOK, patchPrefs(t, env.router,
		"/subscribers/user-1/preferences/deploy-started", `{"channels":{"email":true}}`).Code)

	require.NotNil(t, env.pref.upserted)
	assert.NotNil(t, env.pref.upserted.WorkflowID, "the write must be workflow-scoped")
	assert.Equal(t, global.Channels, env.pref.stored[""].Channels, "the global row is untouched")
}

// With nothing stored anywhere, a workflow update starts from that workflow's
// declared defaults — the same thing GET reports and delivery uses, so the
// subscriber gets exactly the state the settings page showed them.
func TestPreferenceUpdateWorkflow_FirstUpdateBasesOnWorkflowDefaults(t *testing.T) {
	env := prefFixture() // deploy-started defaults: in-app only

	w := patchPrefs(t, env.router, "/subscribers/user-1/preferences/deploy-started", `{"channels":{"email":true}}`)
	require.Equal(t, http.StatusOK, w.Code)

	require.NotNil(t, env.pref.upserted)
	assert.True(t, env.pref.upserted.Channels.Email, "the change they asked for")
	assert.True(t, env.pref.upserted.Channels.InApp, "in-app was on by default and was not mentioned")
	assert.False(t, env.pref.upserted.Channels.SMS, "sms was off by default and was not mentioned")
}

// The global row is an opt-out mask, and a first write merges onto the
// all-allowed identity. Opting out of SMS must store exactly that one opt-out —
// not silently opt the subscriber out of every other channel too.
func TestPreferenceUpdateGlobal_FirstWriteOptsOutOfOnlyTheNamedChannel(t *testing.T) {
	env := prefFixture()

	w := patchPrefs(t, env.router, "/subscribers/user-1/preferences", `{"channels":{"sms":false}}`)
	require.Equal(t, http.StatusOK, w.Code)

	require.NotNil(t, env.pref.upserted)
	assert.Nil(t, env.pref.upserted.WorkflowID)
	assert.False(t, env.pref.upserted.Channels.SMS, "the opt-out they asked for")
	assert.True(t, env.pref.upserted.Channels.Email, "unmentioned channels stay allowed through the mask")
	assert.True(t, env.pref.upserted.Channels.InApp)

	// The mask cannot enable anything: deploy-started's defaults are in-app
	// only, and they must stay that way after this global write.
	rows := getPrefs(t, env.router)
	assert.True(t, rows["deploy-started"].Channels.InApp)
	assert.False(t, rows["deploy-started"].Channels.Email, "an all-allowed mask must not switch on a channel the workflow has off")
}

// A partial global update still merges onto an existing global row rather than
// zeroing the channels it does not mention.
func TestPreferenceUpdateGlobal_PartialUpdateMergesOntoTheStoredRow(t *testing.T) {
	env := prefFixture()
	env.pref.stored[""] = model.SubscriberPreference{
		WorkflowID: nil,
		Channels:   model.ChannelPrefs{Email: true, InApp: true, Push: true},
	}

	require.Equal(t, http.StatusOK,
		patchPrefs(t, env.router, "/subscribers/user-1/preferences", `{"channels":{"email":false}}`).Code)

	require.NotNil(t, env.pref.upserted)
	assert.False(t, env.pref.upserted.Channels.Email, "the change they asked for")
	assert.True(t, env.pref.upserted.Channels.InApp, "must survive an email-only update")
	assert.True(t, env.pref.upserted.Channels.Push)
}

// A no-op PATCH must not change what GET reports. If the read path and the write
// base ever disagree, a subscriber's settings silently shift the first time they
// touch anything — which is the whole class of bug this endpoint keeps producing.
func TestPreference_NoOpUpdateDoesNotChangeWhatGetReports(t *testing.T) {
	env := prefFixture()
	env.pref.stored[""] = model.SubscriberPreference{
		WorkflowID: nil,
		Channels:   model.ChannelPrefs{Email: true, SMS: false, Push: true, InApp: true, Slack: true, MSTeams: true},
	}

	for _, wf := range []string{"deploy-started", "silent-workflow"} {
		before := getPrefs(t, env.router)[wf].Channels

		// Re-assert a channel at the value GET just reported: a pure no-op.
		body := `{"channels":{"email":` + boolJSON(before.Email) + `}}`
		require.Equal(t, http.StatusOK,
			patchPrefs(t, env.router, "/subscribers/user-1/preferences/"+wf, body).Code)

		assert.Equal(t, before, getPrefs(t, env.router)[wf].Channels,
			"a no-op update to %s changed the effective settings", wf)
	}
}
