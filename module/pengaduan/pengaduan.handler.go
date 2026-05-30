package pengaduan

import (
	"net/http"

	"wakirim/config"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler() *Handler {
	if config.DB == nil {
		return &Handler{service: nil}
	}
	repo := NewRepository(config.DB)
	service := NewService(repo)
	return &Handler{service: service}
}

func (h *Handler) RegisterClientRoutes(router *gin.Engine, middlewares ...gin.HandlerFunc) {
	group := router.Group("/oauth/pengaduan")
	group.Use(middlewares...)
	{
		group.POST("", h.CreatePengaduan)
		group.GET("", h.GetMyPengaduan)
	}
}

func (h *Handler) RegisterAdminRoutes(router *gin.Engine, middlewares ...gin.HandlerFunc) {
	group := router.Group("/api/admin/pengaduan")
	group.Use(middlewares...)
	{
		group.GET("", h.GetAllPengaduan)
		group.POST("/:id/tanggapan", h.TanggapiPengaduan)
	}
}

// Client Handlers
func (h *Handler) CreatePengaduan(c *gin.Context) {
	uuidStr, _ := c.Get("client_user_uuid")
	akunUUID, ok := uuidStr.(string)
	if !ok {
		c.JSON(http.StatusUnauthorized, ApiResponse{Success: false, Message: "Unauthorized"})
		return
	}

	var req CreatePengaduanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ApiResponse{Success: false, Message: "Data tidak valid"})
		return
	}

	if err := h.service.CreatePengaduan(akunUUID, req.Email, req); err != nil {
		c.JSON(http.StatusInternalServerError, ApiResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, ApiResponse{Success: true, Message: "Pengaduan berhasil dikirim"})
}

func (h *Handler) GetMyPengaduan(c *gin.Context) {
	uuidStr, _ := c.Get("client_user_uuid")
	akunUUID, ok := uuidStr.(string)
	if !ok {
		c.JSON(http.StatusUnauthorized, ApiResponse{Success: false, Message: "Unauthorized"})
		return
	}

	list, err := h.service.GetByAkunUUID(akunUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ApiResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, ApiResponse{Success: true, Data: list})
}

// Admin Handlers
func (h *Handler) GetAllPengaduan(c *gin.Context) {
	list, err := h.service.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ApiResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, ApiResponse{Success: true, Data: list})
}

func (h *Handler) TanggapiPengaduan(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, ApiResponse{Success: false, Message: "ID pengaduan tidak valid"})
		return
	}

	var req TanggapiPengaduanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ApiResponse{Success: false, Message: "Data tanggapan tidak valid"})
		return
	}

	if err := h.service.Tanggapi(id, req); err != nil {
		c.JSON(http.StatusBadRequest, ApiResponse{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, ApiResponse{Success: true, Message: "Tanggapan berhasil dikirim"})
}
