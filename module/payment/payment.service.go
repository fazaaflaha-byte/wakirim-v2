package payment

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	"wakirim/config"
	"wakirim/model"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// ================== PAKET ==================

func (s *Service) GetAllActivePaket() ([]PaketResponse, error) {
	pakets, err := s.repo.GetAllActivePaket()
	if err != nil {
		return nil, err
	}

	var responses []PaketResponse
	for _, p := range pakets {
		responses = append(responses, PaketResponse{
			ID:          p.ID,
			Nama:        p.Nama,
			Harga:       p.Harga,
			DurasiBulan: p.DurasiBulan,
		})
	}
	return responses, nil
}

// ================== PAYMENT ==================

func (s *Service) CreatePayment(req *CreatePaymentRequest, file multipart.File) (*PaymentResponse, error) {
	// Get paket info
	paket, err := s.repo.GetPaketByID(req.Paket)
	if err != nil {
		return nil, errors.New("paket tidak ditemukan")
	}

	// Compress and encode image to base64
	buktiPembayaran, err := s.compressAndEncodeImage(file)
	if err != nil {
		return nil, errors.New("gagal memproses bukti pembayaran: " + err.Error())
	}

	payment := &model.Payment{
		ID:              uuid.New().String(),
		Username:        req.Username,
		Email:           req.Email,
		Paket:           req.Paket,
		TotalHarga:      paket.Harga,
		BuktiPembayaran: buktiPembayaran,
		TanggalBayar:    time.Now(),
		Catatan:         req.Catatan,
		Status:          "pending",
	}

	if err := s.repo.CreatePayment(payment); err != nil {
		return nil, errors.New("gagal menyimpan pembayaran: " + err.Error())
	}

	return &PaymentResponse{
		ID:              payment.ID,
		Username:        payment.Username,
		Email:           payment.Email,
		Paket:           payment.Paket,
		TotalHarga:      payment.TotalHarga,
		BuktiPembayaran: payment.BuktiPembayaran,
		TanggalBayar:    payment.TanggalBayar.Format("02 Jan 2006, 15:04 WIB"),
		Catatan:         payment.Catatan,
		Status:          payment.Status,
	}, nil
}

func (s *Service) GetAllPayments() ([]PaymentResponse, error) {
	payments, err := s.repo.GetAllPayments()
	if err != nil {
		return nil, err
	}

	var responses []PaymentResponse
	for _, p := range payments {
		responses = append(responses, s.paymentToResponse(p))
	}
	return responses, nil
}

func (s *Service) GetPaymentsByStatus(status string) ([]PaymentResponse, error) {
	payments, err := s.repo.GetPaymentsByStatus(status)
	if err != nil {
		return nil, err
	}

	var responses []PaymentResponse
	for _, p := range payments {
		responses = append(responses, s.paymentToResponse(p))
	}
	return responses, nil
}

func (s *Service) GetPaymentByID(id string) (*PaymentResponse, error) {
	payment, err := s.repo.GetPaymentByID(id)
	if err != nil {
		return nil, err
	}
	resp := s.paymentToResponse(*payment)
	return &resp, nil
}

// ================== VERIFY & CREATE ACCOUNT ==================

func (s *Service) VerifyAndCreateAccount(paymentID string, password string) error {
	// Admin is already authenticated via session cookie in the handler.

	// Get payment
	payment, err := s.repo.GetPaymentByID(paymentID)
	if err != nil {
		return errors.New("pembayaran tidak ditemukan")
	}

	if payment.Status == "verified" {
		return errors.New("pembayaran sudah diverifikasi")
	}

	// Get paket for duration
	paket, err := s.repo.GetPaketByID(payment.Paket)
	if err != nil {
		return errors.New("paket tidak ditemukan")
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return errors.New("gagal membuat password")
	}

	// Create account
	now := time.Now()
	tanggalBerakhir := now.AddDate(0, paket.DurasiBulan, 0)

	akun := &model.Akun{
		UUID:            uuid.New().String(),
		Email:           payment.Email,
		Username:        payment.Username,
		Password:        stringPtr(string(hashedPassword)),
		TanggalDaftar:   now,
		TanggalBerakhir: tanggalBerakhir,
		Status:          "berjalan",
	}

	if err := s.repo.CreateAkun(akun); err != nil {
		return errors.New("gagal membuat akun: " + err.Error())
	}

	// Update payment status
	if err := s.repo.UpdatePaymentStatus(paymentID, "verified"); err != nil {
		return errors.New("gagal update status pembayaran")
	}

	if err := s.sendAccountEmail(payment.Email, payment.Username, password); err != nil {
		return errors.New("akun berhasil dibuat, tetapi email gagal dikirim: " + err.Error())
	}

	return nil
}

