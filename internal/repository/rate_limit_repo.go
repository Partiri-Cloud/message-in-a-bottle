package repository

import (
	"context"
	"time"

	"github.com/partiri-cloud/message-in-a-box/internal/model"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type RateLimitRepository struct {
	col *mongo.Collection
}

func NewRateLimitRepository(db *mongo.Database) *RateLimitRepository {
	return &RateLimitRepository{col: db.Collection("rate_limit_records")}
}

func (r *RateLimitRepository) IncrementAndCheck(ctx context.Context, envID, subscriberID bson.ObjectID, channel string, windowStart time.Time, windowDuration time.Duration, maxPerWindow int) (bool, error) {
	filter := bson.M{
		"environmentId": envID,
		"subscriberId":  subscriberID,
		"channel":       channel,
		"windowStart":   windowStart,
	}

	expireAt := windowStart.Add(windowDuration * 2) // keep a bit longer than the window
	update := bson.M{
		"$inc": bson.M{"count": 1},
		"$setOnInsert": bson.M{
			"environmentId": envID,
			"subscriberId":  subscriberID,
			"channel":       channel,
			"windowStart":   windowStart,
			"createdAt":     time.Now(),
			"expireAt":      expireAt,
		},
	}

	opts := options.FindOneAndUpdate().
		SetUpsert(true).
		SetReturnDocument(options.After)

	var record model.RateLimitRecord
	err := r.col.FindOneAndUpdate(ctx, filter, update, opts).Decode(&record)
	if err != nil {
		return false, err
	}

	return record.Count > maxPerWindow, nil
}

func (r *RateLimitRepository) Collection() *mongo.Collection {
	return r.col
}
