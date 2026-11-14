package transport_http

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/popochoo/go-messenger/internal/domain"
	"github.com/popochoo/go-messenger/internal/service"
)

type ChatHandler struct {
	chatService *service.ChatService
}

func NewChatHandler(chatSevice *service.ChatService) *ChatHandler {
	return &ChatHandler{chatService: chatSevice}
}

type createDirectRequest struct {
	TargetUserID int64 `json:"target_user_id" binding:"required,gt=0"`
}

type createGroupRequest struct {
	Title     string  `json:"title" binding:"required,min=1,max=100"`
	MemberIDs []int64 `json:"member_ids"`
}

type addMemberRequest struct {
	UserID int64 `json:"user_id" binding:"required,gt=0"`
}

func (h *ChatHandler) CreateDirect(c *gin.Context) {
	var req createDirectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})

		return
	}

	currentUserID := c.GetInt64(UserIDKey)
	chat, created, err := h.chatService.CreateDirectChat(c.Request.Context(), currentUserID, req.TargetUserID)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrCannotDirectSelf):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, domain.ErrUserNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "target user not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}

		return
	}

	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}

	c.JSON(status, chat)
}

func (h *ChatHandler) CreateGroup(c *gin.Context) {
	var req createGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})

		return
	}

	currentUserID := c.GetInt64(UserIDKey)
	chat, err := h.chatService.CreateGroupChat(c.Request.Context(), currentUserID, req.Title, req.MemberIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})

		return
	}

	c.JSON(http.StatusCreated, chat)
}

func (h *ChatHandler) ListChats(c *gin.Context) {
	currentUserID := c.GetInt64(UserIDKey)
	chats, err := h.chatService.ListUserChats(c.Request.Context(), currentUserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})

		return
	}

	c.JSON(http.StatusOK, chats)
}

func (h *ChatHandler) AddMember(c *gin.Context) {
	chatID, err := strconv.ParseInt(c.Param("chat_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid chat_id"})

		return
	}

	var req addMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})

		return
	}

	actorID := c.GetInt64(UserIDKey)
	if err := h.chatService.AddMember(c.Request.Context(), actorID, chatID, req.UserID); err != nil {
		h.handleChatError(c, err)

		return
	}

	c.Status(http.StatusNoContent)
}

func (h *ChatHandler) RemoveMember(c *gin.Context) {
	chatID, err := strconv.ParseInt(c.Param("chat_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid chat_id"})

		return
	}

	targetUserID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})

		return
	}

	actorID := c.GetInt64(UserIDKey)
	if err := h.chatService.RemoveMember(c.Request.Context(), actorID, chatID, targetUserID); err != nil {
		h.handleChatError(c, err)

		return
	}

	c.Status(http.StatusNoContent)
}

func (h *ChatHandler) handleChatError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrChatNotFound), errors.Is(err, domain.ErrUserNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrNotChatMember), errors.Is(err, domain.ErrForbiddenNotAdmin):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrInvalidChatType):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		slog.Error("unexpected chat handler error", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}
