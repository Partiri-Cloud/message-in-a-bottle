package repository

import (
	"context"
	"testing"
	"time"

	"github.com/partiri-cloud/message-in-a-bottle/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestRateLimitRepo_IncrementAndCheck_FirstRequest(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	repo := NewRateLimitRepository(db)
	envID := bson.NewObjectID()
	subID := bson.NewObjectID()
	windowStart := time.Now().Truncate(time.Hour)

	exceeded, err := repo.IncrementAndCheck(context.Background(), envID, subID, "email", windowStart, time.Hour, 10)
	require.NoError(t, err)
	assert.False(t, exceeded, "first request should not exceed limit")
}

func TestRateLimitRepo_IncrementAndCheck_UnderLimit(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	repo := NewRateLimitRepository(db)
	envID := bson.NewObjectID()
	subID := bson.NewObjectID()
	windowStart := time.Now().Truncate(time.Hour)

	for i := 0; i < 5; i++ {
		exceeded, err := repo.IncrementAndCheck(context.Background(), envID, subID, "email", windowStart, time.Hour, 10)
		require.NoError(t, err)
		assert.False(t, exceeded)
	}
}

func TestRateLimitRepo_IncrementAndCheck_AtLimit(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	repo := NewRateLimitRepository(db)
	envID := bson.NewObjectID()
	subID := bson.NewObjectID()
	windowStart := time.Now().Truncate(time.Hour)

	// Fill up to max (3)
	for i := 0; i < 3; i++ {
		exceeded, _ := repo.IncrementAndCheck(context.Background(), envID, subID, "sms", windowStart, time.Hour, 3)
		assert.False(t, exceeded)
	}

	// Next one exceeds
	exceeded, err := repo.IncrementAndCheck(context.Background(), envID, subID, "sms", windowStart, time.Hour, 3)
	require.NoError(t, err)
	assert.True(t, exceeded, "should exceed rate limit")
}

func TestRateLimitRepo_IncrementAndCheck_DifferentWindows(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	repo := NewRateLimitRepository(db)
	envID := bson.NewObjectID()
	subID := bson.NewObjectID()

	window1 := time.Now().Truncate(time.Hour)
	window2 := window1.Add(time.Hour)

	// Fill window1
	for i := 0; i < 3; i++ {
		repo.IncrementAndCheck(context.Background(), envID, subID, "email", window1, time.Hour, 3)
	}

	// Window2 should be fresh
	exceeded, err := repo.IncrementAndCheck(context.Background(), envID, subID, "email", window2, time.Hour, 3)
	require.NoError(t, err)
	assert.False(t, exceeded)
}

func TestRateLimitRepo_IncrementAndCheck_DifferentChannels(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	repo := NewRateLimitRepository(db)
	envID := bson.NewObjectID()
	subID := bson.NewObjectID()
	windowStart := time.Now().Truncate(time.Hour)

	// Fill email to limit
	for i := 0; i < 3; i++ {
		repo.IncrementAndCheck(context.Background(), envID, subID, "email", windowStart, time.Hour, 3)
	}

	// SMS should be fresh
	exceeded, err := repo.IncrementAndCheck(context.Background(), envID, subID, "sms", windowStart, time.Hour, 3)
	require.NoError(t, err)
	assert.False(t, exceeded)
}
