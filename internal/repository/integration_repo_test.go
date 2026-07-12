package repository

import (
	"context"
	"testing"

	"github.com/partiri-cloud/message-in-a-bottle/internal/model"
	"github.com/partiri-cloud/message-in-a-bottle/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AvailableChannels has to answer the question the preferences UI asks: "will a
// notification on this channel actually arrive?" It must therefore agree with
// DeliveryHandler's integration lookup, channel for channel. Where it does not,
// a subscriber is shown a toggle that silently delivers nothing — the worker
// marks the channel "failed — no integration configured" and no one ever sees it.
func TestIntegrationRepo_AvailableChannels(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envID, _ := testutil.SeedEnvironmentDoc(t, db, "test-env")
	repo := NewIntegrationRepository(db)
	ctx := context.Background()

	// An environment with no integrations at all — the state this platform was
	// actually in, while the UI offered all six channels.
	avail, err := repo.AvailableChannels(ctx, envID)
	require.NoError(t, err)

	assert.True(t, avail.InApp, "in_app needs no integration and must always be available")
	assert.False(t, avail.Email)
	assert.False(t, avail.SMS)
	assert.False(t, avail.Push)
	assert.False(t, avail.Slack)
	assert.False(t, avail.MSTeams)

	// A non-primary email integration is NOT deliverable: DeliveryHandler resolves
	// email through FindPrimaryByChannel, which requires isPrimary.
	require.NoError(t, repo.Create(ctx, &model.Integration{
		EnvironmentID: envID,
		Channel:       "email",
		ProviderID:    "smtp",
		Name:          "secondary smtp",
		IsPrimary:     false,
		IsActive:      true,
	}))

	avail, err = repo.AvailableChannels(ctx, envID)
	require.NoError(t, err)
	assert.False(t, avail.Email, "a non-primary email integration is never selected by delivery")

	// Primary + active: now it delivers.
	require.NoError(t, repo.Create(ctx, &model.Integration{
		EnvironmentID: envID,
		Channel:       "email",
		ProviderID:    "smtp",
		Name:          "primary smtp",
		IsPrimary:     true,
		IsActive:      true,
	}))

	// Push fans out over every active integration, so it does not need a primary.
	require.NoError(t, repo.Create(ctx, &model.Integration{
		EnvironmentID: envID,
		Channel:       "push",
		ProviderID:    "fcm",
		Name:          "fcm",
		IsPrimary:     false,
		IsActive:      true,
	}))

	// Inactive integrations do not count, whatever their primary flag says.
	require.NoError(t, repo.Create(ctx, &model.Integration{
		EnvironmentID: envID,
		Channel:       "slack",
		ProviderID:    "slack",
		Name:          "disabled slack",
		IsPrimary:     true,
		IsActive:      false,
	}))

	avail, err = repo.AvailableChannels(ctx, envID)
	require.NoError(t, err)

	assert.True(t, avail.Email)
	assert.True(t, avail.Push, "push is deliverable through any active integration")
	assert.False(t, avail.Slack, "an inactive integration delivers nothing")
	assert.False(t, avail.SMS)
	assert.True(t, avail.InApp)
}

// Availability is per environment: one tenant's SMTP must not make another
// tenant's email toggle look deliverable.
func TestIntegrationRepo_AvailableChannels_ScopedToEnvironment(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envA, _ := testutil.SeedEnvironmentDoc(t, db, "env-a")
	envB, _ := testutil.SeedEnvironmentDoc(t, db, "env-b")
	repo := NewIntegrationRepository(db)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &model.Integration{
		EnvironmentID: envA,
		Channel:       "email",
		ProviderID:    "smtp",
		Name:          "env a smtp",
		IsPrimary:     true,
		IsActive:      true,
	}))

	availA, err := repo.AvailableChannels(ctx, envA)
	require.NoError(t, err)
	assert.True(t, availA.Email)

	availB, err := repo.AvailableChannels(ctx, envB)
	require.NoError(t, err)
	assert.False(t, availB.Email, "envB has no integration of its own")
}
