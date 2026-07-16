// Exercise 3 — Prometheus RED-method metrics middleware.
//
// TASK
// ----
// Implement the RED method (Rate, Errors, Duration) as HTTP middleware:
//
//  1. An http_requests_total CounterVec labeled {method, route, code} — the
//     source of Rate (its rate()) and Errors (the ratio of 5xx to total).
//  2. An http_request_duration_seconds HistogramVec labeled {method, route}
//     with prometheus.DefBuckets — the source of Duration (histogram_quantile).
//  3. Register both on a CUSTOM prometheus.Registry (not the global default),
//     so the exposed surface is explicit and tests stay isolated.
//  4. Expose them at /metrics via promhttp.HandlerFor.
//  5. Wire two handlers — a fast one and one with an injected sleep — so the
//     histogram shows a real spread when you scrape after generating load.
//
// Note the cardinality discipline: the route label is a fixed template
// ("/fast", "/slow", "/notes/{id}"), never a concrete path. A label whose
// value is unbounded (a note ID, a user ID) would explode the time-series
// count and is the single most expensive metrics mistake there is.
//
// Run it:
//
//	go mod init ex03 && go get github.com/prometheus/client_golang
//	go run exercise-03-prometheus-red-metrics.go
//	# generate a spread of latencies:
//	for i in $(seq 1 50); do curl -s localhost:8080/fast >/dev/null; done
//	for i in $(seq 1 10); do curl -s localhost:8080/slow >/dev/null; done
//	curl -s localhost:8080/notes/4f2 >/dev/null
//	# scrape:
//	curl -s localhost:8080/metrics | grep http_request
//
// You should see the counter split by route and code, and the histogram
// buckets show most /fast requests in the low buckets and /slow requests in
// the high buckets. The PromQL for the three RED signals is at the bottom.
package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds the two RED instruments.
type Metrics struct {
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

// NewMetrics builds and registers the RED instruments on the given registerer.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests by method, route, and status code.",
		}, []string{"method", "route", "code"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency in seconds by method and route.",
			Buckets: prometheus.DefBuckets, // .005 .01 .025 .05 .1 .25 .5 1 2.5 5 10
		}, []string{"method", "route"}),
	}
	reg.MustRegister(m.requests, m.duration)
	return m
}

// statusRecorder captures the status code so the counter can label by it.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

// Middleware records the RED signals for a request against a fixed route label.
// We pass the route template explicitly here because this exercise uses the
// stdlib mux, which does not expose a route pattern the way chi does. In the
// mini-project (chi), use chi.RouteContext(r.Context()).RoutePattern().
func (m *Metrics) Middleware(route string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next(rec, r)

		// Duration: only method+route — latency does not care about status code,
		// and adding code here would multiply the bucket series unnecessarily.
		m.duration.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())
		// Errors live on the counter via the code label.
		m.requests.WithLabelValues(r.Method, route, strconv.Itoa(rec.status)).Inc()
	}
}

// --- handlers ---------------------------------------------------------------

func handleFast(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte("fast\n"))
}

func handleSlow(w http.ResponseWriter, _ *http.Request) {
	time.Sleep(250 * time.Millisecond) // injected latency, lands in a high bucket
	_, _ = w.Write([]byte("slow\n"))
}

func handleNote(w http.ResponseWriter, r *http.Request) {
	// Even though the path is /notes/<id>, the metric is labeled with the
	// TEMPLATE "/notes/{id}" (see main), never the concrete id — cardinality.
	id := r.PathValue("id")
	if id == "0" {
		http.Error(w, "not found", http.StatusNotFound) // a 4xx, for the counter
		return
	}
	_, _ = fmt.Fprintf(w, `{"id":%q}`+"\n", id)
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)

	mux := http.NewServeMux() // Go 1.22 routing: method + path patterns
	mux.HandleFunc("GET /fast", m.Middleware("/fast", handleFast))
	mux.HandleFunc("GET /slow", m.Middleware("/slow", handleSlow))
	mux.HandleFunc("GET /notes/{id}", m.Middleware("/notes/{id}", handleNote))
	mux.Handle("GET /metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	logger.Info("listening", slog.String("addr", ":8080"),
		slog.String("metrics", "http://localhost:8080/metrics"))
	if err := http.ListenAndServe(":8080", mux); err != nil {
		logger.Error("server stopped", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

// PromQL for the three RED signals (run these in Prometheus at :9090 once a
// Prometheus server is scraping this app's /metrics):
//
// Rate — requests per second, per route:
//   sum by (route) (rate(http_requests_total[5m]))
//
// Errors — 5xx ratio per route (use code=~"4.." for client errors):
//   sum by (route) (rate(http_requests_total{code=~"5.."}[5m]))
//     /
//   sum by (route) (rate(http_requests_total[5m]))
//
// Duration — p99 latency per route (the `le` label MUST survive the sum):
//   histogram_quantile(0.99,
//     sum by (le, route) (rate(http_request_duration_seconds_bucket[5m]))
//   )
