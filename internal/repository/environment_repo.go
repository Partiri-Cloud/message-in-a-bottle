package repository

import (
	"context"
	"time"

	"github.com/partiri-cloud/message-in-a-bottle/internal/model"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type EnvironmentRepository struct {
	col *mongo.Collection
}

func NewEnvironmentRepository(db *mongo.Database) *EnvironmentRepository {
	return &EnvironmentRepository{col: db.Collection("environments")}
}

func (r *EnvironmentRepository) Create(ctx context.Context, env *model.Environment) error {
	now := time.Now()
	env.CreatedAt = now
	env.UpdatedAt = now
	res, err := r.col.InsertOne(ctx, env)
	if err != nil {
		return err
	}
	env.ID = res.InsertedID.(bson.ObjectID)
	return nil
}

func (r *EnvironmentRepository) FindByID(ctx context.Context, id bson.ObjectID) (*model.Environment, error) {
	var env model.Environment
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&env)
	if err != nil {
		return nil, err
	}
	return &env, nil
}

func (r *EnvironmentRepository) FindByIdentifier(ctx context.Context, identifier string) (*model.Environment, error) {
	var env model.Environment
	err := r.col.FindOne(ctx, bson.M{"identifier": identifier}).Decode(&env)
	if err != nil {
		return nil, err
	}
	return &env, nil
}

func (r *EnvironmentRepository) FindByAPIKeyHash(ctx context.Context, keyHash string) (*model.Environment, *model.APIKey, error) {
	var env model.Environment
	err := r.col.FindOne(ctx, bson.M{"apiKeys.keyHash": keyHash}).Decode(&env)
	if err != nil {
		return nil, nil, err
	}
	for i := range env.APIKeys {
		if env.APIKeys[i].KeyHash == keyHash {
			return &env, &env.APIKeys[i], nil
		}
	}
	return nil, nil, mongo.ErrNoDocuments
}

func (r *EnvironmentRepository) UpdateLastUsedAt(ctx context.Context, envID bson.ObjectID, keyHash string, t time.Time) error {
	_, err := r.col.UpdateOne(ctx,
		bson.M{"_id": envID, "apiKeys.keyHash": keyHash},
		bson.M{"$set": bson.M{"apiKeys.$.lastUsedAt": t}},
	)
	return err
}

func (r *EnvironmentRepository) AddAPIKey(ctx context.Context, envID bson.ObjectID, key model.APIKey) error {
	_, err := r.col.UpdateOne(ctx,
		bson.M{"_id": envID},
		bson.M{
			"$push": bson.M{"apiKeys": key},
			"$set":  bson.M{"updatedAt": time.Now()},
		},
	)
	return err
}

func (r *EnvironmentRepository) FindAll(ctx context.Context) ([]model.Environment, error) {
	cursor, err := r.col.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var envs []model.Environment
	if err := cursor.All(ctx, &envs); err != nil {
		return nil, err
	}
	return envs, nil
}

func (r *EnvironmentRepository) Collection() *mongo.Collection {
	return r.col
}
