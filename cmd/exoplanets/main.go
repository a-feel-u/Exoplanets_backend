package main

import (
	"fmt"
	"os"

	"exoplanets/internal/app/dsn"
	"exoplanets/internal/app/handler"
	"exoplanets/internal/app/repository"
	"exoplanets/internal/pkg"

	"exoplanets/internal/app/config"

	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"github.com/redis/go-redis/v9"
)

// @title						Exoplanets API
// @version					1.0
// @description				API для управления экзопланетами.
// @description				Публичное чтение доступно без авторизации.
// @description				Для создания, публикации и лайков необходим JWT.
// @BasePath					/
// @schemes					http
//
// @securityDefinitions.apikey	BearerAuth
// @in							header
// @name						Authorization
func main() {
	router := gin.Default()
	conf, err := config.NewConfig()
	if err != nil {
		logrus.Fatalf("error loading config: %v", err)
	}

	postgresString := dsn.FromEnv()
	fmt.Println(postgresString)

	rep, errRep := repository.New(&repository.RepositorySettings{
		PostgresDSN:     postgresString,
		MinioEndpoint:   os.Getenv("MINIO_ENDPOINT"),
		MinioAccessKey:  os.Getenv("MINIO_ACCESS_KEY"),
		MinioSecretKey:  os.Getenv("MINIO_SECRET_KEY"),
		MinioBucketName: os.Getenv("MINIO_BUCKET_NAME"),
	})

	if errRep != nil {
		logrus.Fatalf("error initializing repository: %v", errRep)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     conf.RedisAddr,
		Password: conf.RedisPassword,
		DB:       0,
	})

	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		logrus.Fatalf("error connecting to Redis: %v", err)
	}

	defer redisClient.Close()

	hand := handler.NewHandler(
		rep,
		redisClient,
		conf.JWTSecret,
		time.Duration(conf.JWTTTLHours)*time.Hour,
	)

	application := pkg.NewApp(conf, router, hand)
	application.RunApp()
}
