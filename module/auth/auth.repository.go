package auth

import (
	"time"

	"wakirim/model"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

type AkunWithPaket struct {
	UUID            string
	Username        string
	Email           string
	TanggalDaftar   time.Time
	TanggalBerakhir time.Time
	Status          string
	Paket           string
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

func (r *Repository) GetAkunWithPaketByUUID(uuid string) (*AkunWithPaket, error) {
	var akun model.Akun
	err := r.db.Where("uuid = ?", uuid).First(&akun).Error
	if err != nil {
		return nil, err
	}

	var payment model.Payment
	err = r.db.
		Where("LOWER(username) = LOWER(?) OR LOWER(email) = LOWER(?)", akun.Username, akun.Email).
		Order("tanggal_bayar DESC").
		First(&payment).Error
	if err == gorm.ErrRecordNotFound {
		err = nil
	}
	if err != nil {
		return nil, err
	}

	return &AkunWithPaket{
		UUID:            akun.UUID,
		Username:        akun.Username,
		Email:           akun.Email,
		TanggalDaftar:   akun.TanggalDaftar,
		TanggalBerakhir: akun.TanggalBerakhir,
		Status:          akun.Status,
		Paket:           payment.Paket,
	}, nil
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

func (r *Repository) UpdateAkunPassword(uuid string, password string) error {
	return r.db.Model(&model.Akun{}).Where("uuid = ?", uuid).Update("password", password).Error
}
