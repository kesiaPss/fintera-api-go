package handlers

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sjperalta/fintera-api/internal/repository"
)

// ParseListQuery standardizes the extraction of common pagination, search, and sorting
// parameters from the request query string.
func ParseListQuery(c *gin.Context) *repository.ListQuery {
	query := repository.NewListQuery()

	query.Page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	query.PerPage, _ = strconv.Atoi(c.DefaultQuery("per_page", "20"))

	// Support both search and search_term
	if search := c.Query("search_term"); search != "" {
		query.Search = search
	} else if search := c.Query("search"); search != "" {
		query.Search = search
	}

	// Parse sort parameter (format: field-direction, e.g. name-asc)
	if sort := c.Query("sort"); sort != "" && sort != "No Sort" {
		parts := strings.Split(sort, "-")
		query.SortBy = parts[0]
		if len(parts) > 1 {
			query.SortDir = parts[1]
		}
	}

	return query
}

const maxCommissionPerPage = 20

// parseCommissionPage reads a 1-based page and a page size from the query. The
// page size defaults to and is capped at maxCommissionPerPage; pages below 1
// are treated as 1.
func parseCommissionPage(c *gin.Context, pageKey, perPageKey string) (int, int) {
	page, err := strconv.Atoi(c.DefaultQuery(pageKey, "1"))
	if err != nil || page < 1 {
		page = 1
	}
	perPage, err := strconv.Atoi(c.DefaultQuery(perPageKey, strconv.Itoa(maxCommissionPerPage)))
	if err != nil || perPage < 1 || perPage > maxCommissionPerPage {
		perPage = maxCommissionPerPage
	}
	return page, perPage
}

// paginateRows returns the requested page of rows and its pagination metadata.
// A page past the last one yields an empty page.
func paginateRows[T any](rows []T, page, perPage int) ([]T, gin.H) {
	total := len(rows)
	totalPages := (total + perPage - 1) / perPage
	pageRows := []T{}
	if page <= totalPages {
		start := (page - 1) * perPage
		pageRows = rows[start:min(start+perPage, total)]
	}
	return pageRows, gin.H{
		"page":        page,
		"per_page":    perPage,
		"total":       total,
		"total_pages": totalPages,
	}
}
