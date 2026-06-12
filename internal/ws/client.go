package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/partiri-cloud/message-in-a-bottle/internal/repository"
	"go.mongodb.org/mongo-driver/v2/bson"
	"nhooyr.io/websocket"
)

const (
	writeWait  = 10 * time.Second
	pingPeriod = 30 * time.Second
	maxMsgSize = 4096
	sendBufLen = 64
	// offlineGrace is how long after a connection closes before the subscriber
	// is marked offline, so quick reconnects don't flap presence.
	offlineGrace = 30 * time.Second
)

type Client struct {
	conn      *websocket.Conn
	hub       *Hub
	room      string
	send      chan []byte
	envID     bson.ObjectID
	subID     bson.ObjectID
	subRepo   *repository.SubscriberRepository
	notifRepo *repository.NotificationRepository
}

func NewClient(conn *websocket.Conn, hub *Hub, room string, envID, subID bson.ObjectID, subRepo *repository.SubscriberRepository, notifRepo *repository.NotificationRepository) *Client {
	return &Client{
		conn:      conn,
		hub:       hub,
		room:      room,
		send:      make(chan []byte, sendBufLen),
		envID:     envID,
		subID:     subID,
		subRepo:   subRepo,
		notifRepo: notifRepo,
	}
}

func (c *Client) Run(ctx context.Context) {
	// Cancel the writer as soon as the reader exits so neither goroutine
	// outlives the connection.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	c.conn.SetReadLimit(maxMsgSize)
	go c.writePump(ctx)
	c.readPump(ctx)
}

func (c *Client) readPump(ctx context.Context) {
	defer func() {
		c.hub.Unregister(c.room, c)
		c.conn.Close(websocket.StatusNormalClosure, "")
		// Grace period for reconnect before marking offline. Only mark offline
		// if the subscriber hasn't opened a new connection in the meantime.
		time.AfterFunc(offlineGrace, func() {
			if c.hub.HasClients(c.room) {
				return
			}
			if err := c.subRepo.SetOnlineStatus(context.Background(), c.subID, false); err != nil {
				slog.Error("failed to mark subscriber offline", "room", c.room, "error", err)
			}
		})
	}()

	for {
		_, data, err := c.conn.Read(ctx)
		if err != nil {
			if websocket.CloseStatus(err) != -1 {
				slog.Debug("websocket closed", "error", err)
			}
			return
		}

		var msg ClientMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		c.handleMessage(ctx, msg)
	}
}

func (c *Client) writePump(ctx context.Context) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	// Closing the connection unblocks readPump when the writer exits first
	// (write timeout, dead peer, server shutdown).
	defer c.conn.Close(websocket.StatusGoingAway, "")

	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				return
			}
			if err := c.writeWithTimeout(ctx, msg); err != nil {
				return
			}
		case <-ticker.C:
			// Ping waits for the pong; an unresponsive peer fails within
			// writeWait instead of parking this goroutine forever.
			if err := c.pingWithTimeout(ctx); err != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (c *Client) writeWithTimeout(ctx context.Context, msg []byte) error {
	wctx, cancel := context.WithTimeout(ctx, writeWait)
	defer cancel()
	return c.conn.Write(wctx, websocket.MessageText, msg)
}

func (c *Client) pingWithTimeout(ctx context.Context) error {
	pctx, cancel := context.WithTimeout(ctx, writeWait)
	defer cancel()
	return c.conn.Ping(pctx)
}

// trySend queues data for the writer without blocking: if the writer is gone
// or backed up, the message is dropped instead of deadlocking readPump.
func (c *Client) trySend(data []byte) {
	select {
	case c.send <- data:
	default:
		slog.Warn("ws client send buffer full, dropping message", "room", c.room)
	}
}

func (c *Client) handleMessage(ctx context.Context, msg ClientMessage) {
	switch msg.Action {
	case ActionSeen:
		var p SeenPayload
		json.Unmarshal(msg.Payload, &p)
		if id, err := bson.ObjectIDFromHex(p.NotificationID); err == nil {
			c.notifRepo.MarkSeen(ctx, c.envID, c.subID, id)
			c.sendUnseenCount(ctx)
		}
	case ActionRead:
		var p ReadPayload
		json.Unmarshal(msg.Payload, &p)
		if id, err := bson.ObjectIDFromHex(p.NotificationID); err == nil {
			c.notifRepo.MarkRead(ctx, c.envID, c.subID, id)
			c.sendUnseenCount(ctx)
		}
	case ActionArchive:
		var p ArchivePayload
		json.Unmarshal(msg.Payload, &p)
		if id, err := bson.ObjectIDFromHex(p.NotificationID); err == nil {
			c.notifRepo.MarkArchived(ctx, c.envID, c.subID, id)
		}
	case ActionFetch:
		var p FetchPayload
		json.Unmarshal(msg.Payload, &p)
		if p.Page <= 0 {
			p.Page = 1
		}
		if p.Limit <= 0 || p.Limit > 50 {
			p.Limit = 20
		}
		filter := repository.FeedFilter{Read: p.Read, Seen: p.Seen}
		notifs, total, _ := c.notifRepo.FindFeed(ctx, c.envID, c.subID, filter, p.Page, p.Limit)
		data, _ := json.Marshal(map[string]any{
			"event": "feed:response",
			"data": map[string]any{
				"notifications": notifs,
				"page":          p.Page,
				"limit":         p.Limit,
				"total":         total,
			},
		})
		c.trySend(data)
	}
}

func (c *Client) sendUnseenCount(ctx context.Context) {
	count, err := c.notifRepo.UnseenCount(ctx, c.envID, c.subID)
	if err != nil {
		return
	}
	data, _ := json.Marshal(map[string]any{
		"event": EventUnseenCount,
		"data":  map[string]any{"count": count},
	})
	c.trySend(data)
}

func RoomKey(envID, subID bson.ObjectID) string {
	return fmt.Sprintf("env:%s:sub:%s", envID.Hex(), subID.Hex())
}