// ================== MANUAL ACCOUNT ==================

func (s *Service) CreateManualAccount(req *CreateManualAccountRequest) error {
	// Check if email already exists
	_, err := s.repo.GetAkunByEmail(req.Email)
	if err == nil {
		return errors.New("email sudah terdaftar")
	}

	// Check if username already exists
	_, err = s.repo.GetAkunByUsername(req.Username)
	if err == nil {
		return errors.New("username sudah digunakan")
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return errors.New("gagal membuat password")
	}

	now := time.Now()
	tanggalBerakhir := now.AddDate(0, 0, req.ActiveDays)

	akun := &model.Akun{
		UUID:            uuid.New().String(),
		Email:           req.Email,
		Username:        req.Username,
		Password:        stringPtr(string(hashedPassword)),
		TanggalDaftar:   now,
		TanggalBerakhir: tanggalBerakhir,
		Status:          "berjalan",
	}

	payment := &model.Payment{
		ID:              uuid.New().String(),
		Username:        req.Username,
		Email:           req.Email,
		Paket:           "manual",
		TotalHarga:      req.TotalPaid,
		BuktiPembayaran: "",
		TanggalBayar:    now,
		Catatan:         "Tambah akun manual oleh admin",
		Status:          "verified",
	}

	if err := s.repo.CreateAkunWithPayment(akun, payment); err != nil {
		return errors.New("gagal membuat akun manual: " + err.Error())
	}

	if err := s.sendAccountEmail(req.Email, req.Username, req.Password); err != nil {
		return errors.New("akun manual berhasil dibuat, tetapi email gagal dikirim: " + err.Error())
	}

	return nil
}

// ================== ACCOUNT MANAGEMENT ==================

func (s *Service) UpgradeAccount(username string, days int) error {
	akun, err := s.repo.GetAkunByUsername(username)
	if err != nil {
		return errors.New("akun tidak ditemukan")
	}

	now := time.Now()
	var newExpiry time.Time

	if akun.TanggalBerakhir.After(now) {
		// Add days to current expiry
		newExpiry = akun.TanggalBerakhir.AddDate(0, 0, days)
	} else {
		// Start from now
		newExpiry = now.AddDate(0, 0, days)
	}

	return s.repo.UpdateAkunExpiry(akun.UUID, newExpiry)
}

func (s *Service) ResetPassword(req ResetPasswordRequest) error {
	akun, err := s.repo.GetAkunByUsername(req.Username)
	if err != nil {
		return errors.New("akun tidak ditemukan")
	}

	if akun.Email != req.Email {
		return errors.New("email tidak cocok")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return errors.New("gagal membuat password")
	}

	if err := s.repo.UpdateAkunPassword(akun.UUID, string(hashedPassword)); err != nil {
		return errors.New("gagal update password")
	}

	if err := s.sendPasswordResetEmail(req.Email, req.Password); err != nil {
		return errors.New("password berhasil direset, tetapi email gagal dikirim: " + err.Error())
	}

	return nil
}

func (s *Service) DeleteAccount(req DeleteAccountRequest) error {
	if _, err := s.repo.GetAkunByUUID(req.UUID); err != nil {
		return errors.New("akun tidak ditemukan")
	}

	if err := s.repo.DeleteAkunByUUID(req.UUID); err != nil {
		return errors.New("gagal menghapus akun")
	}

	return nil
}

func (s *Service) DeletePayment(req DeletePaymentRequest) error {
	payment, err := s.repo.GetPaymentByID(req.PaymentID)
	if err != nil {
		return errors.New("pembayaran tidak ditemukan")
	}

	if err := s.repo.DeletePaymentWithLinkedAkun(payment); err != nil {
		return errors.New("gagal menghapus pembayaran")
	}

	return nil
}

