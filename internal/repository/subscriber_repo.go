package repository

import (
	"context"
	"strings"
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

// defaultLocale is applied on insert when the caller does not supply one.
const defaultLocale = "en"

// subscriberUpdate is the update document for one subscriber upsert, split into
// the stages MongoDB requires. A field may appear in only one of them, or the
// write is rejected as a path conflict.
type subscriberUpdate struct {
	set      bson.M // fields the caller supplied
	addToSet bson.M // token arrays, merged rather than replaced
	insert   bson.M // defaults seeded only when the row is created
}

func (u subscriberUpdate) document() bson.M {
	doc := bson.M{"$set": u.set, "$setOnInsert": u.insert}
	if len(u.addToSet) > 0 {
		doc["$addToSet"] = u.addToSet
	}
	return doc
}

// buildSubscriberUpdate marshals a subscriber into an upsert that writes only
// what the caller actually supplied.
//
// An upsert is re-run for an existing subscriber every time the caller re-posts
// them — Harbor does exactly that on every notification-token mint — so anything
// left in $set that the caller did not supply arrives as a zero value and
// silently overwrites stored state:
//
//   - channels: $set on a nested document replaces it wholesale, so a payload
//     carrying only a Slack webhook would erase the subscriber's push tokens.
//     The subdocument is flattened into one dotted path per leaf instead, so
//     channel config merges per field. The flattening is generic — it walks
//     whatever bson.Marshal produced — so a channel added to the model merges
//     without touching this file. omitempty on the leaves is what makes "the
//     caller supplied it" and "it is present" the same question.
//   - push token arrays: merged with $addToSet, not replaced. A second device
//     registering its own token must not evict the first device's.
//   - locale: no omitempty, so an unset one would stomp the stored value.
//   - isOnline: no omitempty, so every re-post would flip a connected subscriber
//     offline. Presence is owned by SetOnlineStatus and the WebSocket lifecycle;
//     a profile write has no business touching it.
//
// Corollary of both merge rules: a profile upsert cannot *clear* channel config
// or remove a push token. Those need a dedicated operation.
//
// createdAt is managed solely by $setOnInsert (naming it in both stages is a
// MongoDB path conflict, which is what broke subscriber creation outright), and
// _id is immutable.
func buildSubscriberUpdate(sub *model.Subscriber, now time.Time) (subscriberUpdate, error) {
	data, err := bson.Marshal(sub)
	if err != nil {
		return subscriberUpdate{}, err
	}
	var doc bson.M
	if err := bson.Unmarshal(data, &doc); err != nil {
		return subscriberUpdate{}, err
	}

	channels := doc["channels"]
	delete(doc, "createdAt")
	delete(doc, "_id")
	delete(doc, "isOnline")
	delete(doc, "channels")
	if sub.Locale == "" {
		delete(doc, "locale")
	}

	u := subscriberUpdate{
		set:      doc,
		addToSet: bson.M{},
		insert:   bson.M{"createdAt": now, "isOnline": false},
	}
	flattenChannels("channels", channels, u.set, u.addToSet)

	if _, ok := u.set["locale"]; !ok {
		u.insert["locale"] = defaultLocale
	}
	// Seeding "channels" conflicts with any "channels.*" path written above, and
	// where such a path exists MongoDB materializes the nested document itself on
	// insert — so the seed is only needed, and only legal, when there is none.
	if len(u.addToSet) == 0 && !hasChannelPath(u.set) {
		u.insert["channels"] = model.SubscriberChannels{}
	}
	return u, nil
}

// flattenChannels walks a marshaled channels document and records each leaf as a
// dotted path: scalars into set, arrays into addToSet so they merge. Empty
// values never reach here — omitempty on the model drops them — which is exactly
// what makes an unsupplied field leave the stored one alone.
//
// Both bson.D and bson.M are accepted, and that is load-bearing: unmarshaling
// into a bson.M makes only the *top level* a map, and every subdocument beneath
// it — channels included — arrives as a bson.D.
func flattenChannels(prefix string, doc any, set, addToSet bson.M) {
	switch d := doc.(type) {
	case bson.M:
		for key, value := range d {
			flattenChannelValue(prefix+"."+key, value, set, addToSet)
		}
	case bson.D:
		for _, elem := range d {
			flattenChannelValue(prefix+"."+elem.Key, elem.Value, set, addToSet)
		}
	}
}

func flattenChannelValue(path string, value any, set, addToSet bson.M) {
	switch v := value.(type) {
	case bson.M, bson.D:
		flattenChannels(path, v, set, addToSet)
	case bson.A:
		if len(v) > 0 {
			addToSet[path] = bson.M{"$each": v}
		}
	default:
		set[path] = value
	}
}

func hasChannelPath(setDoc bson.M) bool {
	for k := range setDoc {
		if strings.HasPrefix(k, "channels.") {
			return true
		}
	}
	return false
}

func (r *SubscriberRepository) Upsert(ctx context.Context, envID bson.ObjectID, sub *model.Subscriber) error {
	now := time.Now()
	sub.EnvironmentID = envID
	sub.UpdatedAt = now

	update, err := buildSubscriberUpdate(sub, now)
	if err != nil {
		return err
	}

	filter := bson.M{"environmentId": envID, "subscriberId": sub.SubscriberID}

	// Read the document back and decode it over sub, so the caller holds what is
	// actually stored rather than what it sent. On the update path the payload is
	// sparse by design — no id, no createdAt, and none of the fields this upsert
	// deliberately preserves — so returning the caller's struct would report an
	// existing subscriber as having a zero id, a blank locale and no channels.
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
	return r.col.FindOneAndUpdate(ctx, filter, update.document(), opts).Decode(sub)
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
		update, err := buildSubscriberUpdate(&subs[i], now)
		if err != nil {
			return err
		}
		models[i] = mongo.NewUpdateOneModel().
			SetFilter(bson.M{"environmentId": envID, "subscriberId": subs[i].SubscriberID}).
			SetUpdate(update.document()).
			SetUpsert(true)
	}
	_, err := r.col.BulkWrite(ctx, models)
	return err
}

