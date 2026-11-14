package transport_http

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type Handlers struct {
	Auth        *AuthHandler
	Chat        *ChatHandler
	TokenParser TokenParser
}

func NewRouter(h Handlers) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())

	router.GET("/health", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"time":   time.Now().UTC(),
		})
	})

	v1 := router.Group("/api/v1")
	{
		auth := v1.Group("/auth")
		{
			auth.POST("/register", h.Auth.Register)
			auth.POST("/login", h.Auth.Login)
			auth.POST("/refresh", h.Auth.Refresh)
		}

		protected := v1.Group("/")
		protected.Use(AuthMiddleware(h.TokenParser))
		{
			protected.GET("/me", func(ctx *gin.Context) {
				userID := ctx.GetInt64(UserIDKey)
				ctx.JSON(http.StatusOK, gin.H{"user_id": userID})
			})

			chats := protected.Group("/chats")
			{
				chats.GET("", h.Chat.ListChats)
				chats.POST("/direct", h.Chat.CreateDirect)
				chats.POST("/group", h.Chat.CreateGroup)
				chats.POST("/:chat_id/members", h.Chat.AddMember)
				chats.DELETE("/:chat_id/members/:user_id", h.Chat.RemoveMember)
			}
		}
	}

	return router
}
