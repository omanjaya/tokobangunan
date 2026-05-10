//go:build integration

package handler_test

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"testing"
)

// baseURL dapat di-override dengan env TEST_BASE_URL (default localhost:7777).
// App harus running. Ex: TEST_BASE_URL=http://localhost:8080 go test -tags=integration ...
var baseURL = func() string {
	if v := os.Getenv("TEST_BASE_URL"); v != "" {
		return v
	}
	return "http://localhost:7777"
}()

// envOr mengambil env atau pakai default. Untuk kredensial seed.
func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func mustParseURL(s string) *url.URL { u, _ := url.Parse(s); return u }

// noRedirectClient build http.Client dgn cookie jar dan TANPA auto-follow redirect.
// Penting: login flow return 303, dashboard guard juga 303.
func noRedirectClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar: %v", err)
	}
	return &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// login melakukan flow GET /login (ambil cookie _csrf) -> POST /login.
// Skip test bila app tidak running.
func login(t *testing.T) *http.Client {
	t.Helper()
	client := noRedirectClient(t)

	resp, err := client.Get(baseURL + "/login")
	if err != nil {
		t.Skipf("app tidak running di %s: %v", baseURL, err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	var csrf string
	for _, c := range client.Jar.Cookies(mustParseURL(baseURL)) {
		if c.Name == "_csrf" {
			csrf = c.Value
			break
		}
	}
	if csrf == "" {
		t.Fatal("no _csrf cookie set after GET /login")
	}

	form := url.Values{
		"username":   {envOr("TEST_USER", "owner")},
		"password":   {envOr("TEST_PASS", "admin123")},
		"csrf_token": {csrf},
	}
	req, _ := http.NewRequest("POST", baseURL+"/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("post login: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusFound {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("login expected 303/302, got %d. Body: %s", resp.StatusCode, truncate(string(body), 300))
	}
	return client
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

// getStatus helper: GET path lewat client, return status + body.
func getStatus(t *testing.T, client *http.Client, path string) (int, []byte, http.Header) {
	t.Helper()
	resp, err := client.Get(baseURL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, body, resp.Header
}

// ----- Health & metrics (no auth) -----

func TestHealthEndpoints(t *testing.T) {
	for _, p := range []string{"/livez", "/readyz", "/healthz"} {
		resp, err := http.Get(baseURL + p)
		if err != nil {
			t.Skipf("app tidak running: %v", err)
		}
		if resp.StatusCode != 200 {
			t.Errorf("%s expected 200, got %d", p, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

func TestMetricsRequireAuth(t *testing.T) {
	// /metrics dilindungi metricsAuth — boleh 200 (jika mode dev open) atau 401.
	resp, err := http.Get(baseURL + "/metrics")
	if err != nil {
		t.Skipf("app tidak running: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 401 {
		t.Errorf("/metrics expected 200/401, got %d", resp.StatusCode)
	}
}

// ----- Auth flow -----

func TestUnauthorizedDashboardRedirects(t *testing.T) {
	client := noRedirectClient(t)
	resp, err := client.Get(baseURL + "/dashboard")
	if err != nil {
		t.Skipf("app tidak running: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusFound {
		t.Errorf("expected 303/302 redirect ke /login, got %d", resp.StatusCode)
	}
}

func TestLoginPageRenders(t *testing.T) {
	resp, err := http.Get(baseURL + "/login")
	if err != nil {
		t.Skipf("app tidak running: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("/login expected 200, got %d", resp.StatusCode)
	}
}

func TestLoginThenLogout(t *testing.T) {
	client := login(t)
	// confirm session works
	st, _, _ := getStatus(t, client, "/dashboard")
	if st != 200 {
		t.Errorf("post-login /dashboard expected 200, got %d", st)
	}

	// CSRF token utk POST /logout
	var csrf string
	for _, c := range client.Jar.Cookies(mustParseURL(baseURL)) {
		if c.Name == "_csrf" {
			csrf = c.Value
		}
	}
	form := url.Values{"csrf_token": {csrf}}
	req, _ := http.NewRequest("POST", baseURL+"/logout", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("logout: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusFound {
		t.Errorf("logout expected 303/302, got %d", resp.StatusCode)
	}
}

func TestCSRFRejectsPOSTWithoutToken(t *testing.T) {
	resp, err := http.Post(baseURL+"/penjualan", "application/x-www-form-urlencoded", strings.NewReader(""))
	if err != nil {
		t.Skipf("app tidak running: %v", err)
	}
	defer resp.Body.Close()
	// Echo CSRF middleware return 400, atau auth redirect 303 jika auth dicek dulu.
	if resp.StatusCode != 400 && resp.StatusCode != 403 && resp.StatusCode != http.StatusSeeOther {
		t.Errorf("expected 400/403/303, got %d", resp.StatusCode)
	}
}

// ----- Sidebar / app routes (auth required) -----

func TestDashboard(t *testing.T) {
	client := login(t)
	st, body, _ := getStatus(t, client, "/dashboard")
	if st != 200 {
		t.Errorf("expected 200, got %d", st)
	}
	if !strings.Contains(strings.ToLower(string(body)), "dashboard") {
		t.Error("body missing 'dashboard'")
	}
}

func TestPenjualanList(t *testing.T) {
	client := login(t)
	st, _, _ := getStatus(t, client, "/penjualan/list")
	if st != 200 {
		t.Errorf("expected 200, got %d", st)
	}
}

func TestPenjualanPOS(t *testing.T) {
	client := login(t)
	st, _, _ := getStatus(t, client, "/penjualan")
	if st != 200 {
		t.Errorf("expected 200, got %d", st)
	}
}

func TestStokIndex(t *testing.T) {
	client := login(t)
	st, _, _ := getStatus(t, client, "/stok")
	// /stok bisa redirect 303 ke detail gudang default, atau langsung 200.
	if st != 200 && st != http.StatusSeeOther {
		t.Errorf("expected 200/303, got %d", st)
	}
}

func TestMutasiIndex(t *testing.T) {
	client := login(t)
	st, _, _ := getStatus(t, client, "/mutasi")
	if st != 200 {
		t.Errorf("expected 200, got %d", st)
	}
}

// ----- Role-locked: Stok adjust (owner/admin) -----

func TestStokAdjust(t *testing.T) {
	client := login(t)
	st, _, _ := getStatus(t, client, "/stok/adjust")
	// owner role bisa akses 200; non-admin akan 403.
	if st != 200 && st != 403 {
		t.Errorf("expected 200/403, got %d", st)
	}
}

// ----- JSON endpoints -----

func TestSearchProdukJSON(t *testing.T) {
	client := login(t)
	st, body, _ := getStatus(t, client, "/penjualan/search-produk?q=&limit=5")
	if st != 200 {
		t.Errorf("expected 200, got %d", st)
	}
	// Response berupa JSON array — minimal harus diawali '[' atau berisi '"id"' bila ada data.
	s := strings.TrimSpace(string(body))
	if !strings.HasPrefix(s, "[") && !strings.HasPrefix(s, "{") {
		t.Errorf("expected JSON, got: %s", truncate(s, 200))
	}
}

func TestSearchMitraJSON(t *testing.T) {
	client := login(t)
	st, body, _ := getStatus(t, client, "/penjualan/search-mitra?q=&limit=5")
	if st != 200 {
		t.Errorf("expected 200, got %d", st)
	}
	s := strings.TrimSpace(string(body))
	if !strings.HasPrefix(s, "[") && !strings.HasPrefix(s, "{") {
		t.Errorf("expected JSON, got: %s", truncate(s, 200))
	}
}

func TestDiskonApplicableJSON(t *testing.T) {
	client := login(t)
	st, _, _ := getStatus(t, client, "/penjualan/diskon-applicable?subtotal=100000&mitra_id=0")
	// 200 (kosong) atau 400 (validasi) — endpoint exist.
	if st != 200 && st != 400 {
		t.Errorf("expected 200/400, got %d", st)
	}
}

// ----- Laporan & export -----

func TestLaporanPenjualan(t *testing.T) {
	client := login(t)
	st, _, _ := getStatus(t, client, "/laporan/penjualan?from=2025-11-01&to=2025-11-30")
	if st != 200 {
		t.Errorf("expected 200, got %d", st)
	}
}

func TestExportPenjualanPDF(t *testing.T) {
	client := login(t)
	st, _, hdr := getStatus(t, client, "/laporan/penjualan/export.pdf?from=2025-11-01&to=2025-11-30")
	if st != 200 {
		t.Errorf("expected 200, got %d", st)
	}
	ct := hdr.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/pdf") {
		t.Errorf("expected PDF content-type, got %q", ct)
	}
}

func TestExportPenjualanXLSX(t *testing.T) {
	client := login(t)
	st, _, hdr := getStatus(t, client, "/laporan/penjualan/export.xlsx?from=2025-11-01&to=2025-11-30")
	if st != 200 {
		t.Errorf("expected 200, got %d", st)
	}
	ct := hdr.Get("Content-Type")
	// xlsx => application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
	if !strings.Contains(ct, "spreadsheet") && !strings.Contains(ct, "octet-stream") {
		t.Errorf("expected spreadsheet content-type, got %q", ct)
	}
}

// ----- Notifications & search -----

func TestNotificationsCount(t *testing.T) {
	client := login(t)
	st, body, _ := getStatus(t, client, "/notifications/count")
	if st != 200 {
		t.Errorf("expected 200, got %d", st)
	}
	if len(body) == 0 {
		t.Error("expected non-empty body")
	}
}
