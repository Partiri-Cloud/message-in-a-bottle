package repository

import (
	"context"
	"time"

	"github.com/partiri-cloud/message-in-a-box/internal/model"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type IntegrationRepository struct {
	col *mongo.Collection
}

func NewIntegrationRepository(db *mongo.Database) *IntegrationRepository {
	return &IntegrationRepository{col: db.Collection("integrations")}
}

func (r *IntegrationRepository) Create(ctx context.Context, intg *model.Integration) error {
	now := time.Now()
	intg.CreatedAt = now
	intg.UpdatedAt = now
	res, err := r.col.InsertOne(ctx, intg)
	if err != nil {
		return err
	}
	intg.ID = res.InsertedID.(bson.ObjectID)
	return nil
}

func (r *IntegrationRepository) FindByID(ctx context.Context, envID, id bson.ObjectID) (*model.Integration, error) {
	var intg model.Integration
	err := r.col.FindOne(ctx, bson.M{"_id": id, "environmentId": envID}).Decode(&intg)
	if err != nil {
		return nil, err
	}
	return &intg, nil
}

func (r *IntegrationRepository) FindMany(ctx context.Context, envID bson.ObjectID, page, limit int) ([]model.Integration, int64, error) {
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

	var integrations []model.Integration
	if err := cursor.All(ctx, &integrations); err != nil {
		return nil, 0, err
	}
	return integrations, total, nil
}

func (r *IntegrationRepository) FindPrimaryByChannel(ctx context.Context, envID bson.ObjectID, channel string) (*model.Integration, error) {
	var intg model.Integration
	err := r.col.FindOne(ctx, bson.M{
		"environmentId": envID,
		"channel":       channel,
		"isPrimary":     true,
		"isActive":      true,
	}).Decode(&intg)
	if err != nil {
		return nil, err
	}
	return &intg, nil
}

func (r *IntegrationRepository) Update(ctx context.Context, envID, id bson.ObjectID, intg *model.Integration) error {
	intg.UpdatedAt = time.Now()
	_, err := r.col.ReplaceOne(ctx, bson.M{"_id": id, "environmentId": envID}, intg)
	return err
}

func (r *IntegrationRepository) SetPrimary(ctx context.Context, envID bson.ObjectID, id bson.ObjectID) error {
	intg, err := r.FindByID(ctx, envID, id)
	if err != nil {
		return err
	}
	// Unset current primary for this channel
	_, err = r.col.UpdateMany(ctx,
		bson.M{"environmentId": envID, "channel": intg.Channel, "isPrimary": true},
		bson.M{"$set": bson.M{"isPrimary": false, "updatedAt": time.Now()}},
	)
	if err != nil {
		return err
	}
	// Set new primary
	_, err = r.col.UpdateOne(ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"isPrimary": true, "updatedAt": time.Now()}},
	)
	return err
}

func (r *IntegrationRepository) Delete(ctx context.Context, envID, id bson.ObjectID) error {
	_, err := r.col.DeleteOne(ctx, bson.M{"_id": id, "environmentId": envID})
	return err
}

func (r *IntegrationRepository) Collection() *mongo.Collection {
	return r.col
}
