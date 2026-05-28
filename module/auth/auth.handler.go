package auth

import (
	"net/http"
	"strings"

	"wakirim/config"
	"wakirim/middleware"
	"wakirim/model"

	"github.com/gin-gonic/gin"
)

const clientAuthCookie = "wakirim_client_auth"

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

func (h *Handler) RegisterRoutes(router *gin.Engine, tokenMiddlewares ...gin.HandlerFunc) {
	oauth := router.Group("/oauth")
	{
		routeHandlers := make([]gin.HandlerFunc, 0, len(tokenMiddlewares)+1)
		routeHandlers = append(routeHandlers, tokenMiddlewares...)
		routeHandlers = append(routeHandlers, h.Token)
		oauth.POST("/token", routeHandlers...)
		oauth.GET("/login-status", h.LoginStatus)
		oauth.GET("/me", h.Me)
		oauth.POST("/logout", h.Logout)
		oauth.POST("/delete-account", h.DeleteAccount)
	}
}

func (h *Handler) LoginStatus(c *gin.Context) {
	identity := strings.TrimSpace(c.Query("username"))
	status := middleware.GetClientLoginProtectionStatus(c.ClientIP(), identity)

	c.JSON(http.StatusOK, ApiResponse{
		Success: true,
		Data: gin.H{
			"active":            status.Active,
			"mode":              status.Mode,
			"reason":            status.Reason,
			"remaining_seconds": status.RemainingSeconds,
			"remaining_text":    middleware.FormatRemainingDuration(status.RemainingSeconds),
			"until":             status.Until,
		},
	})
}

func (h *Handler) Token(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, ApiResponse{
			Success: false,
			Message: "Database tidak tersedia",
		})
		return
	}

	var req TokenRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, ApiResponse{
			Success: false,
			Message: "Data login tidak valid",
		})
		return
	}

	grantType := strings.TrimSpace(strings.ToLower(req.GrantType))
	if grantType == "" {
		grantType = "password"
	}
	if grantType != "password" {
		c.JSON(http.StatusBadRequest, ApiResponse{
			Success: false,
			Message: BuildOAuthGrantError().Error(),
		})
		return
	}

	username := strings.TrimSpace(req.Username)
	password := req.Password
	if username == "" || strings.TrimSpace(password) == "" {
		c.JSON(http.StatusBadRequest, ApiResponse{
			Success: false,
			Message: "Username/email dan password wajib diisi",
		})
		return
	}

	akun, token, expiresIn, err := h.service.Authenticate(username, password)
	if err != nil {
		c.Set("auth_login_failed", true)
		c.Set("auth_login_identity", username)
		c.JSON(http.StatusUnauthorized, ApiResponse{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	c.Set("auth_login_succeeded", true)
	c.Set("auth_login_identity", username)
	h.setAuthCookie(c, token)
	c.JSON(http.StatusOK, ApiResponse{
		Success: true,
		Data:    BuildBearerTokenResponse(token, expiresIn, h.service.BuildUserResponse(akun)),
	})
}

func (h *Handler) Me(c *gin.Context) {
	akun, _, err := h.currentUserFromRequest(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, ApiResponse{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, ApiResponse{
		Success: true,
		Data:    h.service.BuildUserResponse(akun),
	})
}

func (h *Handler) Logout(c *gin.Context) {
	h.clearAuthCookie(c)
	c.JSON(http.StatusOK, ApiResponse{
		Success: true,
		Message: "Logout berhasil",
	})
}

func (h *Handler) DeleteAccount(c *gin.Context) {
	akun, _, err := h.currentUserFromRequest(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, ApiResponse{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	if err := h.service.DeleteAccountByUUID(akun.UUID); err != nil {
		c.JSON(http.StatusBadRequest, ApiResponse{
			Success: false,
			Message: "Gagal menghapus akun",
		})
		return
	}

	h.clearAuthCookie(c)
	c.JSON(http.StatusOK, ApiResponse{
		Success: true,
		Message: "Akun berhasil dihapus",
	})
}

func (h *Handler) RequireClientAuthPage(c *gin.Context) {
	akun, _, err := h.currentUserFromRequest(c)
	if err != nil {
		c.Redirect(http.StatusFound, "/login")
		c.Abort()
		return
	}

	c.Set("client_user_uuid", akun.UUID)
	c.Set("client_username", akun.Username)
	c.Next()
}

func (h *Handler) IsClientAuthenticated(c *gin.Context) bool {
	_, _, err := h.currentUserFromRequest(c)
	return err == nil
}

func (h *Handler) currentUserFromRequest(c *gin.Context) (*model.Akun, string, error) {
	if h.service == nil {
		return nil, "", errUnauthorized("Database tidak tersedia")
	}

	token, err := h.extractTokenFromRequest(c)
	if err != nil {
		return nil, "", errUnauthorized("Belum login")
	}

	akun, err := h.service.ValidateToken(token)
	if err != nil {
		return nil, "", errUnauthorized("Sesi login tidak valid")
	}

	return akun, token, nil
}

func (h *Handler) extractTokenFromRequest(c *gin.Context) (string, error) {
	if cookieToken, err := c.Cookie(clientAuthCookie); err == nil && strings.TrimSpace(cookieToken) != "" {
		return cookieToken, nil
	}

	authHeader := c.GetHeader("Authorization")
	if strings.TrimSpace(authHeader) == "" {
		return "", errUnauthorized("Token tidak ditemukan")
	}

	token, err := ExtractBearerToken(authHeader)
	if err != nil {
		return "", errUnauthorized(err.Error())
	}
	return token, nil
}

func (h *Handler) setAuthCookie(c *gin.Context, token string) {
	secure := IsSecureRequest(c.GetHeader("X-Forwarded-Proto"), c.Request.TLS != nil)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		clientAuthCookie,
		token,
		CookieMaxAgeSeconds(),
		"/",
		"",
		secure,
		true,
	)
}

func (h *Handler) clearAuthCookie(c *gin.Context) {
	secure := IsSecureRequest(c.GetHeader("X-Forwarded-Proto"), c.Request.TLS != nil)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(clientAuthCookie, "", -1, "/", "", secure, true)
}

type unauthorizedError struct {
	message string
}

func (e unauthorizedError) Error() string {
	return e.message
}

func errUnauthorized(message string) error {
	return unauthorizedError{message: message}
}
