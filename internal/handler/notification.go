package handler

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/omanjaya/tokobangunan/internal/auth"
	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/repo"
	"github.com/omanjaya/tokobangunan/internal/service"
	"github.com/omanjaya/tokobangunan/internal/view/components"
)

// NotificationHandler agregasi notifikasi pengguna:
//   - Stok kritis (qty < min)
//   - Piutang jatuh tempo / overdue
//   - Mutasi dikirim menunggu konfirmasi terima (untuk role gudang/staff)
type NotificationHandler struct {
	laporanSvc *service.LaporanService
	piutangSvc *service.PiutangService
	mutasiRepo *repo.MutasiRepo
}

// NewNotificationHandler konstruktor.
//
// Wiring di main.go (registerAccountAndSearchRoutes atau registrasi terpisah):
//
//	notifH := handler.NewNotificationHandler(laporanSvc, piutangSvc, mutasiRepo)
//	app.GET("/notifications", notifH.List)
//	app.GET("/notifications/count", notifH.Count)
func NewNotificationHandler(
	laporanSvc *service.LaporanService,
	piutangSvc *service.PiutangService,
	mutasiRepo *repo.MutasiRepo,
) *NotificationHandler {
	return &NotificationHandler{
		laporanSvc: laporanSvc,
		piutangSvc: piutangSvc,
		mutasiRepo: mutasiRepo,
	}
}

const notifMaxItems = 10

// List GET /notifications - return Templ partial dropdown.
func (h *NotificationHandler) List(c echo.Context) error {
	user := auth.CurrentUser(c)
	items := h.collect(c.Request().Context(), user)
	if len(items) > notifMaxItems {
		items = items[:notifMaxItems]
	}
	return RenderHTML(c, http.StatusOK, components.NotificationDropdown(items))
}

// Count GET /notifications/count - return JSON {count: N}.
func (h *NotificationHandler) Count(c echo.Context) error {
	user := auth.CurrentUser(c)
	items := h.collect(c.Request().Context(), user)
	return c.JSON(http.StatusOK, echo.Map{"count": len(items)})
}

// collect agregat notif. Error per-source di-isolasi (degraded UX).
func (h *NotificationHandler) collect(ctx context.Context, user *auth.User) []components.NotificationItem {
	out := make([]components.NotificationItem, 0, 16)

	// --- Stok kritis ---------------------------------------------------------
	if h.laporanSvc != nil {
		if rows, err := h.laporanSvc.StokKritis(ctx); err == nil {
			for _, r := range rows {
				// Filter by gudang user kalau bukan owner/admin.
				if user != nil && user.Role != "owner" && user.Role != "admin" &&
					user.GudangID != nil && r.GudangID != *user.GudangID {
					continue
				}
				out = append(out, components.NotificationItem{
					Variant: "danger",
					Title:   fmt.Sprintf("Stok kritis: %s", r.ProdukNama),
					Detail: fmt.Sprintf("%s · %s %s (min %s)",
						r.GudangNama,
						components.FormatNumber(r.Qty), r.SatuanKode,
						components.FormatNumber(r.StokMinimum)),
					Href: "/laporan/stok-kritis",
				})
			}
		}
	}

	// --- Piutang jatuh tempo / overdue --------------------------------------
	if h.piutangSvc != nil {
		f := repo.ListPiutangFilter{Page: 1, PerPage: 50}
		if res, err := h.piutangSvc.Summary(ctx, f); err == nil {
			for _, s := range res.Items {
				// Pakai invoice_tertua + asumsi default 7 hari window.
				// Tampilkan kalau aging != current (sudah/akan overdue) ATAU
				// invoice tertua >= 7 hari (mendekati jatuh tempo default).
				show := false
				switch s.Aging {
				case domain.Aging1to30, domain.Aging31to60, domain.Aging61to90, domain.Aging90Plus:
					show = true
				}
				if !show {
					continue
				}
				detail := fmt.Sprintf("%d invoice · %s outstanding",
					s.JumlahInvoice, components.FormatRupiah(s.Outstanding))
				out = append(out, components.NotificationItem{
					Variant: "warning",
					Title:   fmt.Sprintf("Piutang %s: %s", s.Aging, s.MitraNama),
					Detail:  detail,
					Href:    fmt.Sprintf("/piutang/%d", s.MitraID),
					Time:    agingTimeLabel(s.InvoiceTertua),
				})
			}
		}
	}

	// --- Mutasi dikirim menunggu terima -------------------------------------
	if h.mutasiRepo != nil {
		status := string(domain.StatusDikirim)
		f := repo.ListMutasiFilter{
			Status:  &status,
			Page:    1,
			PerPage: 20,
		}
		// Untuk staff/kasir gudang: hanya mutasi yang ditujukan ke gudang user.
		if user != nil && user.Role != "owner" && user.Role != "admin" && user.GudangID != nil {
			gid := *user.GudangID
			f.GudangTujuanID = &gid
		}
		if list, _, err := h.mutasiRepo.List(ctx, f); err == nil {
			for _, m := range list {
				out = append(out, components.NotificationItem{
					Variant: "info",
					Title:   fmt.Sprintf("Mutasi dikirim: %s", m.NomorMutasi),
					Detail:  fmt.Sprintf("%s → %s · menunggu konfirmasi terima", mutasiKode(m.GudangAsalID), mutasiKode(m.GudangTujuanID)),
					Href:    fmt.Sprintf("/mutasi/%d", m.ID),
					Time:    m.Tanggal.Format("02 Jan"),
				})
			}
		}
	}

	// Order: danger -> warning -> info.
	sort.SliceStable(out, func(i, j int) bool {
		return notifVariantRank(out[i].Variant) < notifVariantRank(out[j].Variant)
	})
	return out
}

func notifVariantRank(v string) int {
	switch v {
	case "danger":
		return 0
	case "warning":
		return 1
	case "info":
		return 2
	}
	return 3
}

func agingTimeLabel(t *time.Time) string {
	if t == nil {
		return ""
	}
	return "Sejak " + t.Format("02 Jan")
}

// mutasiKode placeholder (kita tidak resolve nama gudang per item di sini
// untuk hindari N+1; tampilkan ID saja). Bisa diupgrade nanti.
func mutasiKode(id int64) string {
	return fmt.Sprintf("Gudang %d", id)
}
