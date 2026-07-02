package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/partiri-cloud/message-in-a-bottle/internal/handler/dto"
	"github.com/partiri-cloud/message-in-a-bottle/internal/middleware"
	"github.com/partiri-cloud/message-in-a-bottle/internal/model"
	"github.com/partiri-cloud/message-in-a-bottle/internal/repository"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// ─── Cross-tenant IDOR tests ──────────────────────────────────────────────────
//
// NotificationHandler is constructed against the narrow notificationRepository
// / activityRepository / subscriberLookupRepository interfaces (see
// notification_handler.go), so these tests drive the real handler with stub
// implementations of those interfaces — no hand-copied handler logic.

type notifRepoStub struct {
	envID bson.ObjectID
	subID bson.ObjectID
	notif *model.Notification
}

func (s *notifRepoStub) FindMany(_ context.Context, _ bson.ObjectID, _, _ int) ([]model.Notification, int64, error) {
	return nil, 0, nil
}

func (s *notifRepoStub) FindByID(_ context.Context, envID, id bson.ObjectID) (*model.Notification, error) {
	if s.notif == nil || s.notif.ID != id || s.envID != envID {
		return nil, mongo.ErrNoDocuments
	}
	return s.notif, nil
}

func (s *notifRepoStub) FindFeed(_ context.Context, _, _ bson.ObjectID, _ repository.FeedFilter, _, _ int) ([]model.Notification, int64, error) {
	return nil, 0, nil
}

