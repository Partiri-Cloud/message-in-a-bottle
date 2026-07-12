package repository

import (
	"context"
	"testing"

	"github.com/partiri-cloud/message-in-a-bottle/internal/model"
	"github.com/partiri-cloud/message-in-a-bottle/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func TestTopicRepo_UpdateName(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envID, _ := testutil.SeedEnvironmentDoc(t, db, "test-env")
	repo := NewTopicRepository(db)

	topic := &model.Topic{EnvironmentID: envID, Key: "orders", Name: "Orders"}
	require.NoError(t, repo.Create(context.Background(), topic))

	err := repo.UpdateName(context.Background(), envID, "orders", "Order updates")
	require.NoError(t, err)

	found, err := repo.FindByKey(context.Background(), envID, "orders")
	require.NoError(t, err)
	assert.Equal(t, "Order updates", found.Name)
}

func TestTopicRepo_UpdateName_UnknownTopic(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envID, _ := testutil.SeedEnvironmentDoc(t, db, "test-env")
	repo := NewTopicRepository(db)

	err := repo.UpdateName(context.Background(), envID, "nonexistent", "Ghost")
	assert.ErrorIs(t, err, mongo.ErrNoDocuments)
}
