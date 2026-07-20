package db

import (
	"fmt"
	"strconv"

	"github.com/dev-hyunsang/clone-youtube-backend/internal/config"
	"github.com/hibiken/asynq"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func NewDBConnection() (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable TimeZone=Asia/Seoul",
		config.GetEnv("POSTGRES_HOST"),
		config.GetEnv("POSTGRES_PORT"),
		config.GetEnv("POSTGRES_USER"),
		config.GetEnv("POSTGRES_PASSWORD"),
		config.GetEnv("POSTGRES_DB"))

	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  dsn,
		PreferSimpleProtocol: true,
	}))
	if err != nil {
		return nil, fmt.Errorf("PostgresSQL 연결 실패: %w", err)
	}

	return db, err
}

func NewRedisConnection() *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", config.GetEnv("REDIS_HOST"), config.GetEnv("REDIS_PORT")),
		Password: config.GetEnv("REDIS_PASSWORD"),
		DB:       0,
	})

	return rdb
}

func NewAsynqRedisOpt() asynq.RedisClientOpt {
	return asynq.RedisClientOpt{
		Addr:     fmt.Sprintf("%s:%s", config.GetEnv("REDIS_HOST"), config.GetEnv("REDIS_PORT")),
		Password: config.GetEnv("REDIS_PASSWORD"),
		DB:       0,
	}
}

func NewMinioConnection() (*minio.Client, error) {
	useSSL, _ := strconv.ParseBool(config.GetEnv("MINIO_USE_SSL"))

	client, err := minio.New(config.GetEnv("MINIO_ENDPOINT"), &minio.Options{
		Creds:  credentials.NewStaticV4(config.GetEnv("MINIO_ROOT_USER"), config.GetEnv("MINIO_ROOT_PASSWORD"), ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("MinIO 연결 실패: %w", err)
	}

	return client, nil
}
