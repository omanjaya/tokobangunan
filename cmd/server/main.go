package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"

	"github.com/omanjaya/tokobangunan/internal/auth"
	"github.com/omanjaya/tokobangunan/internal/config"
	"github.com/omanjaya/tokobangunan/internal/handler"
	appmw "github.com/omanjaya/tokobangunan/internal/middleware"
	"github.com/omanjaya/tokobangunan/internal/repo"
	"github.com/omanjaya/tokobangunan/internal/service"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// .env is optional; production typically injects env vars directly.
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := newLogger(cfg)
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := newPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("init db pool: %w", err)
	}
	defer pool.Close()

	e := newEcho(cfg, logger, pool)

	srvErr := make(chan error, 1)
	go func() {
		addr := ":" + cfg.Port
		logger.Info("server starting", slog.String("addr", addr), slog.String("env", cfg.AppEnv))
		if err := e.Start(addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			srvErr <- err
		}
		close(srvErr)
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case err := <-srvErr:
		if err != nil {
			return fmt.Errorf("server: %w", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	logger.Info("server stopped cleanly")
	return nil
}

func newLogger(cfg *config.Config) *slog.Logger {
	var level slog.Level
	switch strings.ToLower(cfg.LogLevel) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if cfg.LogFormat == "json" || cfg.IsProduction() {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	return slog.New(handler)
}

func newPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("pgxpool parse config: %w", err)
	}
	poolCfg.MaxConns = int32(envInt("DB_MAX_CONNS", 20))
	poolCfg.MinConns = int32(envInt("DB_MIN_CONNS", 2))
	poolCfg.MaxConnLifetime = envDuration("DB_MAX_CONN_LIFETIME", time.Hour)
	poolCfg.MaxConnIdleTime = envDuration("DB_MAX_CONN_IDLE_TIME", 30*time.Minute)

	pool, err := pgxpool.NewWithConfig(pingCtx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("pgxpool new: %w", err)
	}
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pgxpool ping: %w", err)
	}
	return pool, nil
}

func envInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return fallback
}

// perIPRateLimiter builds a memory-store rate limiter keyed by client IP.
// rps=10/min ≈ 0.1666 req/s with a small burst.
func perIPRateLimiter(perMinute int, burst int) echo.MiddlewareFunc {
	store := echomw.NewRateLimiterMemoryStoreWithConfig(echomw.RateLimiterMemoryStoreConfig{
		Rate:      rate.Limit(float64(perMinute) / 60.0),
		Burst:     burst,
		ExpiresIn: 3 * time.Minute,
	})
	return echomw.RateLimiterWithConfig(echomw.RateLimiterConfig{
		Store: store,
		IdentifierExtractor: func(c echo.Context) (string, error) {
			return c.RealIP(), nil
		},
		ErrorHandler: func(c echo.Context, err error) error {
			return c.String(http.StatusForbidden, "rate limit error")
		},
		DenyHandler: func(c echo.Context, identifier string, err error) error {
			return c.String(http.StatusTooManyRequests, "too many requests")
		},
	})
}

