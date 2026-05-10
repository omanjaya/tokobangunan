package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/omanjaya/tokobangunan/internal/audit"
	"github.com/omanjaya/tokobangunan/internal/auth"
	"github.com/omanjaya/tokobangunan/internal/service"
)

// sensitiveFields - field yang isinya di-redact sebelum disimpan ke audit_log.
var sensitiveFields = map[string]struct{}{
	"password":                 {},
	"password_lama":            {},
	"password_baru":            {},
	"password_baru_konfirmasi": {},
	"password_konfirmasi":      {},
	"csrf_token":               {},
	"_csrf":                    {},
}

// skipPrefixes - path yang nggak relevan untuk audit (auth, static, health).
var skipPrefixes = []string{
	"/login", "/logout", "/lupa-password",
	"/healthz", "/sw.js", "/manifest.webmanifest",
	"/static", "/assets", "/favicon",
	"/search",
}

// serviceCoveredPrefixes - path yang sudah di-handle audit di service layer
// (richer before/after JSON). Middleware skip menulis audit untuk prefix ini
// supaya tidak duplikat row di audit_log.
var serviceCoveredPrefixes = []string{
	"/penjualan",
	"/pembelian",
	"/pembayaran",
	"/mutasi",
	"/mitra",
	"/produk",
	"/stok/adjust",
}

// isServiceCovered cek apakah path di-cover oleh service-level audit.
func isServiceCovered(path string) bool {
	for _, p := range serviceCoveredPrefixes {
		if path == p || strings.HasPrefix(path, p+"/") {
			return true
		}
	}
	return false
}

// specialActions - last-segment path yang di-treat sebagai aksi khusus.
var specialActions = map[string]string{
	"submit":          "submit",
	"receive":         "receive",
	"cancel":          "cancel",
	"approve":         "approve",
	"toggle-active":   "toggle_active",
	"reset-password":  "reset_password",
	"test":            "test",
	"bayar":           "bayar",
	"setor":           "setor",
	"tarik":           "tarik",
	"batch":           "create_batch",
	"password":        "change_password",
	"delete":          "delete",
}

const maxBodyBytes = 64 * 1024

// AuditLog middleware mencatat setiap request mutation (POST/PUT/PATCH/DELETE)
// ke tabel audit_log. Resolve action+table dari path; capture body sanitised.
// Untuk UPDATE/DELETE (path mengandung :id), middleware juga capture state
// row sebelum handler dijalankan via audit.FetchBefore (whitelist tabel).
func AuditLog(svc *service.AuditLogService, pool *pgxpool.Pool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Inject AuditMeta ke ctx UNCONDITIONALLY (juga untuk GET, juga untuk
			// path service-covered) supaya service-level audit calls bisa pakai
			// IP/UA/RequestID tanpa thread Echo context manual.
			meta := service.AuditMeta{
				IP:        c.RealIP(),
				UserAgent: c.Request().UserAgent(),
				RequestID: extractRequestID(c),
			}
			if u := auth.CurrentUser(c); u != nil {
				meta.UserID = u.ID
			}
			ctx := service.WithAuditMeta(c.Request().Context(), meta)
			c.SetRequest(c.Request().WithContext(ctx))

			method := c.Request().Method
			if !isMutation(method) {
				return next(c)
			}
			path := c.Path()
			if path == "" {
				path = c.Request().URL.Path
			}
			if shouldSkipAudit(path) {
				return next(c)
			}
			// Skip middleware-level write untuk path yang sudah di-cover service.
			// Service layer menulis entri audit dengan before/after JSON yang
			// lebih kaya; middleware tidak perlu duplikasi.
			if isServiceCovered(path) {
				return next(c)
			}

			// Capture body (terbatas) sebelum handler consume.
			// Skip multipart uploads — buang-buang memory dan bisa break form parsing.
			var bodyBytes []byte
			isMultipart := strings.HasPrefix(c.Request().Header.Get("Content-Type"), "multipart/")
			if c.Request().Body != nil && !isMultipart {
				lr := io.LimitReader(c.Request().Body, maxBodyBytes+1)
				bodyBytes, _ = io.ReadAll(lr)
				c.Request().Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			}

			// Capture row state sebelum handler eksekusi (best-effort).
			// Untuk UPDATE/DELETE: path :id present, tabel masuk whitelist.
			var beforeRaw json.RawMessage
			if pool != nil {
				if tabel, _, recordID := parsePathToAudit(path, method, c); tabel != "" && recordID > 0 && audit.IsAllowed(tabel) {
					if b, err := audit.FetchBefore(c.Request().Context(), pool, tabel, recordID); err == nil && b != nil {
						beforeRaw = b
					}
				}
			}

			err := next(c)

			status := c.Response().Status
			// 2xx dan 3xx (redirect after POST) di-anggap sukses.
			if status < 200 || status >= 400 {
				return err
			}

			user := auth.CurrentUser(c)
			var userID *int64
			if user != nil {
				id := user.ID
				userID = &id
			}

			tabel, aksi, recordID := parsePathToAudit(path, method, c)
			if tabel == "" {
				return err
			}

			var payload any
			if isMultipart {
				payload = "<multipart upload skipped>"
			} else {
				payload = normalizeBody(bodyBytes, c.Request().Header.Get("Content-Type"))
			}

			entry := service.RecordEntry{
				UserID:    userID,
				Aksi:      aksi,
				Tabel:     tabel,
				RecordID:  recordID,
				After:     payload,
				IP:        c.RealIP(),
				UserAgent: c.Request().UserAgent(),
				RequestID: extractRequestID(c),
			}
			if len(beforeRaw) > 0 {
				entry.Before = beforeRaw
			}
			// Best-effort. Failure nggak boleh mengganggu response user.
			_ = svc.Record(c.Request().Context(), entry)

			return err
		}
	}
}

