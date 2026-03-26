package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	// ── Scheduler / Data Pipeline ───────────────────────────────────────────

	RefreshDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "greektv_refresh_duration_seconds",
		Help:    "Duration of schedule refresh operations.",
		Buckets: []float64{1, 5, 10, 30, 60, 120, 300},
	}, []string{"source"})

	RefreshTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "greektv_refresh_total",
		Help: "Total number of refresh cycles.",
	}, []string{"status"})

	RefreshLastSuccess = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "greektv_refresh_last_success_timestamp",
		Help: "Unix timestamp of the last successful refresh.",
	})

	SourceFetchErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "greektv_source_fetch_errors_total",
		Help: "Total fetch errors by data source.",
	}, []string{"source"})

	ProgrammesFetched = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "greektv_programmes_fetched_total",
		Help: "Number of programmes fetched in the last refresh, by source.",
	}, []string{"source"})

	ChannelsWithData = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "greektv_channels_with_data",
		Help: "Number of channels that have schedule data after the last refresh.",
	})

	ProgrammesStored = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "greektv_programmes_stored_total",
		Help: "Total programmes stored in Redis across all channels and dates.",
	})

	// ── API / HTTP ──────────────────────────────────────────────────────────

	HTTPRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "greektv_http_requests_total",
		Help: "Total HTTP requests by method, route, and status code.",
	}, []string{"method", "route", "status"})

	HTTPRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "greektv_http_request_duration_seconds",
		Help:    "HTTP request latency by method and route.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "route"})

	// ── Redis ───────────────────────────────────────────────────────────────

	RedisOperations = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "greektv_redis_operations_total",
		Help: "Total Redis operations by type and status.",
	}, []string{"operation", "status"})

	// ── Business / Data Quality ─────────────────────────────────────────────

	ChannelsTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "greektv_channels_total",
		Help: "Total number of channels in the registry.",
	})

	ChannelsLiveNow = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "greektv_channels_live_now",
		Help: "Number of channels with a currently-airing programme.",
	})
)

func init() {
	prometheus.MustRegister(
		// Scheduler
		RefreshDuration,
		RefreshTotal,
		RefreshLastSuccess,
		SourceFetchErrors,
		ProgrammesFetched,
		ChannelsWithData,
		ProgrammesStored,
		// HTTP
		HTTPRequestsTotal,
		HTTPRequestDuration,
		// Redis
		RedisOperations,
		// Business
		ChannelsTotal,
		ChannelsLiveNow,
	)
}
