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

func TestTemplateRepo_Delete(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envID, _ := testutil.SeedEnvironmentDoc(t, db, "test-env")
	repo := NewTemplateRepository(db)

	tmpl := &model.TransactionalTemplate{
		EnvironmentID: envID,
		Identifier:    "welcome",
		Name:          "Welcome",
		Channel:       "email",
		DefaultLocale: "en",
	}
	require.NoError(t, repo.Create(context.Background(), tmpl))

	err := repo.Delete(context.Background(), envID, "welcome")
	require.NoError(t, err)

	_, err = repo.FindByIdentifier(context.Background(), envID, "welcome")
	assert.ErrorIs(t, err, mongo.ErrNoDocuments)
}

func TestTemplateRepo_Delete_UnknownTemplate(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	envID, _ := testutil.SeedEnvironmentDoc(t, db, "test-env")
	repo := NewTemplateRepository(db)

	err := repo.Delete(context.Background(), envID, "nonexistent")
	assert.ErrorIs(t, err, mongo.ErrNoDocuments)
}
