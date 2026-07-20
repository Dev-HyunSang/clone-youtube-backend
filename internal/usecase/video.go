package usecase

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/dev-hyunsang/clone-youtube-backend/internal/domain"
	"github.com/dev-hyunsang/clone-youtube-backend/internal/media"
	"github.com/dev-hyunsang/clone-youtube-backend/internal/repository/postgressql"
	"github.com/dev-hyunsang/clone-youtube-backend/internal/repository/redis"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/minio/minio-go/v7"
)

// renditionLadder is the descending list of HLS rungs generated for an
// upload; a rung is skipped if it would upscale past the source height.
// 2160 = 4K, 1440 = 2K.
var renditionLadder = []int{2160, 1440, 1080, 720, 480, 360}

type VideoUsecase interface {
	// InitiateUpload assigns the upload's id and records a pending Video row.
	InitiateUpload(uploaderID string) (uploadID string, err error)
	// StoreUpload streams r to local disk, uploads the result to MinIO, records
	// its metadata, removes the local copy once persisted, and queues HLS
	// rendition generation to run in the background.
	StoreUpload(ctx context.Context, uploadID string, totalBytes int64, r io.Reader) error
	UploadProgress(uploadID string) (media.JobSnapshot, bool)
	// ProcessRenditions is the background-worker entry point queued by
	// StoreUpload: it fetches the video's stored original from MinIO and
	// generates its HLS rendition ladder.
	ProcessRenditions(ctx context.Context, videoID string) error
	// ListRenditions reports the HLS rendition rows recorded for a video so
	// far, so a client can poll rendition progress after upload returns.
	ListRenditions(videoID string) ([]domain.VideoRendition, error)
}

type videoUsecase struct {
	videoRepo     postgressql.VideoRepository
	renditionRepo postgressql.VideoRenditionRepository
	minio         *minio.Client
	bucketName    string
	uploadDir     string
	taskClient    *asynq.Client
}

func NewVideoUsecase(videoRepo postgressql.VideoRepository, renditionRepo postgressql.VideoRenditionRepository, minioClient *minio.Client, bucketName, uploadDir string, taskClient *asynq.Client) VideoUsecase {
	return &videoUsecase{videoRepo: videoRepo, renditionRepo: renditionRepo, minio: minioClient, bucketName: bucketName, uploadDir: uploadDir, taskClient: taskClient}
}

func (u *videoUsecase) InitiateUpload(uploaderID string) (string, error) {
	id := uuid.NewString()
	video := &domain.Video{
		ID:         id,
		UploaderID: uploaderID,
		ObjectKey:  id + ".mp4",
		Status:     "uploading",
	}
	if err := u.videoRepo.Create(video); err != nil {
		return "", err
	}
	return video.ID, nil
}

func (u *videoUsecase) StoreUpload(ctx context.Context, uploadID string, totalBytes int64, r io.Reader) error {
	if err := os.MkdirAll(u.uploadDir, 0o755); err != nil {
		return err
	}

	job := media.CreateJob(uploadID, totalBytes)
	dst := filepath.Join(u.uploadDir, uploadID+".mp4")
	defer os.Remove(dst)

	if err := media.SaveUpload(job, dst, r); err != nil {
		_ = u.videoRepo.UpdateStatus(uploadID, "failed")
		return err
	}

	info, err := media.Probe(ctx, dst)
	if err != nil {
		_ = u.videoRepo.UpdateStatus(uploadID, "failed")
		return fmt.Errorf("probe upload: %w", err)
	}

	objectKey := uploadID + ".mp4"
	uploadInfo, err := u.minio.FPutObject(ctx, u.bucketName, objectKey, dst, minio.PutObjectOptions{ContentType: "video/mp4"})
	if err != nil {
		_ = u.videoRepo.UpdateStatus(uploadID, "failed")
		return err
	}

	if err := u.videoRepo.UpdateMetadata(uploadID, uploadInfo.Size, info.Width, info.Height, info.DurationSec, "completed"); err != nil {
		return err
	}

	// HLS renditions run in the background (see ProcessRenditions) so the
	// upload response doesn't wait on however long transcoding takes.
	task, err := redis.NewVideoRenditionsTask(uploadID)
	if err != nil {
		return err
	}
	_, err = u.taskClient.Enqueue(task)
	return err
}

