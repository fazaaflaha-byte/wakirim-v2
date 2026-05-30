package payment

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"

	"wakirim/config"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service   *Service
	repo      *Repository
	once      sync.Once
	initError error
}

func NewHandler() *Handler {
	return &Handler{}
}

func (h *Handler) initService() (*Service, error) {
	h.once.Do(func() {
		config.Log("[Handler] initService called, DB = " + fmt.Sprintf("%v", config.DB))
		if config.DB == nil {
			h.initError = nil // Will return empty data instead of error
			config.Log("[Handler] DB is nil")
			return
		}
		h.repo = NewRepository(config.DB)
		h.service = NewService(h.repo)
		config.Log("[Handler] Service initialized successfully")
	})
	return h.service, h.initError
}

func (h *Handler) RegisterRoutes(router *gin.Engine) {
	// Public routes - client payment page
	router.GET("/api/paket", h.GetPakets)
	router.POST("/api/payment", h.CreatePayment)

	// Admin routes - payment management
	admin := router.Group("/admin/api")
	{
		admin.GET("/payments", h.GetAllPayments)
		admin.GET("/payments/stats", h.GetPaymentStats)
		admin.POST("/payments/verify", h.VerifyPayment)
		admin.POST("/payments/delete", h.DeletePayment)
		admin.GET("/payment/:id", h.GetPaymentByID)

		// Account management
		admin.GET("/accounts", h.GetAllAccounts)
		admin.POST("/accounts/manual", h.CreateManualAccount)
		admin.POST("/accounts/upgrade", h.UpgradeAccount)
		admin.POST("/accounts/reset-password", h.ResetPassword)
		admin.POST("/accounts/delete", h.DeleteAccount)

		// Financial stats
		admin.GET("/financial/stats", h.GetFinancialStats)
		admin.GET("/financial/income-by-date", h.GetIncomeByDate)
		admin.GET("/financial/payments", h.GetFinancialPayments)
	}
}

// ================== PUBLIC ROUTES ==================

// GetPakets returns all active paket plans
func (h *Handler) GetPakets(c *gin.Context) {
	svc, err := h.initService()
	if err != nil || svc == nil {
		// Return fallback static data if DB not available
		c.JSON(http.StatusOK, ApiResponse{
			Success: true,
			Data: []PaketResponse{
				{ID: "starter", Nama: "Starter", Harga: 59000, DurasiBulan: 1},
				{ID: "growth", Nama: "Growth", Harga: 149000, DurasiBulan: 3},
				{ID: "best-value", Nama: "Best Value", Harga: 295000, DurasiBulan: 12},
			},
		})
		return
	}

	pakets, err := svc.GetAllActivePaket()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ApiResponse{
			Success: false,
			Message: "Gagal mengambil data paket",
		})
		return
	}

	c.JSON(http.StatusOK, ApiResponse{
		Success: true,
		Data:    pakets,
	})
}

// CreatePayment handles new payment submission from client
func (h *Handler) CreatePayment(c *gin.Context) {
	username := c.PostForm("username")
	email := c.PostForm("email")
	paket := c.PostForm("paket")
	catatan := c.PostForm("catatan")

	// Get file
	file, err := c.FormFile("bukti_pembayaran")
	if err != nil {
		c.JSON(http.StatusBadRequest, ApiResponse{
			Success: false,
			Message: "Bukti pembayaran diperlukan",
		})
		return
	}

	// Validate file size (max 5MB)
	if file.Size > 5*1024*1024 {
		c.JSON(http.StatusBadRequest, ApiResponse{
			Success: false,
			Message: "Ukuran file maksimal 5MB",
		})
		return
	}

	// Open file
	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ApiResponse{
			Success: false,
			Message: "Gagal membaca file",
		})
		return
	}
	defer src.Close()

	svc, err := h.initService()
	if err != nil || svc == nil {
		c.JSON(http.StatusInternalServerError, ApiResponse{
			Success: false,
			Message: "Database tidak tersedia",
		})
		return
	}

	req := &CreatePaymentRequest{
		Username: username,
		Email:    email,
		Paket:    paket,
		Catatan:  catatan,
	}

	payment, err := svc.CreatePayment(req, src)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ApiResponse{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, ApiResponse{
		Success: true,
		Message: "Pembayaran berhasil disimpan",
		Data:    payment,
	})
}

// ================== ADMIN ROUTES ==================

