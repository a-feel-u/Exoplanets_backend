package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type Config struct {
	ServiceHost string
	ServicePort int

	JWTSecret     string
	JWTTTLHours   int
	RedisAddr     string
	RedisPassword string
}

func NewConfig() (*Config, error) {
	var err error

	configName := "config"
	_ = godotenv.Load()
	if os.Getenv("CONFIG_NAME") != "" {
		configName = os.Getenv("CONFIG_NAME")
	}

	viper.SetConfigName(configName)
	viper.SetConfigType("toml")
	viper.AddConfigPath("config")
	viper.AddConfigPath(".")
	viper.WatchConfig()

	err = viper.ReadInConfig()
	if err != nil {
		return nil, err
	}

	cfg := &Config{}           // создаем объект конфига
	err = viper.Unmarshal(cfg) // читаем информацию из файла,
	// конвертируем и затем кладем в нашу переменную cfg
	if err != nil {
		return nil, err
	}

	// Настройки JWT и Redis из переменных окружения
	cfg.JWTSecret = os.Getenv("JWT_SECRET")
	cfg.RedisAddr = os.Getenv("REDIS_ADDR")
	cfg.RedisPassword = os.Getenv("REDIS_PASSWORD")

	// Время жизни JWT по умолчанию — 24 часа
	cfg.JWTTTLHours = 24

	if value := os.Getenv("JWT_TTL_HOURS"); value != "" {
		hours, err := strconv.Atoi(value)
		if err != nil || hours <= 0 {
			return nil, fmt.Errorf(
				"JWT_TTL_HOURS must be a positive integer",
			)
		}

		cfg.JWTTTLHours = hours
	}

	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf(
			"JWT_SECRET is required: add it to your .env file",
		)
	}

	if cfg.RedisAddr == "" {
		cfg.RedisAddr = "localhost:6379"
	}

	log.Info("config parsed")

	return cfg, nil
}
