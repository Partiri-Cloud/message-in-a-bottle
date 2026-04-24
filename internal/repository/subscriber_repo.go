package repository

import (
	"context"
	"time"

	"github.com/partiri-cloud/message-in-a-bottle/internal/model"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type SubscriberRepository struct {
	col *mongo.Collection
}

func NewSubscriberRepository(db *mongo.Database) *SubscriberRepository {
	return &SubscriberRepository{col: db.Collection("subscribers")}
}

func (r *SubscriberRepository) Upsert(ctx context.Context, envID bson.ObjectID, sub *model.Subscriber) error {
	now := time.Now()
	sub.EnvironmentID = envID
	sub.UpdatedAt = now

	filter := bson.M{"environmentId": envID, "subscriberId": sub.SubscriberID}
	update := bson.M{
		"$set": sub,
		"$setOnInsert": bson.M{
			"createdAt": now,
		},
	}

	opts := options.UpdateOne().SetUpsert(true)
	res, err := r.col.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return err
	}
	if res.UpsertedID != nil {
		sub.ID = res.UpsertedID.(bson.ObjectID)
		sub.CreatedAt = now
	}
	return nil
}

func (r *SubscriberRepository) BulkUpsert(ctx context.Context, envID bson.ObjectID, subs []model.Subscriber) error {
	if len(subs) == 0 {
		return nil
	}
	now := time.Now()
	models := make([]mongo.WriteModel, len(subs))
	for i := range subs {
		subs[i].EnvironmentID = envID
		subs[i].UpdatedAt = now
		models[i] = mongo.NewUpdateOneModel().
			SetFilter(bson.M{"environmentId": envID, "subscriberId": subs[i].SubscriberID}).
			SetUpdate(bson.M{
				"$set":         subs[i],
				"$setOnInsert": bson.M{"createdAt": now},
			}).
			SetUpsert(true)
	}
	_, err := r.col.BulkWrite(ctx, models)
	return err
}

func (r *SubscriberRepository) FindByID(ctx context.Context, id bson.ObjectID) (*model.Subscriber, error) {
	var sub model.Subscriber
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&sub)
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

func (r *SubscriberRepository) FindBySubscriberID(ctx context.Context, envID bson.ObjectID, subscriberID string) (*model.Subscriber, error) {
	var sub model.Subscriber
	err := r.col.FindOne(ctx, bson.M{"environmentId": envID, "subscriberId": subscriberID}).Decode(&sub)
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

func (r *SubscriberRepository) FindMany(ctx context.Context, envID bson.ObjectID, page, limit int) ([]model.Subscriber, int64, error) {
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

	var subs []model.Subscriber
	if err := cursor.All(ctx, &subs); err != nil {
		return nil, 0, err
	}
	return subs, total, nil
}

func (r *SubscriberRepository) Update(ctx context.Context, envID bson.ObjectID, subscriberID string, update bson.M) error {
	update["updatedAt"] = time.Now()
	_, err := r.col.UpdateOne(ctx,
		bson.M{"environmentId": envID, "subscriberId": subscriberID},
		bson.M{"$set": update},
	)
	return err
}

func (r *SubscriberRepository) Delete(ctx context.Context, envID bson.ObjectID, subscriberID string) error {
	_, err := r.col.DeleteOne(ctx, bson.M{"environmentId": envID, "subscriberId": subscriberID})
	return err
}

func (r *SubscriberRepository) SetOnlineStatus(ctx context.Context, id bson.ObjectID, online bool) error {
	update := bson.M{"isOnline": online, "updatedAt": time.Now()}
	if online {
		now := time.Now()
		update["lastOnlineAt"] = now
	}
	_, err := r.col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": update})
	return err
}

func (r *SubscriberRepository) Collection() *mongo.Collection {
	return r.col
}
