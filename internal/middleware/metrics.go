package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	RequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_requests_total",
			Help: "Total number of requests processed by the gateway.",
		},
		[]string{"method", "path", "status"},
	)

	RequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gateway_request_duration_seconds",
			Help:    "Duration of requests in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	BackendHealth = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gateway_backend_health",
			Help: "Health status of backend servers (1 = healthy, 0 = unhealthy).",
		},
		[]string{"backend"},
	)

	CircuitBreakerState = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gateway_circuit_breaker_state",
			Help: "Circuit breaker state (0 = closed, 1 = open, 2 = half-open).",
		},
		[]string{"backend"},
	)

	ActiveConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gateway_active_connections",
			Help: "Number of active connections per backend.",
		},
		[]string{"backend"},
	)
)

func init() {
	prometheus.MustRegister(
		RequestsTotal,
		RequestDuration,
		BackendHealth,
		CircuitBreakerState,
		ActiveConnections,
	)
}

func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Capture before serving: routing may strip the path prefix on r.URL.Path
		// in place, and metrics should reflect the path the client requested.
		method := r.Method
		path := r.URL.Path

		rc := &responseCapture{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(rc, r)

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(rc.statusCode)

		RequestsTotal.WithLabelValues(method, path, status).Inc()
		RequestDuration.WithLabelValues(method, path).Observe(duration)
	})
}

func MetricsHandler() http.Handler {
	return promhttp.Handler()
}
