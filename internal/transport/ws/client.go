package ws

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/popochoo/go-messenger/internal/domain"
	"github.com/popochoo/go-messenger/internal/repository/redis"
	"github.com/popochoo/go-messenger/internal/service"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 8192
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type TokenParser interface {
	ParseToken(tokenStr, expectedType string) (int64, error)
}

type Handler struct {
	hub            *Hub
	messageService *service.MessageService
	broker         *redis.EventBroker
	parser         TokenParser
}

func NewHandler(
	hub *Hub,
	messageService *service.MessageService,
	broker *redis.EventBroker,
	parser TokenParser,
) *Handler {
	return &Handler{
		hub:            hub,
		messageService: messageService,
		broker:         broker,
		parser:         parser,
	}
}

func (h *Handler) ServerWS(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing tolen query parameter"})
		return
	}

	userID, err := h.parser.ParseToken(token, "access")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Error("websocket upgrade failed", "error", err)
		return
	}

	client := &Client{
		hub:     h.hub,
		handler: h,
		conn:    conn,
		userID:  userID,
		send:    make(chan []byte, 256),
	}

	h.hub.Register(client)

	go client.writePump()
	go client.readPump()
}

type Client struct {
	hub     *Hub
	handler *Handler
	conn    *websocket.Conn
	userID  int64
	send    chan []byte
}

func (c *Client) readPump() {
	defer func() {
		c.hub.Unregister(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, rawMsg, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Warn("ws unexpected close", "user_id", c.userID, "error", err)
			}

			break
		}

		var pkt Packet
		if err := json.Unmarshal(rawMsg, &pkt); err != nil {
			c.sendError("", "BAD_FORMAT", "invalid JSON packet")
			continue
		}

		switch pkt.Type {
		case "message.send":
			c.handleSendMessage(pkt)
		default:
			c.sendError(pkt.RequestID, "UNKNOW_EVENT", "unsupported packet type")
		}
	}
}

func (c *Client) handleSendMessage(pkt Packet) {
	var payload SendMessagePayload
	if err := json.Unmarshal(pkt.Payload, &payload); err != nil {
		c.sendError(pkt.RequestID, "BAD_PAYLOAD", "invalid message.send payload")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	msg, memberIDs, err := c.handler.messageService.SendMessage(ctx, c.userID, payload.ChatID, payload.Content)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrRateLimitExceeded):
			c.sendError(pkt.RequestID, "RATE_LIMIT_EXCEEDED", err.Error())
		case errors.Is(err, domain.ErrNotChatMember), errors.Is(err, domain.ErrChatNotFound):
			c.sendError(pkt.RequestID, "FORBIDDEN", err.Error())
		case errors.Is(err, domain.ErrEmptyMessage), errors.Is(err, domain.ErrMessageTooLong):
			c.sendError(pkt.RequestID, "VALIDATION_ERROR", err.Error())
		default:
			slog.Error("failed to send message", "error", err)
			c.sendError(pkt.RequestID, "INTERNAL_ERROR", "failed to save message")
		}
		return
	}

	ackBytes, _ := json.Marshal(AckPayload{
		MessageID: msg.ID,
		ChatID:    msg.ChatID,
		Status:    "delivered",
		CreatedAt: msg.CreatedAt.Format(time.RFC3339),
	})
	ackPacket, _ := json.Marshal(Packet{
		Type:      "message.ack",
		RequestID: pkt.RequestID,
		Payload:   ackBytes,
	})
	c.send <- ackPacket

	msgBytes, _ := json.Marshal(msg)
	newEventPacket, _ := json.Marshal(Packet{
		Type:    "message.new",
		Payload: msgBytes,
	})
	// ОТКЛАДКА
	slog.Info("publishing event", "member_ids", memberIDs, "msg_id", msg.ID)

	if err := c.handler.broker.PublishEvent(ctx, memberIDs, newEventPacket); err != nil {
		slog.Error("failed to publish event to redis", "redis", err)
	}
}

func (c *Client) sendError(requestID, code, message string) {
	errPayload, _ := json.Marshal(ErrorPayload{
		Code:    code,
		Message: message,
	})
	pkt, _ := json.Marshal(Packet{
		Type:      "error",
		RequestID: requestID,
		Payload:   errPayload,
	})

	select {
	case c.send <- pkt:
	default:
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
