package domain

import (
	"errors"
	"time"
)

var (
	ErrChatNotFound      = errors.New("chat not found")
	ErrNotChatMember     = errors.New("user is not a member of this chat")
	ErrForbiddenNotAdmin = errors.New("only chat admin can preform this action")
	ErrCannotDirectSelf  = errors.New("cannot create direct caht with yourself")
	ErrInvalidChatType   = errors.New("action is allowed only for gropu chats")
)

type ChatType string

const (
	ChatTypeDirect ChatType = "direct"
	ChatTypeGroup  ChatType = "group"
)

type MemberRole string

const (
	RoleAdmin  MemberRole = "admin"
	RoleMember MemberRole = "member"
)

type Chat struct {
	ID        int64      `json:"id"`
	Type      ChatType   `json:"type"`
	Title     *string    `json:"title,omitempty"`
	Role      MemberRole `json:"role,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}
