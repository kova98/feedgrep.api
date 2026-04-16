package monitor

import (
	"time"

	"github.com/kova98/feedgrep.api/enums"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	arcticShiftKindPost    = "post"
	arcticShiftKindComment = "comment"
)

type ArcticShiftMonitor struct {
	itemsProcessed      *prometheus.CounterVec
	requestDuration     *prometheus.HistogramVec
	processingDuration  *prometheus.HistogramVec
	matchEvaluation     *prometheus.HistogramVec
	lagSeconds          *prometheus.GaugeVec
	registeredCollector []prometheus.Collector
}

func NewArcticShiftMonitor() *ArcticShiftMonitor {
	m := &ArcticShiftMonitor{
		itemsProcessed: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "feedgrep",
				Subsystem: "arcticshift",
				Name:      "items_processed_total",
				Help:      "Total ArcticShift items processed by the poller.",
			},
			[]string{"kind"},
		),
		requestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "feedgrep",
				Subsystem: "arcticshift",
				Name:      "request_duration_seconds",
				Help:      "ArcticShift API request latency in seconds.",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"kind", "status"},
		),
		processingDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "feedgrep",
				Subsystem: "arcticshift",
				Name:      "processing_duration_seconds",
				Help:      "ArcticShift batch processing latency in seconds.",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"kind"},
		),
		matchEvaluation: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "feedgrep",
				Subsystem: "arcticshift",
				Name:      "match_evaluation_duration_seconds",
				Help:      "ArcticShift subscription match evaluation latency in seconds.",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"kind", "match_mode"},
		),
		lagSeconds: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "feedgrep",
				Subsystem: "arcticshift",
				Name:      "lag_seconds",
				Help:      "Age in seconds of the newest ArcticShift item processed in the latest batch.",
			},
			[]string{"kind"},
		),
	}
	m.registeredCollector = []prometheus.Collector{
		m.itemsProcessed,
		m.requestDuration,
		m.processingDuration,
		m.matchEvaluation,
		m.lagSeconds,
	}
	m.initSeries()
	return m
}

func (m *ArcticShiftMonitor) Register(registerer prometheus.Registerer) {
	registerer.MustRegister(m.registeredCollector...)
}

func (m *ArcticShiftMonitor) PostBatch(count int64, requestDuration time.Duration, processingStart time.Time, newestCreatedUTC int64) {
	m.captureBatch(arcticShiftKindPost, count, requestDuration, processingStart, newestCreatedUTC)
}

func (m *ArcticShiftMonitor) CommentBatch(count int64, requestDuration time.Duration, processingStart time.Time, newestCreatedUTC int64) {
	m.captureBatch(arcticShiftKindComment, count, requestDuration, processingStart, newestCreatedUTC)
}

func (m *ArcticShiftMonitor) PostRequestError(requestDuration time.Duration) {
	m.captureRequest(arcticShiftKindPost, "error", requestDuration)
}

func (m *ArcticShiftMonitor) CommentRequestError(requestDuration time.Duration) {
	m.captureRequest(arcticShiftKindComment, "error", requestDuration)
}

func (m *ArcticShiftMonitor) PostRequest(requestDuration time.Duration) {
	m.captureRequest(arcticShiftKindPost, "ok", requestDuration)
}

func (m *ArcticShiftMonitor) CommentRequest(requestDuration time.Duration) {
	m.captureRequest(arcticShiftKindComment, "ok", requestDuration)
}

func (m *ArcticShiftMonitor) PostMatchEvaluation(matchMode string, start time.Time) {
	m.captureMatchEvaluation(arcticShiftKindPost, matchMode, start)
}

func (m *ArcticShiftMonitor) CommentMatchEvaluation(matchMode string, start time.Time) {
	m.captureMatchEvaluation(arcticShiftKindComment, matchMode, start)
}

func (m *ArcticShiftMonitor) captureBatch(kind string, count int64, requestDuration time.Duration, processingStart time.Time, newestCreatedUTC int64) {
	m.itemsProcessed.WithLabelValues(kind).Add(float64(count))
	m.captureRequest(kind, "ok", requestDuration)
	m.processingDuration.WithLabelValues(kind).Observe(time.Since(processingStart).Seconds())
	m.lagSeconds.WithLabelValues(kind).Set(float64(feedLagSeconds(newestCreatedUTC)))
}

func (m *ArcticShiftMonitor) captureRequest(kind string, status string, duration time.Duration) {
	m.requestDuration.WithLabelValues(kind, status).Observe(duration.Seconds())
}

func (m *ArcticShiftMonitor) captureMatchEvaluation(kind string, matchMode string, start time.Time) {
	m.matchEvaluation.WithLabelValues(kind, matchMode).Observe(time.Since(start).Seconds())
}

func feedLagSeconds(newestCreatedUTC int64) int64 {
	if newestCreatedUTC <= 0 {
		return 0
	}
	return time.Now().Unix() - newestCreatedUTC
}

func (m *ArcticShiftMonitor) initSeries() {
	for _, kind := range []string{arcticShiftKindPost, arcticShiftKindComment} {
		m.itemsProcessed.WithLabelValues(kind).Add(0)
		m.requestDuration.WithLabelValues(kind, "ok")
		m.requestDuration.WithLabelValues(kind, "error")
		m.processingDuration.WithLabelValues(kind)
		m.lagSeconds.WithLabelValues(kind).Set(0)
		for _, matchMode := range []enums.MatchMode{enums.MatchModeBroad, enums.MatchModeExact, enums.MatchModeSmart} {
			m.matchEvaluation.WithLabelValues(kind, string(matchMode))
		}
	}
}
