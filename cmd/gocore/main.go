package main

import (
	"context"
	"encoding/json"
	"log"

	"github.com/dev-hyunsang/clone-youtube-backend/internal/config"
	"github.com/dev-hyunsang/clone-youtube-backend/internal/db"
	"github.com/dev-hyunsang/clone-youtube-backend/internal/domain"
	"github.com/dev-hyunsang/clone-youtube-backend/internal/handler"
	"github.com/dev-hyunsang/clone-youtube-backend/internal/middleware"
	"github.com/dev-hyunsang/clone-youtube-backend/internal/repository/postgressql"
	"github.com/dev-hyunsang/clone-youtube-backend/internal/repository/redis"
	"github.com/dev-hyunsang/clone-youtube-backend/internal/service"
	"github.com/dev-hyunsang/clone-youtube-backend/internal/usecase"
	"github.com/gofiber/fiber/v2"
	"github.com/hibiken/asynq"
	"github.com/minio/minio-go/v7"
)

func main() {
	// StreamRequestBody lets upload handlers read the body incrementally so
	// progress can be reported before the request finishes.
	app := fiber.New(fiber.Config{StreamRequestBody: true})
	middleware.Register(app)

	conn, err := db.NewDBConnection()
	if err != nil {
		log.Fatalf("서버 시작 중 데이터베이스 연결 실패 %v", err)
	}

	if err := conn.AutoMigrate(&domain.Users{}, &domain.EmailVerification{}, &domain.Video{}, &domain.VideoRendition{}); err != nil {
		log.Fatal("서버 시작 중 데이터베이스 AutoMigrate 중 오류 발생")
	}

	rdb := db.NewRedisConnection()
	sessionRepo := redis.NewSessionRepository(rdb)
	authRequired := middleware.AuthRequired(sessionRepo)

	userRepo := postgressql.NewUserRepository(conn)
	userUsecase := usecase.NewUserUsecase(userRepo, sessionRepo)
	userHandler := handler.NewUserHandler(userUsecase)

	emailRepo := postgressql.NewEmailVerificationRepository(conn)
	emailSender := service.NewSMTPEmailSender()
	emailUsecase := usecase.NewEmailVerificationUsecase(emailRepo, emailSender)
	emailHandler := handler.NewEmailVerificationHandler(emailUsecase)

	api := app.Group("/api/v1")

	users := api.Group("/users")
	users.Post("/signup", userHandler.SignUp)
	users.Post("/signin", userHandler.SignIn)
	users.Post("/refresh", userHandler.RefreshToken)
	users.Post("/logout", authRequired, userHandler.Logout)
	users.Post("/logout-all", authRequired, userHandler.LogoutAll)

	verify := api.Group("/verify")
	verify.Post("/send", emailHandler.SendCode)
	verify.Post("/verify", emailHandler.VerifyCode)

	minioClient, err := db.NewMinioConnection()
	if err != nil {
		log.Fatalf("서버 시작 중 MinIO 연결 실패 %v", err)
	}
	bucketName := config.GetEnv("MINIO_BUCKET")
	if exists, err := minioClient.BucketExists(context.Background(), bucketName); err != nil {
		log.Fatalf("서버 시작 중 MinIO 버킷 확인 실패 %v", err)
	} else if !exists {
		if err := minioClient.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{}); err != nil {
			log.Fatalf("서버 시작 중 MinIO 버킷 생성 실패 %v", err)
		}
	}

	uploadDir := config.GetEnv("UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "./uploads"
	}
	asynqRedisOpt := db.NewAsynqRedisOpt()
	taskClient := asynq.NewClient(asynqRedisOpt)
	defer taskClient.Close()

	videoRepo := postgressql.NewVideoRepository(conn)
	videoRenditionRepo := postgressql.NewVideoRenditionRepository(conn)
	videoUsecase := usecase.NewVideoUsecase(videoRepo, videoRenditionRepo, minioClient, bucketName, uploadDir, taskClient)
	videoHandler := handler.NewVideoHandler(videoUsecase)

	videos := api.Group("/videos")
	videos.Post("/upload", authRequired, videoHandler.UploadVideo)
	videos.Get("/upload/:id/progress", authRequired, videoHandler.UploadProgress)
	videos.Get("/:id/renditions", authRequired, videoHandler.VideoRenditions)

	// Runs HLS rendition generation queued by StoreUpload in the background,
	// in-process, so uploads don't have to wait on however long transcoding
	// takes.
	taskServer := asynq.NewServer(asynqRedisOpt, asynq.Config{Concurrency: 2})
	mux := asynq.NewServeMux()
	mux.HandleFunc(redis.TypeVideoRenditions, func(ctx context.Context, t *asynq.Task) error {
		var payload redis.VideoRenditionsPayload
		if err := json.Unmarshal(t.Payload(), &payload); err != nil {
			return err
		}
		return videoUsecase.ProcessRenditions(ctx, payload.VideoID)
	})
	// Start (not Run) because Run installs its own SIGTERM/SIGINT handler and
	// blocks forever waiting on it - since it never returns, that signal
	// never reaches the rest of the process and a plain `kill` on the server
	// stops taking new tasks but leaves app.Listen below still running.
	if err := taskServer.Start(mux); err != nil {
		log.Fatalf("서버 시작 중 asynq worker 실행 실패 %v", err)
	}
	defer taskServer.Shutdown()

	if err := app.Listen(":3000"); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}
}
