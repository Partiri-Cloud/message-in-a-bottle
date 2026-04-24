package repository

import (
	"context"
	"time"

	"github.com/partiri-cloud/message-in-a-bottle/internal/model"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type TemplateRepository struct {
	col *mongo.Collection
}

func NewTemplateRepository(db *mongo.Database) *TemplateRepository {
	return &TemplateRepository{col: db.Collection("transactional_templates")}
}

func (r *TemplateRepository) Create(ctx context.Context, tmpl *model.TransactionalTemplate) error {
	now := time.Now()
	tmpl.CreatedAt = now
	tmpl.UpdatedAt = now
	tmpl.IsActive = true
	res, err := r.col.InsertOne(ctx, tmpl)
	if err != nil {
		return err
	}
	tmpl.ID = res.InsertedID.(bson.ObjectID)
	return nil
}

func (r *TemplateRepository) FindByIdentifier(ctx context.Context, envID bson.ObjectID, identifier string) (*model.TransactionalTemplate, error) {
	var tmpl model.TransactionalTemplate
	err := r.col.FindOne(ctx, bson.M{"environmentId": envID, "identifier": identifier}).Decode(&tmpl)
	if err != nil {
		return nil, err
	}
	return &tmpl, nil
}

func (r *TemplateRepository) FindMany(ctx context.Context, envID bson.ObjectID, page, limit int) ([]model.TransactionalTemplate, int64, error) {
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

	var tmpls []model.TransactionalTemplate
	if err := cursor.All(ctx, &tmpls); err != nil {
		return nil, 0, err
	}
	return tmpls, total, nil
}

func (r *TemplateRepository) Update(ctx context.Context, envID bson.ObjectID, identifier string, tmpl *model.TransactionalTemplate) error {
	tmpl.UpdatedAt = time.Now()
	_, err := r.col.ReplaceOne(ctx, bson.M{"environmentId": envID, "identifier": identifier}, tmpl)
	return err
}

func (r *TemplateRepository) Delete(ctx context.Context, envID bson.ObjectID, identifier string) error {
	_, err := r.col.DeleteOne(ctx, bson.M{"environmentId": envID, "identifier": identifier})
	return err
}

func (r *TemplateRepository) Collection() *mongo.Collection {
	return r.col
}
