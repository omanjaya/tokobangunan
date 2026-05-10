package main

import (
	"bytes"
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/time/rate"

	"github.com/omanjaya/tokobangunan/internal/auth"
	"github.com/omanjaya/tokobangunan/internal/config"
	"github.com/omanjaya/tokobangunan/internal/handler"
	appmw "github.com/omanjaya/tokobangunan/internal/middleware"
	"github.com/omanjaya/tokobangunan/internal/repo"
	"github.com/omanjaya/tokobangunan/internal/service"
)

var (
	httpReqTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP requests by method and status",
	}, []string{"method", "status"})
	httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request duration",
		Buckets: prometheus.DefBuckets,
	}, []string{"method"})
	pgxAcquired = promauto.NewGauge(prometheus.GaugeOpts{Name: "pgxpool_acquired_conns"})
	pgxIdle     = promauto.NewGauge(prometheus.GaugeOpts{Name: "pgxpool_idle_conns"})
	pgxMax      = promauto.NewGauge(prometheus.GaugeOpts{Name: "pgxpool_max_conns"})
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
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	go func() {
		logger.Info("server starting", slog.String("addr", srv.Addr), slog.String("env", cfg.AppEnv))
		if err := e.StartServer(srv); err != nil && !errors.Is(err, http.ErrServerClosed) {
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

	shutdownCtx, cancel := context.WithTimeout(context.Background(), envDuration("SHUTDOWN_TIMEOUT", 30*time.Second))
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
	e.Use(echomw.RecoverWithConfig(echomw.RecoverConfig{
		StackSize:         4 << 10,
		DisableStackAll:   true,
		DisablePrintStack: false,
		LogErrorFunc: func(c echo.Context, err error, stack []byte) error {
			logger.Error("panic recovered",
				slog.String("path", c.Request().URL.Path),
				slog.String("method", c.Request().Method),
				slog.Any("error", err),
				slog.String("stack", string(stack)))
			return err
		},
	}))
	e.Use(appmw.RequestLogger(logger))
	e.Use(echomw.BodyLimit("8M"))
	if cfg.IsProduction() {
		e.Use(echomw.SecureWithConfig(echomw.SecureConfig{
			XSSProtection:      "1; mode=block",
			ContentTypeNosniff: "nosniff",
			XFrameOptions:      "DENY",
			HSTSMaxAge:         31536000,
			HSTSPreloadEnabled: true,
			// Content-Security-Policy is set per-request by appmw.CSP below
			// so that script-src can carry a fresh nonce on every response.
		}))
	} else {
		e.Use(echomw.Secure())
	}
	// Per-request CSP with script nonce. Must run BEFORE handlers render so
	// the nonce is available in the request context.Context for templ views.
	e.Use(appmw.CSP(cfg.IsProduction()))

	// Global per-IP rate limit (300/min, burst 100). Skip ops + static.
	e.Use(perIPRateLimiterWithSkipper(300, 100, func(c echo.Context) bool {
		p := c.Request().URL.Path
		return strings.HasPrefix(p, "/static/") || p == "/healthz" || p == "/livez" || p == "/readyz" || p == "/metrics" || p == "/sw.js" || p == "/manifest.webmanifest" || strings.HasPrefix(p, "/favicon")
	}))

	// Static asset Cache-Control. Long-cache for /static/*, no-cache for sw.js.
	e.Use(staticCacheControl())

	// Prometheus instrumentation: count + duration.
	e.Use(promMiddleware())

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
		p := c.Path()
		return p == "/healthz" || p == "/livez" || p == "/readyz" || p == "/metrics"
	}
	e.Use(echomw.CSRFWithConfig(csrfCfg))

	e.Static("/static", "web/static")
	e.GET("/sw.js", swHandler)
	e.File("/manifest.webmanifest", "web/static/manifest.webmanifest")
	e.File("/favicon.ico", "web/static/icon/favicon.svg")
	e.File("/favicon.svg", "web/static/icon/favicon.svg")

	// Prometheus /metrics — basic-auth gated kalau env METRICS_USER/PASS di-set,
	// kalau kosong → loopback only.
	e.GET("/metrics", echo.WrapHandler(promhttp.Handler()), metricsAuth(pool))

	e.GET("/livez", func(c echo.Context) error {
		return c.JSON(http.StatusOK, echo.Map{"status": "ok"})
	})
	readyzHandler := func(c echo.Context) error {
		dbStatus := "ok"
		pingCtx, cancel := context.WithTimeout(c.Request().Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(pingCtx); err != nil {
			dbStatus = "error"
			return c.JSON(http.StatusServiceUnavailable, echo.Map{"status": "degraded", "db": dbStatus})
		}
		return c.JSON(http.StatusOK, echo.Map{"status": "ok", "db": dbStatus})
	}
	e.GET("/readyz", readyzHandler)
	e.GET("/healthz", readyzHandler) // back-compat

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
	app.GET("/dashboard/section/stok-kritis", dashboardHandler.SectionStokKritis)
	app.GET("/dashboard/section/recent-trx", dashboardHandler.SectionRecentTrx)
	app.GET("/dashboard/section/recent-pembayaran", dashboardHandler.SectionRecentPembayaran)
	app.GET("/dashboard/section/recent-mutasi", dashboardHandler.SectionRecentMutasi)
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
		gudangRepo:  gudangRepo,
		mitraRepo:   mitraRepo,
		piutangRepo: piutangRepo,
		piutangSvc:  piutangSvc,
		auditSvc:    auditSvcMw,
		authStore:   authStore,
	}

	registerMasterRoutes(app, pool, sharedRepos)
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

	forecastRepo := repo.NewForecastRepo(pool)
	forecastSvc := service.NewForecastService(forecastRepo)
	forecastHandler := handler.NewForecastHandler(forecastSvc, service.NewGudangService(gudangRepo))
	handler.RegisterLaporanRoutes(app, laporanHandler, forecastHandler)

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
	auditSvc    *service.AuditLogService
	authStore   *auth.Store
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
	produkSvc.SetAudit(sd.auditSvc)
	mitraSvc := service.NewMitraService(mitraRepo)
	mitraSvc.SetAudit(sd.auditSvc)
	penjualanSvc := service.NewPenjualanService(penjualanRepo, produkRepo, mitraRepo, gudangRepo, satuanRepo, sd.piutangRepo)
	penjualanSvc.SetAudit(sd.auditSvc)
	auditSvc := service.NewAuditLogService(auditRepo)
	gudangSvc := service.NewGudangService(gudangRepo)
	printerSvc := service.NewPrinterTemplateService(printerRepo)

	profileH := handler.NewProfileHandler(userAcctSvc, userAcctRepo, gudangRepo, sd.authStore)
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
	pembayaranSvc := service.NewPembayaranService(pool, pembayaranRepo, penjualanRepo, mitraRepo, piutangRepo)
	pembayaranSvc.SetAudit(sd.auditSvc)
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
	produkSvc.SetAudit(sd.auditSvc)
	satuanSvc := service.NewSatuanService(satuanRepo)
	hargaSvc := service.NewHargaService(hargaRepo, produkRepo)
	mitraSvc := service.NewMitraService(mitraRepo)
	mitraSvc.SetAudit(sd.auditSvc)
	supplierSvc := service.NewSupplierService(supplierRepo)
	gudangSvc := service.NewGudangService(gudangRepo)

	// Penjualan
	penjualanRepo := repo.NewPenjualanRepo(pool)
	penjualanSvc := service.NewPenjualanService(penjualanRepo, produkRepo, mitraRepo, gudangRepo, satuanRepo, sd.piutangRepo)
	penjualanSvc.SetAppSetting(appSettingSvc)
	penjualanSvc.SetAudit(sd.auditSvc)
	penjualanHandler := handler.NewPenjualanHandler(penjualanSvc, produkSvc, mitraSvc, gudangSvc, satuanSvc, hargaSvc, appSettingSvc)
	handler.RegisterPenjualanRoutes(g, penjualanHandler)

	// Retur Penjualan
	returRepo := repo.NewReturPenjualanRepo(pool)
	returSvc := service.NewReturPenjualanService(returRepo, penjualanRepo, produkRepo, satuanRepo, gudangRepo)
	returSvc.SetAudit(sd.auditSvc)
	returHandler := handler.NewReturPenjualanHandler(returSvc, penjualanSvc, mitraSvc)
	handler.RegisterReturRoutes(g, returHandler)

	// Mutasi + Stok
	mutasiRepo := repo.NewMutasiRepo(pool)
	stokRepo := repo.NewStokRepo(pool)
	mutasiSvc := service.NewMutasiService(mutasiRepo, stokRepo, produkRepo, gudangRepo, satuanRepo)
	mutasiSvc.SetAudit(sd.auditSvc)
	stokSvc := service.NewStokService(stokRepo, produkRepo, gudangRepo)
	mutasiH := handler.NewMutasiHandler(mutasiSvc, gudangSvc, produkSvc, satuanSvc)
	stokH := handler.NewStokHandler(stokSvc, produkSvc, gudangSvc)

	// Stok Adjustment (penyesuaian stok 1-step).
	adjRepo := repo.NewAdjRepo(pool)
	adjSvc := service.NewStokAdjustmentService(pool, adjRepo, produkRepo, satuanRepo, sd.auditSvc)
	adjH := handler.NewStokAdjustmentHandler(adjSvc, gudangSvc, produkSvc, satuanSvc)

	handler.RegisterInventoryRoutes(g, mutasiH, stokH, adjH)

	// Pembelian + Opname
	pembelianRepo := repo.NewPembelianRepo(pool)
	bayarRepo := repo.NewPembayaranSupplierRepo(pool)
	opnameRepo := repo.NewStokOpnameRepo(pool)
	pembelianSvc := service.NewPembelianService(pembelianRepo, bayarRepo, supplierRepo, produkRepo, gudangRepo, satuanRepo)
	pembelianSvc.SetAudit(sd.auditSvc)
	opnameSvc := service.NewStokOpnameService(opnameRepo, gudangRepo)
	pembelianH := handler.NewPembelianHandler(pembelianSvc, supplierSvc, produkSvc, gudangRepo, satuanRepo, supplierRepo)
	opnameH := handler.NewStokOpnameHandler(opnameSvc, gudangRepo)
	handler.RegisterProcurementRoutes(g, pembelianH, opnameH)
}