func newEcho(cfg *config.Config, logger *slog.Logger, pool *pgxpool.Pool) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.Use(echomw.RequestID())
	e.Use(echomw.Recover())
	e.Use(appmw.RequestLogger(logger))
	e.Use(echomw.BodyLimit("8M"))
	e.Use(echomw.Secure())

	csrfCfg := echomw.DefaultCSRFConfig
	csrfCfg.TokenLookup = "form:csrf_token,header:X-CSRF-Token"
	csrfCfg.CookieName = "_csrf"
	// CookieHTTPOnly intentionally false: JS reads the token from this cookie
	// to inject into htmx headers and dynamic form submissions
	// (double-submit cookie pattern). The cookie is the canonical CSRF secret;
	// XSS protection is handled separately via Echo's Secure() middleware + CSP.
	csrfCfg.CookieHTTPOnly = false
	csrfCfg.CookiePath = "/"
	csrfCfg.CookieSameSite = http.SameSiteLaxMode
	csrfCfg.CookieSecure = cfg.IsProduction()
	csrfCfg.ContextKey = "csrf"
	// Skip CSRF for liveness/readiness probes only. Public portal token endpoints
	// remain protected on POST; only GET-by-design routes naturally bypass via
	// Echo CSRF's safe-method short-circuit.
	csrfCfg.Skipper = func(c echo.Context) bool {
		return c.Path() == "/healthz"
	}
	e.Use(echomw.CSRFWithConfig(csrfCfg))

	e.Static("/static", "web/static")
	e.File("/sw.js", "web/static/js/sw.js")
	e.File("/manifest.webmanifest", "web/static/manifest.webmanifest")
	e.File("/favicon.ico", "web/static/icon/favicon.svg")
	e.File("/favicon.svg", "web/static/icon/favicon.svg")

	e.GET("/healthz", func(c echo.Context) error {
		dbStatus := "ok"
		pingCtx, cancel := context.WithTimeout(c.Request().Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(pingCtx); err != nil {
			dbStatus = "error"
			return c.JSON(http.StatusServiceUnavailable, echo.Map{"status": "degraded", "db": dbStatus})
		}
		return c.JSON(http.StatusOK, echo.Map{"status": "ok", "db": dbStatus})
	})

	e.GET("/", func(c echo.Context) error {
		return c.Redirect(http.StatusSeeOther, "/login")
	})

	authStore := auth.NewStore(pool)
	authHandler := handler.NewAuthHandler(authStore, cfg.IsProduction())

	// Shared repos & services hoisted to avoid duplicate construction across register*.
	gudangRepo := repo.NewGudangRepo(pool)
	mitraRepo := repo.NewMitraRepo(pool)
	piutangRepo := repo.NewPiutangRepo(pool)
	piutangSvc := service.NewPiutangService(piutangRepo, mitraRepo)

	laporanRepo := repo.NewLaporanRepo(pool)
	laporanSvc := service.NewLaporanService(laporanRepo, gudangRepo)
	dashboardHandler := handler.NewDashboardHandler(laporanSvc)
	laporanHandler := handler.NewLaporanHandler(laporanSvc, service.NewGudangService(gudangRepo))

	e.HTTPErrorHandler = handler.NewErrorHandler()

	authExtra := handler.NewAuthExtraHandler()

	// Per-IP rate limiter for POST /login only (60 req/min, burst 30).
	// GET /login excluded — page reload should not be throttled.
	loginLimiter := perIPRateLimiter(60, 30)

	pub := e.Group("")
	pub.Use(auth.RedirectIfAuth(authStore))
	pub.GET("/login", authHandler.ShowLogin)
	pub.POST("/login", authHandler.Login, loginLimiter)
	pub.GET("/lupa-password", authExtra.ShowForgotPassword)

	e.POST("/logout", authHandler.Logout)

	auditRepoMw := repo.NewAuditLogRepo(pool)
	auditSvcMw := service.NewAuditLogService(auditRepoMw)

	// Notification handler deps (reuses hoisted piutangSvc).
	mutasiRepoNotif := repo.NewMutasiRepo(pool)
	notifHandler := handler.NewNotificationHandler(laporanSvc, piutangSvc, mutasiRepoNotif)

	// App setting (toko_info, onboarding flag).
	appSettingRepo := repo.NewAppSettingRepo(pool)
	appSettingSvc := service.NewAppSettingService(appSettingRepo)

	app := e.Group("")
	app.Use(auth.RequireAuth(authStore))
	app.Use(appmw.AuditLog(auditSvcMw, pool))
	app.Use(appmw.RequireOnboarding(appSettingSvc))
	app.GET("/dashboard", dashboardHandler.Index)
	app.GET("/notifications", notifHandler.List)
	app.GET("/notifications/count", notifHandler.Count)

	// Onboarding wizard (owner-only).
	produkRepoOnb := repo.NewProdukRepo(pool)
	satuanRepoOnb := repo.NewSatuanRepo(pool)
	userAcctRepoOnb := repo.NewUserAccountRepo(pool)
	onboardingH := handler.NewOnboardingHandler(
		appSettingSvc,
		service.NewGudangService(gudangRepo),
		service.NewProdukService(produkRepoOnb, satuanRepoOnb),
		service.NewUserAccountService(userAcctRepoOnb),
		satuanRepoOnb,
		gudangRepo,
	)
	onb := app.Group("/onboarding", auth.RequireRole("owner"))
	onb.GET("", onboardingH.Index)
	onb.GET("/step1", onboardingH.Step1)
	onb.POST("/step1", onboardingH.Step1Submit)
	onb.GET("/step2", onboardingH.Step2)
	onb.POST("/step2", onboardingH.Step2Submit)
	onb.GET("/step3", onboardingH.Step3)
	onb.POST("/step3", onboardingH.Step3Submit)
	onb.GET("/step4", onboardingH.Step4)
	onb.POST("/step4", onboardingH.Step4Submit)
	onb.POST("/step4/done", onboardingH.Step4Done)
	onb.GET("/done", onboardingH.Done)

	sharedRepos := sharedDeps{
		gudangRepo: gudangRepo,
		mitraRepo:  mitraRepo,
		piutangRepo: piutangRepo,
		piutangSvc: piutangSvc,
	}

	registerMasterRoutes(app, pool)
	registerPartnerRoutes(app, pool, sharedRepos)
	registerSettingRoutes(app, pool, appSettingSvc, sharedRepos)
	registerTransactionRoutes(app, pool, appSettingSvc, sharedRepos)
	registerCollectionRoutes(app, pool, sharedRepos)
	registerAccountAndSearchRoutes(app, pool, sharedRepos)

	// Cashflow + Pajak laporan integration
	cashflowRepo := repo.NewCashflowRepo(pool)
	cashflowSvc := service.NewCashflowService(cashflowRepo)
	gudangSvcCash := service.NewGudangService(gudangRepo)
	cashflowH := handler.NewCashflowHandler(cashflowSvc, gudangSvcCash)
	handler.RegisterCashflowRoutes(app, cashflowH)
	laporanHandler.SetCashflow(cashflowSvc)

	handler.RegisterLaporanRoutes(app, laporanHandler)

	// Share PDF + Portal mitra (mixed: app group + public)
	shareDeps := handler.BuildShareDeps(pool, appSettingSvc, cfg.SessionSecret, piutangSvc, mitraRepo, gudangRepo)
	handler.RegisterShareAndPortalRoutes(app, e, shareDeps, perIPRateLimiter(60, 30))

	return e
}

