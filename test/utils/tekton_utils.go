package utils

import (
	. "github.com/onsi/ginkgo/v2"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned/scheme"

	"k8s.io/apimachinery/pkg/runtime"
)

func MustParseV1PipelineRun(t GinkgoTInterface, yaml string) *v1.PipelineRun {
	var pr v1.PipelineRun
	yaml = `apiVersion: tekton.dev/v1
kind: PipelineRun
` + yaml
	mustParseYAML(t, yaml, &pr)
	return &pr
}
func mustParseYAML(t GinkgoTInterface, yaml string, i runtime.Object) {
	if _, _, err := scheme.Codecs.UniversalDeserializer().Decode([]byte(yaml), nil, i); err != nil {
		t.Fatalf("mustParseYAML (%s): %v", yaml, err)
	}
}
