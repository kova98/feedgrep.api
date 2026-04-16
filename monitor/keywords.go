package monitor

import "github.com/prometheus/client_golang/prometheus"

type KeywordMonitor struct {
	active prometheus.Gauge
}

func NewKeywordMonitor() *KeywordMonitor {
	return &KeywordMonitor{
		active: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "feedgrep",
				Subsystem: "keywords",
				Name:      "active",
				Help:      "Active keyword subscriptions.",
			},
		),
	}
}

func (m *KeywordMonitor) Register(registerer prometheus.Registerer) {
	registerer.MustRegister(m.active)
}

func (m *KeywordMonitor) Active(count int) {
	m.active.Set(float64(count))
}