func (s *Service) GetAllAkun() ([]AkunResponse, error) {
	// Update all account statuses first
	s.repo.UpdateAllAkunStatuses()

	akuns, err := s.repo.GetAllAkun()
	if err != nil {
		return nil, err
	}

	var responses []AkunResponse
	for _, a := range akuns {
		responses = append(responses, AkunResponse{
			UUID:            a.UUID,
			Email:           a.Email,
			Username:        a.Username,
			TanggalDaftar:   a.TanggalDaftar.Format("02 Jan 2006"),
			TanggalBerakhir: a.TanggalBerakhir.Format("02 Jan 2006"),
			Status:          a.Status,
		})
	}
	return responses, nil
}

// ================== STATS ==================

func (s *Service) GetAccountStats() (*AccountStatsResponse, error) {
	if err := s.repo.UpdateAllAkunStatuses(); err != nil {
		return nil, err
	}

	total, err := s.repo.GetTotalAkunCount()
	if err != nil {
		return nil, err
	}

	aktif, err := s.repo.GetAktifAkunCount()
	if err != nil {
		return nil, err
	}

	berakhir, err := s.repo.GetBerakhirAkunCount()
	if err != nil {
		return nil, err
	}

	menunggu, err := s.repo.GetPendingPaymentsCount()
	if err != nil {
		return nil, err
	}

	return &AccountStatsResponse{
		TotalAkun: total,
		Aktif:     aktif,
		Berakhir:  berakhir,
		Menunggu:  menunggu,
	}, nil
}

// ================== FINANCIAL STATS ==================

func (s *Service) GetFinancialStats() (*FinancialStatsResponse, error) {
	incomeToday, err := s.repo.GetIncomeToday()
	if err != nil {
		return nil, err
	}

	incomeThisWeek, err := s.repo.GetIncomeThisWeek()
	if err != nil {
		return nil, err
	}

	incomeThisMonth, err := s.repo.GetIncomeThisMonth()
	if err != nil {
		return nil, err
	}

	totalIncome, err := s.repo.GetTotalIncome()
	if err != nil {
		return nil, err
	}

	totalTransactions, err := s.repo.GetTotalTransactions()
	if err != nil {
		return nil, err
	}

	pendingTransactions, err := s.repo.GetPendingTransactions()
	if err != nil {
		return nil, err
	}

	pendingIncome, err := s.repo.GetPendingIncome()
	if err != nil {
		return nil, err
	}

	grossIncome, err := s.repo.GetGrossIncome()
	if err != nil {
		return nil, err
	}

	bestPaket, err := s.repo.GetBestSellingPaket()
	if err != nil {
		return nil, err
	}

	bestPaketName := ""
	bestPaketCount := int64(0)
	if bestPaket != nil {
		bestPaketName = bestPaket.PaketName
		bestPaketCount = bestPaket.Count
	}

	return &FinancialStatsResponse{
		IncomeToday:           incomeToday,
		IncomeThisWeek:        incomeThisWeek,
		IncomeThisMonth:       incomeThisMonth,
		TotalIncome:           totalIncome,
		TotalTransactions:     totalTransactions,
		PendingTransactions:   pendingTransactions,
		PendingIncome:         pendingIncome,
		GrossIncome:           grossIncome,
		BestSellingPaket:      bestPaketName,
		BestSellingPaketCount: bestPaketCount,
	}, nil
}

func (s *Service) GetIncomeByDate(date string) (*IncomeByDateResponse, error) {
	parsedDate, err := time.Parse("2006-01-02", date)
	if err != nil {
		return nil, errors.New("format tanggal tidak valid (gunakan YYYY-MM-DD)")
	}

	income, err := s.repo.GetIncomeByDate(parsedDate)
	if err != nil {
		return nil, err
	}

	return &IncomeByDateResponse{
		Date:   parsedDate.Format("02 Jan 2006"),
		Income: income,
	}, nil
}

