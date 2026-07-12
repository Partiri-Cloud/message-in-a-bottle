package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/coder/websocket"
	"github.com/partiri-cloud/message-in-a-bottle/internal/repository"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const (
	// writeWait bounds every write and ping. The library closes the connection
	// when the operation's context expires, so a half-open peer that stops
	// acking is reaped instead of blocking the pumps until process shutdown.
	writeWait  = 10 * time.Second
	pingPeriod = 30 * time.Second
	maxMsgSize = 4096
	sendBufLen = 64
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
	c.conn.SetReadLimit(maxMsgSize)
	go c.writePump(ctx)
	c.readPump(ctx)
}

func (c *Client) readPump(ctx context.Context) {
	defer func() {
		c.hub.Unregister(c.room, c)
		c.conn.Close(websocket.StatusNormalClosure, "")
		// Grace period for reconnect before marking offline
		time.AfterFunc(30*time.Second, func() {
			c.subRepo.SetOnlineStatus(context.Background(), c.subID, false)
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

	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				return
			}
			wctx, cancel := context.WithTimeout(ctx, writeWait)
			err := c.conn.Write(wctx, websocket.MessageText, msg)
			cancel()
			if err != nil {
				return
			}
		case <-ticker.C:
			wctx, cancel := context.WithTimeout(ctx, writeWait)
			err := c.conn.Ping(wctx)
			cancel()
			if err != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (c *Client) handleMessage(ctx context.Context, msg ClientMessage) {
	switch msg.Action {
	case ActionSeen:
		var p SeenPayload
		json.Unmarshal(msg.Payload, &p)
		if id, err := bson.ObjectIDFromHex(p.NotificationID); err == nil {
			if err := c.notifRepo.MarkSeen(ctx, c.envID, c.subID, id); err != nil {
				if err == mongo.ErrNoDocuments {
					slog.Debug("mark seen: notification not found", "notificationId", p.NotificationID)
				} else {
					slog.Debug("mark seen failed", "error", err)
				}
				return
			}
			c.sendUnseenCount(ctx)
		}
	case ActionRead:
		var p ReadPayload
		json.Unmarshal(msg.Payload, &p)
		if id, err := bson.ObjectIDFromHex(p.NotificationID); err == nil {
			if err := c.notifRepo.MarkRead(ctx, c.envID, c.subID, id); err != nil {
				if err == mongo.ErrNoDocuments {
					slog.Debug("mark read: notification not found", "notificationId", p.NotificationID)
				} else {
					slog.Debug("mark read failed", "error", err)
				}
				return
			}
			c.sendUnseenCount(ctx)
		}
	case ActionArchive:
		var p ArchivePayload
		json.Unmarshal(msg.Payload, &p)
		if id, err := bson.ObjectIDFromHex(p.NotificationID); err == nil {
			if err := c.notifRepo.MarkArchived(ctx, c.envID, c.subID, id); err != nil {
				if err == mongo.ErrNoDocuments {
					slog.Debug("mark archived: notification not found", "notificationId", p.NotificationID)
				} else {
					slog.Debug("mark archived failed", "error", err)
				}
				return
			}
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
		c.send <- data
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
	c.send <- data
}

func RoomKey(envID, subID bson.ObjectID) string {
	return fmt.Sprintf("env:%s:sub:%s", envID.Hex(), subID.Hex())
}
