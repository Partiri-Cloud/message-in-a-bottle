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

func (r *WorkflowRepository) FindByID(ctx context.Context, envID, id bson.ObjectID) (*model.Workflow, error) {
	var wf model.Workflow
	err := r.col.FindOne(ctx, bson.M{"_id": id, "environmentId": envID}).Decode(&wf)
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

// FindAllActive returns every active workflow in the environment, unpaginated.
//
// The preferences read path needs all of them: a subscriber's effective settings
// are only meaningful per workflow, and a workflow with no stored preference row
// still has defaults that govern delivery. Paginating here would silently hide
// workflows from the settings UI. Environments hold tens of workflows, not
// thousands.
func (r *WorkflowRepository) FindAllActive(ctx context.Context, envID bson.ObjectID) ([]model.Workflow, error) {
	cursor, err := r.col.Find(ctx, bson.M{"environmentId": envID, "isActive": true},
		options.Find().SetSort(bson.D{{Key: "identifier", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var wfs []model.Workflow
	if err := cursor.All(ctx, &wfs); err != nil {
		return nil, err
	}
	return wfs, nil
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

func (r *WorkflowRepository) Update(ctx context.Context, envID, id bson.ObjectID, wf *model.Workflow) error {
	wf.UpdatedAt = time.Now()
	res, err := r.col.ReplaceOne(ctx, bson.M{"_id": id, "environmentId": envID}, wf)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (r *WorkflowRepository) SetActive(ctx context.Context, envID, id bson.ObjectID, active bool) error {
	res, err := r.col.UpdateOne(ctx,
		bson.M{"_id": id, "environmentId": envID},
		bson.M{"$set": bson.M{"isActive": active, "updatedAt": time.Now()}},
	)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (r *WorkflowRepository) Delete(ctx context.Context, envID, id bson.ObjectID) error {
	res, err := r.col.DeleteOne(ctx, bson.M{"_id": id, "environmentId": envID})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (r *WorkflowRepository) Collection() *mongo.Collection {
	return r.col
}
