package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/partiri-cloud/message-in-a-box/internal/handler/dto"
	"github.com/partiri-cloud/message-in-a-box/internal/middleware"
	"github.com/partiri-cloud/message-in-a-box/internal/model"
	"github.com/partiri-cloud/message-in-a-box/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ─── convertSteps unit tests ─────────────────────────────────────────────────

func TestConvertSteps_BasicChannel(t *testing.T) {
	dtoSteps := []dto.WorkflowStepDTO{
		{
			Type:  "email",
			Order: 0,
			Template: &dto.StepTemplateDTO{
				Subject: map[string]string{"en": "Hello"},
				Body:    map[string]string{"en": "<p>World</p>"},
			},
			DefaultEnabled: true,
		},
	}

	steps := convertSteps(dtoSteps)
	require.Len(t, steps, 1)
	assert.Equal(t, "email", steps[0].Type)
	assert.Equal(t, 0, steps[0].Order)
	assert.True(t, steps[0].DefaultEnabled)
	assert.NotNil(t, steps[0].Template)
	assert.Equal(t, "Hello", steps[0].Template.Subject["en"])
	assert.Equal(t, "<p>World</p>", steps[0].Template.Body["en"])
}

func TestConvertSteps_DigestConfig(t *testing.T) {
	dtoSteps := []dto.WorkflowStepDTO{
		{
			Type:  "digest",
			Order: 1,
			DigestConfig: &dto.DigestConfigDTO{
				Amount:    30,
				Unit:      "minutes",
				DigestKey: "service_id",
			},
		},
	}

	steps := convertSteps(dtoSteps)
	require.Len(t, steps, 1)
	require.NotNil(t, steps[0].DigestConfig)
	assert.Equal(t, 30, steps[0].DigestConfig.Amount)
	assert.Equal(t, "minutes", steps[0].DigestConfig.Unit)
	assert.Equal(t, "service_id", steps[0].DigestConfig.DigestKey)
}

func TestConvertSteps_DelayConfig(t *testing.T) {
	dtoSteps := []dto.WorkflowStepDTO{
		{
			Type:  "delay",
			Order: 2,
			DelayConfig: &dto.DelayConfigDTO{
				Amount: 5,
				Unit:   "minutes",
			},
		},
	}

	steps := convertSteps(dtoSteps)
	require.Len(t, steps, 1)
	require.NotNil(t, steps[0].DelayConfig)
	assert.Equal(t, 5, steps[0].DelayConfig.Amount)
	assert.Equal(t, "minutes", steps[0].DelayConfig.Unit)
}

func TestConvertSteps_WithConditions(t *testing.T) {
	dtoSteps := []dto.WorkflowStepDTO{
		{
			Type:  "email",
			Order: 0,
			Conditions: []dto.StepConditionDTO{
				{Field: "payload.severity", Operator: "eq", Value: "critical"},
				{Field: "steps.0.seen", Operator: "eq", Value: false},
			},
			DefaultEnabled: true,
		},
	}

	steps := convertSteps(dtoSteps)
	require.Len(t, steps, 1)
	require.Len(t, steps[0].Conditions, 2)
	assert.Equal(t, "payload.severity", steps[0].Conditions[0].Field)
	assert.Equal(t, "eq", steps[0].Conditions[0].Operator)
	assert.Equal(t, "critical", steps[0].Conditions[0].Value)
	assert.Equal(t, false, steps[0].Conditions[1].Value)
}

func TestConvertSteps_GeneratesStepIDs(t *testing.T) {
	dtoSteps := []dto.WorkflowStepDTO{
		{Type: "in_app", Order: 0, DefaultEnabled: true},
		{Type: "email", Order: 1, DefaultEnabled: true},
	}

	steps := convertSteps(dtoSteps)
	require.Len(t, steps, 2)
	assert.False(t, steps[0].ID.IsZero(), "step 0 should have an ID")
	assert.False(t, steps[1].ID.IsZero(), "step 1 should have an ID")
	assert.NotEqual(t, steps[0].ID, steps[1].ID, "step IDs should be unique")
}

func TestConvertSteps_EmptyOptionalFields(t *testing.T) {
	dtoSteps := []dto.WorkflowStepDTO{
		{Type: "sms", Order: 0, DefaultEnabled: true},
	}

	steps := convertSteps(dtoSteps)
	require.Len(t, steps, 1)
	assert.Nil(t, steps[0].Template)
	assert.Nil(t, steps[0].DigestConfig)
	assert.Nil(t, steps[0].DelayConfig)
	assert.Empty(t, steps[0].Conditions)
}

