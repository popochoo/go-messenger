package transport_http

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const UserIDKey = "user_id"

type TokenParser interface {
	ParseToken(tokenStr, expectedType string) (int64, error)
}

func AuthMiddleware(parser TokenParser) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		authHeader := ctx.GetHeader("Authorization")
		if authHeader == "" {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
			return
		}

		userID, err := parser.ParseToken(parts[1], "access")
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		ctx.Set(UserIDKey, userID)
		ctx.Next()
	}
}
