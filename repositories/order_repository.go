package repositories

import (
	"cbt-core-api/models"

	"gorm.io/gorm"
)

type OrderRepository interface {
	Create(order *models.Order) error
	FindByID(id string) (*models.Order, error)
	FindByUserID(userID string) ([]models.Order, error)
	FindAll() ([]models.Order, error)
	Update(order *models.Order) error
}

type orderRepositoryImpl struct {
	db *gorm.DB
}

func NewOrderRepository(db *gorm.DB) OrderRepository {
	return &orderRepositoryImpl{db: db}
}

func (r *orderRepositoryImpl) Create(order *models.Order) error {
	return r.db.Create(order).Error
}

func (r *orderRepositoryImpl) FindByID(id string) (*models.Order, error) {
	var order models.Order
	err := r.db.Where("id = ?", id).First(&order).Error
	return &order, err
}

func (r *orderRepositoryImpl) FindByUserID(userID string) ([]models.Order, error) {
	var orders []models.Order
	err := r.db.Where("user_id = ?", userID).Order("created_at desc").Find(&orders).Error
	return orders, err
}

func (r *orderRepositoryImpl) FindAll() ([]models.Order, error) {
	var orders []models.Order
	err := r.db.Order("created_at desc").Find(&orders).Error
	return orders, err
}

func (r *orderRepositoryImpl) Update(order *models.Order) error {
	return r.db.Save(order).Error
}
