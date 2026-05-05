package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/omanjaya/tokobangunan/internal/auth"
	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/dto"
	"github.com/omanjaya/tokobangunan/internal/repo"
	"github.com/omanjaya/tokobangunan/internal/service"
	"github.com/omanjaya/tokobangunan/internal/view/layout"
	profileview "github.com/omanjaya/tokobangunan/internal/view/profile"
)

// ProfileHandler - self-service profil user yang sedang login.
// Tidak memerlukan service baru — reuse UserAccountService + UserAccountRepo.
type ProfileHandler struct {
	svc        *service.UserAccountService
	acctRepo   *repo.UserAccountRepo
	gudangRepo *repo.GudangRepo
}

func NewProfileHandler(svc *service.UserAccountService, acctRepo *repo.UserAccountRepo, gudangRepo *repo.GudangRepo) *ProfileHandler {
	return &ProfileHandler{svc: svc, acctRepo: acctRepo, gudangRepo: gudangRepo}
}

// passwordRule - aturan password Fase 1 (selaras docs/08-security.md).
const profileMinPasswordLen = 10

func (h *ProfileHandler) buildShell(c echo.Context, active string) (layout.NavData, layout.UserData) {
	user := auth.CurrentUser(c)
	nav := layout.DefaultNav(active)
	ud := layout.UserData{}
	if user != nil {
		ud.Name = user.NamaLengkap
		ud.Role = user.Role
	}
	return nav, ud
}

// gudangNama - resolve nama gudang user kalau ada.
func (h *ProfileHandler) gudangNama(c echo.Context, gudangID *int64) string {
	if gudangID == nil {
		return "Semua cabang"
	}
	g, err := h.gudangRepo.GetByID(c.Request().Context(), *gudangID)
	if err != nil || g == nil {
		return "—"
	}
	return g.Nama
}

// Show GET /profil
func (h *ProfileHandler) Show(c echo.Context) error {
	cur := auth.CurrentUser(c)
	if cur == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}
	acct, err := h.svc.Get(c.Request().Context(), cur.ID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "gagal memuat profil")
	}
	csrf, _ := c.Get("csrf").(string)
	nav, ud := h.buildShell(c, "/profil")
	return RenderHTML(c, http.StatusOK, profileview.Show(profileview.ShowProps{
		Nav: nav, User: ud,
		Account:    acct,
		GudangNama: h.gudangNama(c, acct.GudangID),
		CSRFToken:  csrf,
	}))
}

// Update POST /profil
func (h *ProfileHandler) Update(c echo.Context) error {
	cur := auth.CurrentUser(c)
	if cur == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}
	in := dto.ProfileUpdateInput{
		NamaLengkap: strings.TrimSpace(c.FormValue("nama_lengkap")),
		Email:       strings.TrimSpace(c.FormValue("email")),
	}
	csrf, _ := c.Get("csrf").(string)
	nav, ud := h.buildShell(c, "/profil")

	render := func(status int, errMsg, okMsg string, acct *domain.UserAccount) error {
		return RenderHTML(c, status, profileview.Show(profileview.ShowProps{
			Nav: nav, User: ud,
			Account: acct,
			GudangNama: h.gudangNama(c, acct.GudangID),
			CSRFToken: csrf,
			ErrorMsg: errMsg, SuccessMsg: okMsg,
			Form: in,
		}))
	}

	acct, err := h.svc.Get(c.Request().Context(), cur.ID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "gagal memuat profil")
	}

	if err := dto.Validate(in); err != nil {
		return render(http.StatusUnprocessableEntity, err.Error(), "", acct)
	}

	// Update via repo — tidak ubah username/role/gudang/is_active.
	acct.NamaLengkap = in.NamaLengkap
	if in.Email == "" {
		acct.Email = nil
	} else {
		e := in.Email
		acct.Email = &e
	}
	if err := h.acctRepo.Update(c.Request().Context(), acct); err != nil {
		slog.ErrorContext(c.Request().Context(), "profile update", "error", err)
		return render(http.StatusInternalServerError, "Gagal menyimpan profil. Silakan coba lagi.", "", acct)
	}
	return render(http.StatusOK, "", "Profil berhasil diperbarui.", acct)
}

// ShowChangePassword GET /profil/password
func (h *ProfileHandler) ShowChangePassword(c echo.Context) error {
	cur := auth.CurrentUser(c)
	if cur == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}
	csrf, _ := c.Get("csrf").(string)
	nav, ud := h.buildShell(c, "/profil")
	return RenderHTML(c, http.StatusOK, profileview.ChangePassword(profileview.ChangePasswordProps{
		Nav: nav, User: ud, CSRFToken: csrf,
		MinLength: profileMinPasswordLen,
	}))
}

// ChangePassword POST /profil/password
func (h *ProfileHandler) ChangePassword(c echo.Context) error {
	cur := auth.CurrentUser(c)
	if cur == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}
	csrf, _ := c.Get("csrf").(string)
	nav, ud := h.buildShell(c, "/profil")

	oldPwd := c.FormValue("old_password")
	newPwd := c.FormValue("new_password")
	confirm := c.FormValue("confirm_password")

	render := func(status int, errMsg, okMsg string) error {
		return RenderHTML(c, status, profileview.ChangePassword(profileview.ChangePasswordProps{
			Nav: nav, User: ud, CSRFToken: csrf,
			MinLength: profileMinPasswordLen,
			ErrorMsg: errMsg, SuccessMsg: okMsg,
		}))
	}

	if oldPwd == "" || newPwd == "" || confirm == "" {
		return render(http.StatusUnprocessableEntity, "Semua field wajib diisi.", "")
	}
	if newPwd != confirm {
		return render(http.StatusUnprocessableEntity, "Konfirmasi password tidak cocok.", "")
	}
	if !isStrongPassword(newPwd, profileMinPasswordLen) {
		return render(http.StatusUnprocessableEntity,
			"Password baru harus minimal 10 karakter dan mengandung huruf besar, huruf kecil, dan angka.", "")
	}
	if newPwd == oldPwd {
		return render(http.StatusUnprocessableEntity, "Password baru tidak boleh sama dengan password lama.", "")
	}

	if err := h.svc.ChangePassword(c.Request().Context(), cur.ID, oldPwd, newPwd); err != nil {
		switch {
		case errors.Is(err, domain.ErrUserPasswordSalah):
			return render(http.StatusUnprocessableEntity, "Password lama salah.", "")
		case errors.Is(err, domain.ErrUserPasswordLemah):
			return render(http.StatusUnprocessableEntity, "Password terlalu lemah.", "")
		default:
			slog.ErrorContext(c.Request().Context(), "change password", "error", err)
			return render(http.StatusInternalServerError, "Gagal mengubah password. Silakan coba lagi.", "")
		}
	}
	return render(http.StatusOK, "", "Password berhasil diubah.")
}

// isStrongPassword - cek aturan minimum: panjang + huruf besar + huruf kecil + angka.
func isStrongPassword(pwd string, minLen int) bool {
	if len(pwd) < minLen {
		return false
	}
	var hasUpper, hasLower, hasDigit bool
	for _, r := range pwd {
		switch {
		case r >= 'A' && r <= 'Z':
			hasUpper = true
		case r >= 'a' && r <= 'z':
			hasLower = true
		case r >= '0' && r <= '9':
			hasDigit = true
		}
	}
	return hasUpper && hasLower && hasDigit
}
