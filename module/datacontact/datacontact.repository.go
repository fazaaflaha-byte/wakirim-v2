package datacontact

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

func (r *Repository) Create(contact *model.DataContact) error {
	return r.db.Create(contact).Error
}

func (r *Repository) GetByAkunUUID(akunUUID string) ([]model.DataContact, error) {
	var list []model.DataContact
	err := r.db.Where("akun_uuid = ?", akunUUID).Order("tanggal_dibuat desc").Find(&list).Error
	return list, err
}

func (r *Repository) GetByIDAndAkunUUID(id, akunUUID string) (*model.DataContact, error) {
	var contact model.DataContact
	err := r.db.Where("id = ? AND akun_uuid = ?", id, akunUUID).First(&contact).Error
	if err != nil {
		return nil, err
	}
	return &contact, nil
}

func (r *Repository) DeleteByIDAndAkunUUID(id, akunUUID string) error {
	return r.db.Where("id = ? AND akun_uuid = ?", id, akunUUID).Delete(&model.DataContact{}).Error
}
