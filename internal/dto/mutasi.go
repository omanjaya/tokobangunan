package dto

// MutasiItemInput - satu baris item dalam form mutasi.
type MutasiItemInput struct {
	ProdukID      int64   `validate:"required,min=1"`
	Qty           float64 `validate:"required,gt=0"`
	SatuanID      int64   `validate:"required,min=1"`
	HargaInternal *int64
	Catatan       string
}

// MutasiCreateInput - input untuk membuat mutasi baru (status default draft).
type MutasiCreateInput struct {
	Tanggal        string            `validate:"required,datetime=2006-01-02"`
	GudangAsalID   int64             `validate:"required,min=1"`
	GudangTujuanID int64             `validate:"required,min=1,nefield=GudangAsalID"`
	Items          []MutasiItemInput `validate:"required,min=1,dive"`
	Catatan        string
	ClientUUID     string
	// SubmitNow=true berarti langsung beralih ke status 'dikirim' setelah create.
	SubmitNow bool
}
