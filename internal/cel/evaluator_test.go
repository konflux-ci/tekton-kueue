package cel

import (
	"errors"
	"testing"

	. "github.com/onsi/gomega"
	tekv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCompiledProgram_Evaluate_TypeSafety(t *testing.T) {
	// Create a sample PipelineRun
	pipelineRun := &tekv1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipeline",
			Namespace: "test-namespace",
		},
		Spec: tekv1.PipelineRunSpec{
			PipelineRef: &tekv1.PipelineRef{
				Name: "test-pipeline",
			},
		},
	}

	tests := []struct {
		name            string
		expression      string
		pipelineRun     *tekv1.PipelineRun
		expected        []MutationRequest
		expectErr       bool
		expectedErrType any // target for errors.As (e.g., new(*EvaluationError))
		errMsg          string
	}{
		{
			name:        "valid single annotation",
			expression:  `annotation("test-key", "test-value")`,
			pipelineRun: pipelineRun,
			expected: []MutationRequest{
				{Type: MutationTypeAnnotation, Key: "test-key", Value: "test-value"},
			},
		},
		{
			name:        "valid single label",
			expression:  `label("env", "production")`,
			pipelineRun: pipelineRun,
			expected: []MutationRequest{
				{Type: MutationTypeLabel, Key: "env", Value: "production"},
			},
		},
		{
			name:        "valid list of mutations",
			expression:  `[annotation("key1", "value1"), label("key2", "value2")]`,
			pipelineRun: pipelineRun,
			expected: []MutationRequest{
				{Type: MutationTypeAnnotation, Key: "key1", Value: "value1"},
				{Type: MutationTypeLabel, Key: "key2", Value: "value2"},
			},
		},
		{
			name:        "nil PipelineRun",
			expression:  `annotation("test-key", "test-value")`,
			pipelineRun: nil,
			expectErr:   true,
			errMsg:      "pipelineRun cannot be nil",
		},
		{
			name:            "runtime error - empty key",
			expression:      `annotation("", "test-value")`,
			pipelineRun:     pipelineRun,
			expectErr:       true,
			expectedErrType: new(*EvaluationError),
			errMsg:          "annotation key cannot be empty",
		},
		{
			name:            "runtime error - empty value causes validation failure",
			expression:      `annotation("test-key", "")`,
			pipelineRun:     pipelineRun,
			expectErr:       true,
			expectedErrType: new(*EvaluationError),
			errMsg:          "mutation value cannot be empty",
		},
		{
			name:        "plrNamespace variable",
			expression:  `annotation("namespace", plrNamespace)`,
			pipelineRun: pipelineRun,
			expected: []MutationRequest{
				{Type: MutationTypeAnnotation, Key: "namespace", Value: "test-namespace"},
			},
		},
		{
			name:       "pacEventType variable with existing label",
			expression: `annotation("event-type", pacEventType)`,
			pipelineRun: &tekv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pipeline",
					Namespace: "test-namespace",
					Labels: map[string]string{
						"pipelinesascode.tekton.dev/event-type": "push",
					},
				},
				Spec: tekv1.PipelineRunSpec{
					PipelineRef: &tekv1.PipelineRef{
						Name: "test-pipeline",
					},
				},
			},
			expected: []MutationRequest{
				{Type: MutationTypeAnnotation, Key: "event-type", Value: "push"},
			},
		},
		{
			name:            "pacEventType variable with missing label",
			expression:      `annotation("event-type", pacEventType)`,
			pipelineRun:     pipelineRun,
			expectErr:       true,
			expectedErrType: new(*EvaluationError),
			errMsg:          "mutation value cannot be empty",
		},
		{
			name:       "pacTestEventType variable with existing label",
			expression: `annotation("test-event-type", pacTestEventType)`,
			pipelineRun: &tekv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pipeline",
					Namespace: "test-namespace",
					Labels: map[string]string{
						"pac.test.appstudio.openshift.io/event-type": "unit-test",
					},
				},
				Spec: tekv1.PipelineRunSpec{
					PipelineRef: &tekv1.PipelineRef{
						Name: "test-pipeline",
					},
				},
			},
			expected: []MutationRequest{
				{Type: MutationTypeAnnotation, Key: "test-event-type", Value: "unit-test"},
			},
		},
		{
			name:            "pacTestEventType variable with missing label",
			expression:      `annotation("test-event-type", pacTestEventType)`,
			pipelineRun:     pipelineRun,
			expectErr:       true,
			expectedErrType: new(*EvaluationError),
			errMsg:          "mutation value cannot be empty",
		},
		{
			name:        "safe pacEventType usage with conditional",
			expression:  `pacEventType != "" ? annotation("event-type", pacEventType) : annotation("event-type", "unknown")`,
			pipelineRun: pipelineRun,
			expected: []MutationRequest{
				{Type: MutationTypeAnnotation, Key: "event-type", Value: "unknown"},
			},
		},
		{
			name:        "safe pacTestEventType usage with conditional",
			expression:  `pacTestEventType != "" ? annotation("test-event-type", pacTestEventType) : annotation("test-event-type", "none")`,
			pipelineRun: pipelineRun,
			expected: []MutationRequest{
				{Type: MutationTypeAnnotation, Key: "test-event-type", Value: "none"},
			},
		},
		{
			name:        "conditional expression with plrNamespace",
			expression:  `plrNamespace == "test-namespace" ? label("env", "testing") : label("env", "other")`,
			pipelineRun: pipelineRun,
			expected: []MutationRequest{
				{Type: MutationTypeLabel, Key: "env", Value: "testing"},
			},
		},
		{
			name:       "conditional expression with pacEventType",
			expression: `pacEventType == "push" ? label("trigger", "push-event") : label("trigger", "other-event")`,
			pipelineRun: &tekv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pipeline",
					Namespace: "test-namespace",
					Labels: map[string]string{
						"pipelinesascode.tekton.dev/event-type": "push",
					},
				},
				Spec: tekv1.PipelineRunSpec{
					PipelineRef: &tekv1.PipelineRef{
						Name: "test-pipeline",
					},
				},
			},
			expected: []MutationRequest{
				{Type: MutationTypeLabel, Key: "trigger", Value: "push-event"},
			},
		},
		{
			name:       "conditional expression with pacTestEventType",
			expression: `pacTestEventType != "" ? label("test-type", pacTestEventType) : label("test-type", "none")`,
			pipelineRun: &tekv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pipeline",
					Namespace: "test-namespace",
					Labels: map[string]string{
						"pac.test.appstudio.openshift.io/event-type": "integration-test",
					},
				},
				Spec: tekv1.PipelineRunSpec{
					PipelineRef: &tekv1.PipelineRef{
						Name: "test-pipeline",
					},
				},
			},
			expected: []MutationRequest{
				{Type: MutationTypeLabel, Key: "test-type", Value: "integration-test"},
			},
		},
		{
			name:       "multiple variables in one expression",
			expression: `[annotation("namespace", plrNamespace), annotation("event", pacEventType), annotation("test-event", pacTestEventType)]`,
			pipelineRun: &tekv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pipeline",
					Namespace: "test-namespace",
					Labels: map[string]string{
						"pipelinesascode.tekton.dev/event-type":      "pull_request",
						"pac.test.appstudio.openshift.io/event-type": "e2e-test",
					},
				},
				Spec: tekv1.PipelineRunSpec{
					PipelineRef: &tekv1.PipelineRef{
						Name: "test-pipeline",
					},
				},
			},
			expected: []MutationRequest{
				{Type: MutationTypeAnnotation, Key: "namespace", Value: "test-namespace"},
				{Type: MutationTypeAnnotation, Key: "event", Value: "pull_request"},
				{Type: MutationTypeAnnotation, Key: "test-event", Value: "e2e-test"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			programs, err := CompileCELPrograms([]string{tt.expression})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(programs).To(HaveLen(1))

			mutations, err := programs[0].Evaluate(tt.pipelineRun)

			if tt.expectErr {
				g.Expect(err).To(HaveOccurred())
				if tt.errMsg != "" {
					g.Expect(err.Error()).To(ContainSubstring(tt.errMsg))
				}
				if tt.expectedErrType != nil {
					g.Expect(errors.As(err, tt.expectedErrType)).To(BeTrue(), "expected %T, got %T: %v", tt.expectedErrType, err, err)
				}
				return
			}

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(mutations).To(HaveLen(len(tt.expected)))

			for i, expected := range tt.expected {
				g.Expect(mutations[i].Type).To(Equal(expected.Type))
				g.Expect(mutations[i].Key).To(Equal(expected.Key))
				g.Expect(mutations[i].Value).To(Equal(expected.Value))
			}
		})
	}
}
