package repository

import (
	"context"
	"testing"
	"time"

	"github.com/partiri-cloud/message-in-a-bottle/internal/model"
	"github.com/partiri-cloud/message-in-a-bottle/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func TestNotificationRepo_Create(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envID, _ := testutil.SeedEnvironmentDoc(t, db, "test-env")
	subID := testutil.SeedSubscriberDoc(t, db, envID, "usr_notif")
	repo := NewNotificationRepository(db)

	notif := &model.Notification{
		EnvironmentID: envID,
		SubscriberID:  subID,
		WorkflowID:    bson.NewObjectID(),
		TransactionID: "tx_001",
		Payload:       map[string]any{"msg": "hello"},
		Channels:      []model.ChannelDelivery{{Channel: "email", Status: "pending"}},
		ExpireAt:      time.Now().Add(90 * 24 * time.Hour),
	}

	err := repo.Create(context.Background(), notif)
	require.NoError(t, err)
	assert.False(t, notif.ID.IsZero())
	assert.False(t, notif.CreatedAt.IsZero())
}

func TestNotificationRepo_FindByTransactionID(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envID, _ := testutil.SeedEnvironmentDoc(t, db, "test-env")
	subID := testutil.SeedSubscriberDoc(t, db, envID, "usr_tx")
	repo := NewNotificationRepository(db)

	notif := &model.Notification{
		EnvironmentID: envID,
		SubscriberID:  subID,
		WorkflowID:    bson.NewObjectID(),
		TransactionID: "tx_unique_123",
		ExpireAt:      time.Now().Add(90 * 24 * time.Hour),
	}
	require.NoError(t, repo.Create(context.Background(), notif))

	found, err := repo.FindByTransactionID(context.Background(), envID, "tx_unique_123")
	require.NoError(t, err)
	assert.Equal(t, notif.ID, found.ID)
}

func TestNotificationRepo_MarkSeen(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envID, _ := testutil.SeedEnvironmentDoc(t, db, "test-env")
	subID := testutil.SeedSubscriberDoc(t, db, envID, "usr_seen")
	repo := NewNotificationRepository(db)

	notif := &model.Notification{
		EnvironmentID: envID,
		SubscriberID:  subID,
		WorkflowID:    bson.NewObjectID(),
		TransactionID: "tx_seen",
		ExpireAt:      time.Now().Add(90 * 24 * time.Hour),
	}
	require.NoError(t, repo.Create(context.Background(), notif))

	err := repo.MarkSeen(context.Background(), envID, subID, notif.ID)
	require.NoError(t, err)

	found, _ := repo.FindByID(context.Background(), envID, notif.ID)
	assert.True(t, found.Seen)
	assert.NotNil(t, found.SeenAt)
}

func TestNotificationRepo_MarkRead(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envID, _ := testutil.SeedEnvironmentDoc(t, db, "test-env")
	subID := testutil.SeedSubscriberDoc(t, db, envID, "usr_read")
	repo := NewNotificationRepository(db)

	notif := &model.Notification{
		EnvironmentID: envID,
		SubscriberID:  subID,
		WorkflowID:    bson.NewObjectID(),
		TransactionID: "tx_read",
		ExpireAt:      time.Now().Add(90 * 24 * time.Hour),
	}
	require.NoError(t, repo.Create(context.Background(), notif))

	err := repo.MarkRead(context.Background(), envID, subID, notif.ID)
	require.NoError(t, err)

	found, _ := repo.FindByID(context.Background(), envID, notif.ID)
	assert.True(t, found.Read)
	assert.True(t, found.Seen)
	assert.NotNil(t, found.ReadAt)
}

func TestNotificationRepo_UnseenCount(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envID, _ := testutil.SeedEnvironmentDoc(t, db, "test-env")
	subID := testutil.SeedSubscriberDoc(t, db, envID, "usr_count")
	repo := NewNotificationRepository(db)

	wfID := bson.NewObjectID()
	for i := 0; i < 5; i++ {
		notif := &model.Notification{
			EnvironmentID: envID,
			SubscriberID:  subID,
			WorkflowID:    wfID,
			TransactionID: bson.NewObjectID().Hex(),
			ExpireAt:      time.Now().Add(90 * 24 * time.Hour),
		}
		require.NoError(t, repo.Create(context.Background(), notif))
	}

	count, err := repo.UnseenCount(context.Background(), envID, subID)
	require.NoError(t, err)
	assert.Equal(t, int64(5), count)

	// Mark 2 as seen
	notifs, _, _ := repo.FindFeed(context.Background(), envID, subID, FeedFilter{}, 1, 10)
	repo.MarkSeen(context.Background(), envID, subID, notifs[0].ID)
	repo.MarkSeen(context.Background(), envID, subID, notifs[1].ID)

	count, _ = repo.UnseenCount(context.Background(), envID, subID)
	assert.Equal(t, int64(3), count)
}

