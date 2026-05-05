package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/omanjaya/tokobangunan/internal/auth"
	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/dto"
	"github.com/omanjaya/tokobangunan/internal/repo"
	"github.com/omanjaya/tokobangunan/internal/service"
	"github.com/omanjaya/tokobangunan/internal/view/layout"
	penjualanview "github.com/omanjaya/tokobangunan/internal/view/penjualan"
)

// PenjualanHandler - HTTP handler modul penjualan.
type PenjualanHandler struct {
	penjualan  *service.PenjualanService
	produk     *service.ProdukService
	mitra      *service.MitraService
	gudang     *service.GudangService
	satuan     *service.SatuanService
	harga      *service.HargaService
	appSetting *service.AppSettingService
}

// NewPenjualanHandler konstruktor.
// appSetting boleh nil — kalau nil, print fallback ke data gudang.
func NewPenjualanHandler(
	pj *service.PenjualanService,
	pr *service.ProdukService,
	mr *service.MitraService,
	gr *service.GudangService,
	sr *service.SatuanService,
	hr *service.HargaService,
	as *service.AppSettingService,
) *PenjualanHandler {
	return &PenjualanHandler{
		penjualan: pj, produk: pr, mitra: mr, gudang: gr, satuan: sr, harga: hr,
		appSetting: as,
	}
}

// Index GET /penjualan.
func (h *PenjualanHandler) Index(c echo.Context) error {
	ctx := c.Request().Context()

	page, _ := strconv.Atoi(c.QueryParam("page"))
	if page < 1 {
		page = 1
	}
	filter := repo.ListPenjualanFilter{Page: page, PerPage: 25}

	from := strings.TrimSpace(c.QueryParam("from"))
	to := strings.TrimSpace(c.QueryParam("to"))
	if from != "" {
		if t, err := time.Parse("2006-01-02", from); err == nil {
			filter.From = &t
		}
	}
	if to != "" {
		if t, err := time.Parse("2006-01-02", to); err == nil {
			filter.To = &t
		}
	}
	if g, _ := strconv.ParseInt(c.QueryParam("gudang_id"), 10, 64); g > 0 {
		filter.GudangID = &g
	}
	if m, _ := strconv.ParseInt(c.QueryParam("mitra_id"), 10, 64); m > 0 {
		filter.MitraID = &m
	}
	if s := strings.TrimSpace(c.QueryParam("status")); s != "" {
		filter.Status = &s
	}
	filter.Query = strings.TrimSpace(c.QueryParam("q"))

	page1, err := h.penjualan.ListWithRelations(ctx, filter)
	if err != nil {
		return err
	}

	rows := buildRowsFromRelations(page1.Items)

	gudangs, err := h.gudangLite(c)
	if err != nil {
		return err
	}

	gudangID := int64(0)
	if filter.GudangID != nil {
		gudangID = *filter.GudangID
	}
	mitraID := int64(0)
	if filter.MitraID != nil {
		mitraID = *filter.MitraID
	}
	status := ""
	if filter.Status != nil {
		status = *filter.Status
	}

	props := penjualanview.IndexProps{
		Nav:        layout.DefaultNav("/penjualan"),
		User:       penjualanUserData(c),
		Rows:       rows,
		Total:      page1.Total,
		Page:       page1.Page,
		PerPage:    page1.PerPage,
		TotalPages: page1.TotalPages,
		From:       from,
		To:         to,
		GudangID:   gudangID,
		MitraID:    mitraID,
		Status:     status,
		Query:      filter.Query,
		Gudangs:    gudangs,
	}
	return RenderHTML(c, http.StatusOK, penjualanview.Index(props))
}

