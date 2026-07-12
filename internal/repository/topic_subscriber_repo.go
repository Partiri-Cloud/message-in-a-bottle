package repository

import (
	"context"
	"time"

	"github.com/partiri-cloud/message-in-a-bottle/internal/model"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type TopicSubscriberRepository struct {
	col *mongo.Collection
}

func NewTopicSubscriberRepository(db *mongo.Database) *TopicSubscriberRepository {
	return &TopicSubscriberRepository{col: db.Collection("topic_subscribers")}
}

func (r *TopicSubscriberRepository) BulkAdd(ctx context.Context, envID, topicID bson.ObjectID, subscribers []model.TopicSubscriber) error {
	if len(subscribers) == 0 {
		return nil
	}
	now := time.Now()
	models := make([]mongo.WriteModel, len(subscribers))
	for i := range subscribers {
		subscribers[i].EnvironmentID = envID
		subscribers[i].TopicID = topicID
		subscribers[i].CreatedAt = now
		models[i] = mongo.NewUpdateOneModel().
			SetFilter(bson.M{"topicId": topicID, "subscriberId": subscribers[i].SubscriberID}).
			SetUpdate(bson.M{"$setOnInsert": subscribers[i]}).
			SetUpsert(true)
	}
	_, err := r.col.BulkWrite(ctx, models)
	return err
}

func (r *TopicSubscriberRepository) BulkRemove(ctx context.Context, topicID bson.ObjectID, subscriberIDs []bson.ObjectID) error {
	if len(subscriberIDs) == 0 {
		return nil
	}
	_, err := r.col.DeleteMany(ctx, bson.M{
		"topicId":      topicID,
		"subscriberId": bson.M{"$in": subscriberIDs},
	})
	return err
}

func (r *TopicSubscriberRepository) FindByTopic(ctx context.Context, topicID bson.ObjectID, page, limit int) ([]model.TopicSubscriber, int64, error) {
	filter := bson.M{"topicId": topicID}
	total, err := r.col.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	opts := options.Find().
		SetSkip(int64((page - 1) * limit)).
		SetLimit(int64(limit))

	cursor, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var subs []model.TopicSubscriber
	if err := cursor.All(ctx, &subs); err != nil {
		return nil, 0, err
	}
	return subs, total, nil
}

func (r *TopicSubscriberRepository) FindSubscriberIDsByTopic(ctx context.Context, topicID bson.ObjectID) ([]bson.ObjectID, error) {
	cursor, err := r.col.Find(ctx, bson.M{"topicId": topicID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var ids []bson.ObjectID
	for cursor.Next(ctx) {
		var ts model.TopicSubscriber
		if err := cursor.Decode(&ts); err != nil {
			return nil, err
		}
		ids = append(ids, ts.SubscriberID)
	}
	return ids, cursor.Err()
}

func (r *TopicSubscriberRepository) DeleteByTopic(ctx context.Context, topicID bson.ObjectID) error {
	_, err := r.col.DeleteMany(ctx, bson.M{"topicId": topicID})
	return err
}

func (r *TopicSubscriberRepository) DeleteBySubscriber(ctx context.Context, subscriberID bson.ObjectID) error {
	_, err := r.col.DeleteMany(ctx, bson.M{"subscriberId": subscriberID})
	return err
}
