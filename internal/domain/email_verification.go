package domain

import "time"

type EmailVerification struct {
	ID        string    `gorm:"column:id;primaryKey"`
	Email     string    `gorm:"column:email;index"`
	Code      string    `gorm:"column:code"`
	Verified  bool      `gorm:"column:verified"`
	ExpiresAt time.Time `gorm:"column:expires_at"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (EmailVerification) TableName() string {
	return "email_verifications"
}
