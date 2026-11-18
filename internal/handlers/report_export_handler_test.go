package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sjperalta/fintera-api/internal/services"
)

func newReportExportTestRouter(handler *ReportExportHandler, role string, userID uint) *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("userID", userID)
		c.Set("userRole", role)
		c.Next()
	})
	router.GET("/reports/operations_export", handler.Operations)
	router.GET("/reports/commissions_export", handler.Commissions)
	router.GET("/reports/overdue_export", handler.Overdue)
	router.GET("/reports/income_flow_export", handler.IncomeFlow)
	return router
}

func newReportExportTestHandler() *ReportExportHandler {
	// The export service starts with no repositories: every validation test
	// fails before a builder is invoked, and the render tests only need the
	// stateless renderers.
	return NewReportExportHandler(&services.ReportExportService{})
}

// newCommissionsExportTestHandler wires a commissions report backed by the
// shared handler fixture so the seller filter can be asserted in the output.
func newCommissionsExportTestHandler() *ReportExportHandler {
	exportSvc := services.NewReportExportService(
		nil,
		nil,
		nil,
		commissionsFixture(),
		&commissionsCreditRepo{credits: commissionsFixtureCredits()},
	)
	return NewReportExportHandler(exportSvc)
}

func TestReportExportHandlerFormatValidation(t *testing.T) {
	handler := newReportExportTestHandler()
	router := newReportExportTestRouter(handler, "admin", 1)

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantError  string
	}{
		{
			name:       "missing format",
			path:       "/reports/operations_export",
			wantStatus: http.StatusBadRequest,
			wantError:  "el parámetro format es obligatorio: use xlsx, csv o pdf",
		},
		{
			name:       "invalid format",
			path:       "/reports/operations_export?format=docx",
			wantStatus: http.StatusBadRequest,
			wantError:  "formato inválido: use xlsx, csv o pdf",
		},
		{
			name:       "missing format on income flow",
			path:       "/reports/income_flow_export",
			wantStatus: http.StatusBadRequest,
			wantError:  "el parámetro format es obligatorio: use xlsx, csv o pdf",
		},
		{
			name:       "invalid start date",
			path:       "/reports/operations_export?format=xlsx&start_date=2026-13-01",
			wantStatus: http.StatusBadRequest,
			wantError:  "fecha inválida, use AAAA-MM-DD",
		},
		{
			name:       "invalid end date",
			path:       "/reports/income_flow_export?format=csv&end_date=not-a-date",
			wantStatus: http.StatusBadRequest,
			wantError:  "fecha inválida, use AAAA-MM-DD",
		},
		{
			name:       "invalid project id",
			path:       "/reports/operations_export?format=pdf&project_id=abc",
			wantStatus: http.StatusBadRequest,
			wantError:  "project_id inválido",
		},
		{
			name:       "zero seller id on commissions export",
			path:       "/reports/commissions_export?format=csv&seller_id=0",
			wantStatus: http.StatusBadRequest,
			wantError:  "seller_id inválido",
		},
		{
			name:       "non numeric seller id on commissions export",
			path:       "/reports/commissions_export?format=csv&seller_id=abc",
			wantStatus: http.StatusBadRequest,
			wantError:  "seller_id inválido",
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

func TestParseReportExportFiltersDefaultsToCurrentMonth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.Local)

	t.Run("operations and income flow", func(t *testing.T) {
		context, _ := gin.CreateTestContext(httptest.NewRecorder())
		context.Request = httptest.NewRequest(http.MethodGet, "/", nil)
		context.Set("userID", uint(7))
		context.Set("userRole", "admin")

		filters, err := parseReportExportFilters(context, true, true, false, now)
		if err != nil {
			t.Fatalf("parseReportExportFilters returned error: %v", err)
		}
		if filters.StartDate == nil || filters.EndDate == nil {
			t.Fatal("expected the current month range, got nil dates")
		}

		wantStart := time.Date(2026, 10, 1, 0, 0, 0, 0, time.Local)
		wantEnd := time.Date(2026, 10, 31, 23, 59, 59, 0, time.Local)
		if !filters.StartDate.Equal(wantStart) {
			t.Fatalf("start = %s, want %s", filters.StartDate, wantStart)
		}
		if !filters.EndDate.Equal(wantEnd) {
			t.Fatalf("end = %s, want %s", filters.EndDate, wantEnd)
		}
	})

	t.Run("commissions keeps all periods", func(t *testing.T) {
		context, _ := gin.CreateTestContext(httptest.NewRecorder())
		context.Request = httptest.NewRequest(http.MethodGet, "/", nil)
		context.Set("userID", uint(7))
		context.Set("userRole", "admin")

		filters, err := parseReportExportFilters(context, false, false, true, now)
		if err != nil {
			t.Fatalf("parseReportExportFilters returned error: %v", err)
		}
		if filters.StartDate != nil || filters.EndDate != nil {
			t.Fatalf("commissions filters should keep both dates nil, got %v - %v", filters.StartDate, filters.EndDate)
		}
	})

	t.Run("explicit dates win over the default", func(t *testing.T) {
		context, _ := gin.CreateTestContext(httptest.NewRecorder())
		context.Request = httptest.NewRequest(http.MethodGet, "/?start_date=2026-01-01", nil)
		context.Set("userID", uint(7))
		context.Set("userRole", "admin")

		filters, err := parseReportExportFilters(context, true, true, false, now)
		if err != nil {
			t.Fatalf("parseReportExportFilters returned error: %v", err)
		}
		if filters.StartDate == nil || filters.StartDate.Month() != time.January {
			t.Fatalf("explicit start date was overridden: %v", filters.StartDate)
		}
		if filters.EndDate != nil {
			t.Fatalf("end date should stay nil, got %v", filters.EndDate)
		}
	})

	t.Run("seller scope", func(t *testing.T) {
		context, _ := gin.CreateTestContext(httptest.NewRecorder())
		context.Request = httptest.NewRequest(http.MethodGet, "/", nil)
		context.Set("userID", uint(42))
		context.Set("userRole", "seller")

		filters, err := parseReportExportFilters(context, false, false, true, now)
		if err != nil {
			t.Fatalf("parseReportExportFilters returned error: %v", err)
		}
		if !filters.FilterBySeller || filters.SellerID != 42 {
			t.Fatalf("seller filters = %+v, want FilterBySeller true and SellerID 42", filters)
		}
	})
}

