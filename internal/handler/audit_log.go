package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/omanjaya/tokobangunan/internal/auth"
	"github.com/omanjaya/tokobangunan/internal/repo"
	"github.com/omanjaya/tokobangunan/internal/service"
	auditview "github.com/omanjaya/tokobangunan/internal/view/setting/audit_log"
	"github.com/omanjaya/tokobangunan/internal/view/layout"
)

type AuditLogHandler struct {
	svc *service.AuditLogService
}

func NewAuditLogHandler(svc *service.AuditLogService) *AuditLogHandler {
	return &AuditLogHandler{svc: svc}
}

func (h *AuditLogHandler) buildShell(c echo.Context) (layout.NavData, layout.UserData) {
	user := auth.CurrentUser(c)
	nav := layout.DefaultNav("/setting")
	ud := layout.UserData{}
	if user != nil {
		ud.Name = user.NamaLengkap
		ud.Role = user.Role
	}
	return nav, ud
}

// Index GET /setting/audit-log
func (h *AuditLogHandler) Index(c echo.Context) error {
	ctx := c.Request().Context()
	q := c.QueryParams()

	f := repo.ListAuditFilter{}
	if v := strings.TrimSpace(q.Get("tabel")); v != "" {
		f.Tabel = &v
	}
	if v := strings.TrimSpace(q.Get("aksi")); v != "" {
		f.Aksi = &v
	}
	if v := strings.TrimSpace(q.Get("user_id")); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil && id > 0 {
			f.UserID = &id
		}
	}
	if v := strings.TrimSpace(q.Get("record_id")); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil && id > 0 {
			f.RecordID = &id
		}
	}
	if v := strings.TrimSpace(q.Get("from")); v != "" {
		if t, err := time.ParseInLocation("2006-01-02", v, time.Local); err == nil {
			f.From = &t
		}
	}
	if v := strings.TrimSpace(q.Get("to")); v != "" {
		if t, err := time.ParseInLocation("2006-01-02", v, time.Local); err == nil {
			// inklusif sampai akhir hari → +1 hari (filter pakai <).
			end := t.Add(24 * time.Hour)
			f.To = &end
		}
	}
	page, _ := strconv.Atoi(q.Get("page"))
	f.Page = page
	f.PerPage = 50

	res, err := h.svc.List(ctx, f)
	if err != nil {
		slog.ErrorContext(ctx, "list audit", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "gagal memuat audit log")
	}
	tabels, _ := h.svc.ListTabel(ctx)

	nav, ud := h.buildShell(c)
	return RenderHTML(c, http.StatusOK, auditview.Index(auditview.IndexProps{
		Nav: nav, User: ud,
		Result: res,
		Tabels: tabels,
		Filter: auditview.FilterValues{
			Tabel:    q.Get("tabel"),
			Aksi:     q.Get("aksi"),
			UserID:   q.Get("user_id"),
			RecordID: q.Get("record_id"),
			From:     q.Get("from"),
			To:       q.Get("to"),
		},
	}))
}

// Show GET /setting/audit-log/:id
func (h *AuditLogHandler) Show(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "id tidak valid")
	}
	ctx := c.Request().Context()
	l, err := h.svc.Get(ctx, id)
	if err != nil {
		if errors.Is(err, repo.ErrAuditLogNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "audit log tidak ditemukan")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "gagal memuat audit log")
	}
	nav, ud := h.buildShell(c)
	return RenderHTML(c, http.StatusOK, auditview.Show(auditview.ShowProps{
		Nav: nav, User: ud, Log: l,
	}))
}