// sharedDeps holds repos/services constructed once in newEcho and threaded into register*.
type sharedDeps struct {
	gudangRepo  *repo.GudangRepo
	mitraRepo   *repo.MitraRepo
	piutangRepo *repo.PiutangRepo
	piutangSvc  *service.PiutangService
}

func registerAccountAndSearchRoutes(g *echo.Group, pool *pgxpool.Pool, sd sharedDeps) {
	userAcctRepo := repo.NewUserAccountRepo(pool)
	gudangRepo := sd.gudangRepo
	produkRepo := repo.NewProdukRepo(pool)
	satuanRepo := repo.NewSatuanRepo(pool)
	mitraRepo := sd.mitraRepo
	penjualanRepo := repo.NewPenjualanRepo(pool)
	auditRepo := repo.NewAuditLogRepo(pool)
	printerRepo := repo.NewPrinterTemplateRepo(pool)

	userAcctSvc := service.NewUserAccountService(userAcctRepo)
	produkSvc := service.NewProdukService(produkRepo, satuanRepo)
	mitraSvc := service.NewMitraService(mitraRepo)
	penjualanSvc := service.NewPenjualanService(penjualanRepo, produkRepo, mitraRepo, gudangRepo, satuanRepo, sd.piutangRepo)
	auditSvc := service.NewAuditLogService(auditRepo)
	gudangSvc := service.NewGudangService(gudangRepo)
	printerSvc := service.NewPrinterTemplateService(printerRepo)

	profileH := handler.NewProfileHandler(userAcctSvc, userAcctRepo, gudangRepo)
	auditH := handler.NewAuditLogHandler(auditSvc)
	helpH := handler.NewHelpHandler()
	searchH := handler.NewSearchHandler(produkSvc, mitraSvc, penjualanSvc)
	printerH := handler.NewPrinterTemplateHandler(printerSvc, gudangSvc)

	handler.RegisterAccountRoutes(g, profileH, auditH, helpH)
	handler.RegisterSearchRoutes(g, searchH)

	// Printer template (role-locked)
	printer := g.Group("/setting/printer", auth.RequireRole("owner", "admin"))
	printer.GET("", printerH.Index)
	printer.GET("/baru", printerH.New)
	printer.POST("", printerH.Create)
	printer.GET("/:id/edit", printerH.Edit)
	printer.POST("/:id", printerH.Update)
	printer.POST("/:id/delete", printerH.Delete)
	printer.POST("/:id/test", printerH.Test)
}

