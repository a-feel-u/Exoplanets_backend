package handler

import (
	"exoplanets/internal/app/ds"
	"fmt"
	"mime/multipart"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func GetUserID(ctx *gin.Context) uint {
	value, exists := ctx.Get("userID")
	if !exists {
		return 0
	}

	userID, ok := value.(uint)
	if !ok {
		return 0
	}

	return userID
}

func isImage(contentType string) bool {
	imageTypes := []string{
		"image/jpeg",
		"image/jpg",
		"image/png",
		"image/gif",
		"image/webp",
	}

	return slices.Contains(imageTypes, contentType)
}

func isVideo(contentType string) bool {
	videoTypes := []string{
		"video/mp4",
		"video/webm",
		"video/quicktime",
		"video/x-msvideo",
		"video/x-matroska",
	}

	return slices.Contains(videoTypes, contentType)
}

func validateFileUpload(header *multipart.FileHeader, check func(string) bool) (int, error) {
	file, err := header.Open()

	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("не удалось получить файл")
	}

	defer file.Close()

	buffer := make([]byte, 512)
	_, err = file.Read(buffer)

	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("не удалось прочитать файл")
	}

	contentType := http.DetectContentType(buffer)

	if !check(contentType) {
		return http.StatusBadRequest, fmt.Errorf("некорректный файл")
	}

	return 0, nil
}

// GetExoplanets godoc
//
//	@Summary		Получить список экзопланет
//	@Description	Возвращает опубликованные экзопланеты с количеством лайков и признаком владельца.
//	@Tags			Exoplanets
//	@Produce		json
//	@Param			query	query		string	false	"Фильтр по массе, например 7.5"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Router			/api/Exoplanets [get]
func (h *Handler) GetExoplanets(ctx *gin.Context) {
	var exoplanets []ds.Exoplanet
	var err error

	searchQuery := ctx.Query("query")

	// получаем значение из поля поиска
	if searchQuery == "" { // если поле поиска пусто, то просто получаем из репозитория все записи
		exoplanets, err = h.Repository.GetPublishedExoplanets()
		if err != nil {
			h.errorHandler(ctx, http.StatusBadRequest, err)
			return
		}
	} else {
		weight, err := strconv.ParseFloat(
			strings.Replace(searchQuery, ",", ".", 1), 64,
		)
		if err != nil {
			h.errorHandler(ctx, http.StatusBadRequest, err)
			return
		}
		exoplanets, err = h.Repository.GetExoplanetsByWeight(weight) // в ином случае ищем заказ по заголовку
		if err != nil {
			h.errorHandler(ctx, http.StatusInternalServerError, err)
			return
		}
	}

	userId := GetUserID(ctx)

	type ExtendedExoplanet struct {
		ds.Exoplanet
		LikesNumber int  `json:"likes_number"`
		IsMine      bool `json:"is_mine"`
	}

	var extendedExoplanets []ExtendedExoplanet

	for _, exoplanet := range exoplanets {
		likesNumber, err := h.Repository.GetLikesNumber(exoplanet.ID)

		if err != nil {
			return
		}

		isMine := userId != 0 && exoplanet.CreatorID == userId

		extendedExoplanet := ExtendedExoplanet{
			Exoplanet:   exoplanet,
			LikesNumber: likesNumber,
			IsMine:      isMine,
		}

		extendedExoplanets = append(extendedExoplanets, extendedExoplanet)
	}

	ctx.JSON(http.StatusOK, gin.H{
		"exoplanets": extendedExoplanets,
		"query":      searchQuery,
	})
}

// GetExoplanet godoc
//
//	@Summary		Получить экзопланету
//	@Description	Возвращает опубликованную экзопланету по ID.
//	@Tags			Exoplanets
//	@Produce		json
//	@Param			id		path		int		true	"ID экзопланеты"
//	@Param			next	query		boolean	false	"Перейти к следующей записи"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		404		{object}	map[string]interface{}
//	@Router			/api/Exoplanets/{id} [get]
func (h *Handler) GetExoplanet(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		logrus.Error(err)
		return
	}

	nextParam := ctx.Query("next")
	var exoplanet ds.Exoplanet

	switch nextParam {
	case "":
		exoplanet, err = h.Repository.GetPublishedExoplanet(id)
	case "true":
		exoplanet, err = h.Repository.GetNextPublishedExoplanetTo(id)
		if err == nil {
			ctx.Redirect(http.StatusFound, "/api/Exoplanets/"+strconv.Itoa(exoplanet.ID))
			return
		}
	default:
		logrus.Error(err)
		ctx.String(http.StatusBadRequest, "параметр next должен быть true или отсутствовать")
		return
	}
	likesNumber, err := h.Repository.GetLikesNumber(exoplanet.ID)

	if err != nil {
		logrus.Error(err)
		ctx.String(http.StatusNotFound, "запись не найдена")
		return
	}

	userID := GetUserID(ctx)
	isLiked := false

	if userID != 0 {
		isLiked, err = h.Repository.IsLikedByUser(
			exoplanet.ID,
			int(userID),
		)
		if err != nil {
			h.errorHandler(ctx, http.StatusInternalServerError, err)
			return
		}
	}

	type ExtendedExoplanet struct {
		ds.Exoplanet
		LikesNumber int  `json:"likes_number"`
		IsLiked     bool `json:"is_liked"`
	}

	extendedExoplanet := ExtendedExoplanet{
		Exoplanet:   exoplanet,
		LikesNumber: likesNumber,
		IsLiked:     isLiked,
	}

	ctx.JSON(http.StatusOK, gin.H{
		"exoplanets": extendedExoplanet,
	})
}

