package postgressql

import (
	"github.com/dev-hyunsang/clone-youtube-backend/internal/domain"
	"gorm.io/gorm"
)

type UserRepository interface {
	Create(user *domain.Users) error
	FindByEmail(email string) (*domain.Users, error)
}

type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(user *domain.Users) error {
	return r.db.Create(user).Error
}

func (r *userRepository) FindByEmail(email string) (*domain.Users, error) {
	var user domain.Users
	if err := r.db.Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}
