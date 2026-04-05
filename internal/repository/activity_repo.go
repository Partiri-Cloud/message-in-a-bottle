package repository

import (
	"context"
	"time"

	"github.com/partiri/message-in-a-bottle/internal/model"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type ActivityRepository struct {
	col *mongo.Collection
}

func NewActivityRepository(db *mongo.Database) *ActivityRepository {
	return &ActivityRepository{col: db.Collection("activity_log")}
}

func (r *ActivityRepository) Create(ctx context.Context, log *model.ActivityLog) error {
	log.CreatedAt = time.Now()
	res, err := r.col.InsertOne(ctx, log)
	if err != nil {
		return err
	}
	log.ID = res.InsertedID.(bson.ObjectID)
	return nil
}

func (r *ActivityRepository) FindByNotification(ctx context.Context, envID, notifID bson.ObjectID, page, limit int) ([]model.ActivityLog, int64, error) {
	filter := bson.M{"environmentId": envID, "notificationId": notifID}
	total, err := r.col.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	opts := options.Find().
		SetSkip(int64((page - 1) * limit)).
		SetLimit(int64(limit)).
		SetSort(bson.M{"createdAt": 1})

	cursor, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var logs []model.ActivityLog
	if err := cursor.All(ctx, &logs); err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}

func (r *ActivityRepository) FindMany(ctx context.Context, envID bson.ObjectID, page, limit int) ([]model.ActivityLog, int64, error) {
	filter := bson.M{"environmentId": envID}
	total, err := r.col.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	opts := options.Find().
		SetSkip(int64((page - 1) * limit)).
		SetLimit(int64(limit)).
		SetSort(bson.M{"createdAt": -1})

	cursor, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var logs []model.ActivityLog
	if err := cursor.All(ctx, &logs); err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}

func (r *ActivityRepository) Collection() *mongo.Collection {
	return r.col
}