func (s *Service) GetFinancialPaymentDetails(status, search, dateFrom, dateTo string, limit int) ([]FinancialPaymentDetailResponse, error) {
	normalizedStatus := strings.ToLower(strings.TrimSpace(status))
	switch normalizedStatus {
	case "", "all", "pending", "verified":
	default:
		return nil, errors.New("status tidak valid, gunakan all|pending|verified")
	}

	var parsedDateFrom *time.Time
	var parsedDateTo *time.Time

	if strings.TrimSpace(dateFrom) != "" {
		parsed, err := time.Parse("2006-01-02", dateFrom)
		if err != nil {
			return nil, errors.New("format date_from tidak valid (gunakan YYYY-MM-DD)")
		}
		parsedDateFrom = &parsed
	}

	if strings.TrimSpace(dateTo) != "" {
		parsed, err := time.Parse("2006-01-02", dateTo)
		if err != nil {
			return nil, errors.New("format date_to tidak valid (gunakan YYYY-MM-DD)")
		}
		parsedDateTo = &parsed
	}

	if parsedDateFrom != nil && parsedDateTo != nil && parsedDateFrom.After(*parsedDateTo) {
		return nil, errors.New("date_from tidak boleh lebih besar dari date_to")
	}

	payments, err := s.repo.GetFinancialPaymentDetails(
		normalizedStatus,
		strings.TrimSpace(search),
		parsedDateFrom,
		parsedDateTo,
		limit,
	)
	if err != nil {
		return nil, err
	}

	responses := make([]FinancialPaymentDetailResponse, 0, len(payments))
	for _, p := range payments {
		responses = append(responses, FinancialPaymentDetailResponse{
			ID:           p.ID,
			Username:     p.Username,
			Email:        p.Email,
			Paket:        p.Paket,
			PaketDisplay: s.getPaketDisplayName(p.Paket),
			TotalHarga:   p.TotalHarga,
			TanggalBayar: p.TanggalBayar.Format("02 Jan 2006, 15:04 WIB"),
			Catatan:      p.Catatan,
			Status:       p.Status,
		})
	}

	return responses, nil
}

// ================== HELPER FUNCTIONS ==================

func (s *Service) paymentToResponse(p model.Payment) PaymentResponse {
	return PaymentResponse{
		ID:              p.ID,
		Username:        p.Username,
		Email:           p.Email,
		Paket:           p.Paket,
		TotalHarga:      p.TotalHarga,
		BuktiPembayaran: p.BuktiPembayaran,
		TanggalBayar:    p.TanggalBayar.Format("02 Jan 2006, 15:04 WIB"),
		Catatan:         p.Catatan,
		Status:          p.Status,
	}
}

func (s *Service) compressAndEncodeImage(file multipart.File) (string, error) {
	// Read all bytes from file
	imgData, err := io.ReadAll(file)
	if err != nil {
		return "", errors.New("gagal membaca file: " + err.Error())
	}

	// Decode image
	reader := bytes.NewReader(imgData)
	img, format, err := image.Decode(reader)
	if err != nil {
		return "", errors.New("gagal decode gambar: " + err.Error())
	}

	// Resize to max 800px width while maintaining aspect ratio
	bounds := img.Bounds()
	width := bounds.Dx()
	maxWidth := 800

	if width > maxWidth {
		ratio := float64(maxWidth) / float64(width)
		newHeight := int(float64(bounds.Dy()) * ratio)
		img = resizeImage(img, maxWidth, newHeight)
	}

	// Compress to JPEG with quality
	var buf bytes.Buffer
	switch strings.ToLower(format) {
	case "png":
		err = png.Encode(&buf, img)
	case "jpeg", "jpg":
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 70})
	default:
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 70})
	}

	if err != nil {
		return "", errors.New("gagal compress gambar: " + err.Error())
	}

	// Encode to base64
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func resizeImage(src image.Image, width, height int) image.Image {
	bounds := src.Bounds()
	newBounds := image.Rect(0, 0, width, height)
	dst := image.NewRGBA(newBounds)

	// Simple nearest neighbor resize
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			srcX := x * bounds.Dx() / width
			srcY := y * bounds.Dy() / height
			dst.Set(x, y, src.At(srcX, srcY))
		}
	}

	return dst
}

