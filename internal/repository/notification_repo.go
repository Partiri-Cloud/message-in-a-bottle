package repository

import (
	"context"
	"time"

	"github.com/partiri/message-in-a-bottle/internal/model"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type NotificationRepository struct {
	col *mongo.Collection
}

func NewNotificationRepository(db *mongo.Database) *NotificationRepository {
	return &NotificationRepository{col: db.Collection("notifications")}
}

func (r *NotificationRepository) Create(ctx context.Context, notif *model.Notification) error {
	now := time.Now()
	notif.CreatedAt = now
	notif.UpdatedAt = now
	res, err := r.col.InsertOne(ctx, notif)
	if err != nil {
		return err
	}
	notif.ID = res.InsertedID.(bson.ObjectID)
	return nil
}

func (r *NotificationRepository) FindByID(ctx context.Context, id bson.ObjectID) (*model.Notification, error) {
	var notif model.Notification
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&notif)
	if err != nil {
		return nil, err
	}
	return &notif, nil
}

func (r *NotificationRepository) FindByTransactionID(ctx context.Context, envID bson.ObjectID, txID string) (*model.Notification, error) {
	var notif model.Notification
	err := r.col.FindOne(ctx, bson.M{"environmentId": envID, "transactionId": txID}).Decode(&notif)
	if err != nil {
		return nil, err
	}
	return &notif, nil
}

type FeedFilter struct {
	Read     *bool
	Seen     *bool
	Archived bool
}

func (r *NotificationRepository) FindFeed(ctx context.Context, envID, subscriberID bson.ObjectID, filter FeedFilter, page, limit int) ([]model.Notification, int64, error) {
	q := bson.M{
		"environmentId": envID,
		"subscriberId":  subscriberID,
	}
	if filter.Read != nil {
		q["read"] = *filter.Read
	}
	if filter.Seen != nil {
		q["seen"] = *filter.Seen
	}
	if !filter.Archived {
		q["archivedAt"] = nil
	}

	total, err := r.col.CountDocuments(ctx, q)
	if err != nil {
		return nil, 0, err
	}

	opts := options.Find().
		SetSkip(int64((page - 1) * limit)).
		SetLimit(int64(limit)).
		SetSort(bson.M{"createdAt": -1})

	cursor, err := r.col.Find(ctx, q, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var notifs []model.Notification
	if err := cursor.All(ctx, &notifs); err != nil {
		return nil, 0, err
	}
	return notifs, total, nil
}

func (r *NotificationRepository) FindMany(ctx context.Context, envID bson.ObjectID, page, limit int) ([]model.Notification, int64, error) {
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

	var notifs []model.Notification
	if err := cursor.All(ctx, &notifs); err != nil {
		return nil, 0, err
	}
	return notifs, total, nil
}

func (r *NotificationRepository) MarkSeen(ctx context.Context, id bson.ObjectID) error {
	now := time.Now()
	_, err := r.col.UpdateOne(ctx,
		bson.M{"_id": id, "seen": false},
		bson.M{"$set": bson.M{"seen": true, "seenAt": now, "updatedAt": now}},
	)
	return err
}

func (r *NotificationRepository) MarkRead(ctx context.Context, id bson.ObjectID) error {
	now := time.Now()
	_, err := r.col.UpdateOne(ctx,
		bson.M{"_id": id, "read": false},
		bson.M{"$set": bson.M{"read": true, "readAt": now, "seen": true, "seenAt": now, "updatedAt": now}},
	)
	return err
}

func (r *NotificationRepository) MarkArchived(ctx context.Context, id bson.ObjectID) error {
	now := time.Now()
	_, err := r.col.UpdateOne(ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"archivedAt": now, "updatedAt": now}},
	)
	return err
}

func (r *NotificationRepository) BulkMarkRead(ctx context.Context, ids []bson.ObjectID) error {
	now := time.Now()
	_, err := r.col.UpdateMany(ctx,
		bson.M{"_id": bson.M{"$in": ids}},
		bson.M{"$set": bson.M{"read": true, "readAt": now, "seen": true, "seenAt": now, "updatedAt": now}},
	)
	return err
}

func (r *NotificationRepository) BulkMarkSeen(ctx context.Context, ids []bson.ObjectID) error {
	now := time.Now()
	_, err := r.col.UpdateMany(ctx,
		bson.M{"_id": bson.M{"$in": ids}},
		bson.M{"$set": bson.M{"seen": true, "seenAt": now, "updatedAt": now}},
	)
	return err
}

func (r *NotificationRepository) UnseenCount(ctx context.Context, envID, subscriberID bson.ObjectID) (int64, error) {
	return r.col.CountDocuments(ctx, bson.M{
		"environmentId": envID,
		"subscriberId":  subscriberID,
		"seen":          false,
		"archivedAt":    nil,
	})
}

func (r *NotificationRepository) UpdateChannelStatus(ctx context.Context, notifID bson.ObjectID, channel, status string, update bson.M) error {
	setFields := bson.M{
		"channels.$.status": status,
		"updatedAt":         time.Now(),
	}
	for k, v := range update {
		setFields["channels.$."+k] = v
	}
	_, err := r.col.UpdateOne(ctx,
		bson.M{"_id": notifID, "channels.channel": channel},
		bson.M{"$set": setFields},
	)
	return err
}

func (r *NotificationRepository) Collection() *mongo.Collection {
	return r.col
}
