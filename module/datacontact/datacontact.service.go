package datacontact

import (
	"encoding/csv"
	"errors"
	"io"
	"path/filepath"
	"strings"
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

func (s *Service) CreateFromCSV(akunUUID, namaFile string, reader io.Reader) error {
	rows, err := csv.NewReader(reader).ReadAll()
	if err != nil {
		return errors.New("file CSV tidak valid")
	}

	contacts := make([]ManualContactItem, 0, len(rows))
	for index, row := range rows {
		if len(row) == 0 {
			continue
		}
		phone := strings.TrimSpace(row[0])
		if index == 0 && isHeaderPhone(phone) {
			continue
		}
		if phone == "" {
			continue
		}
		name := ""
		if len(row) > 1 {
			name = strings.TrimSpace(row[1])
		}
		contacts = append(contacts, ManualContactItem{Phone: phone, Name: name})
	}

	return s.create(akunUUID, namaFile, "csv", contacts)
}

func (s *Service) CreateManual(akunUUID string, req CreateManualRequest) error {
	return s.create(akunUUID, req.NamaFile, "manual", req.Contacts)
}

func (s *Service) GetByAkunUUID(akunUUID string) ([]DataContactResponse, error) {
	list, err := s.repo.GetByAkunUUID(akunUUID)
	if err != nil {
		return nil, err
	}
	return mapResponses(list, false), nil
}

func (s *Service) GetDetail(id, akunUUID string) (*DataContactResponse, error) {
	contact, err := s.repo.GetByIDAndAkunUUID(id, akunUUID)
	if err != nil {
		return nil, errors.New("data contact tidak ditemukan")
	}
	response := mapResponse(*contact, true)
	return &response, nil
}

func (s *Service) Delete(id, akunUUID string) error {
	return s.repo.DeleteByIDAndAkunUUID(id, akunUUID)
}

func (s *Service) create(akunUUID, namaFile, sumber string, contacts []ManualContactItem) error {
	namaFile = normalizeTXTFileName(namaFile)
	lines := make([]string, 0, len(contacts))
	seen := map[string]bool{}
	for _, contact := range contacts {
		phone := strings.TrimSpace(contact.Phone)
		if phone == "" || seen[phone] {
			continue
		}
		seen[phone] = true
		name := strings.TrimSpace(contact.Name)
		if name != "" {
			lines = append(lines, phone+","+name)
			continue
		}
		lines = append(lines, phone)
	}

	if namaFile == "" {
		return errors.New("nama file contact wajib diisi")
	}
	if len(lines) == 0 {
		return errors.New("minimal satu contact wajib diisi")
	}

	return s.repo.Create(&model.DataContact{
		ID:            uuid.New().String(),
		AkunUUID:      akunUUID,
		NamaFile:      namaFile,
		DataTXT:       strings.Join(lines, "\n"),
		TotalContact:  len(lines),
		Sumber:        sumber,
		TanggalDibuat: time.Now(),
	})
}

func normalizeTXTFileName(name string) string {
	clean := strings.TrimSpace(filepath.Base(name))
	clean = strings.TrimSuffix(clean, filepath.Ext(clean))
	if clean == "" || clean == "." {
		return ""
	}
	return clean + ".txt"
}

func isHeaderPhone(value string) bool {
	value = strings.ToLower(value)
	return value == "phone" || value == "nomor" || value == "nomor telepon" || value == "no" || value == "number"
}

func mapResponses(list []model.DataContact, includeData bool) []DataContactResponse {
	responses := make([]DataContactResponse, 0, len(list))
	for _, item := range list {
		responses = append(responses, mapResponse(item, includeData))
	}
	return responses
}

func mapResponse(item model.DataContact, includeData bool) DataContactResponse {
	response := DataContactResponse{
		ID:            item.ID,
		NamaFile:      item.NamaFile,
		TotalContact:  item.TotalContact,
		Sumber:        item.Sumber,
		TanggalDibuat: item.TanggalDibuat.Format("02 Jan 2006, 15:04 WIB"),
	}
	if includeData {
		response.DataTXT = item.DataTXT
	}
	return response
}