func registerMasterRoutes(g *echo.Group, pool *pgxpool.Pool, sd sharedDeps) {
	produkRepo := repo.NewProdukRepo(pool)
	satuanRepo := repo.NewSatuanRepo(pool)
	hargaRepo := repo.NewHargaRepo(pool)

	produkSvc := service.NewProdukService(produkRepo, satuanRepo)
	produkSvc.SetAudit(sd.auditSvc)
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
	mitraSvc.SetAudit(sd.auditSvc)
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
	backupH := handler.NewBackupHandler(sd.authStore)
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

	// Master Diskon (owner+admin)
	diskonRepo := repo.NewDiskonMasterRepo(pool)
	diskonSvc := service.NewDiskonMasterService(diskonRepo)
	diskonH := handler.NewDiskonMasterHandler(diskonSvc)
	dk := g.Group("/setting/diskon", auth.RequireRole("owner", "admin"))
	dk.GET("", diskonH.Index)
	dk.GET("/baru", diskonH.New)
	dk.POST("", diskonH.Create)
	dk.GET("/:id/edit", diskonH.Edit)
	dk.POST("/:id", diskonH.Update)
	dk.POST("/:id/toggle", diskonH.Toggle)
	dk.POST("/:id/delete", diskonH.Delete)

	// JSON endpoint utk POS (semua user authenticated boleh akses).
	g.GET("/penjualan/diskon-applicable", diskonH.Applicable)
}


