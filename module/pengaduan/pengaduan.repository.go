package pengaduan

import (
	"wakirim/model"

	"gorm.io/gorm"
)

type PengaduanWithUsername struct {
	model.Pengaduan
	Username string `json:"username" gorm:"column:username"`
}

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(pengaduan *model.Pengaduan) error {
	return r.db.Create(pengaduan).Error
}

func (r *Repository) GetAll() ([]PengaduanWithUsername, error) {
	var list []PengaduanWithUsername
	err := r.db.Table("pengaduans").
		Select("pengaduans.*, akuns.username").
		Joins("left join akuns on akuns.uuid = pengaduans.akun_uuid").
		Order("pengaduans.tanggal_dibuat desc").Find(&list).Error
	return list, err
}

func (r *Repository) GetByAkunUUID(akunUUID string) ([]PengaduanWithUsername, error) {
	var list []PengaduanWithUsername
	err := r.db.Table("pengaduans").
		Select("pengaduans.*, akuns.username").
		Joins("left join akuns on akuns.uuid = pengaduans.akun_uuid").
		Where("pengaduans.akun_uuid = ?", akunUUID).
		Order("pengaduans.tanggal_dibuat desc").Find(&list).Error
	return list, err
}

func (r *Repository) GetByID(id string) (*model.Pengaduan, error) {
	var p model.Pengaduan
	err := r.db.Where("id = ?", id).First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *Repository) Update(pengaduan *model.Pengaduan) error {
	return r.db.Save(pengaduan).Error
}
