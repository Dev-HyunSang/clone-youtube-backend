package domain

import "time"

type Video struct {
	ID          string    `gorm:"column:id;primaryKey"`
	UploaderID  string    `gorm:"column:uploader_id;index"`
	ObjectKey   string    `gorm:"column:object_key"`
	Status      string    `gorm:"column:status"` // uploading | completed | failed
	SizeBytes   int64     `gorm:"column:size_bytes"`
	Width       int       `gorm:"column:width"`
	Height      int       `gorm:"column:height"`
	DurationSec float64   `gorm:"column:duration_sec"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (Video) TableName() string {
	return "videos"
}
