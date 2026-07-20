package postgressql

import (
	"github.com/dev-hyunsang/clone-youtube-backend/internal/domain"
	"gorm.io/gorm"
)

type VideoRenditionRepository interface {
	Create(r *domain.VideoRendition) error
	ListByVideoID(videoID string) ([]domain.VideoRendition, error)
	UpdateMetadata(id, playlistKey string, width, height int, status string) error
	UpdateStatus(id, status string) error
}

type videoRenditionRepository struct {
	db *gorm.DB
}

func NewVideoRenditionRepository(db *gorm.DB) VideoRenditionRepository {
	return &videoRenditionRepository{db: db}
}

func (r *videoRenditionRepository) Create(rendition *domain.VideoRendition) error {
	return r.db.Create(rendition).Error
}

func (r *videoRenditionRepository) ListByVideoID(videoID string) ([]domain.VideoRendition, error) {
	var renditions []domain.VideoRendition
	if err := r.db.Where("video_id = ?", videoID).Order("height desc").Find(&renditions).Error; err != nil {
		return nil, err
	}
	return renditions, nil
}

func (r *videoRenditionRepository) UpdateMetadata(id, playlistKey string, width, height int, status string) error {
	return r.db.Model(&domain.VideoRendition{}).Where("id = ?", id).Updates(map[string]any{
		"playlist_key": playlistKey,
		"width":        width,
		"height":       height,
		"status":       status,
	}).Error
}

func (r *videoRenditionRepository) UpdateStatus(id, status string) error {
	return r.db.Model(&domain.VideoRendition{}).Where("id = ?", id).Update("status", status).Error
}
