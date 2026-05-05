package handler

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/omanjaya/tokobangunan/internal/auth"
	"github.com/omanjaya/tokobangunan/internal/repo"
	"github.com/omanjaya/tokobangunan/internal/service"
)

// ShareDeps - dependency bundle untuk RegisterShareAndPortalRoutes.
type ShareDeps struct {
	AppSetting    *service.AppSettingService
	Penjualan     *service.PenjualanService
	Mitra         *service.MitraService
	Gudang        *service.GudangService
	Piutang       *service.PiutangService
	Pembayaran    *service.PembayaranService
	MitraAccess   *service.MitraAccessService
	SessionSecret string
}

// RegisterShareAndPortalRoutes mendaftarkan:
// - SMTP setting (owner only)
// - Penjualan email/WhatsApp/share-link (di group `app` yang ber-auth)
// - Mitra access-link (owner only)
// - Public portal & shared PDF (di root echo, no auth)
//
// Caller harus pass:
// - app: group yang sudah pakai RequireAuth + RequireOnboarding.
// - root: *echo.Echo untuk public routes.
func RegisterShareAndPortalRoutes(
	app *echo.Group,
	root *echo.Echo,
	deps ShareDeps,
	portalRateLimit echo.MiddlewareFunc,
) {
	// SMTP setting is registered in registerSettingRoutes (single home).

	// Penjualan email & share.
	emailH := NewPenjualanEmailHandler(deps.Penjualan, deps.Mitra, deps.Gudang, deps.AppSetting)
	shareH := NewPenjualanShareHandler(deps.Penjualan, deps.Mitra, deps.Gudang, deps.AppSetting, deps.SessionSecret)
	app.POST("/penjualan/:id/email", emailH.EmailKwitansi)
	app.GET("/penjualan/:id/wa-link", shareH.WhatsAppLink)
	app.POST("/penjualan/:id/share-link", shareH.GenerateShareLink)

	// Mitra access link (owner-side).
	macH := NewMitraAccessHandler(deps.MitraAccess)
	app.POST("/mitra/:id/access-link", macH.CreateLink, auth.RequireRole("owner"))
	app.POST("/mitra/:id/access-link/:tokenID/revoke", macH.RevokeLink, auth.RequireRole("owner"))
	app.GET("/mitra/:id/access-tokens", macH.ListLinks, auth.RequireRole("owner"))

	// Public routes (no auth).
	portalH := NewPortalHandler(deps.MitraAccess, deps.Mitra, deps.Gudang, deps.Penjualan,
		deps.Piutang, deps.Pembayaran, deps.AppSetting)
	root.GET("/share/penjualan", shareH.SharePDF)
	if portalRateLimit != nil {
		root.GET("/portal/:token", portalH.Show, portalRateLimit)
		root.GET("/portal/:token/penjualan/:id/pdf", portalH.DownloadPDF, portalRateLimit)
	} else {
		root.GET("/portal/:token", portalH.Show)
		root.GET("/portal/:token/penjualan/:id/pdf", portalH.DownloadPDF)
	}
}

// BuildShareDeps - factory helper untuk dependencies, dipakai dari main.go.
// Mengurangi duplikasi konstruksi service. piutangSvc/mitraRepo/gudangRepo dihoist dari main.
func BuildShareDeps(
	pool *pgxpool.Pool,
	appSettingSvc *service.AppSettingService,
	sessionSecret string,
	piutangSvc *service.PiutangService,
	mitraRepo *repo.MitraRepo,
	gudangRepo *repo.GudangRepo,
) ShareDeps {
	produkRepo := repo.NewProdukRepo(pool)
	satuanRepo := repo.NewSatuanRepo(pool)
	penjualanRepo := repo.NewPenjualanRepo(pool)
	piutangRepo := repo.NewPiutangRepo(pool)
	pembayaranRepo := repo.NewPembayaranRepo(pool)
	mitraAccessRepo := repo.NewMitraAccessRepo(pool)

	mitraSvc := service.NewMitraService(mitraRepo)
	gudangSvc := service.NewGudangService(gudangRepo)
	penjualanSvc := service.NewPenjualanService(penjualanRepo, produkRepo, mitraRepo, gudangRepo, satuanRepo, piutangRepo)
	pembayaranSvc := service.NewPembayaranService(pool, pembayaranRepo, penjualanRepo, mitraRepo, piutangRepo)
	mitraAccessSvc := service.NewMitraAccessService(mitraAccessRepo, mitraRepo)

	return ShareDeps{
		AppSetting:    appSettingSvc,
		Penjualan:     penjualanSvc,
		Mitra:         mitraSvc,
		Gudang:        gudangSvc,
		Piutang:       piutangSvc,
		Pembayaran:    pembayaranSvc,
		MitraAccess:   mitraAccessSvc,
		SessionSecret: sessionSecret,
	}
}
