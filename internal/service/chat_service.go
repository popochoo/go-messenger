package service

import (
	"context"
	"fmt"

	"github.com/popochoo/go-messenger/internal/domain"
)

type ChatRepository interface {
	GetOrCreateDirect(ctx context.Context, user1ID, user2ID int64) (*domain.Chat, bool, error)
	CreateGroup(ctx context.Context, creatorID int64, title string, memberIDs []int64) (*domain.Chat, error)
	GetUsersChats(ctx context.Context, userID int64) ([]domain.Chat, error)
	GetChatAndMemberRole(ctx context.Context, chatID, userID int64) (domain.ChatType, domain.MemberRole, error)
	AddMember(ctx context.Context, chatID, userID int64) error
	RemoveMember(ctx context.Context, chatID, userID int64) error
}

type UserChecker interface {
	GetByID(ctx context.Context, id int64) (*domain.User, error)
}

type ChatService struct {
	chatRepository ChatRepository
	userRepository UserChecker
}

func NewChatService(chatRepository ChatRepository, userRepository UserChecker) *ChatService {
	return &ChatService{
		chatRepository: chatRepository,
		userRepository: userRepository,
	}
}

func (s *ChatService) CreateDirectChat(ctx context.Context, currentUserID, targetUserID int64) (*domain.Chat, bool, error) {
	if currentUserID == targetUserID {
		return nil, false, domain.ErrCannotDirectSelf
	}

	if _, err := s.userRepository.GetByID(ctx, targetUserID); err != nil {
		return nil, false, fmt.Errorf("target user not found: %w", err)
	}

	return s.chatRepository.GetOrCreateDirect(ctx, currentUserID, targetUserID)
}

func (s *ChatService) CreateGroupChat(ctx context.Context, creatorID int64, title string, memberIDs []int64) (*domain.Chat, error) {
	return s.chatRepository.CreateGroup(ctx, creatorID, title, memberIDs)
}

func (s *ChatService) ListUserChats(ctx context.Context, userID int64) ([]domain.Chat, error) {
	return s.chatRepository.GetUsersChats(ctx, userID)
}

func (s *ChatService) AddMember(ctx context.Context, actorID, chatID, targetUserID int64) error {
	chatType, role, err := s.chatRepository.GetChatAndMemberRole(ctx, chatID, actorID)
	if err != nil {
		return fmt.Errorf("add member service: %w", err)
	}
	if chatType != domain.ChatTypeGroup {
		return fmt.Errorf("chatType is not domain.ChatTypeGroup: %w", domain.ErrInvalidChatType)
	}
	if role != domain.RoleAdmin {
		return fmt.Errorf("user is not admin: %w", domain.ErrForbiddenNotAdmin)
	}

	if _, err := s.userRepository.GetByID(ctx, targetUserID); err != nil {
		return fmt.Errorf("target user is not exists: %w", err)
	}

	return s.chatRepository.AddMember(ctx, chatID, targetUserID)
}

func (s *ChatService) RemoveMember(ctx context.Context, actorID, chatID, targetUserID int64) error {
	chatType, role, err := s.chatRepository.GetChatAndMemberRole(ctx, chatID, actorID)
	if err != nil {
		return fmt.Errorf("add member service: %w", err)
	}
	if chatType != domain.ChatTypeGroup {
		return fmt.Errorf("chatType is not domain.ChatTypeGroup: %w", domain.ErrInvalidChatType)
	}
	if role != domain.RoleAdmin {
		return fmt.Errorf("user is not admin: %w", domain.ErrForbiddenNotAdmin)
	}

	return s.chatRepository.RemoveMember(ctx, chatID, targetUserID)
}