func TestNotificationRepo_FindFeed_FilterBySeen(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envID, _ := testutil.SeedEnvironmentDoc(t, db, "test-env")
	subID := testutil.SeedSubscriberDoc(t, db, envID, "usr_feed")
	repo := NewNotificationRepository(db)

	wfID := bson.NewObjectID()
	for i := 0; i < 3; i++ {
		notif := &model.Notification{
			EnvironmentID: envID,
			SubscriberID:  subID,
			WorkflowID:    wfID,
			TransactionID: bson.NewObjectID().Hex(),
			ExpireAt:      time.Now().Add(90 * 24 * time.Hour),
		}
		require.NoError(t, repo.Create(context.Background(), notif))
	}

	all, _, _ := repo.FindFeed(context.Background(), envID, subID, FeedFilter{}, 1, 10)
	repo.MarkSeen(context.Background(), envID, subID, all[0].ID)

	f := false
	unseen, total, err := repo.FindFeed(context.Background(), envID, subID, FeedFilter{Seen: &f}, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, unseen, 2)
}

func TestNotificationRepo_BulkMarkRead(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envID, _ := testutil.SeedEnvironmentDoc(t, db, "test-env")
	subID := testutil.SeedSubscriberDoc(t, db, envID, "usr_bulk")
	repo := NewNotificationRepository(db)

	wfID := bson.NewObjectID()
	var ids []bson.ObjectID
	for i := 0; i < 3; i++ {
		notif := &model.Notification{
			EnvironmentID: envID,
			SubscriberID:  subID,
			WorkflowID:    wfID,
			TransactionID: bson.NewObjectID().Hex(),
			ExpireAt:      time.Now().Add(90 * 24 * time.Hour),
		}
		require.NoError(t, repo.Create(context.Background(), notif))
		ids = append(ids, notif.ID)
	}

	err := repo.BulkMarkRead(context.Background(), envID, subID, ids[:2])
	require.NoError(t, err)

	for _, id := range ids[:2] {
		n, _ := repo.FindByID(context.Background(), envID, id)
		assert.True(t, n.Read)
	}
	n, _ := repo.FindByID(context.Background(), envID, ids[2])
	assert.False(t, n.Read)
}

// ─── Cross-tenant IDOR tests ──────────────────────────────────────────────────

func TestNotificationRepo_FindByID_CrossEnv_ReturnsNotFound(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envA, _ := testutil.SeedEnvironmentDoc(t, db, "env-a")
	envB, _ := testutil.SeedEnvironmentDoc(t, db, "env-b")
	subID := testutil.SeedSubscriberDoc(t, db, envA, "usr_cross")
	repo := NewNotificationRepository(db)

	notif := &model.Notification{
		EnvironmentID: envA,
		SubscriberID:  subID,
		WorkflowID:    bson.NewObjectID(),
		TransactionID: "tx_cross_find",
		ExpireAt:      time.Now().Add(90 * 24 * time.Hour),
	}
	require.NoError(t, repo.Create(context.Background(), notif))

	_, err := repo.FindByID(context.Background(), envB, notif.ID)
	assert.ErrorIs(t, err, mongo.ErrNoDocuments)

	found, err := repo.FindByID(context.Background(), envA, notif.ID)
	require.NoError(t, err)
	assert.Equal(t, notif.ID, found.ID)
}

func TestNotificationRepo_MarkRead_ForeignEnv_ReturnsNotFoundAndLeavesUnchanged(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envA, _ := testutil.SeedEnvironmentDoc(t, db, "env-a")
	envB, _ := testutil.SeedEnvironmentDoc(t, db, "env-b")
	subID := testutil.SeedSubscriberDoc(t, db, envA, "usr_read_cross")
	repo := NewNotificationRepository(db)

	notif := &model.Notification{
		EnvironmentID: envA,
		SubscriberID:  subID,
		WorkflowID:    bson.NewObjectID(),
		TransactionID: "tx_read_cross",
		ExpireAt:      time.Now().Add(90 * 24 * time.Hour),
	}
	require.NoError(t, repo.Create(context.Background(), notif))

	err := repo.MarkRead(context.Background(), envB, subID, notif.ID)
	assert.ErrorIs(t, err, mongo.ErrNoDocuments)

	found, _ := repo.FindByID(context.Background(), envA, notif.ID)
	assert.False(t, found.Read)
	assert.False(t, found.Seen)
}

