package payment

import (
	"time"

	"wakirim/model"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// ================== PAKET ==================

func (r *Repository) GetAllActivePaket() ([]model.Paket, error) {
	var pakets []model.Paket
	err := r.db.Where("aktif = ?", true).Order("harga ASC").Find(&pakets).Error
	return pakets, err
}

func (r *Repository) GetPaketByID(id string) (*model.Paket, error) {
	var paket model.Paket
	err := r.db.First(&paket, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &paket, nil
}

// ================== PAYMENT ==================

func (r *Repository) CreatePayment(payment *model.Payment) error {
	return r.db.Create(payment).Error
}

func (r *Repository) GetPaymentByID(id string) (*model.Payment, error) {
	var payment model.Payment
	err := r.db.First(&payment, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &payment, nil
}

func (r *Repository) GetAllPayments() ([]model.Payment, error) {
	var payments []model.Payment
	err := r.db.Order("tanggal_bayar DESC").Find(&payments).Error
	return payments, err
}

func (r *Repository) GetPaymentsByStatus(status string) ([]model.Payment, error) {
	var payments []model.Payment
	err := r.db.Where("status = ?", status).Order("tanggal_bayar DESC").Find(&payments).Error
	return payments, err
}

func (r *Repository) GetPendingPaymentsCount() (int64, error) {
	var count int64
	err := r.db.Model(&model.Payment{}).Where("status = ?", "pending").Count(&count).Error
	return count, err
}

func (r *Repository) UpdatePayment(payment *model.Payment) error {
	return r.db.Save(payment).Error
}

func (r *Repository) UpdatePaymentStatus(id string, status string) error {
	return r.db.Model(&model.Payment{}).Where("id = ?", id).Update("status", status).Error
}

func (r *Repository) DeletePaymentWithLinkedAkun(payment *model.Payment) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if payment.Status == "verified" || payment.Status == "terverifikasi" {
			if err := tx.Where("email = ? OR username = ?", payment.Email, payment.Username).Delete(&model.Akun{}).Error; err != nil {
				return err
			}
		}

		result := tx.Where("id = ?", payment.ID).Delete(&model.Payment{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		return nil
	})
}

// ================== AKUN ==================

func (r *Repository) CreateAkun(akun *model.Akun) error {
	return r.db.Create(akun).Error
}

func (r *Repository) CreateAkunWithPayment(akun *model.Akun, payment *model.Payment) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(akun).Error; err != nil {
			return err
		}

		if err := tx.Create(payment).Error; err != nil {
			return err
		}

		return nil
	})
}

func (r *Repository) GetAkunByUUID(uuid string) (*model.Akun, error) {
	var akun model.Akun
	err := r.db.First(&akun, "uuid = ?", uuid).Error
	if err != nil {
		return nil, err
	}
	return &akun, nil
}

func (r *Repository) GetAkunByEmail(email string) (*model.Akun, error) {
	var akun model.Akun
	err := r.db.First(&akun, "email = ?", email).Error
	if err != nil {
		return nil, err
	}
	return &akun, nil
}

func (r *Repository) GetAkunByUsername(username string) (*model.Akun, error) {
	var akun model.Akun
	err := r.db.First(&akun, "username = ?", username).Error
	if err != nil {
		return nil, err
	}
	return &akun, nil
}

func (r *Repository) GetAllAkun() ([]model.Akun, error) {
	var akuns []model.Akun
	err := r.db.Order("tanggal_daftar DESC").Find(&akuns).Error
	return akuns, err
}

func (r *Repository) UpdateAkun(akun *model.Akun) error {
	return r.db.Save(akun).Error
}

func (r *Repository) UpdateAkunPassword(uuid string, password string) error {
	return r.db.Model(&model.Akun{}).Where("uuid = ?", uuid).Update("password", password).Error
}

func (r *Repository) UpdateAkunExpiry(uuid string, tanggalBerakhir time.Time) error {
	return r.db.Model(&model.Akun{}).Where("uuid = ?", uuid).Update("tanggal_berakhir", tanggalBerakhir).Error
}