// GetAllPayments returns all payments
func (h *Handler) GetAllPayments(c *gin.Context) {
	if !isAdminAuthenticated(c) {
		c.JSON(http.StatusUnauthorized, ApiResponse{
			Success: false,
			Message: "Unauthorized",
		})
		return
	}

	svc, err := h.initService()
	if err != nil || svc == nil {
		c.JSON(http.StatusOK, ApiResponse{
			Success: true,
			Data:    []PaymentResponse{},
		})
		return
	}

	var payments []PaymentResponse
	paymentType := c.Query("type")
	if paymentType == "renewal" || paymentType == "upgrade" {
		payments, err = svc.GetRenewalPayments()
	} else {
		payments, err = svc.GetAllPayments()
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, ApiResponse{
			Success: false,
			Message: "Gagal mengambil data pembayaran: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, ApiResponse{
		Success: true,
		Data:    payments,
	})
}

// GetPaymentStats returns payment statistics
func (h *Handler) GetPaymentStats(c *gin.Context) {
	if !isAdminAuthenticated(c) {
		c.JSON(http.StatusUnauthorized, ApiResponse{
			Success: false,
			Message: "Unauthorized",
		})
		return
	}

	svc, err := h.initService()
	if err != nil || svc == nil {
		c.JSON(http.StatusOK, ApiResponse{
			Success: true,
			Data: AccountStatsResponse{
				TotalAkun: 0,
				Aktif:     0,
				Berakhir:  0,
				Menunggu:  0,
			},
		})
		return
	}

	stats, err := svc.GetAccountStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ApiResponse{
			Success: false,
			Message: "Gagal mengambil statistik",
		})
		return
	}

	c.JSON(http.StatusOK, ApiResponse{
		Success: true,
		Data:    stats,
	})
}

