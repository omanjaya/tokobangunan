// Package service berisi business logic / use case orchestration. Service
// memanggil repository, melakukan validasi domain, mengembalikan DTO/entity.
package service

// PageResult adalah hasil paginasi standar. Generic agar reusable lintas modul.
type PageResult[T any] struct {
	Items      []T
	Total      int
	Page       int
	PerPage    int
	TotalPages int
}

// NewPageResult membantu compute TotalPages konsisten.
func NewPageResult[T any](items []T, total, page, perPage int) PageResult[T] {
	if perPage <= 0 {
		perPage = 25
	}
	totalPages := total / perPage
	if total%perPage != 0 {
		totalPages++
	}
	if page < 1 {
		page = 1
	}
	return PageResult[T]{
		Items:      items,
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
	}
}