func (r *Repository) UpdateAkunStatus(uuid string, status string) error {
	return r.db.Model(&model.Akun{}).Where("uuid = ?", uuid).Update("status", status).Error
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

// ================== STATS ==================

func (r *Repository) GetTotalAkunCount() (int64, error) {
	var count int64
	err := r.db.Model(&model.Akun{}).Count(&count).Error
	return count, err
}

func (r *Repository) GetAktifAkunCount() (int64, error) {
	var count int64
	err := r.db.Model(&model.Akun{}).Where("status IN ?", []string{"berjalan", "akan_habis"}).Count(&count).Error
	return count, err
}

func (r *Repository) GetBerakhirAkunCount() (int64, error) {
	var count int64
	err := r.db.Model(&model.Akun{}).Where("status = ?", "habis").Count(&count).Error
	return count, err
}

func (r *Repository) GetAkanHabisAkunCount() (int64, error) {
	var count int64
	err := r.db.Model(&model.Akun{}).Where("status = ?", "akan_habis").Count(&count).Error
	return count, err
}

func (r *Repository) UpdateAllAkunStatuses() error {
	now := time.Now()
	threeDaysFromNow := now.Add(3 * 24 * time.Hour)

	// Update "akan_habis" for accounts expiring within 3 days and not already in those statuses
	err := r.db.Model(&model.Akun{}).
		Where("tanggal_berakhir <= ? AND tanggal_berakhir > ? AND status NOT IN (?, ?)",
			threeDaysFromNow, now, "akan_habis", "habis").
		Updates(map[string]interface{}{
			"status": "akan_habis",
		}).Error
	if err != nil {
		return err
	}

	// Update "habis" for expired accounts
	err = r.db.Model(&model.Akun{}).
		Where("tanggal_berakhir < ? AND status != ?", now, "habis").
		Updates(map[string]interface{}{
			"status": "habis",
		}).Error
	if err != nil {
		return err
	}

	// Update "berjalan" for active accounts
	err = r.db.Model(&model.Akun{}).
		Where("tanggal_berakhir > ? AND status IN (?, ?)", threeDaysFromNow, "akan_habis", "habis").
		Updates(map[string]interface{}{
			"status": "berjalan",
		}).Error
	return err
}

// ================== FINANCIAL STATS ==================

// GetIncomeToday returns total income for today
func (r *Repository) GetIncomeToday() (int64, error) {
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	var total int64
	err := r.db.Model(&model.Payment{}).
		Where("tanggal_bayar >= ? AND tanggal_bayar < ? AND status IN ?", startOfDay, endOfDay, []string{"verified", "terverifikasi"}).
		Select("COALESCE(SUM(total_harga), 0)").
		Scan(&total).Error
	return total, err
}

// GetIncomeThisWeek returns total income for this week (Monday to Sunday)
func (r *Repository) GetIncomeThisWeek() (int64, error) {
	now := time.Now()
	// Get start of week (Monday)
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7 // Sunday = 7 in ISO
	}
	startOfWeek := now.AddDate(0, 0, -weekday+1)
	startOfWeek = time.Date(startOfWeek.Year(), startOfWeek.Month(), startOfWeek.Day(), 0, 0, 0, 0, now.Location())
	endOfWeek := startOfWeek.AddDate(0, 0, 7)

	var total int64
	err := r.db.Model(&model.Payment{}).
		Where("tanggal_bayar >= ? AND tanggal_bayar < ? AND status IN ?", startOfWeek, endOfWeek, []string{"verified", "terverifikasi"}).
		Select("COALESCE(SUM(total_harga), 0)").
		Scan(&total).Error
	return total, err
}

// GetIncomeThisMonth returns total income for this month
func (r *Repository) GetIncomeThisMonth() (int64, error) {
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	endOfMonth := startOfMonth.AddDate(0, 1, 0)

	var total int64
	err := r.db.Model(&model.Payment{}).
		Where("tanggal_bayar >= ? AND tanggal_bayar < ? AND status IN ?", startOfMonth, endOfMonth, []string{"verified", "terverifikasi"}).
		Select("COALESCE(SUM(total_harga), 0)").
		Scan(&total).Error
	return total, err
}

