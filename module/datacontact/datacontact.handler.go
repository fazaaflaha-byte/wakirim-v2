package datacontact

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

func (h *Handler) RegisterRoutes(router *gin.Engine, middlewares ...gin.HandlerFunc) {
	group := router.Group("/oauth/data-contact")
	group.Use(middlewares...)
	{
		group.GET("", h.List)
		group.GET("/:id", h.Detail)
		group.POST("/csv", h.CreateFromCSV)
		group.POST("/manual", h.CreateManual)
		group.DELETE("/:id", h.Delete)
	}
}

func (h *Handler) List(c *gin.Context) {
	akunUUID, ok := h.akunUUID(c)
	if !ok {
		return
	}
	list, err := h.service.GetByAkunUUID(akunUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ApiResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, ApiResponse{Success: true, Data: list})
}

func (h *Handler) Detail(c *gin.Context) {
	akunUUID, ok := h.akunUUID(c)
	if !ok {
		return
	}
	detail, err := h.service.GetDetail(c.Param("id"), akunUUID)
	if err != nil {
		c.JSON(http.StatusNotFound, ApiResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, ApiResponse{Success: true, Data: detail})
}

func (h *Handler) CreateFromCSV(c *gin.Context) {
	akunUUID, ok := h.akunUUID(c)
	if !ok {
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, ApiResponse{Success: false, Message: "File CSV wajib diupload"})
		return
	}
	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, ApiResponse{Success: false, Message: "File CSV tidak dapat dibaca"})
		return
	}
	defer src.Close()

	namaFile := c.PostForm("nama_file")
	if namaFile == "" {
		namaFile = file.Filename
	}
	if err := h.service.CreateFromCSV(akunUUID, namaFile, src); err != nil {
		c.JSON(http.StatusBadRequest, ApiResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, ApiResponse{Success: true, Message: "Data contact CSV berhasil disimpan sebagai TXT"})
}

func (h *Handler) CreateManual(c *gin.Context) {
	akunUUID, ok := h.akunUUID(c)
	if !ok {
		return
	}
	var req CreateManualRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ApiResponse{Success: false, Message: "Data contact manual tidak valid"})
		return
	}
	if err := h.service.CreateManual(akunUUID, req); err != nil {
		c.JSON(http.StatusBadRequest, ApiResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, ApiResponse{Success: true, Message: "Data contact manual berhasil disimpan sebagai TXT"})
}

func (h *Handler) Delete(c *gin.Context) {
	akunUUID, ok := h.akunUUID(c)
	if !ok {
		return
	}
	if err := h.service.Delete(c.Param("id"), akunUUID); err != nil {
		c.JSON(http.StatusInternalServerError, ApiResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, ApiResponse{Success: true, Message: "Data contact berhasil dihapus"})
}

func (h *Handler) akunUUID(c *gin.Context) (string, bool) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, ApiResponse{Success: false, Message: "Database tidak tersedia"})
		return "", false
	}
	value, _ := c.Get("client_user_uuid")
	akunUUID, ok := value.(string)
	if !ok || akunUUID == "" {
		c.JSON(http.StatusUnauthorized, ApiResponse{Success: false, Message: "Unauthorized"})
		return "", false
	}
	return akunUUID, true
}
