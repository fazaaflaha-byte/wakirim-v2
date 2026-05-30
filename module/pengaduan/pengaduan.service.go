package pengaduan

import (
	"errors"
	"time"

	"wakirim/model"

	"github.com/google/uuid"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreatePengaduan(akunUUID, email string, req CreatePengaduanRequest) error {
	p := &model.Pengaduan{
		ID:            uuid.New().String(),
		AkunUUID:      akunUUID,
		Email:         email,
		Judul:         req.Judul,
		Deskripsi:     req.Deskripsi,
		Status:        "menunggu",
		TanggalDibuat: time.Now(),
	}
	return s.repo.Create(p)
}

func (s *Service) GetAll() ([]PengaduanResponse, error) {
	list, err := s.repo.GetAll()
	if err != nil {
		return nil, err
	}
	return s.mapToResponse(list), nil
}

func (s *Service) GetByAkunUUID(akunUUID string) ([]PengaduanResponse, error) {
	list, err := s.repo.GetByAkunUUID(akunUUID)
	if err != nil {
		return nil, err
	}
	return s.mapToResponse(list), nil
}

func (s *Service) Tanggapi(id string, req TanggapiPengaduanRequest) error {
	p, err := s.repo.GetByID(id)
	if err != nil {
		return errors.New("pengaduan tidak ditemukan")
	}

	if p.Status == "ditanggapi" {
		return errors.New("pengaduan sudah ditanggapi")
	}

	now := time.Now()
	p.Jawaban = &req.Jawaban
	p.Status = "ditanggapi"
	p.TanggalDitanggapi = &now

	return s.repo.Update(p)
}

func (s *Service) mapToResponse(list []PengaduanWithUsername) []PengaduanResponse {
	var res []PengaduanResponse
	for _, p := range list {
		var tglDitanggapi *string
		if p.TanggalDitanggapi != nil {
			t := p.TanggalDitanggapi.Format("02 Jan 2006, 15:04 WIB")
			tglDitanggapi = &t
		}
		res = append(res, PengaduanResponse{
			ID:                p.ID,
			Username:          p.Username,
			Email:             p.Email,
			Judul:             p.Judul,
			Deskripsi:         p.Deskripsi,
			Jawaban:           p.Jawaban,
			Status:            p.Status,
			TanggalDibuat:     p.TanggalDibuat.Format("02 Jan 2006, 15:04 WIB"),
			TanggalDitanggapi: tglDitanggapi,
		})
	}
	return res
}
