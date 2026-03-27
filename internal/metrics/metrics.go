package metrics

import (
	"runtime"

	"github.com/prometheus/client_golang/prometheus"
)

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

	// ── Data Coverage ───────────────────────────────────────────────────────

	ScheduleDaysAvailable = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "greektv_schedule_days_available",
		Help: "Average number of days with data per channel, by group.",
	}, []string{"channel_group"})

	ProgrammesPerDay = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "greektv_programmes_per_day",
		Help: "Total programmes stored for each broadcast date.",
	}, []string{"date"})

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

	RedisLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "greektv_redis_latency_seconds",
		Help:    "Redis round-trip latency by operation type.",
		Buckets: []float64{0.0005, 0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1},
	}, []string{"operation"})

	// ── Now Updater ─────────────────────────────────────────────────────────

	NowUpdaterDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "greektv_now_updater_duration_seconds",
		Help:    "Duration of each now-playing cache refresh.",
		Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1, 2, 5},
	})

	NowUpdaterRuns = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "greektv_now_updater_runs_total",
		Help: "Total number of now-updater refresh cycles.",
	})

	// ── Business / Data Quality ─────────────────────────────────────────────

	ChannelsTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "greektv_channels_total",
		Help: "Total number of channels in the registry.",
	})

	ChannelsLiveNow = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "greektv_channels_live_now",
		Help: "Number of channels with a currently-airing programme.",
	})

	// ── Resilience ──────────────────────────────────────────────────────────

	PanicsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "greektv_panics_total",
		Help: "Total recovered panics by component.",
	}, []string{"component"})

	// ── Service Info ────────────────────────────────────────────────────────

	BuildInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "greektv_build_info",
		Help: "Build information (always 1). Labels show version and Go version.",
	}, []string{"version", "go_version"})

	StartTime = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "greektv_start_time_seconds",
		Help: "Unix timestamp when the service started.",
	})
)

// Version is set at build time via -ldflags.
var Version = "dev"

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
		// Data coverage
		ScheduleDaysAvailable,
		ProgrammesPerDay,
		// HTTP
		HTTPRequestsTotal,
		HTTPRequestDuration,
		// Redis
		RedisOperations,
		RedisLatency,
		// Now updater
		NowUpdaterDuration,
		NowUpdaterRuns,
		// Business
		ChannelsTotal,
		ChannelsLiveNow,
		// Resilience
		PanicsTotal,
		// Service
		BuildInfo,
		StartTime,
	)

	// Set static build info
	BuildInfo.WithLabelValues(Version, runtime.Version()).Set(1)
}