func TestNotificationRepo_MarkRead_ForeignSubscriber_ReturnsNotFoundAndLeavesUnchanged(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envID, _ := testutil.SeedEnvironmentDoc(t, db, "test-env")
	ownerSubID := testutil.SeedSubscriberDoc(t, db, envID, "usr_owner")
	foreignSubID := testutil.SeedSubscriberDoc(t, db, envID, "usr_foreign")
	repo := NewNotificationRepository(db)

	notif := &model.Notification{
		EnvironmentID: envID,
		SubscriberID:  ownerSubID,
		WorkflowID:    bson.NewObjectID(),
		TransactionID: "tx_read_foreign_sub",
		ExpireAt:      time.Now().Add(90 * 24 * time.Hour),
	}
	require.NoError(t, repo.Create(context.Background(), notif))

	err := repo.MarkRead(context.Background(), envID, foreignSubID, notif.ID)
	assert.ErrorIs(t, err, mongo.ErrNoDocuments)

	found, _ := repo.FindByID(context.Background(), envID, notif.ID)
	assert.False(t, found.Read)
}

func TestNotificationRepo_MarkSeen_Foreign_ReturnsNotFoundAndLeavesUnchanged(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envA, _ := testutil.SeedEnvironmentDoc(t, db, "env-a")
	envB, _ := testutil.SeedEnvironmentDoc(t, db, "env-b")
	subID := testutil.SeedSubscriberDoc(t, db, envA, "usr_seen_cross")
	repo := NewNotificationRepository(db)

	notif := &model.Notification{
		EnvironmentID: envA,
		SubscriberID:  subID,
		WorkflowID:    bson.NewObjectID(),
		TransactionID: "tx_seen_cross",
		ExpireAt:      time.Now().Add(90 * 24 * time.Hour),
	}
	require.NoError(t, repo.Create(context.Background(), notif))

	err := repo.MarkSeen(context.Background(), envB, subID, notif.ID)
	assert.ErrorIs(t, err, mongo.ErrNoDocuments)

	found, _ := repo.FindByID(context.Background(), envA, notif.ID)
	assert.False(t, found.Seen)
}

func TestNotificationRepo_MarkArchived_Foreign_ReturnsNotFoundAndLeavesUnchanged(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envA, _ := testutil.SeedEnvironmentDoc(t, db, "env-a")
	envB, _ := testutil.SeedEnvironmentDoc(t, db, "env-b")
	subID := testutil.SeedSubscriberDoc(t, db, envA, "usr_archive_cross")
	repo := NewNotificationRepository(db)

	notif := &model.Notification{
		EnvironmentID: envA,
		SubscriberID:  subID,
		WorkflowID:    bson.NewObjectID(),
		TransactionID: "tx_archive_cross",
		ExpireAt:      time.Now().Add(90 * 24 * time.Hour),
	}
	require.NoError(t, repo.Create(context.Background(), notif))

	err := repo.MarkArchived(context.Background(), envB, subID, notif.ID)
	assert.ErrorIs(t, err, mongo.ErrNoDocuments)

	found, _ := repo.FindByID(context.Background(), envA, notif.ID)
	assert.Nil(t, found.ArchivedAt)
}

func TestNotificationRepo_BulkMarkRead_SkipsForeignIDs(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envA, _ := testutil.SeedEnvironmentDoc(t, db, "env-a")
	envB, _ := testutil.SeedEnvironmentDoc(t, db, "env-b")
	subA := testutil.SeedSubscriberDoc(t, db, envA, "usr_bulk_a")
	subB := testutil.SeedSubscriberDoc(t, db, envB, "usr_bulk_b")
	repo := NewNotificationRepository(db)

	notifA := &model.Notification{
		EnvironmentID: envA,
		SubscriberID:  subA,
		WorkflowID:    bson.NewObjectID(),
		TransactionID: "tx_bulk_a",
		ExpireAt:      time.Now().Add(90 * 24 * time.Hour),
	}
	require.NoError(t, repo.Create(context.Background(), notifA))

	notifB := &model.Notification{
		EnvironmentID: envB,
		SubscriberID:  subB,
		WorkflowID:    bson.NewObjectID(),
		TransactionID: "tx_bulk_b",
		ExpireAt:      time.Now().Add(90 * 24 * time.Hour),
	}
	require.NoError(t, repo.Create(context.Background(), notifB))

	err := repo.BulkMarkRead(context.Background(), envA, subA, []bson.ObjectID{notifA.ID, notifB.ID})
	require.NoError(t, err)

	foundA, _ := repo.FindByID(context.Background(), envA, notifA.ID)
	assert.True(t, foundA.Read)

	foundB, _ := repo.FindByID(context.Background(), envB, notifB.ID)
	assert.False(t, foundB.Read)
}

