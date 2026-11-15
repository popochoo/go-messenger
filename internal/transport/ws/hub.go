package ws

import (
	"context"
	"log/slog"
	"sync"

	"github.com/popochoo/go-messenger/internal/repository/redis"
)

type Hub struct {
	mu      sync.RWMutex
	clients map[int64]map[*Client]bool
	broker  *redis.EventBroker
}

func NewHub(broker *redis.EventBroker) *Hub {
	return &Hub{
		clients: make(map[int64]map[*Client]bool),
		broker:  broker,
	}
}

func (h *Hub) Run(ctx context.Context) {
	err := h.broker.SubcribeEvents(ctx, func(event redis.BroadcastEvent) {
		h.SendToUsers(event.TargetUserIDs, event.Payload)
	})
	if err != nil && ctx.Err() == nil {
		slog.Error("redis pubsub subscription stopped", "error", err)
	}
}

func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[c.userID]; !ok {
		h.clients[c.userID] = make(map[*Client]bool)
	}
	h.clients[c.userID][c] = true
}

func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if userClients, ok := h.clients[c.userID]; ok {
		if _, exists := userClients[c]; exists {
			delete(userClients, c)
			close(c.send)
		}
		if len(userClients) == 0 {
			delete(h.clients, c.userID)
		}
	}
}

func (h *Hub) SendToUsers(userIDs []int64, payload []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, userID := range userIDs {
		for client := range h.clients[userID] {
			select {
			case client.send <- payload:
			default:
				slog.Warn("dropping ws message for slow client", "user_id", userID)
			}
		}
	}
}
