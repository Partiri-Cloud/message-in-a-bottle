package repository

import (
	"context"
	"log/slog"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func unique() *options.IndexOptionsBuilder {
	return options.Index().SetUnique(true)
}

func uniqueSparse() *options.IndexOptionsBuilder {
	return options.Index().SetUnique(true).SetSparse(true)
}

func ttl() *options.IndexOptionsBuilder {
	return options.Index().SetExpireAfterSeconds(0)
}

func EnsureIndexes(ctx context.Context, db *mongo.Database) error {
	indexes := map[string][]mongo.IndexModel{
		"environments": {
			{Keys: bson.D{{Key: "identifier", Value: 1}}, Options: unique()},
			{Keys: bson.D{{Key: "apiKeys.keyHash", Value: 1}}, Options: uniqueSparse()},
		},
		"subscribers": {
			{Keys: bson.D{{Key: "environmentId", Value: 1}, {Key: "subscriberId", Value: 1}}, Options: unique()},
			{Keys: bson.D{{Key: "environmentId", Value: 1}, {Key: "email", Value: 1}}},
			// Serves the broadcast _id-cursor walk (FindPageAfter).
			{Keys: bson.D{{Key: "environmentId", Value: 1}, {Key: "_id", Value: 1}}},
		},
		"topics": {
			{Keys: bson.D{{Key: "environmentId", Value: 1}, {Key: "key", Value: 1}}, Options: unique()},
		},
		"topic_subscribers": {
			{Keys: bson.D{{Key: "topicId", Value: 1}, {Key: "subscriberId", Value: 1}}, Options: unique()},
			{Keys: bson.D{{Key: "subscriberId", Value: 1}}},
		},
		"integrations": {
			{Keys: bson.D{{Key: "environmentId", Value: 1}, {Key: "channel", Value: 1}, {Key: "isPrimary", Value: 1}}},
			{Keys: bson.D{{Key: "environmentId", Value: 1}, {Key: "providerId", Value: 1}}},
		},
		"workflows": {
			{Keys: bson.D{{Key: "environmentId", Value: 1}, {Key: "identifier", Value: 1}}, Options: unique()},
			{Keys: bson.D{{Key: "environmentId", Value: 1}, {Key: "tags", Value: 1}}},
		},
		"subscriber_preferences": {
			{Keys: bson.D{{Key: "environmentId", Value: 1}, {Key: "subscriberId", Value: 1}, {Key: "workflowId", Value: 1}}, Options: unique()},
		},
		"notifications": {
			{Keys: bson.D{{Key: "environmentId", Value: 1}, {Key: "subscriberId", Value: 1}, {Key: "createdAt", Value: -1}}},
			{Keys: bson.D{{Key: "environmentId", Value: 1}, {Key: "transactionId", Value: 1}, {Key: "subscriberId", Value: 1}}, Options: uniqueSparse()},
			{Keys: bson.D{{Key: "expireAt", Value: 1}}, Options: ttl()},
			{Keys: bson.D{{Key: "environmentId", Value: 1}, {Key: "workflowId", Value: 1}, {Key: "createdAt", Value: -1}}},
		},
		"activity_log": {
			{Keys: bson.D{{Key: "environmentId", Value: 1}, {Key: "notificationId", Value: 1}, {Key: "createdAt", Value: 1}}},
			{Keys: bson.D{{Key: "expireAt", Value: 1}}, Options: ttl()},
			{Keys: bson.D{{Key: "environmentId", Value: 1}, {Key: "createdAt", Value: -1}}},
		},
		"transactional_templates": {
			{Keys: bson.D{{Key: "environmentId", Value: 1}, {Key: "identifier", Value: 1}}, Options: unique()},
		},
		"rate_limit_records": {
			{Keys: bson.D{{Key: "environmentId", Value: 1}, {Key: "subscriberId", Value: 1}, {Key: "channel", Value: 1}, {Key: "windowStart", Value: 1}}, Options: unique()},
			{Keys: bson.D{{Key: "expireAt", Value: 1}}, Options: ttl()},
		},
	}

	for collection, idxModels := range indexes {
		col := db.Collection(collection)
		for _, idx := range idxModels {
			name, err := col.Indexes().CreateOne(ctx, idx)
			if err != nil {
				return err
			}
			slog.Info("created index", "index", name, "collection", collection)
		}
	}

	return nil
}
