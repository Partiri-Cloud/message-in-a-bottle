package ws

import (
	"context"
	"encoding/json"
	"log"
	"sync"

	"github.com/redis/go-redis/v9"
)

type Hub struct {
	mu      sync.RWMutex
	clients map[string][]*Client // room → clients
	rdb     *redis.Client
}

func NewHub(rdb *redis.Client) *Hub {
	return &Hub{
		clients: make(map[string][]*Client),
		rdb:     rdb,
	}
}

func (h *Hub) Register(room string, client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[room] = append(h.clients[room], client)
	log.Printf("client registered to room %s (total: %d)", room, len(h.clients[room]))
}

func (h *Hub) Unregister(room string, client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	clients := h.clients[room]
	for i, c := range clients {
		if c == client {
			h.clients[room] = append(clients[:i], clients[i+1:]...)
			break
		}
	}
	if len(h.clients[room]) == 0 {
		delete(h.clients, room)
	}
}

func (h *Hub) Send(room string, data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, client := range h.clients[room] {
		select {
		case client.send <- data:
		default:
			log.Printf("client send buffer full for room %s, dropping message", room)
		}
	}
}

func (h *Hub) SubscribeRedis(ctx context.Context) {
	sub := h.rdb.Subscribe(ctx, "ws:notifications")
	ch := sub.Channel()

	go func() {
		defer sub.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				var wsMsg WSMessage
				if err := json.Unmarshal([]byte(msg.Payload), &wsMsg); err != nil {
					log.Printf("invalid ws message from redis: %v", err)
					continue
				}
				data, _ := json.Marshal(map[string]any{
					"event": wsMsg.Event,
					"data":  wsMsg.Data,
				})
				h.Send(wsMsg.Room, data)
			}
		}
	}()
}

func (h *Hub) ConnectionCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	total := 0
	for _, clients := range h.clients {
		total += len(clients)
	}
	return total
}
