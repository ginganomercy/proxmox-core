package repositories

import (
	"cbt-core-api/models"
	"gorm.io/gorm"
)

type UserRepository interface {
	Count() (int64, error)
	Create(user *models.User) error
	FindByUsername(username string) (*models.User, error)
	FindByID(id string) (*models.User, error)
	UpdateBalance(id string, addAmount float64) error
	GetVoucherByCode(code string) (*models.Voucher, error)
	RedeemVoucher(code string, userId string) (*models.Voucher, error)
}

type userRepositoryImpl struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepositoryImpl{db: db}
}

func (r *userRepositoryImpl) Count() (int64, error) {
	var count int64
	err := r.db.Model(&models.User{}).Count(&count).Error
	return count, err
}

func (r *userRepositoryImpl) Create(user *models.User) error {
	return r.db.Create(user).Error
}

func (r *userRepositoryImpl) FindByUsername(username string) (*models.User, error) {
	var user models.User
	if err := r.db.Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepositoryImpl) FindByID(id string) (*models.User, error) {
	var user models.User
	if err := r.db.Where("id = ?", id).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepositoryImpl) UpdateBalance(id string, addAmount float64) error {
	return r.db.Model(&models.User{}).Where("id = ?", id).Update("balance", gorm.Expr("balance + ?", addAmount)).Error
}

func (r *userRepositoryImpl) GetVoucherByCode(code string) (*models.Voucher, error) {
	var voucher models.Voucher
	if err := r.db.Where("code = ?", code).First(&voucher).Error; err != nil {
		return nil, err
	}
	return &voucher, nil
}

func (r *userRepositoryImpl) RedeemVoucher(code string, userId string) (*models.Voucher, error) {
	var voucher models.Voucher
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("code = ?", code).First(&voucher).Error; err != nil {
			return err
		}
		if voucher.IsUsed {
			return gorm.ErrInvalidData
		}
		
		voucher.IsUsed = true
		voucher.UsedBy = &userId
		if err := tx.Save(&voucher).Error; err != nil {
			return err
		}

		if err := tx.Model(&models.User{}).Where("id = ?", userId).Update("balance", gorm.Expr("balance + ?", voucher.Amount)).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &voucher, nil
}