// RemovePushTokens deletes the named device tokens from a subscriber.
//
// The upsert path merges tokens with $addToSet so a second device cannot evict
// the first, which means it can only ever add. Removal therefore needs its own
// operation: without one, a token from an uninstalled app stays forever and the
// worker keeps firing push at a dead device. Removing a token the subscriber
// does not have is a no-op, not an error — an unregister call is naturally
// idempotent and clients retry it.
func (r *SubscriberRepository) RemovePushTokens(ctx context.Context, envID bson.ObjectID, subscriberID string, fcmTokens, apnsTokens []string) (*model.Subscriber, error) {
	pull := bson.M{}
	if len(fcmTokens) > 0 {
		pull["channels.push.fcmTokens"] = bson.M{"$in": fcmTokens}
	}
	if len(apnsTokens) > 0 {
		pull["channels.push.apnsTokens"] = bson.M{"$in": apnsTokens}
	}
	if len(pull) == 0 {
		return r.FindBySubscriberID(ctx, envID, subscriberID)
	}

	filter := bson.M{"environmentId": envID, "subscriberId": subscriberID}
	update := bson.M{"$pull": pull, "$set": bson.M{"updatedAt": time.Now()}}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)

	var sub model.Subscriber
	if err := r.col.FindOneAndUpdate(ctx, filter, update, opts).Decode(&sub); err != nil {
		return nil, err
	}
	return &sub, nil
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

// FindPageAfter returns up to limit subscribers of the environment with IDs
// greater than afterID, in _id order. Passing the zero ObjectID starts from
// the beginning. Unlike skip-based pagination, walking pages this way stays
// indexed no matter how deep the walk gets — a full-environment sweep is O(n)
// instead of re-scanning every earlier page each time.
func (r *SubscriberRepository) FindPageAfter(ctx context.Context, envID, afterID bson.ObjectID, limit int) ([]model.Subscriber, error) {
	filter := bson.M{"environmentId": envID}
	if !afterID.IsZero() {
		filter["_id"] = bson.M{"$gt": afterID}
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "_id", Value: 1}}).
		SetLimit(int64(limit))

	cursor, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var subs []model.Subscriber
	if err := cursor.All(ctx, &subs); err != nil {
		return nil, err
	}
	return subs, nil
}

func (r *SubscriberRepository) Update(ctx context.Context, envID bson.ObjectID, subscriberID string, update bson.M) error {
	update["updatedAt"] = time.Now()
	res, err := r.col.UpdateOne(ctx,
		bson.M{"environmentId": envID, "subscriberId": subscriberID},
		bson.M{"$set": update},
	)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
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
