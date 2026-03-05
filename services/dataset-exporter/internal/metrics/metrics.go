package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	SamplesSynced = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "exporter_samples_synced_total",
		Help: "Total number of samples synced from chain.",
	})

	SyncErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "exporter_sync_errors_total",
		Help: "Total number of sync errors.",
	})

	SyncLagBlocks = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "exporter_sync_lag_blocks",
		Help: "Number of blocks behind chain head.",
	})

	SnapshotsCreated = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "exporter_snapshots_created_total",
		Help: "Total number of dataset snapshots created.",
	})

	StagedSamplesTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "exporter_staged_samples_total",
		Help: "Current number of samples in staging database.",
	})

	LastSyncHeight = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "exporter_last_sync_height",
		Help: "Last synced block height.",
	})
)

func init() {
	prometheus.MustRegister(
		SamplesSynced,
		SyncErrors,
		SyncLagBlocks,
		SnapshotsCreated,
		StagedSamplesTotal,
		LastSyncHeight,
	)
}

// Handler returns the Prometheus HTTP handler.
func Handler() http.Handler {
	return promhttp.Handler()
}

// HealthHandler returns a simple health check handler.
func HealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}
}