func registerCollectionRoutes(g *echo.Group, pool *pgxpool.Pool, sd sharedDeps) {
	mitraRepo := sd.mitraRepo
	penjualanRepo := repo.NewPenjualanRepo(pool)
	pembayaranRepo := repo.NewPembayaranRepo(pool)
	piutangRepo := sd.piutangRepo
	tabunganRepo := repo.NewTabunganMitraRepo(pool)

	mitraSvc := service.NewMitraService(mitraRepo)
	piutangSvc := sd.piutangSvc
	pembayaranSvc := service.NewPembayaranService(pembayaranRepo, penjualanRepo, mitraRepo, piutangRepo)
	tabunganSvc := service.NewTabunganService(tabunganRepo, mitraRepo)

	pembayaranH := handler.NewPembayaranHandler(pembayaranSvc, mitraSvc, piutangSvc)
	piutangH := handler.NewPiutangHandler(piutangSvc)
	tabunganH := handler.NewTabunganHandler(tabunganSvc, mitraSvc)

	handler.RegisterCollectionRoutes(g, pembayaranH, piutangH, tabunganH)
}

func registerTransactionRoutes(g *echo.Group, pool *pgxpool.Pool, appSettingSvc *service.AppSettingService, sd sharedDeps) {
	produkRepo := repo.NewProdukRepo(pool)
	satuanRepo := repo.NewSatuanRepo(pool)
	hargaRepo := repo.NewHargaRepo(pool)
	mitraRepo := sd.mitraRepo
	supplierRepo := repo.NewSupplierRepo(pool)
	gudangRepo := sd.gudangRepo

	produkSvc := service.NewProdukService(produkRepo, satuanRepo)
	satuanSvc := service.NewSatuanService(satuanRepo)
	hargaSvc := service.NewHargaService(hargaRepo, produkRepo)
	mitraSvc := service.NewMitraService(mitraRepo)
	supplierSvc := service.NewSupplierService(supplierRepo)
	gudangSvc := service.NewGudangService(gudangRepo)

	// Penjualan
	penjualanRepo := repo.NewPenjualanRepo(pool)
	penjualanSvc := service.NewPenjualanService(penjualanRepo, produkRepo, mitraRepo, gudangRepo, satuanRepo, sd.piutangRepo)
	penjualanSvc.SetAppSetting(appSettingSvc)
	penjualanHandler := handler.NewPenjualanHandler(penjualanSvc, produkSvc, mitraSvc, gudangSvc, satuanSvc, hargaSvc, appSettingSvc)
	handler.RegisterPenjualanRoutes(g, penjualanHandler)

	// Mutasi + Stok
	mutasiRepo := repo.NewMutasiRepo(pool)
	stokRepo := repo.NewStokRepo(pool)
	mutasiSvc := service.NewMutasiService(mutasiRepo, stokRepo, produkRepo, gudangRepo, satuanRepo)
	stokSvc := service.NewStokService(stokRepo, produkRepo, gudangRepo)
	mutasiH := handler.NewMutasiHandler(mutasiSvc, gudangSvc, produkSvc, satuanSvc)
	stokH := handler.NewStokHandler(stokSvc, produkSvc, gudangSvc)
	handler.RegisterInventoryRoutes(g, mutasiH, stokH)

	// Pembelian + Opname
	pembelianRepo := repo.NewPembelianRepo(pool)
	bayarRepo := repo.NewPembayaranSupplierRepo(pool)
	opnameRepo := repo.NewStokOpnameRepo(pool)
	pembelianSvc := service.NewPembelianService(pembelianRepo, bayarRepo, supplierRepo, produkRepo, gudangRepo, satuanRepo)
	opnameSvc := service.NewStokOpnameService(opnameRepo, gudangRepo)
	pembelianH := handler.NewPembelianHandler(pembelianSvc, supplierSvc, produkSvc, gudangRepo, satuanRepo, supplierRepo)
	opnameH := handler.NewStokOpnameHandler(opnameSvc, gudangRepo)
	handler.RegisterProcurementRoutes(g, pembelianH, opnameH)
}

