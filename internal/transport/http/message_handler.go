package transport_http

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/popochoo/go-messenger/internal/domain"
	"github.com/popochoo/go-messenger/internal/service"
)

type MessageHandler struct {
	messageService *service.MessageService
}

func NewMessageHandler(messageService *service.MessageService) *MessageHandler {
	return &MessageHandler{messageService: messageService}
}

type editMessageRequest struct {
	Content string `json:"content" binding:""required`
}

type markReadRequest struct {
	LastReadMessageID int64 `json:"last_read_message_id" binding:"required,gt=0`
}

func (h *MessageHandler) GetHistory(c *gin.Context) {
	chatID, err := strconv.ParseInt(c.Param("chat_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid chat_id"})
		return
	}

	var cursor int64
	if cursorStr := c.Query("cursor"); cursorStr != "" {
		cursor, err = strconv.ParseInt(cursorStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cursor"})
			return
		}
	}

	limit := 30
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil {
			limit = parsedLimit
		}
	}

	userID := c.GetInt64(UserIDKey)
	page, err := h.messageService.GetHistory(c.Request.Context(), userID, chatID, cursor, limit)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, page)
}

func (h *MessageHandler) Search(c *gin.Context) {
	chatID, err := strconv.ParseInt(c.Param("chat_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid chat_id"})
		return
	}

	query := c.Query("q")
	userID := c.GetInt64(UserIDKey)

	messages, err := h.messageService.SearchMessages(c.Request.Context(), userID, chatID, query)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"messages": messages})
}

func (h *MessageHandler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrChatNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrNotChatMember):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrEmptyMessage), errors.Is(err, domain.ErrMessageTooLong):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}

func (h *MessageHandler) EditMessage(c *gin.Context) {
	chatID, err := strconv.ParseInt(c.Param("chat_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid chat_id"})
		return
	}

	messageID, err := strconv.ParseInt(c.Param("message_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid message_id"})
		return
	}

	var req editMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}

	actorID := c.GetInt64(UserIDKey)
	msg, err := h.messageService.EditMessage(c.Request.Context(), actorID, chatID, messageID, req.Content)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, msg)
}

func (h *MessageHandler) DeleteMessage(c *gin.Context) {
	chatID, err := strconv.ParseInt(c.Param("chat_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid chat_id"})
		return
	}

	messageID, err := strconv.ParseInt(c.Param("message_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid message_id"})
		return
	}

	actorID := c.GetInt64(UserIDKey)
	msg, err := h.messageService.DeleteMessage(c.Request.Context(), actorID, chatID, messageID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, msg)
}

func (h *MessageHandler) MarkAsRead(c *gin.Context) {
	chatID, err := strconv.ParseInt(c.Param("chat_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid chat_id"})
		return
	}

	var req markReadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}

	userID := c.GetInt64(UserIDKey)
	updatedID, err := h.messageService.MarkChatAsRead(c.Request.Context(), userID, chatID, req.LastReadMessageID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"chat_id":              chatID,
		"last_read_message_id": updatedID,
	})
}
