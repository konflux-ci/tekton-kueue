package v1

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// celReloadsTotal tracks the total number of CEL Reloads
	configReloadTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tekton_kueue_config_reload_total",
			Help: "Total number of Config reloads",
		},
		[]string{"result"}, // result can be "success" or "failure"
	)

	// celMutationsTotal tracks the total number of CEL mutation operations
	configReloadFailureTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tekton_kueue_config_reload_failure_total",
			Help: "Total number of Config reload failures",
		},
		[]string{"result"}, // result: "success" or "failure"
	)
)

func init() {
	// Register the metrics with controller-runtime's global registry
	metrics.Registry.MustRegister(configReloadTotal)
	metrics.Registry.MustRegister(configReloadFailureTotal)
}

// RecordReloadFailure increments the counter for CEL Reload failures
func RecordReloadFailure() {
	configReloadTotal.WithLabelValues("failure").Inc()
}

// RecordReloadSuccess increments the counter for successful CEL Reloads
func RecordReloadSuccess() {
	configReloadTotal.WithLabelValues("success").Inc()
}