// New GET /penjualan/baru. Optional query param ?from=<id> untuk prefill
// items + mitra dari penjualan referensi.
func (h *PenjualanHandler) New(c echo.Context) error {
	gudangs, err := h.gudangLite(c)
	if err != nil {
		return err
	}
	defaultGudang := int64(0)
	if u := auth.CurrentUser(c); u != nil && u.GudangID != nil {
		defaultGudang = *u.GudangID
	}

	in := dto.PenjualanCreateInput{
		Tanggal:     time.Now().Format("2006-01-02"),
		GudangID:    defaultGudang,
		StatusBayar: "lunas",
	}
	mitraNama := ""

	// Prefill dari penjualan referensi.
	if fromID, _ := strconv.ParseInt(c.QueryParam("from"), 10, 64); fromID > 0 {
		ctx := c.Request().Context()
		if ref, err := h.penjualan.Get(ctx, fromID); err == nil {
			in.MitraID = ref.MitraID
			in.GudangID = ref.GudangID
			for _, it := range ref.Items {
				in.Items = append(in.Items, dto.PenjualanItemInput{
					ProdukID:    it.ProdukID,
					Qty:         it.Qty,
					SatuanID:    it.SatuanID,
					HargaSatuan: it.HargaSatuan / 100, // back to rupiah utuh
				})
			}
			if m, err := h.mitra.Get(ctx, ref.MitraID); err == nil {
				mitraNama = m.Nama
			}
		}
	}

	ppnAvail, ppnPersen := h.pajakUI(c)
	if ppnAvail {
		in.PPNEnabled = true
	}
	props := penjualanview.FormProps{
		Nav:          layout.DefaultNav("/penjualan"),
		User:         penjualanUserData(c),
		Input:        in,
		Gudangs:      gudangs,
		MitraNama:    mitraNama,
		ClientUUID:   uuid.New().String(),
		PPNAvailable: ppnAvail,
		PPNPersen:    ppnPersen,
	}
	return RenderHTML(c, http.StatusOK, penjualanview.Form(props))
}

// pajakUI - resolusi konfigurasi PPN untuk render form.
func (h *PenjualanHandler) pajakUI(c echo.Context) (bool, float64) {
	if h.appSetting == nil {
		return false, 0
	}
	cfg, err := h.appSetting.PajakConfig(c.Request().Context())
	if err != nil || cfg == nil {
		return false, 0
	}
	return cfg.PPNEnabled, cfg.PPNPersen
}

// Create POST /penjualan.
func (h *PenjualanHandler) Create(c echo.Context) error {
	in, err := bindPenjualanForm(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	u := auth.CurrentUser(c)
	if u == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "user tidak terautentikasi")
	}

	created, err := h.penjualan.Create(c.Request().Context(), u.ID, in)
	if err != nil {
		return h.renderFormError(c, in, err)
	}

	target := "/penjualan/" + strconv.FormatInt(created.ID, 10)
	// "Simpan & Cetak" — redirect langsung ke endpoint PDF dengan flag autoprint.
	if c.FormValue("print") == "1" {
		target = "/penjualan/" + strconv.FormatInt(created.ID, 10) + "/print/pdf?copy=asli&autoprint=1"
	}
	if c.Request().Header.Get("HX-Request") == "true" {
		c.Response().Header().Set("HX-Redirect", target)
		return c.NoContent(http.StatusOK)
	}
	return c.Redirect(http.StatusSeeOther, target)
}

// Show GET /penjualan/:id.
func (h *PenjualanHandler) Show(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	pj, err := h.penjualan.Get(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrPenjualanNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "penjualan tidak ditemukan")
		}
		return err
	}
	mitra, _ := h.mitra.Get(ctx, pj.MitraID)
	gudang, _ := h.gudang.Get(ctx, pj.GudangID)

	props := penjualanview.ShowProps{
		Nav:       layout.DefaultNav("/penjualan"),
		User:      penjualanUserData(c),
		Penjualan: pj,
		Mitra:     mitra,
		Gudang:    gudang,
	}
	return RenderHTML(c, http.StatusOK, penjualanview.Show(props))
}

