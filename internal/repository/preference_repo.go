package repository

import (
	"context"
	"time"

	"github.com/partiri/message-in-a-bottle/internal/model"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type PreferenceRepository struct {
	col *mongo.Collection
}

func NewPreferenceRepository(db *mongo.Database) *PreferenceRepository {
	return &PreferenceRepository{col: db.Collection("subscriber_preferences")}
}

func (r *PreferenceRepository) Upsert(ctx context.Context, pref *model.SubscriberPreference) error {
	pref.UpdatedAt = time.Now()
	filter := bson.M{
		"environmentId": pref.EnvironmentID,
		"subscriberId":  pref.SubscriberID,
		"workflowId":    pref.WorkflowID,
	}
	opts := options.UpdateOne().SetUpsert(true)
	_, err := r.col.UpdateOne(ctx, filter, bson.M{"$set": pref}, opts)
	return err
}

func (r *PreferenceRepository) FindBySubscriber(ctx context.Context, envID, subscriberID bson.ObjectID) ([]model.SubscriberPreference, error) {
	cursor, err := r.col.Find(ctx, bson.M{
		"environmentId": envID,
		"subscriberId":  subscriberID,
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var prefs []model.SubscriberPreference
	if err := cursor.All(ctx, &prefs); err != nil {
		return nil, err
	}
	return prefs, nil
}

func (r *PreferenceRepository) FindBySubscriberAndWorkflow(ctx context.Context, envID, subscriberID bson.ObjectID, workflowID *bson.ObjectID) (*model.SubscriberPreference, error) {
	var pref model.SubscriberPreference
	err := r.col.FindOne(ctx, bson.M{
		"environmentId": envID,
		"subscriberId":  subscriberID,
		"workflowId":    workflowID,
	}).Decode(&pref)
	if err != nil {
		return nil, err
	}
	return &pref, nil
}

func (r *PreferenceRepository) Collection() *mongo.Collection {
	return r.col
}
