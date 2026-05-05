// Package printerview berisi templ component untuk modul setting/printer.
package printerview

import (
	"strconv"

	"github.com/omanjaya/tokobangunan/internal/domain"
)

func idToStr(id int64) string { return strconv.FormatInt(id, 10) }

func intToStr(v int) string { return strconv.Itoa(v) }

// gudangNameByID lookup nama gudang dari slice. Return "(?)" bila tidak ketemu.
func gudangNameByID(gudangs []domain.Gudang, id int64) string {
	for i := range gudangs {
		if gudangs[i].ID == id {
			return gudangs[i].Nama
		}
	}
	return "(?)"
}

// groupByGudang mengembalikan slice unik gudang_id yang muncul di items,
// urut sesuai urutan items (sudah ORDER BY gudang_id ASC dari repo).
func groupByGudang(items []domain.PrinterTemplate) []int64 {
	seen := make(map[int64]bool, 8)
	out := make([]int64, 0, 8)
	for _, t := range items {
		if !seen[t.GudangID] {
			seen[t.GudangID] = true
			out = append(out, t.GudangID)
		}
	}
	return out
}

// itemsForGudang return items milik gudangID.
func itemsForGudang(items []domain.PrinterTemplate, gudangID int64) []domain.PrinterTemplate {
	out := make([]domain.PrinterTemplate, 0, 4)
	for _, t := range items {
		if t.GudangID == gudangID {
			out = append(out, t)
		}
	}
	return out
}

func jenisLabel(j string) string {
	switch j {
	case "kwitansi":
		return "Kwitansi"
	case "struk":
		return "Struk"
	case "label":
		return "Label"
	}
	return j
}