// SearchMitraJSON GET /penjualan/search-mitra?q=...
func (h *PenjualanHandler) SearchMitraJSON(c echo.Context) error {
	ctx := c.Request().Context()
	q := strings.TrimSpace(c.QueryParam("q"))
	items, err := h.mitra.Search(ctx, q, 10)
	if err != nil {
		return err
	}
	out := make([]map[string]any, 0, len(items))
	for _, m := range items {
		// Outstanding piutang per mitra (cents).
		outstanding, _ := h.penjualan.OutstandingByMitra(ctx, m.ID)
		out = append(out, map[string]any{
			"id":          m.ID,
			"kode":        m.Kode,
			"nama":        m.Nama,
			"tipe":        m.Tipe,
			"limit":       m.LimitKredit,
			"outstanding": outstanding,
		})
	}
	return c.JSON(http.StatusOK, out)
}

// SearchProdukJSON GET /penjualan/search-produk?q=...&gudang_id=&tipe=
func (h *PenjualanHandler) SearchProdukJSON(c echo.Context) error {
	ctx := c.Request().Context()
	q := strings.TrimSpace(c.QueryParam("q"))

	// Resolve gudang dari query, fallback ke gudang user aktif.
	gudangID, _ := strconv.ParseInt(c.QueryParam("gudang_id"), 10, 64)
	if gudangID <= 0 {
		if u := auth.CurrentUser(c); u != nil && u.GudangID != nil {
			gudangID = *u.GudangID
		}
	}

	// Tipe harga (sesuai tipe mitra). Default eceran.
	tipe := strings.TrimSpace(c.QueryParam("tipe"))
	if !domain.IsValidTipeHarga(tipe) {
		tipe = domain.TipeHargaEceran
	}

	items, err := h.produk.Search(ctx, q, 10)
	if err != nil {
		return err
	}
	// Attach default satuan + harga sesuai tipe + stok info.
	out := make([]map[string]any, 0, len(items))
	for _, p := range items {
		satuans := []map[string]any{}
		if s, err := h.satuan.Get(ctx, p.SatuanKecilID); err == nil {
			satuans = append(satuans, map[string]any{
				"id": s.ID, "kode": s.Kode, "nama": s.Nama,
			})
		}
		if p.SatuanBesarID != nil {
			if s, err := h.satuan.Get(ctx, *p.SatuanBesarID); err == nil {
				satuans = append(satuans, map[string]any{
					"id": s.ID, "kode": s.Kode, "nama": s.Nama,
				})
			}
		}
		// Harga sesuai tipe → fallback eceran bila tipe lain tidak ada.
		var hargaDefault int64 // Rupiah utuh untuk form
		var gudangPtr *int64
		if gudangID > 0 {
			gid := gudangID
			gudangPtr = &gid
		}
		if hp, err := h.harga.GetAktif(ctx, p.ID, gudangPtr, tipe); err == nil {
			hargaDefault = hp.HargaJual / 100
		} else if tipe != domain.TipeHargaEceran {
			if hp, err := h.harga.GetAktif(ctx, p.ID, gudangPtr, domain.TipeHargaEceran); err == nil {
				hargaDefault = hp.HargaJual / 100
			}
		}

		// Stok per gudang.
		var stokQty, stokMin float64
		var stokSatuan string
		if gudangID > 0 {
			if info, err := h.penjualan.StokInfoOf(ctx, gudangID, p.ID); err == nil {
				stokQty = info.Qty
				stokMin = info.StokMinimum
			}
		}
		if s, err := h.satuan.Get(ctx, p.SatuanKecilID); err == nil {
			stokSatuan = s.Kode
		}

		out = append(out, map[string]any{
			"id":            p.ID,
			"sku":           p.SKU,
			"nama":          p.Nama,
			"satuans":       satuans,
			"harga_default": hargaDefault,
			"stok":          stokQty,
			"stok_satuan":   stokSatuan,
			"stok_minimum":  stokMin,
		})
	}
	return c.JSON(http.StatusOK, out)
}

