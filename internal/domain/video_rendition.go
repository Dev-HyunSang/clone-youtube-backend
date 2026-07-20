package domain

import "time"

// VideoRendition is one HLS output (a resolution rung) generated from a Video.
type VideoRendition struct {
	ID          string    `gorm:"column:id;primaryKey"`
	VideoID     string    `gorm:"column:video_id;index"`
	Width       int       `gorm:"column:width"`
	Height      int       `gorm:"column:height"`
	PlaylistKey string    `gorm:"column:playlist_key"` // MinIO object key for playlist.m3u8
	Status      string    `gorm:"column:status"`       // processing | completed | failed
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (VideoRendition) TableName() string {
	return "video_renditions"
}
