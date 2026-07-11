package repository

import (
	"context"
	"sync"
	"testing"

	"github.com/partiri-cloud/message-in-a-bottle/internal/model"
	"github.com/partiri-cloud/message-in-a-bottle/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// The channels a caller did not name are seeded only when the row is created.
func TestPreferenceRepo_UpdateChannels_SeedsOnInsert(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envID, _ := testutil.SeedEnvironmentDoc(t, db, "test-env")
	repo := NewPreferenceRepository(db)
	subID := bson.NewObjectID()

	pref, err := repo.UpdateChannels(context.Background(), envID, subID, nil,
		map[string]bool{"sms": false}, model.AllChannelsEnabled())
	require.NoError(t, err)

	assert.False(t, pref.Channels.SMS, "the channel the caller named")
	assert.True(t, pref.Channels.Email, "an unnamed channel takes the seed on insert")
	assert.True(t, pref.Channels.InApp)
	assert.Nil(t, pref.WorkflowID, "this is the global row")
	assert.False(t, pref.UpdatedAt.IsZero())
}

// On an existing row the seed is ignored: the unnamed channels keep whatever is
// stored. Otherwise a stale seed computed by the handler would quietly overwrite
// a value another request had just written.
func TestPreferenceRepo_UpdateChannels_ExistingRowIgnoresSeed(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envID, _ := testutil.SeedEnvironmentDoc(t, db, "test-env")
	repo := NewPreferenceRepository(db)
	subID := bson.NewObjectID()
	wfID := bson.NewObjectID()

	_, err := repo.UpdateChannels(context.Background(), envID, subID, &wfID,
		map[string]bool{"email": false, "push": true}, model.ChannelPrefs{})
	require.NoError(t, err)

	// A second update naming only SMS, with a seed that would enable everything.
	pref, err := repo.UpdateChannels(context.Background(), envID, subID, &wfID,
		map[string]bool{"sms": true}, model.AllChannelsEnabled())
	require.NoError(t, err)

	assert.True(t, pref.Channels.SMS, "the channel the caller named")
	assert.False(t, pref.Channels.Email, "the stored value survives — the seed must not resurrect it")
	assert.True(t, pref.Channels.Push, "and so does this one")
}

// Two concurrent FIRST writes both find no row and both try to insert; the unique
// index rejects one with a duplicate-key error. That is recoverable — the row now
// exists — so UpdateChannels retries once. Without the retry the loser surfaces as
// a 500 and its channel change is silently dropped.
func TestPreferenceRepo_UpdateChannels_ConcurrentFirstWritesBothSucceed(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envID, _ := testutil.SeedEnvironmentDoc(t, db, "test-env")
	require.NoError(t, EnsureIndexes(context.Background(), db))
	repo := NewPreferenceRepository(db)
	subID := bson.NewObjectID()

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, channel := range []string{"email", "sms"} {
		wg.Add(1)
		go func(i int, channel string) {
			defer wg.Done()
			_, errs[i] = repo.UpdateChannels(context.Background(), envID, subID, nil,
				map[string]bool{channel: false}, model.AllChannelsEnabled())
		}(i, channel)
	}
	wg.Wait()

	require.NoError(t, errs[0], "a duplicate-key race on insert must be retried, not surfaced")
	require.NoError(t, errs[1])

	prefs, err := repo.FindBySubscriber(context.Background(), envID, subID)
	require.NoError(t, err)
	require.Len(t, prefs, 1, "the unique index must leave exactly one row")
	assert.False(t, prefs[0].Channels.Email, "both opt-outs land")
	assert.False(t, prefs[0].Channels.SMS)
}

func TestPreferenceRepo_FindForWorkflow(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envID, _ := testutil.SeedEnvironmentDoc(t, db, "test-env")
	repo := NewPreferenceRepository(db)
	subID := bson.NewObjectID()
	wfID := bson.NewObjectID()
	otherWfID := bson.NewObjectID()

	wfPref, globalPref, err := repo.FindForWorkflow(context.Background(), envID, subID, wfID)
	require.NoError(t, err)
	assert.Nil(t, wfPref, "nothing stored yet")
	assert.Nil(t, globalPref)

	_, err = repo.UpdateChannels(context.Background(), envID, subID, nil,
		map[string]bool{"sms": false}, model.AllChannelsEnabled())
	require.NoError(t, err)
	_, err = repo.UpdateChannels(context.Background(), envID, subID, &wfID,
		map[string]bool{"email": false}, model.ChannelPrefs{})
	require.NoError(t, err)
	// A row on a different workflow must not be mistaken for this one's.
	_, err = repo.UpdateChannels(context.Background(), envID, subID, &otherWfID,
		map[string]bool{"push": false}, model.ChannelPrefs{})
	require.NoError(t, err)

	wfPref, globalPref, err = repo.FindForWorkflow(context.Background(), envID, subID, wfID)
	require.NoError(t, err)

	require.NotNil(t, wfPref)
	require.NotNil(t, wfPref.WorkflowID)
	assert.Equal(t, wfID, *wfPref.WorkflowID, "the row for this workflow, not the other one")
	assert.False(t, wfPref.Channels.Email)

	require.NotNil(t, globalPref)
	assert.Nil(t, globalPref.WorkflowID)
	assert.False(t, globalPref.Channels.SMS)
}

// The bug this pins: merging in the handler and writing all six channels back is
// a read-modify-write, so two concurrent updates to different channels lose one
// of them. Writing one dotted path per named channel makes them independent.
func TestPreferenceRepo_UpdateChannels_ConcurrentUpdatesToDifferentChannelsBothLand(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envID, _ := testutil.SeedEnvironmentDoc(t, db, "test-env")
	repo := NewPreferenceRepository(db)
	subID := bson.NewObjectID()

	// The row must exist first: two concurrent upserts racing to *insert* is a
	// different question (the unique index settles it), and not the one here.
	_, err := repo.UpdateChannels(context.Background(), envID, subID, nil,
		map[string]bool{}, model.AllChannelsEnabled())
	require.NoError(t, err)

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, channel := range []string{"email", "sms"} {
		wg.Add(1)
		go func(i int, channel string) {
			defer wg.Done()
			_, errs[i] = repo.UpdateChannels(context.Background(), envID, subID, nil,
				map[string]bool{channel: false}, model.AllChannelsEnabled())
		}(i, channel)
	}
	wg.Wait()
	require.NoError(t, errs[0])
	require.NoError(t, errs[1])

	prefs, err := repo.FindBySubscriber(context.Background(), envID, subID)
	require.NoError(t, err)
	require.Len(t, prefs, 1)

	assert.False(t, prefs[0].Channels.Email, "the email opt-out must not be lost to the concurrent sms write")
	assert.False(t, prefs[0].Channels.SMS, "and vice versa")
	assert.True(t, prefs[0].Channels.Push, "a channel neither request named is untouched")
}