// ProcessRenditions fetches videoID's stored original from MinIO into a
// scratch local file and generates its HLS rendition ladder. It's the
// asynq worker entry point for the task StoreUpload queues.
func (u *videoUsecase) ProcessRenditions(ctx context.Context, videoID string) error {
	video, err := u.videoRepo.FindByID(videoID)
	if err != nil {
		return err
	}

	srcDir, err := os.MkdirTemp(u.uploadDir, videoID+"-src-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(srcDir)

	srcPath := filepath.Join(srcDir, "source.mp4")
	if err := u.minio.FGetObject(ctx, u.bucketName, video.ObjectKey, srcPath, minio.GetObjectOptions{}); err != nil {
		return fmt.Errorf("fetch original from minio: %w", err)
	}

	u.generateRenditions(ctx, videoID, srcPath, video.Height)
	return nil
}

func (u *videoUsecase) ListRenditions(videoID string) ([]domain.VideoRendition, error) {
	return u.renditionRepo.ListByVideoID(videoID)
}

// generateRenditions transcodes srcPath into an HLS rendition per rung in
// renditionLadder (skipping rungs taller than the source) and uploads each
// to MinIO under "<videoID>/<height>p/...", recording one VideoRendition row
// per rung.
func (u *videoUsecase) generateRenditions(ctx context.Context, videoID, srcPath string, sourceHeight int) {
	renditionsRoot := filepath.Join(u.uploadDir, videoID)
	defer os.RemoveAll(renditionsRoot)

	for _, height := range renditionLadder {
		if height > sourceHeight {
			continue
		}
		label := fmt.Sprintf("%dp", height)
		outDir := filepath.Join(renditionsRoot, label)

		rendition := &domain.VideoRendition{
			ID:      uuid.NewString(),
			VideoID: videoID,
			Height:  height,
			Status:  "processing",
		}
		if err := u.renditionRepo.Create(rendition); err != nil {
			log.Printf("rendition %s %s: create row failed: %v", videoID, label, err)
			continue
		}

		// Progress is tracked in-memory under the rendition's own ID so a
		// client polling ListRenditions can read percent-complete via
		// media.GetJob while status stays "processing".
		job := media.CreateJob(rendition.ID, 0)
		if err := media.TranscodeHLS(ctx, job, srcPath, outDir, height); err != nil {
			log.Printf("rendition %s %s: transcode failed: %v", videoID, label, err)
			_ = u.renditionRepo.UpdateStatus(rendition.ID, "failed")
			continue
		}

		objectPrefix := videoID + "/" + label
		if err := u.uploadDirToMinio(ctx, outDir, objectPrefix); err != nil {
			log.Printf("rendition %s %s: minio upload failed: %v", videoID, label, err)
			_ = u.renditionRepo.UpdateStatus(rendition.ID, "failed")
			continue
		}

		info, err := media.Probe(ctx, filepath.Join(outDir, "playlist.m3u8"))
		if err != nil {
			log.Printf("rendition %s %s: probe output failed: %v", videoID, label, err)
			_ = u.renditionRepo.UpdateStatus(rendition.ID, "failed")
			continue
		}

		_ = u.renditionRepo.UpdateMetadata(rendition.ID, objectPrefix+"/playlist.m3u8", info.Width, info.Height, "completed")
	}
}

// uploadDirToMinio uploads every file directly under localDir to MinIO as
// "<objectPrefix>/<filename>".
func (u *videoUsecase) uploadDirToMinio(ctx context.Context, localDir, objectPrefix string) error {
	entries, err := os.ReadDir(localDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		contentType := "video/mp2t"
		if strings.HasSuffix(entry.Name(), ".m3u8") {
			contentType = "application/vnd.apple.mpegurl"
		}

		localPath := filepath.Join(localDir, entry.Name())
		objectKey := objectPrefix + "/" + entry.Name()
		if _, err := u.minio.FPutObject(ctx, u.bucketName, objectKey, localPath, minio.PutObjectOptions{ContentType: contentType}); err != nil {
			return err
		}
	}
	return nil
}

func (u *videoUsecase) UploadProgress(uploadID string) (media.JobSnapshot, bool) {
	job, ok := media.GetJob(uploadID)
	if !ok {
		return media.JobSnapshot{}, false
	}
	return job.Snapshot(), true
}
