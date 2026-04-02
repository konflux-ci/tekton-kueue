// Package v1 implements the mutating admission webhook for Tekton PipelineRuns.
//
// When a PipelineRun is created, the webhook intercepts it and:
//  1. Sets it to Pending so Kueue can control when it starts
//  2. Assigns it to a Kueue LocalQueue via a label
//  3. Optionally sets the managedBy field for multiKueue
//  4. Applies CEL-based mutations (labels, annotations, resource requests)
//
// Configuration is loaded from a ConfigMap and can be updated at runtime
// via the ConfigMapReconciler in the controller package.
package v1

import (
	"errors"
	"sync"

	"github.com/konflux-ci/tekton-kueue/internal/cel"
	"github.com/konflux-ci/tekton-kueue/pkg/config"
	tekv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	logger = ctrl.Log.WithName("config-store")
)

// ConfigStore holds the current webhook configuration and compiled CEL mutators.
// It is safe for concurrent use — the ConfigMapReconciler writes to it while
// the webhook reads from it on every admission request.
type ConfigStore struct {
	mu       sync.RWMutex
	config   *config.Config
	mutators []PipelineRunMutator
}

// PipelineRunMutator applies a mutation to a PipelineRun during webhook admission.
// The primary implementation is cel.CELMutator, which evaluates user-defined
// CEL expressions to dynamically set labels, annotations, or resource requests.
type PipelineRunMutator interface {
	Mutate(*tekv1.PipelineRun) error
}

func (s *ConfigStore) GetConfigAndMutators() (*config.Config, []PipelineRunMutator) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config, s.mutators
}

// Update parses and validates the raw YAML configuration, compiles any CEL
// expressions, and atomically swaps the config and mutators. If any step fails,
// the previous configuration is preserved (last-known-good behavior).
func (s *ConfigStore) Update(rawConfig []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	logger.Info("Updating config", "config", string(rawConfig))
	cfg, err := parseConfig(rawConfig)
	if err != nil {
		RecordReloadFailure()
		return err
	}
	if err := validateConfig(cfg); err != nil {
		RecordReloadFailure()
		return err
	}
	mutators := []PipelineRunMutator{}
	if len(cfg.CEL.Expressions) != 0 {
		programs, err := cel.CompileCELPrograms(cfg.CEL.Expressions)
		if err != nil {
			RecordReloadFailure()
			logger.Error(err, "failed to compile CEL programs")
			return err
		}
		mutators = append(mutators, cel.NewCELMutator(programs))
	}
	s.mutators = mutators
	s.config = &cfg
	RecordReloadSuccess()
	logger.Info("Updated config", "config", s.config)

	return nil
}

func validateConfig(config config.Config) error {
	if config.QueueName == "" {
		return errors.New("queue name is not set in the PipelineRunCustomDefaulter")
	}
	return nil
}

func parseConfig(raw []byte) (config.Config, error) {
	cfg := config.Config{}
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		// Log and keep last-known-good config
		logger.Error(err, "failed to parse config")
		return cfg, err
	}
	return cfg, nil
}