// VerifyPayment verifies a payment and creates account
func (h *Handler) VerifyPayment(c *gin.Context) {
	if !isAdminAuthenticated(c) {
		c.JSON(http.StatusUnauthorized, ApiResponse{
			Success: false,
			Message: "Unauthorized",
		})
		return
	}

	var req struct {
		PaymentID   string `json:"payment_id" binding:"required"`
		AccountPass string `json:"account_password"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ApiResponse{
			Success: false,
			Message: "Data tidak valid",
		})
		return
	}

	svc, err := h.initService()
	if err != nil || svc == nil {
		c.JSON(http.StatusInternalServerError, ApiResponse{
			Success: false,
			Message: "Database tidak tersedia",
		})
		return
	}

	err = svc.VerifyAndCreateAccount(req.PaymentID, req.AccountPass)
	if err != nil {
		c.JSON(http.StatusBadRequest, ApiResponse{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, ApiResponse{
		Success: true,
		Message: "Pembayaran berhasil diverifikasi",
	})
}

// DeletePayment deletes a payment and its linked account data.
func (h *Handler) DeletePayment(c *gin.Context) {
	if !isAdminAuthenticated(c) {
		c.JSON(http.StatusUnauthorized, ApiResponse{
			Success: false,
			Message: "Unauthorized",
		})
		return
	}

	var req DeletePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ApiResponse{
			Success: false,
			Message: "Data tidak valid",
		})
		return
	}

	svc, err := h.initService()
	if err != nil || svc == nil {
		c.JSON(http.StatusInternalServerError, ApiResponse{
			Success: false,
			Message: "Database tidak tersedia",
		})
		return
	}

	if err := svc.DeletePayment(req); err != nil {
		c.JSON(http.StatusBadRequest, ApiResponse{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, ApiResponse{
		Success: true,
		Message: "Pembayaran dan akun terkait berhasil dihapus",
	})
}

// GetPaymentByID returns a single payment by ID
func (h *Handler) GetPaymentByID(c *gin.Context) {
	if !isAdminAuthenticated(c) {
		c.JSON(http.StatusUnauthorized, ApiResponse{
			Success: false,
			Message: "Unauthorized",
		})
		return
	}

	id := c.Param("id")

	svc, err := h.initService()
	if err != nil || svc == nil {
		c.JSON(http.StatusServiceUnavailable, ApiResponse{
			Success: false,
			Message: "Database tidak tersedia",
		})
		return
	}

	payment, err := svc.GetPaymentByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, ApiResponse{
			Success: false,
			Message: "Pembayaran tidak ditemukan",
		})
		return
	}

	c.JSON(http.StatusOK, ApiResponse{
		Success: true,
		Data:    payment,
	})
}

// GetAllAccounts returns all accounts
func (h *Handler) GetAllAccounts(c *gin.Context) {
	if !isAdminAuthenticated(c) {
		c.JSON(http.StatusUnauthorized, ApiResponse{
			Success: false,
			Message: "Unauthorized",
		})
		return
	}

	svc, err := h.initService()
	if err != nil || svc == nil {
		c.JSON(http.StatusOK, ApiResponse{
			Success: true,
			Data:    []AkunResponse{},
		})
		return
	}

	accounts, err := svc.GetAllAkun()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ApiResponse{
			Success: false,
			Message: "Gagal mengambil data akun: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, ApiResponse{
		Success: true,
		Data:    accounts,
	})
}

// CreateManualAccount creates a manual account
func (h *Handler) CreateManualAccount(c *gin.Context) {
	if !isAdminAuthenticated(c) {
		c.JSON(http.StatusUnauthorized, ApiResponse{
			Success: false,
			Message: "Unauthorized",
		})
		return
	}

	var req CreateManualAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ApiResponse{
			Success: false,
			Message: "Data tidak valid",
		})
		return
	}

	svc, err := h.initService()
	if err != nil || svc == nil {
		c.JSON(http.StatusInternalServerError, ApiResponse{
			Success: false,
			Message: "Database tidak tersedia",
		})
		return
	}

	if err := svc.CreateManualAccount(&req); err != nil {
		c.JSON(http.StatusBadRequest, ApiResponse{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, ApiResponse{
		Success: true,
		Message: "Akun berhasil dibuat dan email dikirim",
	})
}

// UpgradeAccount extends account duration
func (h *Handler) UpgradeAccount(c *gin.Context) {
	if !isAdminAuthenticated(c) {
		c.JSON(http.StatusUnauthorized, ApiResponse{
			Success: false,
			Message: "Unauthorized",
		})
		return
	}

	var req struct {
		Username string `json:"username" binding:"required"`
		Days     int    `json:"days" binding:"required,min=1"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ApiResponse{
			Success: false,
			Message: "Data tidak valid",
		})
		return
	}

	svc, err := h.initService()
	if err != nil || svc == nil {
		c.JSON(http.StatusInternalServerError, ApiResponse{
			Success: false,
			Message: "Database tidak tersedia",
		})
		return
	}

	if err := svc.UpgradeAccount(req.Username, req.Days); err != nil {
		c.JSON(http.StatusBadRequest, ApiResponse{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, ApiResponse{
		Success: true,
		Message: "Masa aktif berhasil diperpanjang",
	})
}

// ResetPassword resets account password
func (h *Handler) ResetPassword(c *gin.Context) {
	if !isAdminAuthenticated(c) {
		c.JSON(http.StatusUnauthorized, ApiResponse{
			Success: false,
			Message: "Unauthorized",
		})
		return
	}

	var req struct {
		Username    string `json:"username" binding:"required"`
		Email       string `json:"email" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ApiResponse{
			Success: false,
			Message: "Data tidak valid",
		})
		return
	}

	svc, err := h.initService()
	if err != nil || svc == nil {
		c.JSON(http.StatusInternalServerError, ApiResponse{
			Success: false,
			Message: "Database tidak tersedia",
		})
		return
	}

	if err := svc.ResetPassword(ResetPasswordRequest{
		Username: req.Username,
		Email:    req.Email,
		Password: req.NewPassword,
	}); err != nil {
		c.JSON(http.StatusBadRequest, ApiResponse{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, ApiResponse{
		Success: true,
		Message: "Password berhasil direset",
	})
}

// DeleteAccount deletes a client account.
func (h *Handler) DeleteAccount(c *gin.Context) {
	if !isAdminAuthenticated(c) {
		c.JSON(http.StatusUnauthorized, ApiResponse{
			Success: false,
			Message: "Unauthorized",
		})
		return
	}

	var req DeleteAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ApiResponse{
			Success: false,
			Message: "Data tidak valid",
		})
		return
	}

	svc, err := h.initService()
	if err != nil || svc == nil {
		c.JSON(http.StatusInternalServerError, ApiResponse{
			Success: false,
			Message: "Database tidak tersedia",
		})
		return
	}

	if err := svc.DeleteAccount(req); err != nil {
		c.JSON(http.StatusBadRequest, ApiResponse{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, ApiResponse{
		Success: true,
		Message: "Akun berhasil dihapus",
	})
}

// ================== FINANCIAL STATS ROUTES ==================

// GetFinancialStats returns overall financial statistics
func (h *Handler) GetFinancialStats(c *gin.Context) {
	if !isAdminAuthenticated(c) {
		c.JSON(http.StatusUnauthorized, ApiResponse{
			Success: false,
			Message: "Unauthorized",
		})
		return
	}

	svc, err := h.initService()
	if err != nil || svc == nil {
		// Debug: check if DB is initialized
		config.Log("[Financial] Service is nil, DB might not be initialized")
		c.JSON(http.StatusOK, ApiResponse{
			Success: true,
			Data: FinancialStatsResponse{
				IncomeToday:         0,
				IncomeThisWeek:      0,
				IncomeThisMonth:     0,
				TotalIncome:         0,
				TotalTransactions:   0,
				PendingTransactions: 0,
				PendingIncome:       0,
				GrossIncome:         0,
				BestSellingPaket:    "-",
			},
		})
		return
	}

	stats, err := svc.GetFinancialStats()
	if err != nil {
		config.Log("[Financial] Error getting stats: " + err.Error())
		c.JSON(http.StatusInternalServerError, ApiResponse{
			Success: false,
			Message: "Gagal mengambil statistik keuangan: " + err.Error(),
		})
		return
	}

	config.Log("[Financial] Stats loaded successfully")
	c.JSON(http.StatusOK, ApiResponse{
		Success: true,
		Data:    stats,
	})
}

// GetIncomeByDate returns income for a specific date
func (h *Handler) GetIncomeByDate(c *gin.Context) {
	if !isAdminAuthenticated(c) {
		c.JSON(http.StatusUnauthorized, ApiResponse{
			Success: false,
			Message: "Unauthorized",
		})
		return
	}

	date := c.Query("date")
	if date == "" {
		c.JSON(http.StatusBadRequest, ApiResponse{
			Success: false,
			Message: "Parameter date diperlukan (format: YYYY-MM-DD)",
		})
		return
	}

	svc, err := h.initService()
	if err != nil || svc == nil {
		c.JSON(http.StatusOK, ApiResponse{
			Success: true,
			Data: IncomeByDateResponse{
				Date:   date,
				Income: 0,
			},
		})
		return
	}

	result, err := svc.GetIncomeByDate(date)
	if err != nil {
		c.JSON(http.StatusBadRequest, ApiResponse{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, ApiResponse{
		Success: true,
		Data:    result,
	})
}

// GetFinancialPayments returns detailed financial rows directly from payment table
func (h *Handler) GetFinancialPayments(c *gin.Context) {
	if !isAdminAuthenticated(c) {
		c.JSON(http.StatusUnauthorized, ApiResponse{
			Success: false,
			Message: "Unauthorized",
		})
		return
	}

	status := c.DefaultQuery("status", "all")
	search := c.Query("search")
	dateFrom := c.Query("date_from")
	dateTo := c.Query("date_to")

	limit := 100
	if rawLimit := c.Query("limit"); rawLimit != "" {
		parsedLimit, err := strconv.Atoi(rawLimit)
		if err != nil {
			c.JSON(http.StatusBadRequest, ApiResponse{
				Success: false,
				Message: "Parameter limit harus berupa angka",
			})
			return
		}
		limit = parsedLimit
	}

	svc, err := h.initService()
	if err != nil || svc == nil {
		c.JSON(http.StatusOK, ApiResponse{
			Success: true,
			Data:    []FinancialPaymentDetailResponse{},
		})
		return
	}

	rows, err := svc.GetFinancialPaymentDetails(status, search, dateFrom, dateTo, limit)
	if err != nil {
		c.JSON(http.StatusBadRequest, ApiResponse{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, ApiResponse{
		Success: true,
		Data:    rows,
	})
}

// ================== HELPER FUNCTIONS ==================

func isAdminAuthenticated(c *gin.Context) bool {
	sessionToken, err := c.Cookie(adminSessionCookie)
	if err != nil {
		return false
	}

	return secureCompare(sessionToken, adminSessionToken())
}

func adminSessionToken() string {
	mac := hmac.New(sha256.New, []byte(adminPassword()))
	mac.Write([]byte(adminUsername()))
	return hex.EncodeToString(mac.Sum(nil))
}

func adminUsername() string {
	username := os.Getenv("ADMIN_USERNAME")
	if username == "" {
		return "admin"
	}
	return username
}

func adminPassword() string {
	password := os.Getenv("ADMIN_PASSWORD")
	if password == "" {
		return "admin123"
	}
	return password
}

func secureCompare(a string, b string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

const adminSessionCookie = "wakirim_admin_session"