func (s *Service) sendAccountEmail(email, username, password string) error {
	cfg := config.GetSMTPConfig()
	if !cfg.Enabled() {
		config.Log("[Email] SMTP belum lengkap (" + strings.Join(cfg.MissingFields(), ", ") + "), email akun dilewati untuk " + email)
		return nil
	}

	subject := "Akun Wakirim Anda Sudah Aktif"
	plainBody := fmt.Sprintf(
		"Halo %s,\n\nTerima kasih telah order Wakirim.\n\nDetail akun Anda:\nUsername: %s\nEmail: %s\nPassword: %s\n\nSilakan login dan segera ganti password jika diperlukan.\n\nTerima kasih telah order.\nWakirim",
		username,
		username,
		email,
		password,
	)

	htmlBody := buildAccountEmailHTML(username, email, password, cfg.LogoURL, cfg.LoginURL)
	return sendSMTPEmail(cfg, []string{email}, subject, plainBody, htmlBody)
}

func (s *Service) sendPasswordResetEmail(email, newPassword string) error {
	cfg := config.GetSMTPConfig()
	if !cfg.Enabled() {
		config.Log("[Email] SMTP belum lengkap (" + strings.Join(cfg.MissingFields(), ", ") + "), email reset password dilewati untuk " + email)
		return nil
	}

	subject := "Password Wakirim Anda Telah Direset"
	plainBody := fmt.Sprintf(
		"Halo,\n\nPassword akun Wakirim Anda telah direset.\n\nPassword baru: %s\n\nSilakan login dan ganti password jika diperlukan.\n\nTerima kasih.\nWakirim",
		newPassword,
	)
	htmlBody := buildPasswordResetEmailHTML(newPassword, cfg.LogoURL)
	return sendSMTPEmail(cfg, []string{email}, subject, plainBody, htmlBody)
}

func buildAccountEmailHTML(username, email, password, logoURL, loginURL string) string {
	return fmt.Sprintf(`<!doctype html>
<html lang="id">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Akun Wakirim Aktif</title>
  <style>
    @media only screen and (max-width: 620px) {
      .email-shell { padding: 18px 10px !important; }
      .email-card { width: 100%% !important; border-radius: 12px !important; }
      .email-pad { padding-left: 18px !important; padding-right: 18px !important; }
      .email-title { font-size: 22px !important; }
      .detail-label, .detail-value { display: block !important; width: 100%% !important; padding-left: 0 !important; padding-right: 0 !important; }
      .detail-label { padding-bottom: 4px !important; border-bottom: 0 !important; }
      .detail-value { padding-top: 0 !important; }
    }
  </style>
</head>
<body style="margin:0;background:#f3f4f6;font-family:Arial,Helvetica,sans-serif;color:#111111;">
  <div style="display:none;max-height:0;overflow:hidden;opacity:0;color:transparent;">Pembayaran Anda sudah diverifikasi. Berikut detail akun Wakirim Anda.</div>
  <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" class="email-shell" style="background:#f3f4f6;padding:34px 16px;">
    <tr>
      <td align="center">
        <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" class="email-card" style="max-width:640px;background:#ffffff;border:1px solid #e5e7eb;border-radius:18px;overflow:hidden;box-shadow:0 18px 45px rgba(17,24,39,.08);">
          <tr>
            <td class="email-pad" style="padding:30px 30px 24px;text-align:center;background:#111111;">
              %s
              <p style="margin:20px 0 8px;font-size:12px;line-height:1.4;color:#a3a3a3;font-weight:700;text-transform:uppercase;letter-spacing:.16em;">Pembayaran Terverifikasi</p>
              <h1 class="email-title" style="margin:0;font-size:28px;line-height:1.25;color:#ffffff;">Akun Wakirim Anda Sudah Aktif</h1>
              <p style="margin:12px auto 0;max-width:460px;font-size:14px;line-height:1.8;color:#d4d4d4;">Terima kasih, pembayaran Anda sudah kami verifikasi. Gunakan detail akun berikut untuk mulai memakai layanan Wakirim.</p>
            </td>
          </tr>
          <tr>
            <td class="email-pad" style="padding:28px 30px;">
              <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" style="border:1px solid #e5e7eb;border-radius:14px;overflow:hidden;background:#ffffff;">
                <tr>
                  <td style="padding:15px 18px;background:#fafafa;border-bottom:1px solid #e5e7eb;font-size:12px;font-weight:700;color:#737373;text-transform:uppercase;letter-spacing:.12em;">Detail Akun Login</td>
                </tr>
                <tr>
                  <td style="padding:0 18px;">
                    <table role="presentation" width="100%%" cellspacing="0" cellpadding="0">
                      <tr>
                        <td class="detail-label" style="padding:16px 0;border-bottom:1px solid #eeeeee;font-size:13px;color:#737373;width:150px;">Username</td>
                        <td class="detail-value" style="padding:16px 0;border-bottom:1px solid #eeeeee;font-size:15px;font-weight:700;color:#111111;">%s</td>
                      </tr>
                      <tr>
                        <td class="detail-label" style="padding:16px 0;border-bottom:1px solid #eeeeee;font-size:13px;color:#737373;width:150px;">Email</td>
                        <td class="detail-value" style="padding:16px 0;border-bottom:1px solid #eeeeee;font-size:15px;font-weight:700;color:#111111;">%s</td>
                      </tr>
                      <tr>
                        <td class="detail-label" style="padding:16px 0;font-size:13px;color:#737373;width:150px;">Password</td>
                        <td class="detail-value" style="padding:16px 0;font-size:15px;font-weight:700;color:#111111;">%s</td>
                      </tr>
                    </table>
                  </td>
                </tr>
              </table>
              <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" style="margin-top:18px;border-radius:14px;background:#f8fafc;border:1px solid #e5e7eb;">
                <tr>
                  <td style="padding:18px 20px;">
                    <p style="margin:0;font-size:14px;line-height:1.8;color:#525252;">Silakan login menggunakan detail di atas. Simpan email ini dengan baik dan jangan bagikan password kepada siapa pun.</p>
                    <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" style="margin-top:14px;">
                      <tr>
                        <td align="center">
                          <a href="%s" style="display:inline-block;border-radius:10px;background:#111111;border:1px solid #111111;color:#ffffff;text-decoration:none;font-size:14px;font-weight:700;line-height:1;padding:12px 18px;">Login</a>
                        </td>
                      </tr>
                    </table>
                  </td>
                </tr>
              </table>
              <p style="margin:22px 0 0;font-size:16px;line-height:1.7;color:#111111;font-weight:700;text-align:center;">Terima kasih telah order.</p>
            </td>
          </tr>
          <tr>
            <td class="email-pad" style="padding:18px 30px;background:#fafafa;text-align:center;border-top:1px solid #e5e7eb;">
              <p style="margin:0;font-size:12px;line-height:1.7;color:#737373;">Wakirim - WhatsApp automation service</p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`, emailLogoHTML(logoURL), html.EscapeString(username), html.EscapeString(email), html.EscapeString(password), html.EscapeString(loginURL))
}

