package testutil

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var (
	mongoOnce      sync.Once
	mongoAvailable bool
)

// MongoURI returns the MongoDB URI for testing.
func MongoURI() string {
	if uri := os.Getenv("MONGO_TEST_URI"); uri != "" {
		return uri
	}
	return "mongodb://localhost:27017"
}

func checkMongo() bool {
	mongoOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		opts := options.Client().ApplyURI(MongoURI()).SetConnectTimeout(2 * time.Second).SetServerSelectionTimeout(2 * time.Second)
		client, err := mongo.Connect(opts)
		if err != nil {
			return
		}
		defer client.Disconnect(context.Background())

		if err := client.Ping(ctx, nil); err != nil {
			return
		}
		mongoAvailable = true
	})
	return mongoAvailable
}

// SetupTestDB creates a test database with a unique name. Skips the test if MongoDB is not available.
func SetupTestDB(t *testing.T) (*mongo.Database, func()) {
	t.Helper()

	if !checkMongo() {
		t.Skip("skipping integration test: MongoDB not available")
	}

	ctx := context.Background()
	opts := options.Client().ApplyURI(MongoURI()).SetConnectTimeout(3 * time.Second).SetServerSelectionTimeout(3 * time.Second)
	client, err := mongo.Connect(opts)
	if err != nil {
		t.Fatalf("failed to connect to MongoDB: %v", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		client.Disconnect(context.Background())
		t.Fatalf("failed to ping MongoDB: %v", err)
	}

	dbName := fmt.Sprintf("mib_test_%d", time.Now().UnixNano())
	db := client.Database(dbName)

	cleanup := func() {
		db.Drop(context.Background())
		client.Disconnect(context.Background())
	}

	return db, cleanup
}

// SeedEnvironmentDoc creates a test environment document directly via the driver.
func SeedEnvironmentDoc(t *testing.T, db *mongo.Database, name string) (bson.ObjectID, string) {
	t.Helper()
	rawKey := fmt.Sprintf("nv_test_%s_%d", name, time.Now().UnixNano())
	hash := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(hash[:])

	doc := bson.M{
		"name":       name,
		"identifier": name,
		"apiKeys": []bson.M{{
			"keyHash":     keyHash,
			"name":        "Test Key",
			"permissions": []string{"subscribers:read", "subscribers:write", "topics:read", "topics:write", "workflows:read", "workflows:write", "notifications:trigger", "notifications:read", "preferences:read", "preferences:write"},
			"isActive":    true,
			"createdAt":   time.Now(),
		}},
		"createdAt": time.Now(),
		"updatedAt": time.Now(),
	}

	result, err := db.Collection("environments").InsertOne(context.Background(), doc)
	if err != nil {
		t.Fatalf("failed to seed environment: %v", err)
	}

	return result.InsertedID.(bson.ObjectID), rawKey
}

// SeedSubscriberDoc creates a test subscriber document directly via the driver.
func SeedSubscriberDoc(t *testing.T, db *mongo.Database, envID bson.ObjectID, subscriberID string) bson.ObjectID {
	t.Helper()
	now := time.Now()
	doc := bson.M{
		"environmentId": envID,
		"subscriberId":  subscriberID,
		"email":         subscriberID + "@test.com",
		"firstName":     "Test",
		"lastName":      "User",
		"locale":        "en",
		"isOnline":      false,
		"channels":      bson.M{},
		"createdAt":     now,
		"updatedAt":     now,
	}

	result, err := db.Collection("subscribers").InsertOne(context.Background(), doc)
	if err != nil {
		t.Fatalf("failed to seed subscriber: %v", err)
	}

	return result.InsertedID.(bson.ObjectID)
}
