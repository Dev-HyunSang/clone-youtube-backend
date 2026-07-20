package postgressql

import (
	"github.com/dev-hyunsang/clone-youtube-backend/internal/domain"
	"gorm.io/gorm"
)

type VideoRepository interface {
	Create(video *domain.Video) error
	FindByID(id string) (*domain.Video, error)
	UpdateMetadata(id string, sizeBytes int64, width, height int, durationSec float64, status string) error
	UpdateStatus(id, status string) error
}

type videoRepository struct {
	db *gorm.DB
}

func NewVideoRepository(db *gorm.DB) VideoRepository {
	return &videoRepository{db: db}
}

func (r *videoRepository) Create(video *domain.Video) error {
	return r.db.Create(video).Error
}

func (r *videoRepository) FindByID(id string) (*domain.Video, error) {
	var video domain.Video
	if err := r.db.Where("id = ?", id).First(&video).Error; err != nil {
		return nil, err
	}
	return &video, nil
}

func (r *videoRepository) UpdateMetadata(id string, sizeBytes int64, width, height int, durationSec float64, status string) error {
	return r.db.Model(&domain.Video{}).Where("id = ?", id).Updates(map[string]any{
		"size_bytes":   sizeBytes,
		"width":        width,
		"height":       height,
		"duration_sec": durationSec,
		"status":       status,
	}).Error
}

func (r *videoRepository) UpdateStatus(id, status string) error {
	return r.db.Model(&domain.Video{}).Where("id = ?", id).Update("status", status).Error
}
