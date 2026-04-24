package repository

import (
	"context"
	"testing"

	"github.com/partiri-cloud/message-in-a-bottle/internal/model"
	"github.com/partiri-cloud/message-in-a-bottle/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func TestSubscriberRepo_Upsert_Create(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envID, _ := testutil.SeedEnvironmentDoc(t, db, "test-env")
	repo := NewSubscriberRepository(db)

	sub := &model.Subscriber{
		SubscriberID: "usr_001",
		Email:        "test@example.com",
		FirstName:    "Alice",
		Locale:       "en",
	}

	err := repo.Upsert(context.Background(), envID, sub)
	require.NoError(t, err)

	found, err := repo.FindBySubscriberID(context.Background(), envID, "usr_001")
	require.NoError(t, err)
	assert.Equal(t, "Alice", found.FirstName)
	assert.Equal(t, "test@example.com", found.Email)
	assert.False(t, found.ID.IsZero())
}

func TestSubscriberRepo_Upsert_Update(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envID, _ := testutil.SeedEnvironmentDoc(t, db, "test-env")
	repo := NewSubscriberRepository(db)

	sub := &model.Subscriber{SubscriberID: "usr_002", FirstName: "Bob", Locale: "en"}
	require.NoError(t, repo.Upsert(context.Background(), envID, sub))

	sub.FirstName = "Robert"
	require.NoError(t, repo.Upsert(context.Background(), envID, sub))

	found, err := repo.FindBySubscriberID(context.Background(), envID, "usr_002")
	require.NoError(t, err)
	assert.Equal(t, "Robert", found.FirstName)
}

func TestSubscriberRepo_BulkUpsert(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envID, _ := testutil.SeedEnvironmentDoc(t, db, "test-env")
	repo := NewSubscriberRepository(db)

	subs := []model.Subscriber{
		{SubscriberID: "bulk_1", FirstName: "One", Locale: "en"},
		{SubscriberID: "bulk_2", FirstName: "Two", Locale: "en"},
		{SubscriberID: "bulk_3", FirstName: "Three", Locale: "en"},
	}

	err := repo.BulkUpsert(context.Background(), envID, subs)
	require.NoError(t, err)

	result, total, err := repo.FindMany(context.Background(), envID, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, result, 3)
}

func TestSubscriberRepo_FindBySubscriberID_NotFound(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envID, _ := testutil.SeedEnvironmentDoc(t, db, "test-env")
	repo := NewSubscriberRepository(db)

	_, err := repo.FindBySubscriberID(context.Background(), envID, "nonexistent")
	assert.ErrorIs(t, err, mongo.ErrNoDocuments)
}

func TestSubscriberRepo_FindMany_Pagination(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envID, _ := testutil.SeedEnvironmentDoc(t, db, "test-env")
	repo := NewSubscriberRepository(db)

	for i := 0; i < 5; i++ {
		sub := &model.Subscriber{SubscriberID: bson.NewObjectID().Hex(), Locale: "en"}
		require.NoError(t, repo.Upsert(context.Background(), envID, sub))
	}

	page1, total, err := repo.FindMany(context.Background(), envID, 1, 2)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, page1, 2)

	page3, _, err := repo.FindMany(context.Background(), envID, 3, 2)
	require.NoError(t, err)
	assert.Len(t, page3, 1)
}

func TestSubscriberRepo_Delete(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envID, _ := testutil.SeedEnvironmentDoc(t, db, "test-env")
	repo := NewSubscriberRepository(db)

	sub := &model.Subscriber{SubscriberID: "to_delete", Locale: "en"}
	require.NoError(t, repo.Upsert(context.Background(), envID, sub))

	err := repo.Delete(context.Background(), envID, "to_delete")
	require.NoError(t, err)

	_, err = repo.FindBySubscriberID(context.Background(), envID, "to_delete")
	assert.ErrorIs(t, err, mongo.ErrNoDocuments)
}

func TestSubscriberRepo_SetOnlineStatus(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envID, _ := testutil.SeedEnvironmentDoc(t, db, "test-env")
	subID := testutil.SeedSubscriberDoc(t, db, envID, "usr_online")
	repo := NewSubscriberRepository(db)

	err := repo.SetOnlineStatus(context.Background(), subID, true)
	require.NoError(t, err)

	found, _ := repo.FindByID(context.Background(), subID)
	assert.True(t, found.IsOnline)
	assert.NotNil(t, found.LastOnlineAt)
}