// perIPRateLimiterWithSkipper - rate limiter dgn skipper.
func perIPRateLimiterWithSkipper(perMinute, burst int, skip func(echo.Context) bool) echo.MiddlewareFunc {
	store := echomw.NewRateLimiterMemoryStoreWithConfig(echomw.RateLimiterMemoryStoreConfig{
		Rate: rate.Limit(float64(perMinute) / 60.0), Burst: burst, ExpiresIn: 3 * time.Minute,
	})
	return echomw.RateLimiterWithConfig(echomw.RateLimiterConfig{
		Skipper: skip,
		Store:   store,
		IdentifierExtractor: func(c echo.Context) (string, error) { return c.RealIP(), nil },
		ErrorHandler: func(c echo.Context, err error) error {
			return c.String(http.StatusForbidden, "rate limit error")
		},
		DenyHandler: func(c echo.Context, identifier string, err error) error {
			return c.String(http.StatusTooManyRequests, "too many requests")
		},
	})
}

// staticCacheControl - long cache untuk /static/*, no-cache untuk /sw.js.
func staticCacheControl() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			p := c.Request().URL.Path
			h := c.Response().Header()
			switch {
			case p == "/sw.js":
				h.Set("Cache-Control", "no-cache, must-revalidate")
			case strings.HasPrefix(p, "/static/"):
				h.Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			return next(c)
		}
	}
}

