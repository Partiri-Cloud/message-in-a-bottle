package repository

import (
	"context"
	"time"

	"github.com/partiri-cloud/message-in-a-bottle/internal/model"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type WorkflowRepository struct {
	col *mongo.Collection
}

func NewWorkflowRepository(db *mongo.Database) *WorkflowRepository {
	return &WorkflowRepository{col: db.Collection("workflows")}
}

func (r *WorkflowRepository) Create(ctx context.Context, wf *model.Workflow) error {
	now := time.Now()
	wf.CreatedAt = now
	wf.UpdatedAt = now
	wf.IsActive = true
	res, err := r.col.InsertOne(ctx, wf)
	if err != nil {
		return err
	}
	wf.ID = res.InsertedID.(bson.ObjectID)
	return nil
}

func (r *WorkflowRepository) FindByID(ctx context.Context, id bson.ObjectID) (*model.Workflow, error) {
	var wf model.Workflow
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&wf)
	if err != nil {
		return nil, err
	}
	return &wf, nil
}

func (r *WorkflowRepository) FindByIdentifier(ctx context.Context, envID bson.ObjectID, identifier string) (*model.Workflow, error) {
	var wf model.Workflow
	err := r.col.FindOne(ctx, bson.M{"environmentId": envID, "identifier": identifier}).Decode(&wf)
	if err != nil {
		return nil, err
	}
	return &wf, nil
}

func (r *WorkflowRepository) FindMany(ctx context.Context, envID bson.ObjectID, page, limit int) ([]model.Workflow, int64, error) {
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

	var workflows []model.Workflow
	if err := cursor.All(ctx, &workflows); err != nil {
		return nil, 0, err
	}
	return workflows, total, nil
}

func (r *WorkflowRepository) Update(ctx context.Context, id bson.ObjectID, wf *model.Workflow) error {
	wf.UpdatedAt = time.Now()
	_, err := r.col.ReplaceOne(ctx, bson.M{"_id": id}, wf)
	return err
}

func (r *WorkflowRepository) SetActive(ctx context.Context, id bson.ObjectID, active bool) error {
	_, err := r.col.UpdateOne(ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"isActive": active, "updatedAt": time.Now()}},
	)
	return err
}

func (r *WorkflowRepository) Delete(ctx context.Context, id bson.ObjectID) error {
	_, err := r.col.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (r *WorkflowRepository) Collection() *mongo.Collection {
	return r.col
}
