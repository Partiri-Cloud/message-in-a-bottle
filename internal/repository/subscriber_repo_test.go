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

// Harbor re-posts a subscriber on every notification-token mint to backfill
// users who never got one. That upsert must not destroy state it did not send:
// $set on a nested document replaces it wholesale, so an empty `channels` would
// wipe the subscriber's push tokens and webhooks on every page load.
func TestSubscriberRepo_Upsert_SparseRepostPreservesChannels(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envID, _ := testutil.SeedEnvironmentDoc(t, db, "test-env")
	repo := NewSubscriberRepository(db)

	full := &model.Subscriber{
		SubscriberID: "usr_channels",
		Email:        "c@example.com",
		Locale:       "de",
		Channels: model.SubscriberChannels{
			Push:  model.PushChannelConfig{FCMTokens: []string{"tok-1"}},
			Slack: model.SlackConfig{WebhookURL: "https://hooks.slack.com/services/T/B/X"},
		},
	}
	require.NoError(t, repo.Upsert(context.Background(), envID, full))

	// The backfill payload Harbor sends: id + email, nothing else.
	require.NoError(t, repo.Upsert(context.Background(), envID, &model.Subscriber{
		SubscriberID: "usr_channels",
		Email:        "c@example.com",
	}))

	found, err := repo.FindBySubscriberID(context.Background(), envID, "usr_channels")
	require.NoError(t, err)
	assert.Equal(t, []string{"tok-1"}, found.Channels.Push.FCMTokens, "push tokens must survive a sparse re-post")
	assert.Equal(t, "https://hooks.slack.com/services/T/B/X", found.Channels.Slack.WebhookURL, "slack webhook must survive a sparse re-post")
	assert.Equal(t, "de", found.Locale, "locale must not be stomped back to the default")
}

// Channel config merges per field, not per subdocument: a re-post that carries
// only a Slack webhook must not erase the push tokens stored earlier. A
// wholesale $set of `channels` would.
func TestSubscriberRepo_Upsert_PartialChannelPostMergesPerField(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envID, _ := testutil.SeedEnvironmentDoc(t, db, "test-env")
	repo := NewSubscriberRepository(db)

	require.NoError(t, repo.Upsert(context.Background(), envID, &model.Subscriber{
		SubscriberID: "usr_partial",
		Channels: model.SubscriberChannels{
			Push: model.PushChannelConfig{FCMTokens: []string{"tok-1"}},
		},
	}))

	// A later post carrying only a Slack webhook.
	require.NoError(t, repo.Upsert(context.Background(), envID, &model.Subscriber{
		SubscriberID: "usr_partial",
		Channels: model.SubscriberChannels{
			Slack: model.SlackConfig{WebhookURL: "https://hooks.slack.com/services/T/B/X"},
		},
	}))

	found, err := repo.FindBySubscriberID(context.Background(), envID, "usr_partial")
	require.NoError(t, err)
	assert.Equal(t, []string{"tok-1"}, found.Channels.Push.FCMTokens, "push tokens must survive a slack-only post")
	assert.Equal(t, "https://hooks.slack.com/services/T/B/X", found.Channels.Slack.WebhookURL, "the slack webhook that was supplied is stored")
}

// Push tokens accumulate across devices. A wholesale $set of the token array
// would evict device A's token the moment device B registers its own, silently
// stopping push to the first device.
func TestSubscriberRepo_Upsert_PushTokensMergeAcrossDevices(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envID, _ := testutil.SeedEnvironmentDoc(t, db, "test-env")
	repo := NewSubscriberRepository(db)

	require.NoError(t, repo.Upsert(context.Background(), envID, &model.Subscriber{
		SubscriberID: "usr_devices",
		Channels: model.SubscriberChannels{
			Push: model.PushChannelConfig{FCMTokens: []string{"tok-a"}},
		},
	}))

	// Device B registers with its own token.
	require.NoError(t, repo.Upsert(context.Background(), envID, &model.Subscriber{
		SubscriberID: "usr_devices",
		Channels: model.SubscriberChannels{
			Push: model.PushChannelConfig{FCMTokens: []string{"tok-b"}},
		},
	}))

	found, err := repo.FindBySubscriberID(context.Background(), envID, "usr_devices")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"tok-a", "tok-b"}, found.Channels.Push.FCMTokens,
		"both devices must keep receiving push")

	// Re-registering an existing token must not duplicate it.
	require.NoError(t, repo.Upsert(context.Background(), envID, &model.Subscriber{
		SubscriberID: "usr_devices",
		Channels: model.SubscriberChannels{
			Push: model.PushChannelConfig{FCMTokens: []string{"tok-a"}},
		},
	}))

	found, err = repo.FindBySubscriberID(context.Background(), envID, "usr_devices")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"tok-a", "tok-b"}, found.Channels.Push.FCMTokens,
		"a repeated token is not stored twice")
}