func TestParseReportExportFiltersSellerID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.Local)

	t.Run("admin seller id scopes the export", func(t *testing.T) {
		context, _ := gin.CreateTestContext(httptest.NewRecorder())
		context.Request = httptest.NewRequest(http.MethodGet, "/?seller_id=7", nil)
		context.Set("userID", uint(1))
		context.Set("userRole", "admin")

		filters, err := parseReportExportFilters(context, false, false, true, now)
		if err != nil {
			t.Fatalf("parseReportExportFilters returned error: %v", err)
		}
		if !filters.FilterBySeller || filters.SellerID != 7 {
			t.Fatalf("filters = %+v, want FilterBySeller true and SellerID 7", filters)
		}
	})

	t.Run("admin without seller id sees every seller", func(t *testing.T) {
		context, _ := gin.CreateTestContext(httptest.NewRecorder())
		context.Request = httptest.NewRequest(http.MethodGet, "/", nil)
		context.Set("userID", uint(1))
		context.Set("userRole", "admin")

		filters, err := parseReportExportFilters(context, false, false, true, now)
		if err != nil {
			t.Fatalf("parseReportExportFilters returned error: %v", err)
		}
		if filters.FilterBySeller || filters.SellerID != 0 {
			t.Fatalf("filters = %+v, want no seller filter", filters)
		}
	})

	t.Run("seller ignores a foreign seller id", func(t *testing.T) {
		context, _ := gin.CreateTestContext(httptest.NewRecorder())
		context.Request = httptest.NewRequest(http.MethodGet, "/?seller_id=7", nil)
		context.Set("userID", uint(42))
		context.Set("userRole", "seller")

		filters, err := parseReportExportFilters(context, false, false, true, now)
		if err != nil {
			t.Fatalf("parseReportExportFilters returned error: %v", err)
		}
		if !filters.FilterBySeller || filters.SellerID != 42 {
			t.Fatalf("filters = %+v, want the seller's own id 42", filters)
		}
	})

	t.Run("seller ignores an invalid seller id", func(t *testing.T) {
		context, _ := gin.CreateTestContext(httptest.NewRecorder())
		context.Request = httptest.NewRequest(http.MethodGet, "/?seller_id=abc", nil)
		context.Set("userID", uint(42))
		context.Set("userRole", "seller")

		filters, err := parseReportExportFilters(context, false, false, true, now)
		if err != nil {
			t.Fatalf("parseReportExportFilters returned error: %v", err)
		}
		if !filters.FilterBySeller || filters.SellerID != 42 {
			t.Fatalf("filters = %+v, want the seller's own id 42", filters)
		}
	})
}

