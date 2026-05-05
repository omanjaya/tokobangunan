package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/omanjaya/tokobangunan/internal/domain"
	"github.com/omanjaya/tokobangunan/internal/dto"
	"github.com/omanjaya/tokobangunan/internal/repo"
)

// PrinterTemplateService - business logic template printer per gudang.
type PrinterTemplateService struct {
	repo *repo.PrinterTemplateRepo
}

func NewPrinterTemplateService(r *repo.PrinterTemplateRepo) *PrinterTemplateService {
	return &PrinterTemplateService{repo: r}
}

func (s *PrinterTemplateService) List(ctx context.Context) ([]domain.PrinterTemplate, error) {
	return s.repo.List(ctx)
}

func (s *PrinterTemplateService) Get(ctx context.Context, id int64) (*domain.PrinterTemplate, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *PrinterTemplateService) Create(ctx context.Context, in dto.PrinterTemplateCreateInput) (*domain.PrinterTemplate, error) {
	t := &domain.PrinterTemplate{
		GudangID:     in.GudangID,
		Jenis:        strings.TrimSpace(in.Jenis),
		Nama:         strings.TrimSpace(in.Nama),
		LebarChar:    in.LebarChar,
		PanjangBaris: in.PanjangBaris,
		OffsetX:      in.OffsetX,
		OffsetY:      in.OffsetY,
		Koordinat:    normalizeJSON(in.Koordinat),
		IsDefault:    in.IsDefault,
	}
	if err := t.Validate(); err != nil {
		return nil, err
	}

	exists, err := s.repo.ExistsByName(ctx, t.GudangID, t.Jenis, t.Nama, 0)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, domain.ErrPrinterTemplateDuplikat
	}

	if err := s.repo.Create(ctx, t); err != nil {
		return nil, fmt.Errorf("create printer_template: %w", err)
	}
	if t.IsDefault {
		if err := s.repo.SetDefault(ctx, t.ID); err != nil {
			return nil, fmt.Errorf("set default: %w", err)
		}
	}
	return t, nil
}

func (s *PrinterTemplateService) Update(ctx context.Context, id int64, in dto.PrinterTemplateUpdateInput) (*domain.PrinterTemplate, error) {
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	t.GudangID = in.GudangID
	t.Jenis = strings.TrimSpace(in.Jenis)
	t.Nama = strings.TrimSpace(in.Nama)
	t.LebarChar = in.LebarChar
	t.PanjangBaris = in.PanjangBaris
	t.OffsetX = in.OffsetX
	t.OffsetY = in.OffsetY
	t.Koordinat = normalizeJSON(in.Koordinat)
	t.IsDefault = in.IsDefault

	if err := t.Validate(); err != nil {
		return nil, err
	}

	exists, err := s.repo.ExistsByName(ctx, t.GudangID, t.Jenis, t.Nama, t.ID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, domain.ErrPrinterTemplateDuplikat
	}

	if err := s.repo.Update(ctx, t); err != nil {
		return nil, fmt.Errorf("update printer_template: %w", err)
	}
	if t.IsDefault {
		if err := s.repo.SetDefault(ctx, t.ID); err != nil {
			return nil, fmt.Errorf("set default: %w", err)
		}
	}
	return t, nil
}

func (s *PrinterTemplateService) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

func (s *PrinterTemplateService) SetDefault(ctx context.Context, id int64) error {
	return s.repo.SetDefault(ctx, id)
}

// GeneratePreview render ASCII preview sederhana untuk verify konfigurasi.
// MVP: sekedar tampilkan border kotak dengan ukuran sesuai (lebar_char, panjang_baris)
// + label jenis + nama. Real ESC/P koordinat akan dikembangkan kemudian.
func (s *PrinterTemplateService) GeneratePreview(t *domain.PrinterTemplate) string {
	w := t.LebarChar
	if w <= 0 {
		w = 80
	}
	if w > 120 {
		w = 120
	}
	h := t.PanjangBaris
	if h <= 0 {
		h = 33
	}
	if h > 50 {
		h = 50
	}

	var b strings.Builder
	border := "+" + strings.Repeat("-", w-2) + "+"
	b.WriteString(border + "\n")
	title := fmt.Sprintf(" Template: %s | Jenis: %s ", t.Nama, t.Jenis)
	if len(title) > w-2 {
		title = title[:w-2]
	}
	b.WriteString("|" + padRight(title, w-2) + "|\n")
	info := fmt.Sprintf(" Gudang ID: %d  Lebar: %d  Baris: %d  OffsetX: %d  OffsetY: %d ",
		t.GudangID, t.LebarChar, t.PanjangBaris, t.OffsetX, t.OffsetY)
	if len(info) > w-2 {
		info = info[:w-2]
	}
	b.WriteString("|" + padRight(info, w-2) + "|\n")
	b.WriteString("|" + strings.Repeat(" ", w-2) + "|\n")

	// Render koordinat sebagai daftar key:value (kalau JSON object).
	var koord map[string]any
	if err := json.Unmarshal([]byte(t.Koordinat), &koord); err == nil {
		for k, v := range koord {
			line := fmt.Sprintf(" %s = %v", k, v)
			if len(line) > w-2 {
				line = line[:w-2]
			}
			b.WriteString("|" + padRight(line, w-2) + "|\n")
		}
	}

	// Pad sampai panjang baris.
	used := 4
	if koord != nil {
		used += len(koord)
	}
	for i := used; i < h-1; i++ {
		b.WriteString("|" + strings.Repeat(" ", w-2) + "|\n")
	}
	b.WriteString(border + "\n")
	return b.String()
}

func padRight(s string, w int) string {
	if len(s) >= w {
		return s[:w]
	}
	return s + strings.Repeat(" ", w-len(s))
}

func normalizeJSON(s string) string {
	t := strings.TrimSpace(s)
	if t == "" {
		return "{}"
	}
	// Validasi JSON; bila rusak, fallback ke "{}".
	var any interface{}
	if err := json.Unmarshal([]byte(t), &any); err != nil {
		return "{}"
	}
	return t
}
