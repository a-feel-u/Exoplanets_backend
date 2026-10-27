package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// tokenBlacklistKey создаёт ключ Redis для токена.
func tokenBlacklistKey(token string) string {
	hash := sha256.Sum256([]byte(token))
	return "exoplanets:blacklist:" + hex.EncodeToString(hash[:])
}

// AuthMiddleware проверяет JWT и наличие токена в Redis blacklist.
func (h *Handler) AuthMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		authHeader := ctx.GetHeader("Authorization")

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 ||
			!strings.EqualFold(parts[0], "Bearer") ||
			strings.TrimSpace(parts[1]) == "" {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "требуется Bearer JWT",
			})
			return
		}

		tokenString := strings.TrimSpace(parts[1])
		claims := &Claims{}

		token, err := jwt.ParseWithClaims(
			tokenString,
			claims,
			func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrTokenSignatureInvalid
				}
				return []byte(h.JWTSecret), nil
			},
		)

		if err != nil || !token.Valid ||
			claims.UserID == 0 ||
			claims.ExpiresAt == nil ||
			claims.ExpiresAt.Time.Before(time.Now()) {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "недействительный или истёкший токен",
			})
			return
		}

		// Проверяем, не был ли токен отозван при выходе.
		exists, err := h.Redis.Exists(
			ctx.Request.Context(),
			tokenBlacklistKey(tokenString),
		).Result()

		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "не удалось проверить Redis",
			})
			return
		}

		if exists > 0 {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "токен отозван",
			})
			return
		}

		// Сохраняем данные авторизованного пользователя для обработчиков.
		ctx.Set("userID", claims.UserID)
		ctx.Set("jwtToken", tokenString)
		ctx.Set("jwtExpiresAt", claims.ExpiresAt.Time)

		ctx.Next()
	}
}
