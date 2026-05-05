// Package onboarding berisi view templ untuk wizard setup awal.
package onboarding

import (
	"github.com/omanjaya/tokobangunan/internal/domain"
)

// WizardStep - 1 step di stepper progress.
type WizardStep struct {
	Label  string
	Done   bool
	Active bool
}

// Step1Props - form info toko.
type Step1Props struct {
	CSRF  string
	Info  *domain.TokoInfo
	Error string
	Steps []WizardStep
}

// Step2Props - list gudang.
type Step2Props struct {
	CSRF    string
	Gudangs []domain.Gudang
	Steps   []WizardStep
}

// Step3Props - bulk import produk.
type Step3Props struct {
	CSRF     string
	Steps    []WizardStep
	Imported int
	Failed   int
	ErrMsgs  []string
	Done     bool
}

// CreatedKasir - hasil create user kasir (password sekali tampil).
type CreatedKasir struct {
	Username   string
	Nama       string
	GudangNama string
	Password   string
}

// Step4Props - form user kasir per gudang.
type Step4Props struct {
	CSRF    string
	Gudangs []domain.Gudang
	Steps   []WizardStep
	Created []CreatedKasir
}

// DoneProps - halaman selesai.
type DoneProps struct {
	Info        *domain.TokoInfo
	GudangCount int
	ProdukCount int
	Steps       []WizardStep
}
