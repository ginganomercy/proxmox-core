package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type User struct {
	ID           string    `gorm:"primaryKey;type:varchar(36)" json:"id"`
	Username     string    `gorm:"uniqueIndex;not null" json:"username"`
	PasswordHash string    `gorm:"not null" json:"-"`
	Role         string    `gorm:"default:'CLIENT'" json:"role"`
	Balance      float64   `gorm:"default:0.0" json:"balance"`
	Servers      []Server  `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"servers"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type Server struct {
	ID        string    `gorm:"primaryKey;type:varchar(36)" json:"id"`
	VMID      int       `gorm:"uniqueIndex;not null" json:"vmid"`
	Node      string    `gorm:"default:'Capybara'" json:"node"`
	Type      string    `gorm:"default:'qemu'" json:"type"`
	Name      string    `json:"name"`
	UserID    string    `gorm:"type:varchar(36);not null" json:"userId"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Voucher struct {
	ID        string    `gorm:"primaryKey;type:varchar(36)" json:"id"`
	Code      string    `gorm:"uniqueIndex;not null" json:"code"`
	Amount    float64   `gorm:"not null" json:"amount"`
	IsUsed    bool      `gorm:"default:false" json:"isUsed"`
	UsedBy    *string   `gorm:"type:varchar(36)" json:"usedBy,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// BeforeCreate hooks to automatically generate UUIDs
func (u *User) BeforeCreate(tx *gorm.DB) (err error) {
	if u.ID == "" {
		u.ID = uuid.NewString()
	}
	return
}

func (s *Server) BeforeCreate(tx *gorm.DB) (err error) {
	if s.ID == "" {
		s.ID = uuid.NewString()
	}
	return
}

func (v *Voucher) BeforeCreate(tx *gorm.DB) (err error) {
	if v.ID == "" {
		v.ID = uuid.NewString()
	}
	return
}
