package v1

import (
	"errors"
	"sync"

	"github.com/konflux-ci/tekton-kueue/internal/cel"
	"github.com/konflux-ci/tekton-kueue/pkg/config"
	"gopkg.in/yaml.v3"
)

type ConfigStore struct {
	mu       sync.RWMutex
	config   config.Config
	mutators []PipelineRunMutator
}

func (s *ConfigStore) GetConfig() config.Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.config
}

func (s *ConfigStore) GetMutators() []PipelineRunMutator {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.mutators
}

func (s *ConfigStore) Update(rawConfig []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg, err := parseConfig(rawConfig)
	if err != nil {
		return err
	}

	mutators := []PipelineRunMutator{}
	if len(cfg.CEL.Expressions) != 0 {
		programs, err := cel.CompileCELPrograms(cfg.CEL.Expressions)
		if err != nil {
			return err
		}
		mutators = append(mutators, cel.NewCELMutator(programs))
	}
	if err := validateConfig(cfg); err != nil {
		return err
	}
	s.mutators = mutators
	s.config = cfg
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
		return cfg, err
	}
	return cfg, nil
}
