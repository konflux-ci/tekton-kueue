package v1

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// configReloadTotal tracks the total number of webhook configuration reloads,
	// labeled by result ("success" or "failure"). Incremented each time the
	// ConfigMapReconciler triggers a config update.
	configReloadTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tekton_kueue_config_reload_total",
			Help: "Total number of Config reloads",
		},
		[]string{"result"},
	)

	// configReloadFailureTotal is currently unused but registered for
	// backwards compatibility with existing dashboards.
	configReloadFailureTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tekton_kueue_config_reload_failure_total",
			Help: "Total number of Config reload failures",
		},
		[]string{"result"},
	)
)

func init() {
	// Register the metrics with controller-runtime's global registry
	metrics.Registry.MustRegister(configReloadTotal)
	metrics.Registry.MustRegister(configReloadFailureTotal)
}

// RecordReloadFailure increments the counter for config reload failures.
func RecordReloadFailure() {
	configReloadTotal.WithLabelValues("failure").Inc()
}

// RecordReloadSuccess increments the counter for successful config reloads.
func RecordReloadSuccess() {
	configReloadTotal.WithLabelValues("success").Inc()
}
