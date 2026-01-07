package v1

import (
	"errors"
	"sync"

	"github.com/konflux-ci/tekton-kueue/internal/cel"
	"github.com/konflux-ci/tekton-kueue/internal/config"
	tekv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	logger = ctrl.Log.WithName("config-store")
)

type ConfigStore struct {
	mu       sync.RWMutex
	config   *config.Config
	mutators []PipelineRunMutator
}

type PipelineRunMutator interface {
	Mutate(*tekv1.PipelineRun) error
}

func (s *ConfigStore) GetConfigAndMutators() (*config.Config, []PipelineRunMutator) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config, s.mutators
}

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