// The upsert can only add tokens, so removal is its own operation. Without it a
// token from an uninstalled app lives forever and the worker keeps pushing to a
// dead device.
func TestSubscriberRepo_RemovePushTokens(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envID, _ := testutil.SeedEnvironmentDoc(t, db, "test-env")
	repo := NewSubscriberRepository(db)

	require.NoError(t, repo.Upsert(context.Background(), envID, &model.Subscriber{
		SubscriberID: "usr_unregister",
		Channels: model.SubscriberChannels{
			Push: model.PushChannelConfig{
				FCMTokens:  []string{"tok-a", "tok-b"},
				APNSTokens: []string{"apns-a"},
			},
		},
	}))

	sub, err := repo.RemovePushTokens(context.Background(), envID, "usr_unregister", []string{"tok-a"}, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"tok-b"}, sub.Channels.Push.FCMTokens, "only the named token goes")
	assert.Equal(t, []string{"apns-a"}, sub.Channels.Push.APNSTokens, "the other list is untouched")

	// Unregistering is idempotent: clients retry it, and a token that is already
	// gone is not an error.
	sub, err = repo.RemovePushTokens(context.Background(), envID, "usr_unregister", []string{"tok-a"}, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"tok-b"}, sub.Channels.Push.FCMTokens)

	sub, err = repo.RemovePushTokens(context.Background(), envID, "usr_unregister", []string{"tok-b"}, []string{"apns-a"})
	require.NoError(t, err)
	assert.Empty(t, sub.Channels.Push.FCMTokens, "the last token can be removed")
	assert.Empty(t, sub.Channels.Push.APNSTokens)
}

func TestSubscriberRepo_RemovePushTokens_UnknownSubscriber(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envID, _ := testutil.SeedEnvironmentDoc(t, db, "test-env")
	repo := NewSubscriberRepository(db)

	_, err := repo.RemovePushTokens(context.Background(), envID, "nobody", []string{"tok"}, nil)
	assert.ErrorIs(t, err, mongo.ErrNoDocuments)
}

// A brand-new subscriber whose first post already carries channel config: the
// dotted-path $set must coexist with $setOnInsert (no "channels" path conflict)
// and the nested document must come out complete.
func TestSubscriberRepo_Upsert_InsertWithChannelConfig(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envID, _ := testutil.SeedEnvironmentDoc(t, db, "test-env")
	repo := NewSubscriberRepository(db)

	require.NoError(t, repo.Upsert(context.Background(), envID, &model.Subscriber{
		SubscriberID: "usr_firstchan",
		Channels: model.SubscriberChannels{
			Push:  model.PushChannelConfig{FCMTokens: []string{"tok-1"}},
			Slack: model.SlackConfig{WebhookURL: "https://hooks.slack.com/services/T/B/X"},
		},
	}))

	found, err := repo.FindBySubscriberID(context.Background(), envID, "usr_firstchan")
	require.NoError(t, err)
	assert.Equal(t, []string{"tok-1"}, found.Channels.Push.FCMTokens)
	assert.Equal(t, "https://hooks.slack.com/services/T/B/X", found.Channels.Slack.WebhookURL)
	assert.Equal(t, "en", found.Locale, "the insert-only defaults are still seeded")
}

// Presence is owned by the WebSocket lifecycle, not by profile writes. Harbor
// re-posts a subscriber on every token mint, so leaving `isOnline` in $set would
// knock a connected subscriber offline every time they load a page.
func TestSubscriberRepo_Upsert_SparseRepostPreservesOnlineStatus(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envID, _ := testutil.SeedEnvironmentDoc(t, db, "test-env")
	repo := NewSubscriberRepository(db)

	sub := &model.Subscriber{SubscriberID: "usr_online", Email: "o@example.com"}
	require.NoError(t, repo.Upsert(context.Background(), envID, sub))
	require.NoError(t, repo.SetOnlineStatus(context.Background(), sub.ID, true))

	// The backfill payload Harbor sends on every page load.
	require.NoError(t, repo.Upsert(context.Background(), envID, &model.Subscriber{
		SubscriberID: "usr_online",
		Email:        "o@example.com",
	}))

	found, err := repo.FindBySubscriberID(context.Background(), envID, "usr_online")
	require.NoError(t, err)
	assert.True(t, found.IsOnline, "a connected subscriber must not be knocked offline by a profile re-post")
}

// A brand-new subscriber that supplies no locale still gets the default, seeded
// via $setOnInsert. isOnline is seeded there too, so it must exist and be false.
func TestSubscriberRepo_Upsert_SeedsDefaultsOnInsert(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envID, _ := testutil.SeedEnvironmentDoc(t, db, "test-env")
	repo := NewSubscriberRepository(db)

	require.NoError(t, repo.Upsert(context.Background(), envID, &model.Subscriber{SubscriberID: "usr_nolocale"}))

	found, err := repo.FindBySubscriberID(context.Background(), envID, "usr_nolocale")
	require.NoError(t, err)
	assert.Equal(t, "en", found.Locale)
	assert.False(t, found.IsOnline)
}

