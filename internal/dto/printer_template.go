package dto

// PrinterTemplateCreateInput - data form create template printer.
type PrinterTemplateCreateInput struct {
	GudangID     int64  `validate:"required,gt=0"`
	Jenis        string `validate:"required,oneof=kwitansi struk label"`
	Nama         string `validate:"required,max=128"`
	LebarChar    int    `validate:"gte=0,lte=512"`
	PanjangBaris int    `validate:"gte=0,lte=200"`
	OffsetX      int    `validate:"gte=-100,lte=500"`
	OffsetY      int    `validate:"gte=-100,lte=500"`
	Koordinat    string // JSON string mentah; default "{}"
	IsDefault    bool
}

// PrinterTemplateUpdateInput - sama dengan create.
type PrinterTemplateUpdateInput struct {
	GudangID     int64  `validate:"required,gt=0"`
	Jenis        string `validate:"required,oneof=kwitansi struk label"`
	Nama         string `validate:"required,max=128"`
	LebarChar    int    `validate:"gte=0,lte=512"`
	PanjangBaris int    `validate:"gte=0,lte=200"`
	OffsetX      int    `validate:"gte=-100,lte=500"`
	OffsetY      int    `validate:"gte=-100,lte=500"`
	Koordinat    string
	IsDefault    bool
}
