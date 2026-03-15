package telemetry

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetricsHandlerExposesRegisteredMetrics(t *testing.T) {
	t.Parallel()

	handler := InstrumentHTTP("/test", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	metricsReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec = httptest.NewRecorder()
	MetricsHandler().ServeHTTP(rec, metricsReq)

	if rec.Code != http.StatusOK {
		t.Fatalf("/metrics status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "go_saga_lab_http_requests_total") {
		t.Fatal("expected http metric in /metrics output")
	}
}
