package monitor

import "github.com/prometheus/client_golang/prometheus"

type NotificationsMonitor struct {
	emailsSent *prometheus.CounterVec
}

func NewNotificationsMonitor() *NotificationsMonitor {
	return &NotificationsMonitor{
		emailsSent: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "feedgrep",
				Subsystem: "notifications",
				Name:      "emails_sent_total",
				Help:      "Total notification emails sent.",
			},
			[]string{"type"},
		),
	}
}

func (m *NotificationsMonitor) Register(registerer prometheus.Registerer) {
	registerer.MustRegister(m.emailsSent)
}

func (m *NotificationsMonitor) MatchEmailSent() {
	m.emailsSent.WithLabelValues("match").Inc()
}

func (m *NotificationsMonitor) DigestEmailSent() {
	m.emailsSent.WithLabelValues("digest").Inc()
}