// Upsert decodes the stored document back over the caller's struct. Without that,
// re-posting an existing subscriber left the struct exactly as the caller built
// it — zero id, blank locale, no channels — and POST /subscribers handed that
// straight back as its response body.
func TestSubscriberRepo_Upsert_RepostReturnsTheStoredDocument(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envID, _ := testutil.SeedEnvironmentDoc(t, db, "test-env")
	repo := NewSubscriberRepository(db)

	first := &model.Subscriber{SubscriberID: "usr_echo", Email: "e@example.com", Locale: "de"}
	require.NoError(t, repo.Upsert(context.Background(), envID, first))

	repost := &model.Subscriber{SubscriberID: "usr_echo", Email: "e@example.com"}
	require.NoError(t, repo.Upsert(context.Background(), envID, repost))

	assert.Equal(t, first.ID, repost.ID, "the caller must get the real id back, not the zero value")
	assert.False(t, repost.CreatedAt.IsZero(), "createdAt must reflect the stored document")
	assert.Equal(t, "de", repost.Locale, "the stored locale, not the blank one that was sent")
}

// BulkUpsert shares subscriberSetDoc with Upsert, so it must preserve the same
// fields on a sparse re-post.
func TestSubscriberRepo_BulkUpsert_SparseRepostPreservesState(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envID, _ := testutil.SeedEnvironmentDoc(t, db, "test-env")
	repo := NewSubscriberRepository(db)

	require.NoError(t, repo.BulkUpsert(context.Background(), envID, []model.Subscriber{{
		SubscriberID: "usr_bulk",
		Locale:       "fr",
		Channels: model.SubscriberChannels{
			Slack: model.SlackConfig{WebhookURL: "https://hooks.slack.com/services/T/B/X"},
		},
	}}))

	require.NoError(t, repo.BulkUpsert(context.Background(), envID, []model.Subscriber{{
		SubscriberID: "usr_bulk",
		Email:        "b@example.com",
	}}))

	found, err := repo.FindBySubscriberID(context.Background(), envID, "usr_bulk")
	require.NoError(t, err)
	assert.Equal(t, "fr", found.Locale, "locale must survive a sparse bulk re-post")
	assert.Equal(t, "https://hooks.slack.com/services/T/B/X", found.Channels.Slack.WebhookURL)
	assert.Equal(t, "b@example.com", found.Email, "the field that WAS supplied still updates")
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

func TestSubscriberRepo_Update(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envID, _ := testutil.SeedEnvironmentDoc(t, db, "test-env")
	repo := NewSubscriberRepository(db)

	sub := &model.Subscriber{SubscriberID: "usr_upd", FirstName: "Alice", Locale: "en"}
	require.NoError(t, repo.Upsert(context.Background(), envID, sub))

	err := repo.Update(context.Background(), envID, "usr_upd", bson.M{"firstName": "Alicia"})
	require.NoError(t, err)

	found, err := repo.FindBySubscriberID(context.Background(), envID, "usr_upd")
	require.NoError(t, err)
	assert.Equal(t, "Alicia", found.FirstName)
}

func TestSubscriberRepo_Update_UnknownSubscriber(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envID, _ := testutil.SeedEnvironmentDoc(t, db, "test-env")
	repo := NewSubscriberRepository(db)

	err := repo.Update(context.Background(), envID, "nonexistent", bson.M{"firstName": "Ghost"})
	assert.ErrorIs(t, err, mongo.ErrNoDocuments)
}

func TestSubscriberRepo_FindPageAfter_WalksAllWithoutDuplicates(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envID, _ := testutil.SeedEnvironmentDoc(t, db, "test-env")
	repo := NewSubscriberRepository(db)

	for i := 0; i < 5; i++ {
		sub := &model.Subscriber{SubscriberID: bson.NewObjectID().Hex(), Locale: "en"}
		require.NoError(t, repo.Upsert(context.Background(), envID, sub))
	}

	seen := make(map[bson.ObjectID]bool)
	var lastID bson.ObjectID
	pages := 0
	for {
		subs, err := repo.FindPageAfter(context.Background(), envID, lastID, 2)
		require.NoError(t, err)
		if len(subs) == 0 {
			break
		}
		pages++
		for _, s := range subs {
			assert.False(t, seen[s.ID], "subscriber returned twice")
			seen[s.ID] = true
		}
		lastID = subs[len(subs)-1].ID
	}

	assert.Len(t, seen, 5)
	assert.Equal(t, 3, pages)
}

func TestSubscriberRepo_FindPageAfter_ScopedToEnvironment(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envA, _ := testutil.SeedEnvironmentDoc(t, db, "env-a")
	envB, _ := testutil.SeedEnvironmentDoc(t, db, "env-b")
	repo := NewSubscriberRepository(db)

	require.NoError(t, repo.Upsert(context.Background(), envA, &model.Subscriber{SubscriberID: "usr_a", Locale: "en"}))
	require.NoError(t, repo.Upsert(context.Background(), envB, &model.Subscriber{SubscriberID: "usr_b", Locale: "en"}))

	subs, err := repo.FindPageAfter(context.Background(), envA, bson.ObjectID{}, 10)
	require.NoError(t, err)
	require.Len(t, subs, 1)
	assert.Equal(t, "usr_a", subs[0].SubscriberID)
}