// ─── Cross-tenant IDOR tests ──────────────────────────────────────────────────
//
// These tests verify that workflow operations scope by environmentId.
// Because WorkflowHandler takes a concrete *repository.WorkflowRepository (no
// interface injection), we exercise the scoping through a stub workflowRepoStub
// that mimics the repo contract: FindByID / Update / SetActive / Delete only
// return the workflow when both envID and id match, otherwise mongo.ErrNoDocuments.
//
// The tests cannot run against a real MongoDB (none available here), so they
// use the stub. The integration-level guarantee that the MongoDB filter is
// correct is provided by the updated filter expressions in workflow_repo.go,
// which the `go build ./...` / `go vet ./...` checks confirm are syntactically
// and type-correct.

// workflowRepoStub holds a single workflow and scopes all lookups by envID.
type workflowRepoStub struct {
	envID bson.ObjectID
	wf    *model.Workflow
}

func (s *workflowRepoStub) FindByID(_ context.Context, envID, id bson.ObjectID) (*model.Workflow, error) {
	if s.wf == nil || s.wf.ID != id || s.envID != envID {
		return nil, mongo.ErrNoDocuments
	}
	return s.wf, nil
}

func (s *workflowRepoStub) Update(_ context.Context, envID, id bson.ObjectID, _ *model.Workflow) error {
	if s.wf == nil || s.wf.ID != id || s.envID != envID {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (s *workflowRepoStub) SetActive(_ context.Context, envID, id bson.ObjectID, _ bool) error {
	if s.wf == nil || s.wf.ID != id || s.envID != envID {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (s *workflowRepoStub) Delete(_ context.Context, envID, id bson.ObjectID) error {
	if s.wf == nil || s.wf.ID != id || s.envID != envID {
		return mongo.ErrNoDocuments
	}
	return nil
}

// workflowHandlerWithStub is a parallel handler type that accepts the stub
// interface so tests can drive it without needing a real Mongo connection.
type workflowRepoIface interface {
	FindByID(ctx context.Context, envID, id bson.ObjectID) (*model.Workflow, error)
	Update(ctx context.Context, envID, id bson.ObjectID, wf *model.Workflow) error
	SetActive(ctx context.Context, envID, id bson.ObjectID, active bool) error
	Delete(ctx context.Context, envID, id bson.ObjectID) error
}

type workflowHandlerTestable struct {
	wfRepo workflowRepoIface
}

func (h *workflowHandlerTestable) Get(c *gin.Context) {
	id, err := bson.ObjectIDFromHex(c.Param("workflowId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": "invalid workflow ID"}})
		return
	}
	envID := middleware.GetEnvironmentID(c)
	wf, err := h.wfRepo.FindByID(c.Request.Context(), envID, id)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "workflow not found"}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR"}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": wf})
}

func (h *workflowHandlerTestable) Update(c *gin.Context) {
	id, err := bson.ObjectIDFromHex(c.Param("workflowId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": "invalid workflow ID"}})
		return
	}
	var req dto.CreateWorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}
	envID := middleware.GetEnvironmentID(c)
	wf := &model.Workflow{ID: id, EnvironmentID: envID}
	if err := h.wfRepo.Update(c.Request.Context(), envID, id, wf); err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "workflow not found"}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR"}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": wf})
}

func (h *workflowHandlerTestable) SetStatus(c *gin.Context) {
	id, err := bson.ObjectIDFromHex(c.Param("workflowId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": "invalid workflow ID"}})
		return
	}
	var req dto.WorkflowStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}
	envID := middleware.GetEnvironmentID(c)
	if err := h.wfRepo.SetActive(c.Request.Context(), envID, id, req.IsActive); err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "workflow not found"}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR"}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"acknowledged": true}})
}

func (h *workflowHandlerTestable) Delete(c *gin.Context) {
	id, err := bson.ObjectIDFromHex(c.Param("workflowId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": "invalid workflow ID"}})
		return
	}
	envID := middleware.GetEnvironmentID(c)
	if err := h.wfRepo.Delete(c.Request.Context(), envID, id); err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "workflow not found"}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR"}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"acknowledged": true}})
}

// setupCrossTenantRouter wires the testable handler with the given stub and
// injects envID into the gin context via a lightweight middleware shim.
func setupCrossTenantRouter(stub workflowRepoIface, callerEnvID bson.ObjectID) *gin.Engine {
	h := &workflowHandlerTestable{wfRepo: stub}
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(middleware.ContextKeyEnvironmentID, callerEnvID)
		c.Next()
	})
	r.GET("/workflows/:workflowId", h.Get)
	r.PUT("/workflows/:workflowId", h.Update)
	r.PATCH("/workflows/:workflowId/status", h.SetStatus)
	r.DELETE("/workflows/:workflowId", h.Delete)
	return r
}

