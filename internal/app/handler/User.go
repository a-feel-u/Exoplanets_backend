package handler

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"exoplanets/internal/app/ds"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type Claims struct {
	UserID uint `json:"user_id"`
	jwt.RegisteredClaims
}

// SignUp godoc
//
//	@Summary		Регистрация пользователя
//	@Description	Создаёт нового пользователя и сохраняет хеш пароля.
//	@Tags			Auth
//	@Accept			x-www-form-urlencoded
//	@Produce		json
//	@Param			login		formData	string	true	"Логин"
//	@Param			password	formData	string	true	"Пароль"
//	@Success		201			{object}	map[string]interface{}
//	@Failure		400			{object}	map[string]interface{}
//	@Failure		409			{object}	map[string]interface{}
//	@Router			/api/SignUp [post]
func (h *Handler) SignUp(ctx *gin.Context) {
	login := strings.TrimSpace(ctx.Request.FormValue("login"))
	password := ctx.Request.FormValue("password")

	if login == "" || password == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "login и password обязательны",
		})
		return
	}

	if len(login) > 40 {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "login не должен превышать 40 символов",
		})
		return
	}

	// Проверяем, существует ли такой логин.
	_, err := h.Repository.GetUserByLogin(login)
	if err == nil {
		ctx.JSON(http.StatusConflict, gin.H{
			"error": "пользователь с таким логином уже существует",
		})
		return
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		h.errorHandler(ctx, http.StatusInternalServerError, err)
		return
	}

	// Никогда не храним пароль в открытом виде.
	passwordHash, err := bcrypt.GenerateFromPassword(
		[]byte(password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		h.errorHandler(ctx, http.StatusInternalServerError, err)
		return
	}

	newUser := ds.User{
		Login:    login,
		Password: string(passwordHash),
	}

	if err := h.Repository.AddUser(&newUser); err != nil {
		h.errorHandler(ctx, http.StatusInternalServerError, err)
		return
	}

	// Не возвращаем пароль клиенту.
	ctx.JSON(http.StatusCreated, gin.H{
		"user": gin.H{
			"id":    newUser.ID,
			"login": newUser.Login,
		},
	})
}

// SignIn godoc
//
//	@Summary		Вход пользователя
//	@Description	Проверяет логин и пароль и возвращает JWT.
//	@Tags			Auth
//	@Accept			x-www-form-urlencoded
//	@Produce		json
//	@Param			login		formData	string	true	"Логин"
//	@Param			password	formData	string	true	"Пароль"
//	@Success		200			{object}	map[string]interface{}
//	@Failure		400			{object}	map[string]interface{}
//	@Failure		401			{object}	map[string]interface{}
//	@Router			/api/SignIn [post]
func (h *Handler) SignIn(ctx *gin.Context) {
	login := strings.TrimSpace(ctx.Request.FormValue("login"))
	password := ctx.Request.FormValue("password")

	if login == "" || password == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "login и password обязательны",
		})
		return
	}

	user, err := h.Repository.GetUserByLogin(login)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{
			"error": "неверный логин или пароль",
		})
		return
	}

	// Сравниваем введённый пароль с сохранённым хешем.
	if err := bcrypt.CompareHashAndPassword(
		[]byte(user.Password),
		[]byte(password),
	); err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{
			"error": "неверный логин или пароль",
		})
		return
	}

	now := time.Now()
	expiresAt := now.Add(h.JWTTTL)

	claims := Claims{
		UserID: user.ID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.Login,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(h.JWTSecret))
	if err != nil {
		h.errorHandler(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"user_id":      user.ID,
		"login":        user.Login,
		"access_token": tokenString,
		"token_type":   "Bearer",
		"expires_in":   int(h.JWTTTL.Seconds()),
	})
}

// SignOut godoc
//
//	@Summary		Выход пользователя
//	@Description	Отзывает JWT через Redis blacklist.
//	@Tags			Auth
//	@Security		BearerAuth
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}
//	@Failure		401	{object}	map[string]interface{}
//	@Failure		500	{object}	map[string]interface{}
//	@Router			/api/SignOut [post]
func (h *Handler) SignOut(ctx *gin.Context) {
	tokenValue, exists := ctx.Get("jwtToken")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{
			"error": "токен не найден",
		})
		return
	}

	expiresValue, exists := ctx.Get("jwtExpiresAt")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{
			"error": "срок действия токена не найден",
		})
		return
	}

	tokenString, ok := tokenValue.(string)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{
			"error": "некорректный токен",
		})
		return
	}

	expiresAt, ok := expiresValue.(time.Time)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{
			"error": "некорректный срок действия токена",
		})
		return
	}

	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		ctx.JSON(http.StatusUnauthorized, gin.H{
			"error": "токен истёк",
		})
		return
	}

	key := tokenBlacklistKey(tokenString)

	if err := h.Redis.Set(
		ctx.Request.Context(),
		key,
		"revoked",
		ttl,
	).Err(); err != nil {
		h.errorHandler(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message": "выход выполнен",
	})
}
