/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package mutate

import (
	"context"
	"fmt"
	"os"
	"path"

	webhookv1 "github.com/konflux-ci/tekton-kueue/internal/webhook/v1"
	tekv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"sigs.k8s.io/yaml"
)

// MutatePipelineRun reads a PipelineRun from a file, applies mutations based on the config,
// and returns the mutated PipelineRun as YAML bytes.
func MutatePipelineRun(pipelineRunFile, configDir string) ([]byte, error) {
	// Validate inputs
	if pipelineRunFile == "" {
		return nil, fmt.Errorf("pipelineRunFile cannot be empty")
	}
	if configDir == "" {
		return nil, fmt.Errorf("configDir cannot be empty")
	}

	// Load PipelineRun from file
	pipelineRunData, err := os.ReadFile(pipelineRunFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read PipelineRun file %q: %w", pipelineRunFile, err)
	}

	// Parse PipelineRun YAML
	var pipelineRun tekv1.PipelineRun
	if err := yaml.Unmarshal(pipelineRunData, &pipelineRun); err != nil {
		return nil, fmt.Errorf("failed to parse PipelineRun YAML: %w", err)
	}

	// Load config and create defaulter
	cfgStore, err := LoadDefaulter(configDir)
	if err != nil {
		return nil, err
	}

	defaulter, err := webhookv1.NewCustomDefaulter(cfgStore)
	if err != nil {
		return nil, fmt.Errorf("failed to create custom defaulter: %w", err)
	}

	// Apply mutation
	ctx := context.Background()
	if err := defaulter.Default(ctx, &pipelineRun); err != nil {
		return nil, fmt.Errorf("failed to apply mutation to PipelineRun: %w", err)
	}

	// Marshal the mutated PipelineRun back to YAML
	mutatedData, err := yaml.Marshal(&pipelineRun)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal mutated PipelineRun to YAML: %w", err)
	}

	return mutatedData, nil
}

// LoadDefaulter loads the webhook configuration and returns a ConfigStore.
func LoadDefaulter(configDir string) (*webhookv1.ConfigStore, error) {
	if configDir == "" {
		return nil, fmt.Errorf("configDir cannot be empty")
	}

	configPath := path.Join(configDir, "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %q: %w", configPath, err)
	}

	cfgStore := &webhookv1.ConfigStore{}
	if err := cfgStore.Update(data); err != nil {
		return nil, fmt.Errorf("failed to update config: %w", err)
	}

	return cfgStore, nil
}
