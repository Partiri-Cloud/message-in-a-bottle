package ws

import (
	"context"

	"github.com/partiri/message-in-a-bottle/internal/repository"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type PresenceTracker struct {
	subRepo *repository.SubscriberRepository
}

func NewPresenceTracker(subRepo *repository.SubscriberRepository) *PresenceTracker {
	return &PresenceTracker{subRepo: subRepo}
}

func (p *PresenceTracker) SetOnline(ctx context.Context, subID bson.ObjectID) {
	p.subRepo.SetOnlineStatus(ctx, subID, true)
}

func (p *PresenceTracker) SetOffline(ctx context.Context, subID bson.ObjectID) {
	p.subRepo.SetOnlineStatus(ctx, subID, false)
}
