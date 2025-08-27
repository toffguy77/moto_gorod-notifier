package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	// Counters
	SubscriptionsTotal   prometheus.Counter
	UnsubscriptionsTotal prometheus.Counter
	UniqueUsersTotal     prometheus.Gauge
	NewSlotsTotal        prometheus.Counter
	NotificationsSent    prometheus.Counter
	ErrorsTotal          *prometheus.CounterVec

	// Gauges
	ActiveSubscribers prometheus.Gauge
	SeenSlotsTotal    prometheus.Gauge

	// Histograms
	SlotCheckDuration prometheus.Histogram
	NotificationDelay prometheus.Histogram
}

func New() *Metrics {
	m := &Metrics{
		SubscriptionsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "moto_gorod_subscriptions_total",
			Help: "Total number of user subscriptions",
		}),
		UnsubscriptionsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "moto_gorod_unsubscriptions_total",
			Help: "Total number of user unsubscriptions",
		}),
		UniqueUsersTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "moto_gorod_unique_users_total",
			Help: "Total number of unique users who interacted with bot",
		}),
		NewSlotsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "moto_gorod_new_slots_total",
			Help: "Total number of new slots found",
		}),
		NotificationsSent: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "moto_gorod_notifications_sent_total",
			Help: "Total number of notifications sent to users",
		}),
		ErrorsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "moto_gorod_errors_total",
			Help: "Total number of errors by type",
		}, []string{"type"}),
		ActiveSubscribers: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "moto_gorod_active_subscribers",
			Help: "Current number of active subscribers",
		}),
		SeenSlotsTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "moto_gorod_seen_slots_total",
			Help: "Total number of seen slots in database",
		}),
		SlotCheckDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "moto_gorod_slot_check_duration_seconds",
			Help:    "Duration of slot availability checks",
			Buckets: prometheus.DefBuckets,
		}),
		NotificationDelay: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "moto_gorod_notification_delay_seconds",
			Help:    "Delay between slot discovery and notification",
			Buckets: []float64{0.1, 0.5, 1.0, 2.0, 5.0, 10.0},
		}),
	}

	// Register all metrics
	prometheus.MustRegister(
		m.SubscriptionsTotal,
		m.UnsubscriptionsTotal,
		m.UniqueUsersTotal,
		m.NewSlotsTotal,
		m.NotificationsSent,
		m.ErrorsTotal,
		m.ActiveSubscribers,
		m.SeenSlotsTotal,
		m.SlotCheckDuration,
		m.NotificationDelay,
	)

	return m
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.Handler()
}

func (m *Metrics) RecordSubscription() {
	m.SubscriptionsTotal.Inc()
}

func (m *Metrics) RecordUnsubscription() {
	m.UnsubscriptionsTotal.Inc()
}

func (m *Metrics) RecordUniqueUser() {
	m.UniqueUsersTotal.Inc()
}

func (m *Metrics) SetUniqueUsersTotal(count float64) {
	m.UniqueUsersTotal.Set(count)
}

func (m *Metrics) RecordNewSlot() {
	m.NewSlotsTotal.Inc()
}

func (m *Metrics) RecordNotificationSent() {
	m.NotificationsSent.Inc()
}

func (m *Metrics) RecordError(errorType string) {
	m.ErrorsTotal.WithLabelValues(errorType).Inc()
}

func (m *Metrics) SetActiveSubscribers(count float64) {
	m.ActiveSubscribers.Set(count)
}

func (m *Metrics) SetSeenSlotsTotal(count float64) {
	m.SeenSlotsTotal.Set(count)
}

func (m *Metrics) ObserveSlotCheckDuration(duration float64) {
	m.SlotCheckDuration.Observe(duration)
}

func (m *Metrics) ObserveNotificationDelay(delay float64) {
	m.NotificationDelay.Observe(delay)
}