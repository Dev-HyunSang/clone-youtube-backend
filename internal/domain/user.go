package domain

import "time"

type Users struct {
	ID string `gorm:"column:id;primaryKey"`
	// PublicID is the identifier exposed to clients (API responses, JWT
	// subject) so the internal DB primary key never leaves the server.
	PublicID  string    `gorm:"column:public_id"`
	Email     string    `gorm:"column:email;uniqueIndex"`
	Password  string    `gorm:"column:password"`
	FirstName string    `gorm:"column:first_name"`
	LastName  string    `gorm:"column:last_name"`
	Nickname  string    `gorm:"column:nickname"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (Users) TableName() string {
	return "users"
}
