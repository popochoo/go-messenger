package domain

import (
	"errors"
	"time"
)

var (
	ErrEmptyMessage      = errors.New("message content cannot be empty")
	ErrMessageTooLong    = errors.New("message content exceeds 4000 characters")
	ErrRateLimitExceeded = errors.New("rate limit exceeded: too many messages")
	ErrMessageNotFound   = errors.New("message not found")
	ErrNotMessageAuthor  = errors.New("only message author can edit or delet it")
)

type Message struct {
	ID        int64     `json:"id"`
	ChatID    int64     `json:"chat_id"`
	SenderID  int64     `json:"sender_id"`
	Content   string    `json:"content"`
	ReplyToID *int64    `json:"reply_to_id,omitempty"`
	IsEdited  bool      `json:"is_edited"`
	IsDeleted bool      `json:"is_deleted"`
	CreatedAt time.Time `json:"created_at"`
}

type MessagesPage struct {
	Messages   []Message `json:"messages"`
	NextCursor int64     `json:"next_cursor,omitempty"`
	HasMore    bool      `json:"has_more"`
}