// promMiddleware - count requests + measure duration.
func promMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)
			method := c.Request().Method
			status := strconv.Itoa(c.Response().Status)
			httpReqTotal.WithLabelValues(method, status).Inc()
			httpDuration.WithLabelValues(method).Observe(time.Since(start).Seconds())
			return err
		}
	}
}

// metricsAuth - basic auth via env METRICS_USER/PASS, kalau kosong allow loopback only.
func metricsAuth(pool *pgxpool.Pool) echo.MiddlewareFunc {
	user := os.Getenv("METRICS_USER")
	pass := os.Getenv("METRICS_PASS")
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Refresh pool stats on each scrape.
			st := pool.Stat()
			pgxAcquired.Set(float64(st.AcquiredConns()))
			pgxIdle.Set(float64(st.IdleConns()))
			pgxMax.Set(float64(st.MaxConns()))
			if user != "" && pass != "" {
				u, p, ok := c.Request().BasicAuth()
				if !ok || subtle.ConstantTimeCompare([]byte(u), []byte(user)) != 1 || subtle.ConstantTimeCompare([]byte(p), []byte(pass)) != 1 {
					c.Response().Header().Set("WWW-Authenticate", `Basic realm="metrics"`)
					return c.NoContent(http.StatusUnauthorized)
				}
			} else {
				ip := c.RealIP()
				if ip != "127.0.0.1" && ip != "::1" && !strings.HasPrefix(ip, "172.") && !strings.HasPrefix(ip, "10.") && !strings.HasPrefix(ip, "192.168.") {
					return c.NoContent(http.StatusForbidden)
				}
			}
			return next(c)
		}
	}
}

// swHandler - serve sw.js dgn replace placeholder __BUILD_SHA__ runtime.
var swCache struct {
	once sync.Once
	body []byte
	err  error
}

func swHandler(c echo.Context) error {
	swCache.once.Do(func() {
		raw, err := os.ReadFile("web/static/js/sw.js")
		if err != nil {
			swCache.err = err
			return
		}
		sha := os.Getenv("BUILD_SHA")
		if sha == "" {
			sha = time.Now().Format("20060102150405")
		}
		swCache.body = bytes.ReplaceAll(raw, []byte("__BUILD_SHA__"), []byte(sha))
	})
	if swCache.err != nil {
		return swCache.err
	}
	c.Response().Header().Set("Cache-Control", "no-cache, must-revalidate")
	c.Response().Header().Set("Service-Worker-Allowed", "/")
	return c.Blob(http.StatusOK, "application/javascript; charset=utf-8", swCache.body)
}

