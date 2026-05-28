package auth

import (
	"wakirim/model"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetAkunByUsernameOrEmail(identity string) (*model.Akun, error) {
	var akun model.Akun
	err := r.db.
		Where("username = ? OR email = ?", identity, identity).
		First(&akun).Error
	if err != nil {
		return nil, err
	}
	return &akun, nil
}

func (r *Repository) GetAkunByUUID(uuid string) (*model.Akun, error) {
	var akun model.Akun
	err := r.db.Where("uuid = ?", uuid).First(&akun).Error
	if err != nil {
		return nil, err
	}
	return &akun, nil
}

func (r *Repository) DeleteAkunByUUID(uuid string) error {
	result := r.db.Where("uuid = ?", uuid).Delete(&model.Akun{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