// GetDraftExoplanet godoc
//
//	@Summary		Получить черновик
//	@Description	Возвращает черновик текущего пользователя.
//	@Tags			Exoplanets
//	@Security		BearerAuth
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}
//	@Failure		401	{object}	map[string]interface{}
//	@Router			/api/DraftExoplanets [get]
func (h *Handler) GetDraftExoplanet(ctx *gin.Context) {
	creatorID := GetUserID(ctx)

	draftExoplanet, err := h.Repository.GetDraftExoplanet(creatorID)

	if err != nil {
		h.errorHandler(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"exoplanet": draftExoplanet,
	})
}

// AddDraftExoplanet godoc
//
//	@Summary		Создать черновик
//	@Description	Создаёт черновик экзопланеты и при необходимости загружает изображение и видео.
//	@Tags			Exoplanets
//	@Security		BearerAuth
//	@Accept			mpfd
//	@Produce		json
//	@Param			title	formData	string	false	"Название экзопланеты"
//	@Param			image	formData	file	false	"Изображение"
//	@Param			video	formData	file	false	"Видео"
//	@Success		201		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		401		{object}	map[string]interface{}
//	@Router			/api/AddExoplanets [post]
func (h *Handler) AddDraftExoplanet(ctx *gin.Context) {
	err := ctx.Request.ParseMultipartForm(2 << 20)

	if err != nil {
		h.errorHandler(ctx, http.StatusBadRequest, err)
		return
	}

	imageHeader, err := ctx.FormFile("image")
	imageFound := false

	if err != nil {
		if err != http.ErrMissingFile {
			h.errorHandler(ctx, http.StatusBadRequest, err)
			return
		}
	} else {
		imageFound = true

		code, err := validateFileUpload(imageHeader, isImage)

		if err != nil {
			h.errorHandler(ctx, code, err)
			return
		}
	}

	videoHeader, err := ctx.FormFile("video")
	videoFound := false
	if err != nil {
		if err != http.ErrMissingFile {
			h.errorHandler(ctx, http.StatusBadRequest, err)
			return
		}
	} else {
		videoFound = true

		code, err := validateFileUpload(videoHeader, isVideo)

		if err != nil {
			h.errorHandler(ctx, code, err)
			return
		}
	}

	creatorID := GetUserID(ctx)

	draftExoplanet := ds.Exoplanet{
		Title:     ctx.Request.FormValue("title"),
		Status:    "draft",
		CreatorID: creatorID,
	}

	err = h.Repository.AddDraftExoplanet(&draftExoplanet)

	if err != nil {
		h.errorHandler(ctx, http.StatusInternalServerError, err)
		return
	}

	if imageFound {
		err = h.Repository.AddOrReplaceExoplanetImage(uint(draftExoplanet.ID), imageHeader, ctx.Request.Context())

		if err != nil {
			h.errorHandler(ctx, http.StatusInternalServerError, err)
			return
		}
	}

	if videoFound {
		err = h.Repository.AddOrReplaceExoplanetVideo(uint(draftExoplanet.ID), videoHeader, ctx.Request.Context())

		if err != nil {
			h.errorHandler(ctx, http.StatusInternalServerError, err)
			return
		}
	}

	ctx.JSON(http.StatusCreated, gin.H{
		"exoplanet": draftExoplanet,
		//"message":   "Черновик успешно добавлен",
	})
}

type PublishExoplanetRequest struct {
	Title       string  `json:"title"`
	Distance    float64 `json:"distance"`
	Temperature float64 `json:"temperature"`
	Description string  `json:"description"`
}

