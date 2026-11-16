package service

import (
	"context"
	"encoding/json"
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
	GetByID(ctx context.Context, messageID int64) (*domain.Message, error)
	UpdateContent(ctx context.Context, messageID int64, newContent string) (*domain.Message, error)
	SoftDelete(ctx context.Context, messageID int64) (*domain.Message, error)
	MarkAsRead(ctx context.Context, chatID, userID, messageID int64) (int64, error)
}

type MembersshipChecker interface {
	GetChatAndMemberRole(ctx context.Context, chatID, userID int64) (domain.ChatType, domain.MemberRole, error)
}

type EventBroker interface {
	CheckRateLimit(ctx context.Context, userID int64, maxMsgs int64, window time.Duration) (bool, error)
	PublishEvent(ctx context.Context, targetUserIDs []int64, payload []byte) error
}

type wsEnvelope struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

type MessageService struct {
	messageRepository MessageRepository
	chatRepository    ChatRepository
	eventBroker       EventBroker
}

func NewMessageService(
	messageRepository MessageRepository,
	chatRepository ChatRepository,
	eventBroker EventBroker,
) *MessageService {
	return &MessageService{
		messageRepository: messageRepository,
		chatRepository:    chatRepository,
		eventBroker:       eventBroker,
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

	allowed, err := s.eventBroker.CheckRateLimit(ctx, senderID, 5, 10*time.Second)
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

func (s MessageService) EditMessage(ctx context.Context, actorID, chatID, messageID int64, newContent string) (*domain.Message, error) {
	newContent = strings.TrimSpace(newContent)
	if newContent == "" {
		return nil, domain.ErrEmptyMessage
	}
	if len([]rune(newContent)) > 4000 {
		return nil, domain.ErrMessageTooLong
	}

	msg, err := s.messageRepository.GetByID(ctx, messageID)
	if err != nil {
		return nil, err
	}
	if msg.ChatID != chatID || msg.IsDeleted {
		return nil, domain.ErrMessageNotFound
	}
	if msg.SenderID != actorID {
		return nil, domain.ErrNotMessageAuthor
	}

	updatedMsg, err := s.messageRepository.UpdateContent(ctx, messageID, newContent)
	if err != nil {
		return nil, err
	}

	if memberIDs, err := s.messageRepository.GetChatMemberIDs(ctx, chatID); err == nil {
		if pkt, err := json.Marshal(wsEnvelope{Type: "message.updated", Payload: updatedMsg}); err == nil {
			_ = s.eventBroker.PublishEvent(ctx, memberIDs, pkt)
		}
	}

	return updatedMsg, nil
}

func (s *MessageService) DeleteMessage(ctx context.Context, actorID, chatID, messageID int64) (*domain.Message, error) {
	msg, err := s.messageRepository.GetByID(ctx, messageID)
	if err != nil {
		return nil, err
	}
	if msg.ChatID != chatID || msg.IsDeleted {
		return nil, domain.ErrMessageNotFound
	}
	if msg.SenderID != actorID {
		return nil, domain.ErrNotMessageAuthor
	}

	deletedMsg, err := s.messageRepository.SoftDelete(ctx, messageID)
	if err != nil {
		return nil, err
	}

	if memberIDs, err := s.messageRepository.GetChatMemberIDs(ctx, chatID); err == nil {
		if pkt, err := json.Marshal(wsEnvelope{Type: "message.deleted", Payload: deletedMsg}); err == nil {
			_ = s.eventBroker.PublishEvent(ctx, memberIDs, pkt)
		}
	}

	return deletedMsg, nil
}

func (s *MessageService) MarkChatAsRead(ctx context.Context, userID, chatID, lastReadMessageID int64) (int64, error) {
	if _, _, err := s.chatRepository.GetChatAndMemberRole(ctx, chatID, userID); err != nil {
		return 0, err
	}

	updatedID, err := s.messageRepository.MarkAsRead(ctx, chatID, userID, lastReadMessageID)
	if err != nil {
		return 0, err
	}

	if memberIDs, err := s.messageRepository.GetChatMemberIDs(ctx, chatID); err == nil {
		payload := map[string]any{
			"chat_id":              chatID,
			"user_id":              userID,
			"last_read_message_id": lastReadMessageID,
		}
		if pkt, err := json.Marshal(wsEnvelope{Type: "chat.read", Payload: payload}); err != nil {
			_ = s.eventBroker.PublishEvent(ctx, memberIDs, pkt)
		}
	}

	return updatedID, nil
}