// extractRequestID ambil dari response header (set echomw.RequestID) atau
// fallback ke request header X-Request-Id.
func extractRequestID(c echo.Context) string {
	if v := c.Response().Header().Get(echo.HeaderXRequestID); v != "" {
		return v
	}
	return c.Request().Header.Get(echo.HeaderXRequestID)
}

func isMutation(m string) bool {
	switch m {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	}
	return false
}

func shouldSkipAudit(path string) bool {
	for _, s := range skipPrefixes {
		if strings.HasPrefix(path, s) {
			return true
		}
	}
	return false
}

// parsePathToAudit map route pattern (e.g. "/produk/:id/delete") ke tuple
// (tabel, aksi, recordID).
func parsePathToAudit(path, method string, c echo.Context) (string, string, int64) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return "", "", 0
	}
	// Tabel = first segment (kecuali "setting" / prefix grouping yang umum).
	tabel := parts[0]
	if tabel == "setting" && len(parts) > 1 {
		tabel = parts[1]
	}

	// Default aksi.
	aksi := "create"
	hasID := false
	for _, p := range parts {
		if p == ":id" {
			hasID = true
			break
		}
	}
	if hasID {
		aksi = "update"
	}

	// Cek special action di segmen terakhir.
	last := parts[len(parts)-1]
	if v, ok := specialActions[last]; ok {
		aksi = v
	}
	// DELETE method (REST) — over-ride.
	if method == http.MethodDelete {
		aksi = "delete"
	}

	var recordID int64
	if v := c.Param("id"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			recordID = n
		}
	}
	return tabel, aksi, recordID
}

// normalizeBody parse form-urlencoded -> map; JSON dipassthrough sebagai
// map[string]any kalau bisa di-decode, raw string kalau gagal.
// Return any sehingga service.Record akan json.Marshal-kan.
func normalizeBody(body []byte, contentType string) any {
	if len(body) == 0 {
		return nil
	}
	if len(body) > maxBodyBytes {
		return map[string]any{"_truncated": true, "_size": len(body)}
	}
	ct := strings.ToLower(contentType)
	switch {
	case strings.Contains(ct, "application/x-www-form-urlencoded"),
		strings.Contains(ct, "multipart/form-data"):
		// Parse form-urlencoded. multipart hanya di-handle untuk part text;
		// untuk simplicity, fallback ke ParseQuery (akan gagal pada multipart
		// dan kita simpan size-only).
		if strings.Contains(ct, "multipart/form-data") {
			return map[string]any{"_multipart": true, "_size": len(body)}
		}
		vals, err := url.ParseQuery(string(body))
		if err != nil {
			return map[string]any{"_raw_size": len(body)}
		}
		return sanitizeValues(vals)
	case strings.Contains(ct, "application/json"):
		var decoded any
		if err := json.Unmarshal(body, &decoded); err != nil {
			return map[string]any{"_raw": string(body)}
		}
		if m, ok := decoded.(map[string]any); ok {
			return sanitizeMap(m)
		}
		return decoded
	}
	return map[string]any{"_size": len(body)}
}

// sanitizeValues - flatten url.Values ke map[string]any, redact field sensitif.
func sanitizeValues(v url.Values) map[string]any {
	out := make(map[string]any, len(v))
	for k, vs := range v {
		if isSensitive(k) {
			out[k] = "[REDACTED]"
			continue
		}
		if len(vs) == 1 {
			out[k] = vs[0]
		} else {
			out[k] = vs
		}
	}
	return out
}

// sanitizeMap - recursive redact field sensitif pada map JSON.
func sanitizeMap(m map[string]any) map[string]any {
	for k, v := range m {
		if isSensitive(k) {
			m[k] = "[REDACTED]"
			continue
		}
		if sub, ok := v.(map[string]any); ok {
			m[k] = sanitizeMap(sub)
		}
	}
	return m
}

func isSensitive(k string) bool {
	lk := strings.ToLower(k)
	if _, ok := sensitiveFields[lk]; ok {
		return true
	}
	// Heuristik: apa pun yang mengandung "password" atau "token" di-redact.
	if strings.Contains(lk, "password") || strings.Contains(lk, "token") {
		return true
	}
	return false
}
