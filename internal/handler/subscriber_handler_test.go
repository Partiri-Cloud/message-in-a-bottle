package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/partiri-cloud/message-in-a-bottle/internal/middleware"
	"github.com/partiri-cloud/message-in-a-bottle/internal/repository"
	"github.com/partiri-cloud/message-in-a-bottle/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// SubscriberHandler takes concrete repositories, so these drive it against a real
// MongoDB (skipped when one is not reachable). The behaviour under test — that a
// re-post preserves state the caller did not send — is a property of the $set /
// $setOnInsert split, which only a real server can adjudicate.

func setupSubscriberRouter(t *testing.T) (*gin.Engine, *repository.SubscriberRepository, bson.ObjectID) {
	t.Helper()
	db, cleanup := testutil.SetupTestDB(t)
	t.Cleanup(cleanup)

	envID, _ := testutil.SeedEnvironmentDoc(t, db, "test-env")
	subRepo := repository.NewSubscriberRepository(db)
	h := NewSubscriberHandler(subRepo, repository.NewTopicSubscriberRepository(db))

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(middleware.ContextKeyEnvironmentID, envID)
		c.Next()
	})
	r.POST("/subscribers", h.Create)
	r.POST("/subscribers/bulk", h.BulkCreate)
	return r, subRepo, envID
}

func postJSON(t *testing.T, router *gin.Engine, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// The handler used to force Locale = "en" when the payload omitted it. Because
// this is an upsert — and Harbor re-posts a subscriber on every page load — that
// stomped a German user back to English every time they opened the dashboard.
func TestSubscriberCreate_RepostDoesNotStompLocale(t *testing.T) {
	router, repo, envID := setupSubscriberRouter(t)

	require.Equal(t, http.StatusCreated,
		postJSON(t, router, "/subscribers", `{"subscriberId":"usr_1","email":"a@example.com","locale":"de"}`).Code)

	// Harbor's backfill payload: id + email, no locale.
	require.Equal(t, http.StatusCreated,
		postJSON(t, router, "/subscribers", `{"subscriberId":"usr_1","email":"a@example.com"}`).Code)

	found, err := repo.FindBySubscriberID(context.Background(), envID, "usr_1")
	require.NoError(t, err)
	assert.Equal(t, "de", found.Locale)
}

func TestSubscriberCreate_NewSubscriberStillGetsDefaultLocale(t *testing.T) {
	router, repo, envID := setupSubscriberRouter(t)

	require.Equal(t, http.StatusCreated,
		postJSON(t, router, "/subscribers", `{"subscriberId":"usr_2","email":"b@example.com"}`).Code)

	found, err := repo.FindBySubscriberID(context.Background(), envID, "usr_2")
	require.NoError(t, err)
	assert.Equal(t, "en", found.Locale, "the repository seeds the default on insert")
}

func TestSubscriberBulkCreate_RepostDoesNotStompLocale(t *testing.T) {
	router, repo, envID := setupSubscriberRouter(t)

	require.Equal(t, http.StatusCreated, postJSON(t, router, "/subscribers/bulk",
		`{"subscribers":[{"subscriberId":"usr_3","locale":"fr"}]}`).Code)

	require.Equal(t, http.StatusCreated, postJSON(t, router, "/subscribers/bulk",
		`{"subscribers":[{"subscriberId":"usr_3","email":"c@example.com"}]}`).Code)

	found, err := repo.FindBySubscriberID(context.Background(), envID, "usr_3")
	require.NoError(t, err)
	assert.Equal(t, "fr", found.Locale, "bulk must match Create — it used to force \"en\" here")
	assert.Equal(t, "c@example.com", found.Email)
}

// The response body is what Harbor and the SDK parse. On the re-post path the
// handler used to echo back the caller's sparse struct — zero id, blank locale —
// because Upsert only populated those fields when it actually inserted.
func TestSubscriberCreate_RepostResponseReportsStoredState(t *testing.T) {
	router, _, _ := setupSubscriberRouter(t)

	require.Equal(t, http.StatusCreated,
		postJSON(t, router, "/subscribers", `{"subscriberId":"usr_4","email":"d@example.com","locale":"de"}`).Code)

	w := postJSON(t, router, "/subscribers", `{"subscriberId":"usr_4","email":"d@example.com"}`)
	require.Equal(t, http.StatusCreated, w.Code)

	var body struct {
		Data struct {
			ID     string `json:"id"`
			Locale string `json:"locale"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	assert.NotEqual(t, bson.NilObjectID.Hex(), body.Data.ID, "must not report a zero id for an existing subscriber")
	assert.Equal(t, "de", body.Data.Locale, "must report the stored locale, not the blank one that was sent")
}
