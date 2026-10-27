package handler

import (
	"exoplanets/internal/app/repository"

	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

type Handler struct {
	Repository *repository.Repository
	Redis      *redis.Client
	JWTSecret  string
	JWTTTL     time.Duration
}

func NewHandler(
	r *repository.Repository,
	redisClient *redis.Client,
	jwtSecret string,
	jwtTTL time.Duration,
) *Handler {
	return &Handler{
		Repository: r,
		Redis:      redisClient,
		JWTSecret:  jwtSecret,
		JWTTTL:     jwtTTL,
	}
}

func (h *Handler) RegisterHandler(router *gin.Engine) {
	// Публичные методы: доступны без JWT
	router.GET("/api/Exoplanets", h.GetExoplanets)
	router.GET("/api/Exoplanets/:id", h.GetExoplanet)

	// Регистрация и вход доступны без JWT
	router.POST("/api/SignUp", h.SignUp)
	router.POST("/api/SignIn", h.SignIn)

	// Защищённые методы: требуется действующий JWT
	router.GET(
		"/api/DraftExoplanets",
		h.AuthMiddleware(),
		h.GetDraftExoplanet,
	)
	router.POST(
		"/api/AddExoplanets",
		h.AuthMiddleware(),
		h.AddDraftExoplanet,
	)
	router.PUT(
		"/api/PublishExoplanets",
		h.AuthMiddleware(),
		h.PublishExoplanet,
	)
	router.DELETE(
		"/api/DeleteExoplanets/:id",
		h.AuthMiddleware(),
		h.DeleteExoplanet,
	)
	router.POST(
		"/api/LikeExoplanets/:id/like",
		h.AuthMiddleware(),
		h.LikeExoplanet,
	)
	router.POST(
		"/api/SignOut",
		h.AuthMiddleware(),
		h.SignOut,
	)
}

// func (h *Handler) RegisterStatic(router *gin.Engine) {
// 	router.LoadHTMLGlob("./templates/*")
// 	router.Static("/static", "./resources")
// }

func (h *Handler) errorHandler(ctx *gin.Context, errorStatusCode int, err error) {
	logrus.Error(err.Error())
	ctx.JSON(errorStatusCode, gin.H{
		"status":      "error",
		"description": err.Error(),
	})
}
