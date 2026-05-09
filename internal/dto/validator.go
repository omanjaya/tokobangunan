// Package dto berisi struct input/output handler. Pisah dari entity domain
// supaya validator tag dan binding tidak bocor ke layer dalam.
package dto

import (
	"errors"
	"strings"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

// Validate menjalankan struct validation berdasarkan tag `validate:"..."`.
func Validate(s any) error {
	return validate.Struct(s)
}

// FieldErrors memetakan nama field (lowercase) → pesan error human-friendly.
type FieldErrors map[string]string

// Error mengimplementasikan interface error sehingga FieldErrors bisa
// di-return langsung dari service. Pesan ringkas: "validasi gagal".
func (f FieldErrors) Error() string {
	if len(f) == 0 {
		return ""
	}
	return "validasi gagal"
}

// CollectFieldErrors konversi error validator menjadi map per field.
// Return (nil, false) bila err bukan validator.ValidationErrors.
func CollectFieldErrors(err error) (FieldErrors, bool) {
	if err == nil {
		return nil, true
	}
	var verr validator.ValidationErrors
	if !errors.As(err, &verr) {
		return nil, false
	}
	out := FieldErrors{}
	for _, fe := range verr {
		out[strings.ToLower(fe.Field())] = humanize(fe)
	}
	return out, true
}

func humanize(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "Wajib diisi."
	case "min":
		return "Nilai minimum: " + fe.Param() + "."
	case "max":
		return "Nilai maksimum: " + fe.Param() + "."
	case "gt":
		return "Harus lebih besar dari " + fe.Param() + "."
	case "gte":
		return "Harus minimal " + fe.Param() + "."
	case "lte":
		return "Maksimal " + fe.Param() + "."
	case "oneof":
		return "Pilihan tidak valid."
	case "email":
		return "Format email tidak valid."
	default:
		return "Nilai tidak valid."
	}
}