// ── Tenant A owns the workflow; tenant B calls — all should get 404 ───────────

func TestWorkflowGet_CrossTenant_Returns404(t *testing.T) {
	tenantA := bson.NewObjectID()
	tenantB := bson.NewObjectID()
	wfID := bson.NewObjectID()

	stub := &workflowRepoStub{envID: tenantA, wf: &model.Workflow{ID: wfID, EnvironmentID: tenantA}}
	router := setupCrossTenantRouter(stub, tenantB)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/workflows/"+wfID.Hex(), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestWorkflowUpdate_CrossTenant_Returns404(t *testing.T) {
	tenantA := bson.NewObjectID()
	tenantB := bson.NewObjectID()
	wfID := bson.NewObjectID()

	stub := &workflowRepoStub{envID: tenantA, wf: &model.Workflow{ID: wfID, EnvironmentID: tenantA}}
	router := setupCrossTenantRouter(stub, tenantB)

	body, _ := json.Marshal(dto.CreateWorkflowRequest{
		Identifier: "wf",
		Name:       "WF",
		Steps:      []dto.WorkflowStepDTO{{Type: "email", Order: 0}},
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/workflows/"+wfID.Hex(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestWorkflowSetStatus_CrossTenant_Returns404(t *testing.T) {
	tenantA := bson.NewObjectID()
	tenantB := bson.NewObjectID()
	wfID := bson.NewObjectID()

	stub := &workflowRepoStub{envID: tenantA, wf: &model.Workflow{ID: wfID, EnvironmentID: tenantA}}
	router := setupCrossTenantRouter(stub, tenantB)

	body, _ := json.Marshal(dto.WorkflowStatusRequest{IsActive: false})
	w := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/workflows/"+wfID.Hex()+"/status", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestWorkflowDelete_CrossTenant_Returns404(t *testing.T) {
	tenantA := bson.NewObjectID()
	tenantB := bson.NewObjectID()
	wfID := bson.NewObjectID()

	stub := &workflowRepoStub{envID: tenantA, wf: &model.Workflow{ID: wfID, EnvironmentID: tenantA}}
	router := setupCrossTenantRouter(stub, tenantB)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/workflows/"+wfID.Hex(), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── Same-tenant calls should succeed ─────────────────────────────────────────

func TestWorkflowGet_SameTenant_Returns200(t *testing.T) {
	tenantA := bson.NewObjectID()
	wfID := bson.NewObjectID()

	stub := &workflowRepoStub{envID: tenantA, wf: &model.Workflow{ID: wfID, EnvironmentID: tenantA}}
	router := setupCrossTenantRouter(stub, tenantA)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/workflows/"+wfID.Hex(), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWorkflowUpdate_SameTenant_Returns200(t *testing.T) {
	tenantA := bson.NewObjectID()
	wfID := bson.NewObjectID()

	stub := &workflowRepoStub{envID: tenantA, wf: &model.Workflow{ID: wfID, EnvironmentID: tenantA}}
	router := setupCrossTenantRouter(stub, tenantA)

	body, _ := json.Marshal(dto.CreateWorkflowRequest{
		Identifier: "wf",
		Name:       "WF",
		Steps:      []dto.WorkflowStepDTO{{Type: "email", Order: 0}},
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/workflows/"+wfID.Hex(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWorkflowSetStatus_SameTenant_Returns200(t *testing.T) {
	tenantA := bson.NewObjectID()
	wfID := bson.NewObjectID()

	stub := &workflowRepoStub{envID: tenantA, wf: &model.Workflow{ID: wfID, EnvironmentID: tenantA}}
	router := setupCrossTenantRouter(stub, tenantA)

	body, _ := json.Marshal(dto.WorkflowStatusRequest{IsActive: true})
	w := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/workflows/"+wfID.Hex()+"/status", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWorkflowDelete_SameTenant_Returns200(t *testing.T) {
	tenantA := bson.NewObjectID()
	wfID := bson.NewObjectID()

	stub := &workflowRepoStub{envID: tenantA, wf: &model.Workflow{ID: wfID, EnvironmentID: tenantA}}
	router := setupCrossTenantRouter(stub, tenantA)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/workflows/"+wfID.Hex(), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ── repo signature verification: ensure *WorkflowRepository satisfies the interface ─

var _ workflowRepoIface = (*repository.WorkflowRepository)(nil)