func TestNotificationRepo_BulkMarkSeen_SkipsForeignIDs(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envA, _ := testutil.SeedEnvironmentDoc(t, db, "env-a")
	envB, _ := testutil.SeedEnvironmentDoc(t, db, "env-b")
	subA := testutil.SeedSubscriberDoc(t, db, envA, "usr_bulk_seen_a")
	subB := testutil.SeedSubscriberDoc(t, db, envB, "usr_bulk_seen_b")
	repo := NewNotificationRepository(db)

	notifA := &model.Notification{
		EnvironmentID: envA,
		SubscriberID:  subA,
		WorkflowID:    bson.NewObjectID(),
		TransactionID: "tx_bulk_seen_a",
		ExpireAt:      time.Now().Add(90 * 24 * time.Hour),
	}
	require.NoError(t, repo.Create(context.Background(), notifA))

	notifB := &model.Notification{
		EnvironmentID: envB,
		SubscriberID:  subB,
		WorkflowID:    bson.NewObjectID(),
		TransactionID: "tx_bulk_seen_b",
		ExpireAt:      time.Now().Add(90 * 24 * time.Hour),
	}
	require.NoError(t, repo.Create(context.Background(), notifB))

	err := repo.BulkMarkSeen(context.Background(), envA, subA, []bson.ObjectID{notifA.ID, notifB.ID})
	require.NoError(t, err)

	foundA, _ := repo.FindByID(context.Background(), envA, notifA.ID)
	assert.True(t, foundA.Seen)

	foundB, _ := repo.FindByID(context.Background(), envB, notifB.ID)
	assert.False(t, foundB.Seen)
}

// ─── Idempotency: re-marking an owned, already-marked notification must not 404 ─

func TestNotificationRepo_MarkSeen_Idempotent_SecondCallReturnsNil(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envID, _ := testutil.SeedEnvironmentDoc(t, db, "test-env")
	subID := testutil.SeedSubscriberDoc(t, db, envID, "usr_seen_idem")
	repo := NewNotificationRepository(db)

	notif := &model.Notification{
		EnvironmentID: envID,
		SubscriberID:  subID,
		WorkflowID:    bson.NewObjectID(),
		TransactionID: "tx_seen_idem",
		ExpireAt:      time.Now().Add(90 * 24 * time.Hour),
	}
	require.NoError(t, repo.Create(context.Background(), notif))

	require.NoError(t, repo.MarkSeen(context.Background(), envID, subID, notif.ID))

	err := repo.MarkSeen(context.Background(), envID, subID, notif.ID)
	assert.NoError(t, err)

	found, _ := repo.FindByID(context.Background(), envID, notif.ID)
	assert.True(t, found.Seen)
}

func TestNotificationRepo_MarkRead_Idempotent_SecondCallReturnsNil(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envID, _ := testutil.SeedEnvironmentDoc(t, db, "test-env")
	subID := testutil.SeedSubscriberDoc(t, db, envID, "usr_read_idem")
	repo := NewNotificationRepository(db)

	notif := &model.Notification{
		EnvironmentID: envID,
		SubscriberID:  subID,
		WorkflowID:    bson.NewObjectID(),
		TransactionID: "tx_read_idem",
		ExpireAt:      time.Now().Add(90 * 24 * time.Hour),
	}
	require.NoError(t, repo.Create(context.Background(), notif))

	require.NoError(t, repo.MarkRead(context.Background(), envID, subID, notif.ID))

	err := repo.MarkRead(context.Background(), envID, subID, notif.ID)
	assert.NoError(t, err)

	found, _ := repo.FindByID(context.Background(), envID, notif.ID)
	assert.True(t, found.Read)
}
