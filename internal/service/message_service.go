package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/popochoo/go-messenger/internal/domain"
)

type MessageRepository interface {
	Create(ctx context.Context, chatID, senderID int64, content string) (*domain.Message, error)
	GetChatMemberIDs(ctx context.Context, chatID int64) ([]int64, error)
	GetHistory(ctx context.Context, chatID, cursor int64, limit int) (*domain.MessagesPage, error)
	Search(ctx context.Context, chatID int64, searchQuery string, limit int) ([]domain.Message, error)
}

type MembersshipChecker interface {
	GetChatAndMemberRole(ctx context.Context, chatID, userID int64) (domain.ChatType, domain.MemberRole, error)
}

type RateLimiter interface {
	CheckRateLimit(ctx context.Context, userID int64, maxMsgs int64, window time.Duration) (bool, error)
}

type MessageService struct {
	messageRepository MessageRepository
	chatRepository    ChatRepository
	rateLimiter       RateLimiter
}

func NewMessageService(
	messageRepository MessageRepository,
	chatRepository ChatRepository,
	rateLimiter RateLimiter,
) *MessageService {
	return &MessageService{
		messageRepository: messageRepository,
		chatRepository:    chatRepository,
		rateLimiter:       rateLimiter,
	}
}

func (s *MessageService) SendMessage(ctx context.Context, senderID, chatID int64, content string) (*domain.Message, []int64, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, nil, domain.ErrEmptyMessage
	}

	if len([]rune(content)) > 4000 {
		return nil, nil, domain.ErrMessageTooLong
	}

	allowed, err := s.rateLimiter.CheckRateLimit(ctx, senderID, 5, 10*time.Second)
	if err == nil && !allowed {
		return nil, nil, domain.ErrRateLimitExceeded
	}

	if _, _, err := s.chatRepository.GetChatAndMemberRole(ctx, chatID, senderID); err != nil {
		return nil, nil, fmt.Errorf("sender not found to chat: %w", err)
	}

	msg, err := s.messageRepository.Create(ctx, chatID, senderID, content)
	if err != nil {
		return nil, nil, fmt.Errorf("create message error: %w", err)
	}

	memberIDs, err := s.messageRepository.GetChatMemberIDs(ctx, chatID)
	if err != nil {
		return msg, nil, fmt.Errorf("get chat member ids error: %w", err)
	}

	return msg, memberIDs, nil
}

func (s *MessageService) GetHistory(ctx context.Context, userID, chatID, cursor int64, limit int) (*domain.MessagesPage, error) {
	if _, _, err := s.chatRepository.GetChatAndMemberRole(ctx, chatID, userID); err != nil {
		return nil, fmt.Errorf("get chat and member role error: %w", err)
	}

	return s.messageRepository.GetHistory(ctx, chatID, cursor, limit)
}

func (s *MessageService) SearchMessages(ctx context.Context, userID, chatID int64, query string) ([]domain.Message, error) {
	if _, _, err := s.chatRepository.GetChatAndMemberRole(ctx, chatID, userID); err != nil {
		return nil, fmt.Errorf("get chat and member role error: %w", err)
	}

	query = strings.TrimSpace(query)
	if query == "" {
		return []domain.Message{}, nil
	}

	return s.messageRepository.Search(ctx, chatID, query, 30)
}