// GetIncomeByDate returns total income for a specific date
func (r *Repository) GetIncomeByDate(date time.Time) (int64, error) {
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	var total int64
	err := r.db.Model(&model.Payment{}).
		Where("tanggal_bayar >= ? AND tanggal_bayar < ? AND status IN ?", startOfDay, endOfDay, []string{"verified", "terverifikasi"}).
		Select("COALESCE(SUM(total_harga), 0)").
		Scan(&total).Error
	return total, err
}

// GetTotalIncome returns total income for all time
func (r *Repository) GetTotalIncome() (int64, error) {
	var total int64
	err := r.db.Model(&model.Payment{}).
		Where("status IN ?", []string{"verified", "terverifikasi"}).
		Select("COALESCE(SUM(total_harga), 0)").
		Scan(&total).Error
	return total, err
}

// GetTotalTransactions returns total number of verified transactions
func (r *Repository) GetTotalTransactions() (int64, error) {
	var count int64
	err := r.db.Model(&model.Payment{}).Where("status IN ?", []string{"verified", "terverifikasi"}).Count(&count).Error
	return count, err
}

// GetPendingTransactions returns total number of pending transactions
func (r *Repository) GetPendingTransactions() (int64, error) {
	var count int64
	err := r.db.Model(&model.Payment{}).Where("status = ?", "pending").Count(&count).Error
	return count, err
}

// GetPendingIncome returns total income from pending payments
func (r *Repository) GetPendingIncome() (int64, error) {
	var total int64
	err := r.db.Model(&model.Payment{}).
		Where("status = ?", "pending").
		Select("COALESCE(SUM(total_harga), 0)").
		Scan(&total).Error
	return total, err
}

// GetGrossIncome returns total income from all payments regardless of status
func (r *Repository) GetGrossIncome() (int64, error) {
	var total int64
	err := r.db.Model(&model.Payment{}).
		Select("COALESCE(SUM(total_harga), 0)").
		Scan(&total).Error
	return total, err
}

// BestSellingPaket represents the most sold paket
type BestSellingPaket struct {
	PaketName string `gorm:"column:paket_name"`
	Count     int64  `gorm:"column:count"`
}

// GetBestSellingPaket returns the most popular paket by transaction count
func (r *Repository) GetBestSellingPaket() (*BestSellingPaket, error) {
	var result BestSellingPaket
	err := r.db.Model(&model.Payment{}).
		Where("status IN ? AND paket != ?", []string{"verified", "terverifikasi"}, "manual").
		Select("paket as paket_name, COUNT(*) as count").
		Group("paket").
		Order("count DESC").
		Limit(1).
		Scan(&result).Error
	if err != nil {
		return nil, err
	}
	// Get paket display name
	if result.PaketName != "" {
		var paket model.Paket
		if err := r.db.First(&paket, "id = ?", result.PaketName).Error; err == nil {
			result.PaketName = paket.Nama
		}
	}
	return &result, nil
}

// GetFinancialPaymentDetails returns payment rows for financial reporting.
func (r *Repository) GetFinancialPaymentDetails(status, search string, dateFrom, dateTo *time.Time, limit int) ([]model.Payment, error) {
	query := r.db.Model(&model.Payment{})

	switch status {
	case "verified":
		query = query.Where("status IN ?", []string{"verified", "terverifikasi"})
	case "pending":
		query = query.Where("status = ?", "pending")
	}

	if search != "" {
		keyword := "%" + search + "%"
		query = query.Where(
			"id ILIKE ? OR username ILIKE ? OR email ILIKE ? OR catatan ILIKE ?",
			keyword, keyword, keyword, keyword,
		)
	}

	if dateFrom != nil {
		start := time.Date(dateFrom.Year(), dateFrom.Month(), dateFrom.Day(), 0, 0, 0, 0, dateFrom.Location())
		query = query.Where("tanggal_bayar >= ?", start)
	}

	if dateTo != nil {
		endExclusive := time.Date(dateTo.Year(), dateTo.Month(), dateTo.Day(), 0, 0, 0, 0, dateTo.Location()).Add(24 * time.Hour)
		query = query.Where("tanggal_bayar < ?", endExclusive)
	}

	if limit <= 0 || limit > 500 {
		limit = 100
	}

	var payments []model.Payment
	err := query.Order("tanggal_bayar DESC").Limit(limit).Find(&payments).Error
	return payments, err
}
