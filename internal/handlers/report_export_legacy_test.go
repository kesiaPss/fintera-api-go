package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/services"
)

// newLegacyReportExportTestRouter mirrors the deprecated routes registered in
// cmd/api/main.go inside the sellerAdmin group.
func newLegacyReportExportTestRouter(handler *ReportExportHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("userID", uint(1))
		c.Set("userRole", "admin")
		c.Next()
	})
	router.GET("/analytics/export", handler.LegacyAnalyticsExport)
	router.GET("/reports/commissions_csv", handler.LegacyCommissionsCSV)
	router.GET("/reports/total_revenue_csv", handler.LegacyTotalRevenueCSV)
	router.GET("/reports/overdue_payments_csv", handler.LegacyOverduePaymentsCSV)
	return router
}

// stubOverdueReportRepository returns an empty snapshot so the overdue alias can
// be exercised end to end without a database.
type stubOverdueReportRepository struct{}

func (stubOverdueReportRepository) FindOverdueForActiveContractsDetailed(_ context.Context) ([]models.Payment, error) {
	return nil, nil
}

func TestLegacyReportExportRoutesRegistered(t *testing.T) {
	router := newLegacyReportExportTestRouter(newReportExportTestHandler())

	registered := make(map[string]bool, len(router.Routes()))
	for _, route := range router.Routes() {
		registered[route.Method+" "+route.Path] = true
	}

	for _, path := range []string{
		"/analytics/export",
		"/reports/commissions_csv",
		"/reports/total_revenue_csv",
		"/reports/overdue_payments_csv",
	} {
		if !registered[http.MethodGet+" "+path] {
			t.Fatalf("legacy route GET %s is not registered", path)
		}
	}
}

func TestLegacyReportExportValidation(t *testing.T) {
	handler := newReportExportTestHandler()
	router := newLegacyReportExportTestRouter(handler)

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantError  string
	}{
		{
			name:       "commissions forwards the start date",
			path:       "/reports/commissions_csv?start_date=2026-13-01",
			wantStatus: http.StatusBadRequest,
			wantError:  "fecha inválida, use AAAA-MM-DD",
		},
		{
			name:       "commissions forwards the end date",
			path:       "/reports/commissions_csv?end_date=not-a-date",
			wantStatus: http.StatusBadRequest,
			wantError:  "fecha inválida, use AAAA-MM-DD",
		},
		{
			name:       "total revenue forwards the project id",
			path:       "/reports/total_revenue_csv?project_id=abc",
			wantStatus: http.StatusBadRequest,
			wantError:  "project_id inválido",
		},
		{
			name:       "total revenue forwards the dates",
			path:       "/reports/total_revenue_csv?start_date=2026-13-01",
			wantStatus: http.StatusBadRequest,
			wantError:  "fecha inválida, use AAAA-MM-DD",
		},
		{
			name:       "analytics rejects an invalid format",
			path:       "/analytics/export?format=docx",
			wantStatus: http.StatusBadRequest,
			wantError:  "formato inválido: use xlsx, csv o pdf",
		},
		{
			// No format defaults to CSV, then the shared filters still reject
			// the malformed project id: both the default and the parse ran.
			name:       "analytics defaults to csv and parses the project id",
			path:       "/analytics/export?project_id=abc",
			wantStatus: http.StatusBadRequest,
			wantError:  "project_id inválido",
		},
		{
			name:       "analytics forwards the dates",
			path:       "/analytics/export?format=csv&end_date=2026-13-01",
			wantStatus: http.StatusBadRequest,
			wantError:  "fecha inválida, use AAAA-MM-DD",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			router.ServeHTTP(recorder, request)

			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", recorder.Code, test.wantStatus, recorder.Body.String())
			}

			var body map[string]string
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatalf("response is not a JSON error: %v (%s)", err, recorder.Body.String())
			}
			if body["error"] != test.wantError {
				t.Fatalf("error = %q, want %q", body["error"], test.wantError)
			}
		})
	}
}

func TestLegacyExportFormatDefaultsToCSV(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		query      string
		wantFormat string
		wantError  bool
	}{
		{name: "missing format defaults to csv", query: "", wantFormat: services.ReportFormatCSV},
		{name: "blank format defaults to csv", query: "?format=%20", wantFormat: services.ReportFormatCSV},
		{name: "explicit csv", query: "?format=csv", wantFormat: services.ReportFormatCSV},
		{name: "case and spaces are normalized", query: "?format=%20XLSX%20", wantFormat: services.ReportFormatXLSX},
		{name: "invalid format", query: "?format=docx", wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			context, _ := gin.CreateTestContext(httptest.NewRecorder())
			context.Request = httptest.NewRequest(http.MethodGet, "/analytics/export"+test.query, nil)

			format, err := legacyExportFormat(context)
			if test.wantError {
				if err == nil {
					t.Fatalf("legacyExportFormat returned %q, want an error", format)
				}
				return
			}
			if err != nil {
				t.Fatalf("legacyExportFormat returned error: %v", err)
			}
			if format != test.wantFormat {
				t.Fatalf("format = %q, want %q", format, test.wantFormat)
			}
		})
	}
}

func TestLegacyOverduePaymentsCSVIgnoresDatesAndServesCSV(t *testing.T) {
	exportSvc := services.NewReportExportService(nil, stubOverdueReportRepository{}, nil, nil, nil)
	handler := NewReportExportHandler(exportSvc)
	router := newLegacyReportExportTestRouter(handler)

	// The deprecated overdue export is a snapshot: it accepts no query at all
	// and ignores dates, so even malformed ones must not fail.
	for _, path := range []string{
		"/reports/overdue_payments_csv",
		"/reports/overdue_payments_csv?start_date=not-a-date&end_date=also-bad",
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		router.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, want 200 (body: %s)", path, recorder.Code, recorder.Body.String())
		}
		if got := recorder.Header().Get("Content-Type"); got != "text/csv" {
			t.Fatalf("%s: Content-Type = %q, want text/csv", path, got)
		}
		disposition := recorder.Header().Get("Content-Disposition")
		if !strings.Contains(disposition, ".csv") {
			t.Fatalf("%s: Content-Disposition = %q, want a .csv attachment", path, disposition)
		}
		if recorder.Body.Len() == 0 {
			t.Fatalf("%s: expected a non-empty CSV body", path)
		}
	}
}