// PreviewNomor GET /penjualan/preview-nomor?gudang_id=&tanggal=YYYY-MM-DD
// Hanya dipakai untuk preview di form (read-only, non-allocating).
func (h *PenjualanHandler) PreviewNomor(c echo.Context) error {
	gudangID, _ := strconv.ParseInt(c.QueryParam("gudang_id"), 10, 64)
	tanggalStr := strings.TrimSpace(c.QueryParam("tanggal"))
	if gudangID <= 0 || tanggalStr == "" {
		return c.JSON(http.StatusOK, map[string]any{"nomor_preview": ""})
	}
	tgl, err := time.Parse("2006-01-02", tanggalStr)
	if err != nil {
		return c.JSON(http.StatusOK, map[string]any{"nomor_preview": ""})
	}
	nomor, err := h.penjualan.PreviewNomor(c.Request().Context(), gudangID, tgl)
	if err != nil {
		return c.JSON(http.StatusOK, map[string]any{"nomor_preview": ""})
	}
	return c.JSON(http.StatusOK, map[string]any{"nomor_preview": nomor})
}

// ----- helpers ---------------------------------------------------------------

// buildRowsFromRelations adaptasi PenjualanWithRelations (hasil 1 query JOIN
// di repo) ke penjualanview.Row. Tidak perlu lagi per-row lookup ke
// mitra/gudang service — eliminating N+1.
func buildRowsFromRelations(items []repo.PenjualanWithRelations) []penjualanview.Row {
	if len(items) == 0 {
		return nil
	}
	out := make([]penjualanview.Row, 0, len(items))
	for _, it := range items {
		out = append(out, penjualanview.Row{
			Penjualan:  it.Penjualan,
			MitraNama:  it.MitraNama,
			GudangKode: it.GudangKode,
		})
	}
	return out
}

func (h *PenjualanHandler) gudangLite(c echo.Context) ([]penjualanview.GudangLite, error) {
	list, err := h.gudang.List(c.Request().Context(), false)
	if err != nil {
		return nil, err
	}
	out := make([]penjualanview.GudangLite, 0, len(list))
	for _, g := range list {
		out = append(out, penjualanview.GudangLite{ID: g.ID, Kode: g.Kode, Nama: g.Nama})
	}
	return out, nil
}

func (h *PenjualanHandler) renderFormError(c echo.Context, in dto.PenjualanCreateInput, err error) error {
	gudangs, lerr := h.gudangLite(c)
	if lerr != nil {
		return lerr
	}
	ppnAvail, ppnPersen := h.pajakUI(c)
	props := penjualanview.FormProps{
		Nav:          layout.DefaultNav("/penjualan"),
		User:         penjualanUserData(c),
		Input:        in,
		Gudangs:      gudangs,
		ClientUUID:   in.ClientUUID,
		PPNAvailable: ppnAvail,
		PPNPersen:    ppnPersen,
	}
	if fes, ok := dto.CollectFieldErrors(err); ok {
		props.Errors = fes
	} else {
		props.General = humanizePenjualanError(err)
	}
	if c.Request().Header.Get("HX-Request") == "true" {
		return RenderHTML(c, http.StatusUnprocessableEntity, penjualanview.FormCard(props))
	}
	return RenderHTML(c, http.StatusUnprocessableEntity, penjualanview.Form(props))
}

func humanizePenjualanError(err error) string {
	switch {
	case errors.Is(err, domain.ErrPenjualanKosong):
		return "Tambahkan minimal 1 item penjualan."
	case errors.Is(err, domain.ErrTotalTidakCocok):
		return "Total tidak konsisten dengan item & diskon."
	case errors.Is(err, domain.ErrStatusBayarInvalid):
		return "Status bayar tidak valid."
	case errors.Is(err, domain.ErrJatuhTempoWajib):
		return "Jatuh tempo wajib diisi untuk pembayaran kredit/sebagian."
	case errors.Is(err, domain.ErrLimitKreditTerlampaui):
		return "Total transaksi melebihi limit kredit mitra."
	case errors.Is(err, domain.ErrStokTidakCukup):
		return "Stok tidak mencukupi: " + err.Error()
	case errors.Is(err, domain.ErrItemQtyInvalid):
		return "Qty item harus lebih dari 0."
	case errors.Is(err, domain.ErrItemHargaInvalid):
		return "Harga satuan tidak valid."
	case errors.Is(err, domain.ErrMitraTidakDitemukan):
		return "Mitra tidak ditemukan atau tidak aktif."
	case errors.Is(err, domain.ErrGudangNotFound):
		return "Gudang tidak ditemukan atau tidak aktif."
	case errors.Is(err, domain.ErrProdukNotFound):
		return "Salah satu produk tidak ditemukan."
	default:
		return "Gagal menyimpan penjualan: " + err.Error()
	}
}

