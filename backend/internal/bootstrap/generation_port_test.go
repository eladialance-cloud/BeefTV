package bootstrap

import (
	"os"
	"strings"
	"testing"
)

func TestRuntimeWiresCreationPromptAwareGenerationPort(t *testing.T) {
	raw, err := os.ReadFile("runtime.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	if !strings.Contains(source, "Generation: creationAwareGenerationPort{service: svc}") {
		t.Fatal("runtime must route /tasks through CreateLocalTask so private creation prompts are compiled before admission")
	}
}
