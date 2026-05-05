package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/omanjaya/tokobangunan/internal/auth"
	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/dto"
	"github.com/omanjaya/tokobangunan/internal/repo"
	"github.com/omanjaya/tokobangunan/internal/service"
	onboardingview "github.com/omanjaya/tokobangunan/internal/view/onboarding"
)

// OnboardingHandler - wizard 4 step setup awal.
type OnboardingHandler struct {
	appSetting *service.AppSettingService
	gudang     *service.GudangService
	produk     *service.ProdukService
	user       *service.UserAccountService
	satuanRepo *repo.SatuanRepo
	gudangRepo *repo.GudangRepo
}

// NewOnboardingHandler konstruktor.
func NewOnboardingHandler(
	as *service.AppSettingService,
	gs *service.GudangService,
	ps *service.ProdukService,
	us *service.UserAccountService,
	sr *repo.SatuanRepo,
	gr *repo.GudangRepo,
) *OnboardingHandler {
	return &OnboardingHandler{
		appSetting: as, gudang: gs, produk: ps, user: us,
		satuanRepo: sr, gudangRepo: gr,
	}
}

// Index GET /onboarding — redirect ke step1 atau dashboard kalau done.
func (h *OnboardingHandler) Index(c echo.Context) error {
	done, _ := h.appSetting.IsOnboardingDone(c.Request().Context())
	if done {
		return c.Redirect(http.StatusSeeOther, "/dashboard")
	}
	return c.Redirect(http.StatusSeeOther, "/onboarding/step1")
}

// guardDone redirect ke /dashboard kalau onboarding sudah selesai.
// Mencegah user direct-visit /onboarding/stepN setelah marked done.
func (h *OnboardingHandler) guardDone(c echo.Context) error {
	done, _ := h.appSetting.IsOnboardingDone(c.Request().Context())
	if done {
		return c.Redirect(http.StatusSeeOther, "/dashboard")
	}
	return nil
}

// Step1 GET — form info toko.
func (h *OnboardingHandler) Step1(c echo.Context) error {
	if r := h.guardDone(c); r != nil {
		return r
	}
	info, _ := h.appSetting.TokoInfo(c.Request().Context())
	if info == nil {
		info = &domain.TokoInfo{}
	}
	props := onboardingview.Step1Props{
		CSRF:  csrfToken(c),
		Info:  info,
		Steps: makeSteps(1),
	}
	return RenderHTML(c, http.StatusOK, onboardingview.Step1(props))
}

// Step1Submit POST — simpan toko_info, lanjut step2.
func (h *OnboardingHandler) Step1Submit(c echo.Context) error {
	if r := h.guardDone(c); r != nil {
		return r
	}
	info := &domain.TokoInfo{
		Nama:        strings.TrimSpace(c.FormValue("nama")),
		Alamat:      strings.TrimSpace(c.FormValue("alamat")),
		Telepon:     strings.TrimSpace(c.FormValue("telepon")),
		NPWP:        strings.TrimSpace(c.FormValue("npwp")),
		KopKwitansi: strings.TrimSpace(c.FormValue("kop_kwitansi")),
	}
	u := auth.CurrentUser(c)
	var uid *int64
	if u != nil {
		id := u.ID
		uid = &id
	}
	if err := h.appSetting.UpdateTokoInfo(c.Request().Context(), info, uid); err != nil {
		props := onboardingview.Step1Props{
			CSRF: csrfToken(c), Info: info, Error: err.Error(),
			Steps: makeSteps(1),
		}
		return RenderHTML(c, http.StatusUnprocessableEntity, onboardingview.Step1(props))
	}
	return c.Redirect(http.StatusSeeOther, "/onboarding/step2")
}

// Step2 GET — list gudang.
func (h *OnboardingHandler) Step2(c echo.Context) error {
	if r := h.guardDone(c); r != nil {
		return r
	}
	list, err := h.gudang.List(c.Request().Context(), true)
	if err != nil {
		return err
	}
	props := onboardingview.Step2Props{
		CSRF:    csrfToken(c),
		Gudangs: list,
		Steps:   makeSteps(2),
	}
	return RenderHTML(c, http.StatusOK, onboardingview.Step2(props))
}

// Step2Submit POST — bulk update aktif/nonaktif + rename gudang.
func (h *OnboardingHandler) Step2Submit(c echo.Context) error {
	if r := h.guardDone(c); r != nil {
		return r
	}
	ctx := c.Request().Context()
	list, err := h.gudang.List(ctx, true)
	if err != nil {
		return err
	}
	for _, g := range list {
		idStr := strconv.FormatInt(g.ID, 10)
		newNama := strings.TrimSpace(c.FormValue("nama_" + idStr))
		active := c.FormValue("active_"+idStr) == "1"
		alamat := strings.TrimSpace(c.FormValue("alamat_" + idStr))
		telepon := strings.TrimSpace(c.FormValue("telepon_" + idStr))
		if newNama == "" {
			newNama = g.Nama
		}
		_, _ = h.gudang.Update(ctx, g.ID, dto.GudangUpdateInput{
			Kode: g.Kode, Nama: newNama,
			Alamat: alamat, Telepon: telepon,
			IsActive: active,
		})
	}
	// Tambah gudang baru kalau ada.
	newKode := strings.ToUpper(strings.TrimSpace(c.FormValue("new_kode")))
	newNama := strings.TrimSpace(c.FormValue("new_nama"))
	if newKode != "" && newNama != "" {
		_, _ = h.gudang.Create(ctx, dto.GudangCreateInput{
			Kode: newKode, Nama: newNama, IsActive: true,
		})
	}
	return c.Redirect(http.StatusSeeOther, "/onboarding/step3")
}