// bindPenjualanForm parse form data ke DTO. Items diparse manual karena
// echo Bind tidak menangani array of struct dari form repeated nama "Items[i].X".
func bindPenjualanForm(c echo.Context) (dto.PenjualanCreateInput, error) {
	in := dto.PenjualanCreateInput{
		Tanggal:     strings.TrimSpace(c.FormValue("tanggal")),
		StatusBayar: strings.TrimSpace(c.FormValue("status_bayar")),
		JatuhTempo:  strings.TrimSpace(c.FormValue("jatuh_tempo")),
		Catatan:     strings.TrimSpace(c.FormValue("catatan")),
		ClientUUID:  strings.TrimSpace(c.FormValue("client_uuid")),
	}
	in.MitraID, _ = strconv.ParseInt(c.FormValue("mitra_id"), 10, 64)
	in.GudangID, _ = strconv.ParseInt(c.FormValue("gudang_id"), 10, 64)
	in.Diskon, _ = strconv.ParseInt(c.FormValue("diskon"), 10, 64)
	if v := strings.TrimSpace(c.FormValue("ppn_enabled")); v == "on" || v == "1" || v == "true" {
		in.PPNEnabled = true
	}

	form, err := c.FormParams()
	if err != nil {
		return in, err
	}
	// Kumpulkan key Items[i].* lalu rakit per index.
	type rawItem struct {
		ProdukID    string
		Qty         string
		SatuanID    string
		HargaSatuan string
	}
	indexed := map[int]*rawItem{}
	for k, vals := range form {
		if !strings.HasPrefix(k, "Items[") || len(vals) == 0 {
			continue
		}
		// Items[N].Field
		end := strings.Index(k, "]")
		if end < 0 {
			continue
		}
		idx, perr := strconv.Atoi(k[6:end])
		if perr != nil {
			continue
		}
		field := strings.TrimPrefix(k[end+1:], ".")
		row, ok := indexed[idx]
		if !ok {
			row = &rawItem{}
			indexed[idx] = row
		}
		switch field {
		case "ProdukID":
			row.ProdukID = vals[0]
		case "Qty":
			row.Qty = vals[0]
		case "SatuanID":
			row.SatuanID = vals[0]
		case "HargaSatuan":
			row.HargaSatuan = vals[0]
		}
	}
	// Sort by index ascending.
	maxIdx := -1
	for k := range indexed {
		if k > maxIdx {
			maxIdx = k
		}
	}
	for i := 0; i <= maxIdx; i++ {
		row, ok := indexed[i]
		if !ok {
			continue
		}
		pid, _ := strconv.ParseInt(row.ProdukID, 10, 64)
		if pid <= 0 {
			continue
		}
		qty, _ := strconv.ParseFloat(row.Qty, 64)
		sid, _ := strconv.ParseInt(row.SatuanID, 10, 64)
		harga, _ := strconv.ParseInt(row.HargaSatuan, 10, 64)
		in.Items = append(in.Items, dto.PenjualanItemInput{
			ProdukID:    pid,
			Qty:         qty,
			SatuanID:    sid,
			HargaSatuan: harga,
		})
	}
	return in, nil
}

func penjualanUserData(c echo.Context) layout.UserData {
	u := auth.CurrentUser(c)
	if u == nil {
		return layout.UserData{}
	}
	return layout.UserData{Name: u.NamaLengkap, Role: u.Role}
}
