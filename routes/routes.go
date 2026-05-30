package routes

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"wakirim/middleware"
	"wakirim/module/auth"
	"wakirim/module/datacontact"
	"wakirim/module/payment"
	"wakirim/module/pengaduan"

	"github.com/gin-gonic/gin"
)

const adminSessionCookie = "wakirim_admin_session"

var paymentHandler *payment.Handler
var authHandler *auth.Handler
var pengaduanHandler *pengaduan.Handler
var dataContactHandler *datacontact.Handler

func SetupRouter() *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	router.Use(middleware.CORSMiddleware())
	router.Use(middleware.SecurityHeadersMiddleware())
	RegisterRoutes(router)

	return router
}

func RegisterRoutes(router *gin.Engine) {
	// Initialize payment handler
	paymentHandler = payment.NewHandler()
	authHandler = auth.NewHandler()
	pengaduanHandler = pengaduan.NewHandler()
	dataContactHandler = datacontact.NewHandler()

	clientPageGuard := middleware.ClientPageAuthGuard(authHandler.IsClientAuthenticated)

	router.Static("/assets", "./view/assets")
	router.Static("/page/payment", "./view/page/payment")
	router.GET("/page/admin/*filepath", serveProtectedAdminPage)
	router.GET("/page/client/*filepath", clientPageGuard, serveProtectedClientPage)

	router.GET("/", func(ctx *gin.Context) {
		ctx.File("./view/index.html")
	})

	router.GET("/client", func(ctx *gin.Context) {
		ctx.Redirect(302, "/login")
	})

	router.GET("/login", func(ctx *gin.Context) {
		if authHandler.IsClientAuthenticated(ctx) {
			ctx.Redirect(http.StatusFound, "/dashboard")
			return
		}
		ctx.File("./view/page/login/index.html")
	})

	router.GET("/dashboard", clientPageGuard, func(ctx *gin.Context) {
		ctx.File("./view/page/client/index.html")
	})

	router.GET("/client/dashboard", clientPageGuard, func(ctx *gin.Context) {
		ctx.Redirect(302, "/dashboard")
	})

	router.GET("/admin", serveAdmin)
	router.POST("/admin", handleAdminLogin)
	router.GET("/admin/login", serveAdminLogin)
	router.POST("/admin/login", handleAdminLogin)
	router.POST("/admin/logout", handleAdminLogout)

	router.GET("/payment", func(ctx *gin.Context) {
		ctx.File("./view/page/payment/index.html")
	})

	// Register payment API routes
	paymentHandler.RegisterRoutes(router)
	authHandler.RegisterRoutes(router, middleware.ClientLoginProtectionMiddleware())
	pengaduanHandler.RegisterClientRoutes(router, middleware.ClientLoginProtectionMiddleware(), authHandler.RequireClientAuthPage)
	pengaduanHandler.RegisterAdminRoutes(router, requireAdminSessionMiddleware)
	dataContactHandler.RegisterRoutes(router, middleware.ClientLoginProtectionMiddleware(), authHandler.RequireClientAuthPage)
}

func requireAdminSessionMiddleware(ctx *gin.Context) {
	if !isAdminAuthenticated(ctx) {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Unauthorized admin"})
		return
	}
	ctx.Next()
}

func serveAdmin(ctx *gin.Context) {
	if !isAdminAuthenticated(ctx) {
		ctx.File("./view/page/admin/login.html")
		return
	}

	ctx.File("./view/page/admin/index.html")
}

func serveProtectedAdminPage(ctx *gin.Context) {
	if !isAdminAuthenticated(ctx) {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	requestedFile := strings.TrimPrefix(path.Clean("/"+ctx.Param("filepath")), "/")
	if requestedFile == "" || requestedFile == "." {
		ctx.AbortWithStatus(http.StatusNotFound)
		return
	}

	ctx.File(filepath.Join("view", "page", "admin", filepath.FromSlash(requestedFile)))
}

func serveProtectedClientPage(ctx *gin.Context) {
	requestedFile := strings.TrimPrefix(path.Clean("/"+ctx.Param("filepath")), "/")
	if requestedFile == "" || requestedFile == "." {
		ctx.AbortWithStatus(http.StatusNotFound)
		return
	}

	ctx.File(filepath.Join("view", "page", "client", filepath.FromSlash(requestedFile)))
}

func serveAdminLogin(ctx *gin.Context) {
	if isAdminAuthenticated(ctx) {
		ctx.Redirect(http.StatusFound, "/admin")
		return
	}

	ctx.File("./view/page/admin/login.html")
}

func handleAdminLogin(ctx *gin.Context) {
	username := ctx.PostForm("username")
	password := ctx.PostForm("password")

	if !secureCompare(username, adminUsername()) || !secureCompare(password, adminPassword()) {
		ctx.Redirect(http.StatusFound, "/admin?error=1")
		return
	}

	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie(adminSessionCookie, adminSessionToken(), 60*60*12, "/", "", false, true)
	ctx.File("./view/page/admin/index.html")
}

func handleAdminLogout(ctx *gin.Context) {
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie(adminSessionCookie, "", -1, "/", "", false, true)
	ctx.Redirect(http.StatusFound, "/admin")
}

func isAdminAuthenticated(ctx *gin.Context) bool {
	sessionToken, err := ctx.Cookie(adminSessionCookie)
	if err != nil {
		return false
	}

	return secureCompare(sessionToken, adminSessionToken())
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

func adminSessionToken() string {
	mac := hmac.New(sha256.New, []byte(adminPassword()))
	mac.Write([]byte(adminUsername()))
	return hex.EncodeToString(mac.Sum(nil))
}

func secureCompare(a string, b string) bool {
	if len(a) != len(b) {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
