package postgressql

import (
	"github.com/dev-hyunsang/clone-youtube-backend/internal/domain"
	"gorm.io/gorm"
)

type EmailVerificationRepository interface {
	Create(v *domain.EmailVerification) error
	FindLatestByEmail(email string) (*domain.EmailVerification, error)
	MarkVerified(id string) error
}

type emailVerificationRepository struct {
	db *gorm.DB
}

func NewEmailVerificationRepository(db *gorm.DB) EmailVerificationRepository {
	return &emailVerificationRepository{db: db}
}

func (r *emailVerificationRepository) Create(v *domain.EmailVerification) error {
	return r.db.Create(v).Error
}

func (r *emailVerificationRepository) FindLatestByEmail(email string) (*domain.EmailVerification, error) {
	var v domain.EmailVerification
	if err := r.db.Where("email = ?", email).Order("created_at DESC").First(&v).Error; err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *emailVerificationRepository) MarkVerified(id string) error {
	return r.db.Model(&domain.EmailVerification{}).Where("id = ?", id).Update("verified", true).Error
}