func TestCommissionsExportAdminSellerFilter(t *testing.T) {
	handler := newCommissionsExportTestHandler()
	router := newReportExportTestRouter(handler, "admin", 1)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/reports/commissions_export?format=csv&seller_id=7", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "Ana Vendedora") {
		t.Fatal("expected the scoped seller commissions and credits")
	}
	for _, unexpected := range []string{"Carlos Vendedor", "Sin vendedor"} {
		if strings.Contains(body, unexpected) {
			t.Fatalf("export should not contain %q with seller_id=7", unexpected)
		}
	}
}

func TestCommissionsExportSellerIgnoresForeignSellerID(t *testing.T) {
	handler := newCommissionsExportTestHandler()
	router := newReportExportTestRouter(handler, "seller", 8)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/reports/commissions_export?format=csv&seller_id=7", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "Carlos Vendedor") {
		t.Fatal("expected the authenticated seller commissions")
	}
	if strings.Contains(body, "Ana Vendedora") {
		t.Fatal("a seller must not see a foreign seller's rows or credits")
	}
}

func newReportExportTestDocument() *services.ReportDocument {
	amount := 1000.0
	return &services.ReportDocument{
		Title:       "Reporte de Operaciones del Mes",
		Period:      "Octubre 2026",
		PeriodKey:   "2026-10",
		BaseName:    "reporte_operaciones",
		GeneratedAt: time.Date(2026, 10, 15, 10, 0, 0, 0, time.Local),
		Currency:    "HNL",
		Sections: []services.ReportSection{
			{
				Title:   "Resumen",
				Kind:    services.ReportSectionTable,
				Headers: []string{"Concepto", "Monto"},
				Rows: []services.ReportRow{
					{Cells: []services.ReportCell{
						services.TextCell("Ventas"),
						services.MoneyCell(amount, "HNL"),
					}},
				},
			},
		},
	}
}

func TestServeReportExportFilenameAndContentType(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		format          string
		wantFilename    string
		wantContentType string
	}{
		{
			format:          services.ReportFormatXLSX,
			wantFilename:    "reporte_operaciones_2026-10.xlsx",
			wantContentType: "application/octet-stream",
		},
		{
			format:          services.ReportFormatCSV,
			wantFilename:    "reporte_operaciones_2026-10.csv",
			wantContentType: "text/csv",
		},
		{
			format:          services.ReportFormatPDF,
			wantFilename:    "reporte_operaciones_2026-10.pdf",
			wantContentType: "application/octet-stream",
		},
	}

	for _, test := range tests {
		t.Run(test.format, func(t *testing.T) {
			handler := newReportExportTestHandler()

			router := gin.New()
			router.GET("/export", func(c *gin.Context) {
				handler.serveReportExport(c, newReportExportTestDocument(), test.format, services.ReportSheetOperations)
			})

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/export", nil)
			router.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 (body: %s)", recorder.Code, recorder.Body.String())
			}
			if got := recorder.Header().Get("Content-Type"); got != test.wantContentType {
				t.Fatalf("Content-Type = %q, want %q", got, test.wantContentType)
			}

			disposition := recorder.Header().Get("Content-Disposition")
			if !strings.Contains(disposition, test.wantFilename) {
				t.Fatalf("Content-Disposition = %q, want it to contain %q", disposition, test.wantFilename)
			}
			if recorder.Body.Len() == 0 {
				t.Fatal("expected a non-empty export body")
			}
		})
	}
}
