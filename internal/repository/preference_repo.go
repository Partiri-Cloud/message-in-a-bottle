package repository

import (
	"context"
	"time"

	"github.com/partiri-cloud/message-in-a-bottle/internal/model"
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

// UpdateChannels applies a partial channel update and returns the stored row.
//
// overrides is keyed by delivery-side channel name and holds only the channels
// the caller actually named. Each one is written as its own dotted path, so the
// update is atomic per channel: two concurrent requests, one disabling email and
// one disabling SMS, both land. Merging in the handler and writing all six bools
// back would make the second read-modify-write revert the first — a lost update
// the client never sees, since both requests answer 200.
//
// seed supplies the values for the channels the caller did NOT name, and applies
// only when the row does not exist yet ($setOnInsert). On an existing row the
// unnamed channels are left exactly as they are stored.
//
// The seeded channels go in as dotted paths too: naming "channels" wholesale in
// $setOnInsert while $set holds "channels.email" is a MongoDB path conflict.
//
// An upsert that inserts races the unique index on
// {environmentId, subscriberId, workflowId}: two concurrent first-writes for the
// same subscriber both find no document and both try to insert, and the loser
// comes back with a duplicate-key error. That is expected and recoverable — the
// row now exists, so a single retry takes the update path and merges cleanly.
// Without it, a subscriber toggling two channels at once on a fresh account gets
// a 500 and silently loses one of them.
func (r *PreferenceRepository) UpdateChannels(
	ctx context.Context,
	envID, subscriberID bson.ObjectID,
	workflowID *bson.ObjectID,
	overrides map[string]bool,
	seed model.ChannelPrefs,
) (*model.SubscriberPreference, error) {
	pref, err := r.updateChannelsOnce(ctx, envID, subscriberID, workflowID, overrides, seed)
	if mongo.IsDuplicateKeyError(err) {
		return r.updateChannelsOnce(ctx, envID, subscriberID, workflowID, overrides, seed)
	}
	return pref, err
}

func (r *PreferenceRepository) updateChannelsOnce(
	ctx context.Context,
	envID, subscriberID bson.ObjectID,
	workflowID *bson.ObjectID,
	overrides map[string]bool,
	seed model.ChannelPrefs,
) (*model.SubscriberPreference, error) {
	now := time.Now()

	set := bson.M{"updatedAt": now}
	insert := bson.M{
		"environmentId": envID,
		"subscriberId":  subscriberID,
		"workflowId":    workflowID,
	}

	for _, channel := range model.ChannelNames() {
		field, ok := model.ChannelBSONField(channel)
		if !ok {
			continue
		}
		path := "channels." + field
		if v, named := overrides[channel]; named {
			set[path] = v
			continue
		}
		v, _ := seed.Get(channel)
		insert[path] = v
	}

	filter := bson.M{
		"environmentId": envID,
		"subscriberId":  subscriberID,
		"workflowId":    workflowID,
	}
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)

	var pref model.SubscriberPreference
	err := r.col.FindOneAndUpdate(ctx, filter, bson.M{
		"$set":         set,
		"$setOnInsert": insert,
	}, opts).Decode(&pref)
	if err != nil {
		return nil, err
	}
	return &pref, nil
}

// FindForWorkflow returns the two rows that govern one workflow — the
// subscriber's row for it, and their global row — in a single query. Either may
// be nil. A write path needs only these; loading every row the subscriber has
// just to find them is wasted I/O on a subscriber with many explicit choices.
func (r *PreferenceRepository) FindForWorkflow(ctx context.Context, envID, subscriberID, workflowID bson.ObjectID) (workflowPref, globalPref *model.SubscriberPreference, err error) {
	cursor, err := r.col.Find(ctx, bson.M{
		"environmentId": envID,
		"subscriberId":  subscriberID,
		"workflowId":    bson.M{"$in": bson.A{workflowID, nil}},
	})
	if err != nil {
		return nil, nil, err
	}
	defer cursor.Close(ctx)

	var rows []model.SubscriberPreference
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, nil, err
	}

	for i := range rows {
		row := &rows[i]
		if row.WorkflowID == nil {
			globalPref = row
			continue
		}
		if *row.WorkflowID == workflowID {
			workflowPref = row
		}
	}
	return workflowPref, globalPref, nil
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
