package ws

import "encoding/json"

type Packet struct {
	Type      string          `json:"type"`
	RequestID string          `json:"request_id,omitempty"`
	Payload   json.RawMessage `json:"payload"`
}

type SendMessagePayload struct {
	ChatID  int64  `json:"chat_id"`
	Content string `json:"content"`
}

type AckPayload struct {
	MessageID int64  `json:"message_id"`
	ChatID    int64  `json:"chat_id"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
