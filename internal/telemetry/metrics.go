package telemetry

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "go_saga_lab_http_requests_total",
			Help: "Total number of HTTP requests handled by the API.",
		},
		[]string{"route", "method", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "go_saga_lab_http_request_duration_seconds",
			Help:    "HTTP request duration for the API.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"route", "method"},
	)

	outboxPublishTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "go_saga_lab_outbox_publish_total",
			Help: "Total outbox publish attempts by result and event type.",
		},
		[]string{"backend", "event_type", "result"},
	)
)

func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

func InstrumentHTTP(route string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		next(recorder, r)

		status := strconv.Itoa(recorder.statusCode)
		httpRequestsTotal.WithLabelValues(route, r.Method, status).Inc()
		httpRequestDuration.WithLabelValues(route, r.Method).Observe(time.Since(start).Seconds())
	}
}

func RecordOutboxPublish(backend, eventType, result string) {
	outboxPublishTotal.WithLabelValues(backend, eventType, result).Inc()
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}
