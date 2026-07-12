package repository

import (
	"context"
	"time"

	"github.com/partiri-cloud/message-in-a-bottle/internal/model"
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

func (r *NotificationRepository) FindByID(ctx context.Context, envID, id bson.ObjectID) (*model.Notification, error) {
	var notif model.Notification
	err := r.col.FindOne(ctx, bson.M{"_id": id, "environmentId": envID}).Decode(&notif)
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

// deliveredInApp restricts a query to notifications whose in_app step actually
// went out.
//
// A notification document is created for every recipient at trigger time, before
// the worker evaluates preferences — so a workflow whose in_app step the
// subscriber has turned off, or which has no in_app step at all, still leaves a
// row behind. That row is never rendered (SetRenderedContent only runs on
// delivery), so it carries no subject and no content: in a feed it shows up as a
// blank entry with nothing but a timestamp, and it counts toward the unseen
// badge. Deploying a service produced exactly that — one real notification from
// deploy-succeeded, and one empty one from deploy-started, which the user had
// switched off.
//
// The feed is the in-app inbox: it holds what was delivered in-app, nothing else.
// Delivery status for the other channels stays on the document and remains
// visible through the notifications/activity endpoints.
func deliveredInApp() bson.M {
	return bson.M{
		"$elemMatch": bson.M{
			"channel": "in_app",
			"status":  bson.M{"$in": []string{"sent", "delivered"}},
		},
	}
}

func (r *NotificationRepository) FindFeed(ctx context.Context, envID, subscriberID bson.ObjectID, filter FeedFilter, page, limit int) ([]model.Notification, int64, error) {
	q := bson.M{
		"environmentId": envID,
		"subscriberId":  subscriberID,
		"channels":      deliveredInApp(),
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

func (r *NotificationRepository) MarkSeen(ctx context.Context, envID, subID, id bson.ObjectID) error {
	now := time.Now()
	res, err := r.col.UpdateOne(ctx,
		bson.M{"_id": id, "environmentId": envID, "subscriberId": subID},
		bson.M{"$set": bson.M{"seen": true, "seenAt": now, "updatedAt": now}},
	)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (r *NotificationRepository) MarkRead(ctx context.Context, envID, subID, id bson.ObjectID) error {
	now := time.Now()
	res, err := r.col.UpdateOne(ctx,
		bson.M{"_id": id, "environmentId": envID, "subscriberId": subID},
		bson.M{"$set": bson.M{"read": true, "readAt": now, "seen": true, "seenAt": now, "updatedAt": now}},
	)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (r *NotificationRepository) MarkArchived(ctx context.Context, envID, subID, id bson.ObjectID) error {
	now := time.Now()
	res, err := r.col.UpdateOne(ctx,
		bson.M{"_id": id, "environmentId": envID, "subscriberId": subID},
		bson.M{"$set": bson.M{"archivedAt": now, "updatedAt": now}},
	)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (r *NotificationRepository) BulkMarkRead(ctx context.Context, envID, subID bson.ObjectID, ids []bson.ObjectID) error {
	now := time.Now()
	_, err := r.col.UpdateMany(ctx,
		bson.M{"_id": bson.M{"$in": ids}, "environmentId": envID, "subscriberId": subID},
		bson.M{"$set": bson.M{"read": true, "readAt": now, "seen": true, "seenAt": now, "updatedAt": now}},
	)
	return err
}

func (r *NotificationRepository) BulkMarkSeen(ctx context.Context, envID, subID bson.ObjectID, ids []bson.ObjectID) error {
	now := time.Now()
	_, err := r.col.UpdateMany(ctx,
		bson.M{"_id": bson.M{"$in": ids}, "environmentId": envID, "subscriberId": subID},
		bson.M{"$set": bson.M{"seen": true, "seenAt": now, "updatedAt": now}},
	)
	return err
}

// UnseenCount counts what the feed would show — the same delivered-in-app
// predicate. Counting undelivered rows would put a badge on the bell for
// notifications the user cannot open, and which cannot be cleared by reading
// them, because they are not in the feed to begin with.
func (r *NotificationRepository) UnseenCount(ctx context.Context, envID, subscriberID bson.ObjectID) (int64, error) {
	return r.col.CountDocuments(ctx, bson.M{
		"environmentId": envID,
		"subscriberId":  subscriberID,
		"seen":          false,
		"archivedAt":    nil,
		"channels":      deliveredInApp(),
	})
}

// SetRenderedContent persists the in-app step's rendered subject and body on the
// notification itself, so the feed can return them to clients that were not
// connected when the WebSocket push went out.
//
// Scoped by environmentId like every other query in this repository: the caller
// happens to hold an id read off the notification document, but tenant isolation
// is an invariant of the collection, not of one call site.
func (r *NotificationRepository) SetRenderedContent(ctx context.Context, envID, notifID bson.ObjectID, subject, content string) error {
	_, err := r.col.UpdateOne(ctx,
		bson.M{"_id": notifID, "environmentId": envID},
		bson.M{"$set": bson.M{
			"subject":   subject,
			"content":   content,
			"updatedAt": time.Now(),
		}},
	)
	return err
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
