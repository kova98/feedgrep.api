package monitor

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type APIMonitor struct {
	requestDuration *prometheus.HistogramVec
	requestsTotal   *prometheus.CounterVec
}

func NewAPIMonitor() *APIMonitor {
	return &APIMonitor{
		requestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "feedgrep",
				Subsystem: "api",
				Name:      "request_duration_seconds",
				Help:      "API endpoint response time in seconds.",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"method", "route", "status"},
		),
		requestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "feedgrep",
				Subsystem: "api",
				Name:      "requests_total",
				Help:      "Total API requests by method, route, and status.",
			},
			[]string{"method", "route", "status"},
		),
	}
}

func (m *APIMonitor) Register(registerer prometheus.Registerer) {
	registerer.MustRegister(m.requestDuration, m.requestsTotal)
}

func (m *APIMonitor) Request(r *http.Request, status int, start time.Time) {
	if status == 0 {
		status = http.StatusOK
	}
	method, route := requestLabels(r)
	statusLabel := strconv.Itoa(status)

	m.requestsTotal.WithLabelValues(method, route, statusLabel).Inc()
	m.requestDuration.WithLabelValues(method, route, statusLabel).Observe(time.Since(start).Seconds())
}

func requestLabels(r *http.Request) (string, string) {
	if r.Pattern != "" {
		method, route := splitServeMuxPattern(r.Pattern)
		if method == "" {
			method = r.Method
		}
		return method, route
	}
	return r.Method, r.URL.Path
}

func splitServeMuxPattern(pattern string) (string, string) {
	method, route, ok := strings.Cut(strings.TrimSpace(pattern), " ")
	if !ok {
		return "", pattern
	}
	return method, route
}