func registerMasterRoutes(g *echo.Group, pool *pgxpool.Pool) {
	produkRepo := repo.NewProdukRepo(pool)
	satuanRepo := repo.NewSatuanRepo(pool)
	hargaRepo := repo.NewHargaRepo(pool)

	produkSvc := service.NewProdukService(produkRepo, satuanRepo)
	satuanSvc := service.NewSatuanService(satuanRepo)
	hargaSvc := service.NewHargaService(hargaRepo, produkRepo)

	ph := handler.NewProdukHandler(produkSvc, satuanSvc, hargaSvc)
	sh := handler.NewSatuanHandler(satuanSvc)
	hh := handler.NewHargaHandler(produkSvc, hargaSvc)
	pfh := handler.NewProdukFotoHandler(produkSvc)
	plh := handler.NewProdukLabelHandler(produkSvc, hargaSvc)

	handler.RegisterMasterRoutes(g, ph, sh, hh, pfh, plh)
}

func registerPartnerRoutes(g *echo.Group, pool *pgxpool.Pool, sd sharedDeps) {
	mitraRepo := sd.mitraRepo
	supplierRepo := repo.NewSupplierRepo(pool)

	mitraSvc := service.NewMitraService(mitraRepo)
	supplierSvc := service.NewSupplierService(supplierRepo)

	mh := handler.NewMitraHandler(mitraSvc)
	sh := handler.NewSupplierHandler(supplierSvc)

	handler.RegisterPartnerRoutes(g, mh, sh)
}

func registerSettingRoutes(g *echo.Group, pool *pgxpool.Pool, appSettingSvc *service.AppSettingService, sd sharedDeps) {
	gudangRepo := sd.gudangRepo
	userAcctRepo := repo.NewUserAccountRepo(pool)

	gudangSvc := service.NewGudangService(gudangRepo)
	userAcctSvc := service.NewUserAccountService(userAcctRepo)

	gh := handler.NewGudangHandler(gudangSvc)
	uh := handler.NewUserAccountHandler(userAcctSvc, gudangRepo)

	handler.RegisterSettingRoutes(g, gh, uh, gudangRepo)

	// Backup & Restore (owner only)
	backupH := handler.NewBackupHandler()
	bk := g.Group("/setting/backup", auth.RequireRole("owner"))
	bk.GET("", backupH.Index)
	bk.POST("/run", backupH.Trigger)
	bk.GET("/:filename/download", backupH.Download)
	bk.POST("/:filename/restore", backupH.Restore)

	// Pajak setting (owner+admin)
	pajakH := handler.NewSettingPajakHandler(appSettingSvc)
	sp := g.Group("/setting/pajak", auth.RequireRole("owner", "admin"))
	sp.GET("", pajakH.Show)
	sp.POST("", pajakH.Update)

	// SMTP setting (owner only)
	smtpH := handler.NewSettingSMTPHandler(appSettingSvc)
	sm := g.Group("/setting/smtp", auth.RequireRole("owner"))
	sm.GET("", smtpH.Show)
	sm.POST("", smtpH.Update)
	sm.POST("/test", smtpH.Test)
}
