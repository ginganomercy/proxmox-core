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
	Role             string     `gorm:"default:'CLIENT'" json:"role"`
	Balance          float64    `gorm:"default:0.0" json:"balance"`
	ResetToken       *string    `gorm:"type:varchar(255);index" json:"-"`
	ResetTokenExpiry *time.Time `json:"-"`
	Servers          []Server   `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"servers"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
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

type Order struct {
	ID             string    `gorm:"primaryKey;type:varchar(36)" json:"id"`
	UserID         string    `gorm:"type:varchar(36);not null" json:"userId"`
	UserEmail      string    `json:"userEmail"`
	Node           string    `json:"node"`
	Name           string    `json:"name"`
	Cores          int       `json:"cores"`
	Memory         int       `json:"memory"`
	Storage        int       `json:"storage"`
	Ciuser         string    `json:"ciuser"`
	Cipassword     string    `json:"cipassword"`
	Ipconfig0      string    `json:"ipconfig0"`
	TotalCost      float64   `json:"totalCost"`
	// Status flow: PENDING → READY_TO_ACTIVATE → PROVISIONING → COMPLETED | FAILED
	Status         string    `gorm:"default:'PENDING'" json:"status"`
	ActivationCode string    `gorm:"type:varchar(10)" json:"activationCode"`
	ProvisionError string    `gorm:"type:text" json:"provisionError,omitempty"` // Set on FAILED
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
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

func (o *Order) BeforeCreate(tx *gorm.DB) (err error) {
	if o.ID == "" {
		o.ID = uuid.NewString()
	}
	return
}