// Step3 GET — bulk import produk via CSV.
func (h *OnboardingHandler) Step3(c echo.Context) error {
	if r := h.guardDone(c); r != nil {
		return r
	}
	props := onboardingview.Step3Props{
		CSRF:  csrfToken(c),
		Steps: makeSteps(3),
	}
	return RenderHTML(c, http.StatusOK, onboardingview.Step3(props))
}

// Step3Submit POST — parse CSV.
func (h *OnboardingHandler) Step3Submit(c echo.Context) error {
	if r := h.guardDone(c); r != nil {
		return r
	}
	ctx := c.Request().Context()
	if c.FormValue("skip") == "1" {
		return c.Redirect(http.StatusSeeOther, "/onboarding/step4")
	}
	raw := strings.TrimSpace(c.FormValue("csv"))
	res := importProdukCSV(ctx, raw, h.produk, h.satuanRepo)

	props := onboardingview.Step3Props{
		CSRF:     csrfToken(c),
		Steps:    makeSteps(3),
		Imported: res.Imported,
		Failed:   res.Failed,
		ErrMsgs:  res.ErrMsgs,
		Done:     true,
	}
	return RenderHTML(c, http.StatusOK, onboardingview.Step3(props))
}

// Step4 GET — buat user kasir.
func (h *OnboardingHandler) Step4(c echo.Context) error {
	if r := h.guardDone(c); r != nil {
		return r
	}
	list, err := h.gudang.List(c.Request().Context(), false)
	if err != nil {
		return err
	}
	props := onboardingview.Step4Props{
		CSRF:    csrfToken(c),
		Gudangs: list,
		Steps:   makeSteps(4),
	}
	return RenderHTML(c, http.StatusOK, onboardingview.Step4(props))
}

// Step4Submit POST — buat user kasir per gudang.
func (h *OnboardingHandler) Step4Submit(c echo.Context) error {
	if r := h.guardDone(c); r != nil {
		return r
	}
	ctx := c.Request().Context()
	if c.FormValue("skip") == "1" {
		return h.finalize(c)
	}
	list, err := h.gudang.List(ctx, false)
	if err != nil {
		return err
	}
	created := []onboardingview.CreatedKasir{}
	for _, g := range list {
		idStr := strconv.FormatInt(g.ID, 10)
		username := strings.TrimSpace(c.FormValue("username_" + idStr))
		nama := strings.TrimSpace(c.FormValue("nama_" + idStr))
		if username == "" || nama == "" {
			continue
		}
		gid := g.ID
		res, err := h.user.Create(ctx, dto.UserCreateInput{
			Username:    username,
			NamaLengkap: nama,
			Role:        "kasir",
			GudangID:    &gid,
			IsActive:    true,
		})
		if err != nil {
			continue
		}
		created = append(created, onboardingview.CreatedKasir{
			Username:   res.User.Username,
			Nama:       res.User.NamaLengkap,
			GudangNama: g.Nama,
			Password:   res.PlaintextPassword,
		})
	}
	// Lanjut finalize tapi tampilkan password.
	if len(created) > 0 {
		props := onboardingview.Step4Props{
			CSRF: csrfToken(c), Gudangs: list,
			Steps:    makeSteps(4),
			Created:  created,
		}
		return RenderHTML(c, http.StatusOK, onboardingview.Step4(props))
	}
	return h.finalize(c)
}

// Step4Done POST /onboarding/step4/done - mark done dari step4.
func (h *OnboardingHandler) Step4Done(c echo.Context) error {
	if r := h.guardDone(c); r != nil {
		return r
	}
	return h.finalize(c)
}

func (h *OnboardingHandler) finalize(c echo.Context) error {
	u := auth.CurrentUser(c)
	var uid *int64
	if u != nil {
		id := u.ID
		uid = &id
	}
	_ = h.appSetting.MarkOnboardingDone(c.Request().Context(), uid)
	return c.Redirect(http.StatusSeeOther, "/onboarding/done")
}

// Done GET — celebration page.
func (h *OnboardingHandler) Done(c echo.Context) error {
	ctx := c.Request().Context()
	info, _ := h.appSetting.TokoInfo(ctx)
	gudangs, _ := h.gudang.List(ctx, false)
	produkCount := 0
	if list, err := h.produk.List(ctx, repo.ListProdukFilter{Page: 1, PerPage: 1}); err == nil {
		produkCount = list.Total
	}
	props := onboardingview.DoneProps{
		Info:        info,
		GudangCount: countActiveGudang(gudangs),
		ProdukCount: produkCount,
		Steps:       makeSteps(5),
	}
	return RenderHTML(c, http.StatusOK, onboardingview.Done(props))
}

// ----- helpers ---------------------------------------------------------------

func makeSteps(active int) []onboardingview.WizardStep {
	labels := []string{"Info Toko", "Gudang", "Produk", "Kasir", "Selesai"}
	out := make([]onboardingview.WizardStep, len(labels))
	for i, l := range labels {
		out[i] = onboardingview.WizardStep{
			Label:  l,
			Done:   i+1 < active,
			Active: i+1 == active,
		}
	}
	return out
}

func countActiveGudang(gs []domain.Gudang) int {
	n := 0
	for _, g := range gs {
		if g.IsActive {
			n++
		}
	}
	return n
}

func csrfToken(c echo.Context) string {
	if v, ok := c.Get("csrf").(string); ok {
		return v
	}
	return ""
}