// PublishExoplanet godoc
//
//	@Summary		Опубликовать черновик
//	@Description	Обновляет поля экзопланеты и переводит черновик в статус published.
//	@Tags			Exoplanets
//	@Security		BearerAuth
//	@Accept			mpfd
//	@Produce		json
//	@Param			distance	formData	integer	false	"Расстояние"
//	@Param			weight		formData	number	false	"Масса"
//	@Param			description	formData	string	false	"Описание"
//	@Success		200			{object}	map[string]interface{}
//	@Failure		401			{object}	map[string]interface{}
//	@Failure		404			{object}	map[string]interface{}
//	@Router			/api/PublishExoplanets [put]
func (h *Handler) PublishExoplanet(ctx *gin.Context) {
	creatorID := GetUserID(ctx)
	exoplanet, err := h.Repository.GetDraftExoplanet(creatorID)

	if err != nil {
		h.errorHandler(ctx, http.StatusInternalServerError, err)
		return
	}

	if exoplanet == nil {
		h.errorHandler(ctx, http.StatusNotFound, fmt.Errorf("черновик не найден"))
		return
	}

	distanceString := ctx.Request.FormValue("distance")
	if distanceString != "" {
		exoplanet.Distance, err = strconv.Atoi(distanceString)
		if err != nil {
			h.errorHandler(ctx, http.StatusBadRequest, err)
			return
		}
	}

	weightString := ctx.Request.FormValue("weight")
	if weightString != "" {
		weightString = strings.Replace(weightString, ",", ".", 1)
		exoplanet.Weight, err = strconv.ParseFloat(weightString, 64)

		if err != nil {
			h.errorHandler(ctx, http.StatusBadRequest, err)
			return
		}
	}

	exoplanet.Status = "published"
	exoplanet.Description = ctx.Request.FormValue("description")

	err = h.Repository.UpdateExoplanet(*exoplanet)

	if err != nil {
		h.errorHandler(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"exoplanet": exoplanet,
		//"message":   "запись успешно обновлена",
	})
}

// DeleteExoplanet godoc
//
//	@Summary		Удалить экзопланету
//	@Description	Выполняет мягкое удаление опубликованной записи владельцем.
//	@Tags			Exoplanets
//	@Security		BearerAuth
//	@Produce		json
//	@Param			id	path		int	true	"ID экзопланеты"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		401	{object}	map[string]interface{}
//	@Failure		403	{object}	map[string]interface{}
//	@Router			/api/DeleteExoplanets/{id} [delete]
func (h *Handler) DeleteExoplanet(ctx *gin.Context) {
	idString := ctx.Param("id")
	id, err := strconv.Atoi(idString)

	if err != nil {
		h.errorHandler(ctx, http.StatusBadRequest, err)
		return
	}

	Exoplanet, err := h.Repository.GetPublishedExoplanet(id)

	if err != nil {
		h.errorHandler(ctx, http.StatusInternalServerError, err)
		return
	}

	userID := GetUserID(ctx)

	if Exoplanet.CreatorID != userID {
		ctx.JSON(http.StatusForbidden, gin.H{
			"message": "нет прав для удаления записи",
		})
		return
	}

	err = h.Repository.DeleteExoplanet(id)

	if err != nil {
		h.errorHandler(ctx, http.StatusInternalServerError, err)
		return
	}

	// ctx.JSON(http.StatusOK, gin.H{
	// 	"message": "запись успешно удалена",
	// })
}

type LikeRequest struct {
	Like bool `json:"like"`
}

// LikeExoplanet godoc
//
//	@Summary		Поставить или снять лайк
//	@Description	Принимает JSON с полем like: true или like: false.
//	@Tags			Exoplanets
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int				true	"ID экзопланеты"
//	@Param			body	body		map[string]bool	true	"Например: {\"like\": true}"
//	@Success		200		{object}	map[string]interface{}
//	@Success		201		{object}	map[string]interface{}
//	@Failure		401		{object}	map[string]interface{}
//	@Router			/api/LikeExoplanets/{id}/like [post]
func (h *Handler) LikeExoplanet(ctx *gin.Context) {
	userID := GetUserID(ctx)

	exoplanetIdString := ctx.Param("id")
	exoplanetID, err := strconv.Atoi(exoplanetIdString)
	if err != nil {
		h.errorHandler(ctx, http.StatusBadRequest, err)
		return
	}

	var likeRequest LikeRequest
	err = ctx.ShouldBindJSON(&likeRequest)
	if err != nil {
		h.errorHandler(ctx, http.StatusBadRequest, err)
		return
	}

	isLikedByUser, err := h.Repository.IsLikedByUser(exoplanetID, int(userID))
	if err != nil {
		h.errorHandler(ctx, http.StatusInternalServerError, err)
		return
	}

	if likeRequest.Like {
		if !isLikedByUser {
			like := ds.Likes{
				UserID:      userID,
				ExoplanetID: uint(exoplanetID),
			}

			err = h.Repository.AddLike(&like)

			if err != nil {
				h.errorHandler(ctx, http.StatusInternalServerError, err)
				return
			}
		}

		// ctx.JSON(http.StatusCreated, gin.H{
		// 	"message": "лайк поставлен",
		// })

		return
	}

	if isLikedByUser {
		err = h.Repository.DeleteLike(userID, uint(exoplanetID))

		if err != nil {
			h.errorHandler(ctx, http.StatusInternalServerError, err)
			return
		}
	}

	// ctx.JSON(http.StatusOK, gin.H{
	// 	"message": "лайк отменён",
	// })
}