func buildPasswordResetEmailHTML(newPassword, logoURL string) string {
	return fmt.Sprintf(`<!doctype html>
<html lang="id">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <style>
    @media only screen and (max-width: 620px) {
      .email-shell { padding: 18px 10px !important; }
      .email-card { width: 100%% !important; border-radius: 12px !important; }
      .email-pad { padding-left: 18px !important; padding-right: 18px !important; }
    }
  </style>
</head>
<body style="margin:0;background:#f3f4f6;font-family:Arial,Helvetica,sans-serif;color:#111111;">
  <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" class="email-shell" style="background:#f3f4f6;padding:34px 16px;">
    <tr>
      <td align="center">
        <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" class="email-card" style="max-width:560px;background:#ffffff;border:1px solid #e5e7eb;border-radius:18px;overflow:hidden;box-shadow:0 18px 45px rgba(17,24,39,.08);">
          <tr>
            <td class="email-pad" style="padding:28px;text-align:center;background:#111111;">
              %s
              <h1 style="margin:18px 0 0;font-size:22px;line-height:1.3;color:#ffffff;">Password Wakirim Anda Telah Direset</h1>
              <p style="margin:10px 0 0;font-size:14px;line-height:1.7;color:#d4d4d4;">Gunakan password baru berikut untuk login.</p>
            </td>
          </tr>
          <tr>
            <td class="email-pad" style="padding:26px 28px;">
              <div style="border:1px solid #e5e7eb;border-radius:12px;background:#fafafa;padding:18px;">
                <p style="margin:0 0 8px;font-size:13px;font-weight:700;color:#737373;text-transform:uppercase;letter-spacing:.08em;">Password Baru</p>
                <p style="margin:0;font-size:18px;font-weight:700;color:#111111;">%s</p>
              </div>
              <p style="margin:22px 0 0;font-size:14px;line-height:1.7;color:#525252;">Silakan login dan ganti password jika diperlukan.</p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`, emailLogoHTML(logoURL), html.EscapeString(newPassword))
}

