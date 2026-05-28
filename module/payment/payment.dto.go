package payment

// Request DTOs
type CreatePaymentRequest struct {
	Username        string `form:"username" binding:"required"`
	Email           string `form:"email" binding:"required,email"`
	Paket           string `form:"paket" binding:"required"`
	Catatan         string `form:"catatan"`
	BuktiPembayaran string `form:"bukti_pembayaran"` // base64 encoded image
}

type VerifyPaymentRequest struct {
	Password string `json:"password" binding:"required"`
}

type UpgradeAccountRequest struct {
	Username string `json:"username" binding:"required"`
	Days     int    `json:"days" binding:"required,min=1"`
}

type CreateManualAccountRequest struct {
	Email      string `json:"email" binding:"required,email"`
	Username   string `json:"username" binding:"required"`
	Password   string `json:"password" binding:"required"`
	ActiveDays int    `json:"active_days" binding:"required,min=1"`
	TotalPaid  int64  `json:"total_paid"`
}

type ResetPasswordRequest struct {
	Username string `json:"username" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type DeleteAccountRequest struct {
	UUID string `json:"uuid" binding:"required"`
}

type DeletePaymentRequest struct {
	PaymentID string `json:"payment_id" binding:"required"`
}

// Response DTOs
type PaymentResponse struct {
	ID              string `json:"id"`
	Username        string `json:"username"`
	Email           string `json:"email"`
	Paket           string `json:"paket"`
	TotalHarga      int64  `json:"total_harga"`
	BuktiPembayaran string `json:"bukti_pembayaran"`
	TanggalBayar    string `json:"tanggal_bayar"`
	Catatan         string `json:"catatan"`
	Status          string `json:"status"`
}

type AkunResponse struct {
	UUID            string `json:"uuid"`
	Email           string `json:"email"`
	Username        string `json:"username"`
	TanggalDaftar   string `json:"tanggal_daftar"`
	TanggalBerakhir string `json:"tanggal_berakhir"`
	Status          string `json:"status"`
}

type PaketResponse struct {
	ID          string `json:"id"`
	Nama        string `json:"nama"`
	Harga       int64  `json:"harga"`
	DurasiBulan int    `json:"durasi_bulan"`
}

type AccountStatsResponse struct {
	TotalAkun int64 `json:"total_akun"`
	Aktif     int64 `json:"aktif"`
	Berakhir  int64 `json:"berakhir"`
	Menunggu  int64 `json:"menunggu"`
}

type FinancialStatsResponse struct {
	IncomeToday           int64  `json:"income_today"`
	IncomeThisWeek        int64  `json:"income_this_week"`
	IncomeThisMonth       int64  `json:"income_this_month"`
	TotalIncome           int64  `json:"total_income"`
	TotalTransactions     int64  `json:"total_transactions"`
	PendingTransactions   int64  `json:"pending_transactions"`
	PendingIncome         int64  `json:"pending_income"`
	GrossIncome           int64  `json:"gross_income"`
	BestSellingPaket      string `json:"best_selling_paket"`
	BestSellingPaketCount int64  `json:"best_selling_paket_count"`
}

type IncomeByDateResponse struct {
	Date   string `json:"date"`
	Income int64  `json:"income"`
}

type FinancialPaymentDetailResponse struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	Email        string `json:"email"`
	Paket        string `json:"paket"`
	PaketDisplay string `json:"paket_display"`
	TotalHarga   int64  `json:"total_harga"`
	TanggalBayar string `json:"tanggal_bayar"`
	Catatan      string `json:"catatan"`
	Status       string `json:"status"`
}

type ApiResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}