func (s *notifRepoStub) MarkSeen(_ context.Context, envID, subID, id bson.ObjectID) error {
	if s.notif == nil || s.notif.ID != id || s.envID != envID || s.subID != subID {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (s *notifRepoStub) MarkRead(_ context.Context, envID, subID, id bson.ObjectID) error {
	if s.notif == nil || s.notif.ID != id || s.envID != envID || s.subID != subID {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (s *notifRepoStub) MarkArchived(_ context.Context, envID, subID, id bson.ObjectID) error {
	if s.notif == nil || s.notif.ID != id || s.envID != envID || s.subID != subID {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (s *notifRepoStub) BulkMarkRead(_ context.Context, envID, subID bson.ObjectID, ids []bson.ObjectID) error {
	if s.notif == nil || s.envID != envID || s.subID != subID {
		return nil
	}
	for _, id := range ids {
		if id == s.notif.ID {
			s.notif.Read = true
		}
	}
	return nil
}

func (s *notifRepoStub) BulkMarkSeen(_ context.Context, envID, subID bson.ObjectID, ids []bson.ObjectID) error {
	if s.notif == nil || s.envID != envID || s.subID != subID {
		return nil
	}
	for _, id := range ids {
		if id == s.notif.ID {
			s.notif.Seen = true
		}
	}
	return nil
}

func (s *notifRepoStub) UnseenCount(_ context.Context, _, _ bson.ObjectID) (int64, error) {
	return 0, nil
}

type activityRepoStub struct{}

func (s *activityRepoStub) FindMany(_ context.Context, _ bson.ObjectID, _, _ int) ([]model.ActivityLog, int64, error) {
	return nil, 0, nil
}

type subRepoStub struct {
	envID        bson.ObjectID
	subscriberID string
	sub          *model.Subscriber
}

func (s *subRepoStub) FindBySubscriberID(_ context.Context, envID bson.ObjectID, subscriberID string) (*model.Subscriber, error) {
	if s.sub == nil || s.subscriberID != subscriberID || s.envID != envID {
		return nil, mongo.ErrNoDocuments
	}
	return s.sub, nil
}

// setupNotificationRouter wires the real NotificationHandler with the given
// stubs and injects envID into the gin context via a lightweight middleware
// shim, mirroring setupCrossTenantRouter in workflow_handler_test.go.
func setupNotificationRouter(notifStub *notifRepoStub, subStub *subRepoStub, callerEnvID bson.ObjectID) *gin.Engine {
	h := &NotificationHandler{notifRepo: notifStub, activityRepo: &activityRepoStub{}, subRepo: subStub}
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(middleware.ContextKeyEnvironmentID, callerEnvID)
		c.Next()
	})
	r.GET("/notifications/:id", h.Get)
	r.POST("/subscribers/:subscriberId/feed/:notifId/seen", h.MarkSeen)
	r.POST("/subscribers/:subscriberId/feed/:notifId/read", h.MarkRead)
	r.POST("/subscribers/:subscriberId/feed/:notifId/archive", h.Archive)
	r.POST("/subscribers/:subscriberId/feed/bulk-action", h.BulkAction)
	return r
}

// ── Get ──────────────────────────────────────────────────────────────────────

func TestNotificationGet_CrossTenant_Returns404(t *testing.T) {
	tenantA := bson.NewObjectID()
	tenantB := bson.NewObjectID()
	notifID := bson.NewObjectID()

	stub := &notifRepoStub{envID: tenantA, notif: &model.Notification{ID: notifID, EnvironmentID: tenantA}}
	router := setupNotificationRouter(stub, &subRepoStub{}, tenantB)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/notifications/"+notifID.Hex(), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestNotificationGet_NotFound_Returns404(t *testing.T) {
	tenantA := bson.NewObjectID()
	missingID := bson.NewObjectID()

	stub := &notifRepoStub{envID: tenantA, notif: nil}
	router := setupNotificationRouter(stub, &subRepoStub{}, tenantA)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/notifications/"+missingID.Hex(), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestNotificationGet_SameTenant_Returns200(t *testing.T) {
	tenantA := bson.NewObjectID()
	notifID := bson.NewObjectID()

	stub := &notifRepoStub{envID: tenantA, notif: &model.Notification{ID: notifID, EnvironmentID: tenantA}}
	router := setupNotificationRouter(stub, &subRepoStub{}, tenantA)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/notifications/"+notifID.Hex(), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ── MarkSeen ─────────────────────────────────────────────────────────────────

func TestNotificationMarkSeen_SubscriberNotFound_Returns404(t *testing.T) {
	tenantA := bson.NewObjectID()
	notifID := bson.NewObjectID()

	notifStub := &notifRepoStub{envID: tenantA, notif: &model.Notification{ID: notifID, EnvironmentID: tenantA}}
	router := setupNotificationRouter(notifStub, &subRepoStub{}, tenantA)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/subscribers/usr_1/feed/"+notifID.Hex()+"/seen", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestNotificationMarkSeen_ForeignSubscriber_Returns404(t *testing.T) {
	tenantA := bson.NewObjectID()
	notifID := bson.NewObjectID()
	ownerSubID := bson.NewObjectID()
	callerSubID := bson.NewObjectID()

	notifStub := &notifRepoStub{envID: tenantA, subID: ownerSubID, notif: &model.Notification{ID: notifID, EnvironmentID: tenantA}}
	subStub := &subRepoStub{envID: tenantA, subscriberID: "usr_1", sub: &model.Subscriber{ID: callerSubID, EnvironmentID: tenantA, SubscriberID: "usr_1"}}
	router := setupNotificationRouter(notifStub, subStub, tenantA)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/subscribers/usr_1/feed/"+notifID.Hex()+"/seen", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestNotificationMarkSeen_SameTenantAndSubscriber_Returns200(t *testing.T) {
	tenantA := bson.NewObjectID()
	notifID := bson.NewObjectID()
	subID := bson.NewObjectID()

	notifStub := &notifRepoStub{envID: tenantA, subID: subID, notif: &model.Notification{ID: notifID, EnvironmentID: tenantA}}
	subStub := &subRepoStub{envID: tenantA, subscriberID: "usr_1", sub: &model.Subscriber{ID: subID, EnvironmentID: tenantA, SubscriberID: "usr_1"}}
	router := setupNotificationRouter(notifStub, subStub, tenantA)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/subscribers/usr_1/feed/"+notifID.Hex()+"/seen", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ── MarkRead ─────────────────────────────────────────────────────────────────

func TestNotificationMarkRead_SubscriberNotFound_Returns404(t *testing.T) {
	tenantA := bson.NewObjectID()
	notifID := bson.NewObjectID()

	notifStub := &notifRepoStub{envID: tenantA, notif: &model.Notification{ID: notifID, EnvironmentID: tenantA}}
	subStub := &subRepoStub{} // no subscriber seeded, always ErrNoDocuments
	router := setupNotificationRouter(notifStub, subStub, tenantA)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/subscribers/usr_1/feed/"+notifID.Hex()+"/read", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestNotificationMarkRead_ForeignSubscriber_Returns404(t *testing.T) {
	tenantA := bson.NewObjectID()
	notifID := bson.NewObjectID()
	ownerSubID := bson.NewObjectID()
	callerSubID := bson.NewObjectID()

	notifStub := &notifRepoStub{envID: tenantA, subID: ownerSubID, notif: &model.Notification{ID: notifID, EnvironmentID: tenantA}}
	subStub := &subRepoStub{envID: tenantA, subscriberID: "usr_1", sub: &model.Subscriber{ID: callerSubID, EnvironmentID: tenantA, SubscriberID: "usr_1"}}
	router := setupNotificationRouter(notifStub, subStub, tenantA)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/subscribers/usr_1/feed/"+notifID.Hex()+"/read", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestNotificationMarkRead_SameTenantAndSubscriber_Returns200(t *testing.T) {
	tenantA := bson.NewObjectID()
	notifID := bson.NewObjectID()
	subID := bson.NewObjectID()

	notifStub := &notifRepoStub{envID: tenantA, subID: subID, notif: &model.Notification{ID: notifID, EnvironmentID: tenantA}}
	subStub := &subRepoStub{envID: tenantA, subscriberID: "usr_1", sub: &model.Subscriber{ID: subID, EnvironmentID: tenantA, SubscriberID: "usr_1"}}
	router := setupNotificationRouter(notifStub, subStub, tenantA)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/subscribers/usr_1/feed/"+notifID.Hex()+"/read", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ── Archive ──────────────────────────────────────────────────────────────────

func TestNotificationArchive_ForeignSubscriber_Returns404(t *testing.T) {
	tenantA := bson.NewObjectID()
	notifID := bson.NewObjectID()
	ownerSubID := bson.NewObjectID()
	callerSubID := bson.NewObjectID()

	notifStub := &notifRepoStub{envID: tenantA, subID: ownerSubID, notif: &model.Notification{ID: notifID, EnvironmentID: tenantA}}
	subStub := &subRepoStub{envID: tenantA, subscriberID: "usr_1", sub: &model.Subscriber{ID: callerSubID, EnvironmentID: tenantA, SubscriberID: "usr_1"}}
	router := setupNotificationRouter(notifStub, subStub, tenantA)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/subscribers/usr_1/feed/"+notifID.Hex()+"/archive", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestNotificationArchive_SameTenantAndSubscriber_Returns200(t *testing.T) {
	tenantA := bson.NewObjectID()
	notifID := bson.NewObjectID()
	subID := bson.NewObjectID()

	notifStub := &notifRepoStub{envID: tenantA, subID: subID, notif: &model.Notification{ID: notifID, EnvironmentID: tenantA}}
	subStub := &subRepoStub{envID: tenantA, subscriberID: "usr_1", sub: &model.Subscriber{ID: subID, EnvironmentID: tenantA, SubscriberID: "usr_1"}}
	router := setupNotificationRouter(notifStub, subStub, tenantA)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/subscribers/usr_1/feed/"+notifID.Hex()+"/archive", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ── BulkAction ───────────────────────────────────────────────────────────────

func TestNotificationBulkAction_SubscriberNotFound_Returns404(t *testing.T) {
	tenantA := bson.NewObjectID()
	notifID := bson.NewObjectID()

	notifStub := &notifRepoStub{envID: tenantA, notif: &model.Notification{ID: notifID, EnvironmentID: tenantA}}
	subStub := &subRepoStub{} // no subscriber seeded
	router := setupNotificationRouter(notifStub, subStub, tenantA)

	body, _ := json.Marshal(dto.BulkActionRequest{Action: "read", NotificationIDs: []string{notifID.Hex()}})
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/subscribers/usr_1/feed/bulk-action", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestNotificationBulkAction_SameTenantAndSubscriber_MarksOwnedOnly(t *testing.T) {
	tenantA := bson.NewObjectID()
	notifID := bson.NewObjectID()
	subID := bson.NewObjectID()

	notifStub := &notifRepoStub{envID: tenantA, subID: subID, notif: &model.Notification{ID: notifID, EnvironmentID: tenantA}}
	subStub := &subRepoStub{envID: tenantA, subscriberID: "usr_1", sub: &model.Subscriber{ID: subID, EnvironmentID: tenantA, SubscriberID: "usr_1"}}
	router := setupNotificationRouter(notifStub, subStub, tenantA)

	foreignID := bson.NewObjectID()
	body, _ := json.Marshal(dto.BulkActionRequest{Action: "read", NotificationIDs: []string{notifID.Hex(), foreignID.Hex()}})
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/subscribers/usr_1/feed/bulk-action", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, notifStub.notif.Read)
}

// ── repo signature verification: ensure concrete repos satisfy the interfaces ─

var _ notificationRepository = (*repository.NotificationRepository)(nil)
var _ activityRepository = (*repository.ActivityRepository)(nil)
var _ subscriberLookupRepository = (*repository.SubscriberRepository)(nil)