func emailLogoHTML(logoURL string) string {
	if strings.TrimSpace(logoURL) == "" {
		return `<div style="display:inline-block;border:1px solid rgba(255,255,255,.18);border-radius:12px;padding:12px 18px;color:#ffffff;font-size:20px;font-weight:800;letter-spacing:.02em;">Wakirim</div>`
	}

	return fmt.Sprintf(
		`<img src="%s" width="156" alt="Wakirim" style="display:inline-block;max-width:156px;width:156px;height:auto;margin:0 auto;border:0;outline:none;text-decoration:none;">`,
		html.EscapeString(logoURL),
	)
}

func sendSMTPEmail(cfg config.SMTPConfig, recipients []string, subject, plainBody, htmlBody string) error {
	message, err := buildEmailMessage(cfg, recipients, subject, plainBody, htmlBody)
	if err != nil {
		return err
	}

	address := net.JoinHostPort(cfg.Host, cfg.Port)
	var client *smtp.Client

	if cfg.Secure {
		conn, err := tls.Dial("tcp", address, &tls.Config{ServerName: cfg.Host})
		if err != nil {
			return err
		}

		client, err = smtp.NewClient(conn, cfg.Host)
		if err != nil {
			conn.Close()
			return err
		}
	} else {
		client, err = smtp.Dial(address)
		if err != nil {
			return err
		}

		if cfg.StartTLS {
			if err := client.StartTLS(&tls.Config{ServerName: cfg.Host}); err != nil {
				client.Close()
				return err
			}
		}
	}
	defer client.Close()

	if cfg.Username != "" || cfg.Password != "" {
		if err := client.Auth(smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)); err != nil {
			return err
		}
	}

	if err := client.Mail(cfg.FromEmail); err != nil {
		return err
	}

	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			return err
		}
	}

	writer, err := client.Data()
	if err != nil {
		return err
	}

	if _, err := writer.Write(message); err != nil {
		writer.Close()
		return err
	}

	if err := writer.Close(); err != nil {
		return err
	}

	return client.Quit()
}

func buildEmailMessage(cfg config.SMTPConfig, recipients []string, subject, plainBody, htmlBody string) ([]byte, error) {
	alternativeBoundary := "wakirim_alt_" + strings.ReplaceAll(uuid.New().String(), "-", "")
	from := mail.Address{Name: cfg.FromName, Address: cfg.FromEmail}

	var msg bytes.Buffer
	msg.WriteString("From: " + from.String() + "\r\n")
	msg.WriteString("To: " + strings.Join(recipients, ", ") + "\r\n")
	msg.WriteString("Subject: " + mime.QEncoding.Encode("utf-8", subject) + "\r\n")
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: multipart/alternative; boundary=\"" + alternativeBoundary + "\"\r\n")
	msg.WriteString("\r\n")

	msg.WriteString("--" + alternativeBoundary + "\r\n")
	msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	msg.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(quotedPrintable(plainBody))
	msg.WriteString("\r\n")

	msg.WriteString("--" + alternativeBoundary + "\r\n")
	msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	msg.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(quotedPrintable(htmlBody))
	msg.WriteString("\r\n")
	msg.WriteString("--" + alternativeBoundary + "--\r\n")

	return msg.Bytes(), nil
}

func quotedPrintable(value string) string {
	var buf bytes.Buffer
	writer := quotedprintable.NewWriter(&buf)
	_, _ = writer.Write([]byte(value))
	_ = writer.Close()
	return buf.String()
}

func stringPtr(s string) *string {
	return &s
}

func (s *Service) getPaketDisplayName(paketID string) string {
	switch paketID {
	case "starter":
		return "Starter"
	case "growth":
		return "Growth"
	case "best-value":
		return "Best Value"
	case "manual":
		return "Manual"
	}

	paket, err := s.repo.GetPaketByID(paketID)
	if err == nil && strings.TrimSpace(paket.Nama) != "" {
		return paket.Nama
	}

	if strings.TrimSpace(paketID) == "" {
		return "-"
	}

	return paketID
}

// ReadAll reads all bytes from a reader
func ReadAll(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}
