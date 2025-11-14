package tests

import (
	"testing"

	"github.com/ah-its-andy/jpeg2heif/internal/workflow"
)

func TestParseWorkflow(t *testing.T) {
	yaml := `
name: test-workflow
description: "Test workflow"
runs-on: shell
timeout: 60

steps:
  - name: test-step
    run: echo "Hello {{INPUT_FILE}}"
    timeout: 30

outputs:
  output_file: "{{TMP_OUTPUT}}"
`

	spec, err := workflow.ParseWorkflow(yaml)
	if err != nil {
		t.Fatalf("Failed to parse workflow: %v", err)
	}

	if spec.Name != "test-workflow" {
		t.Errorf("Expected name 'test-workflow', got '%s'", spec.Name)
	}

	if len(spec.Steps) != 1 {
		t.Errorf("Expected 1 step, got %d", len(spec.Steps))
	}

	if spec.Steps[0].Name != "test-step" {
		t.Errorf("Expected step name 'test-step', got '%s'", spec.Steps[0].Name)
	}
}

func TestValidateWorkflow(t *testing.T) {
	tests := []struct {
		name        string
		spec        *workflow.WorkflowSpec
		expectValid bool
	}{
		{
			name: "valid workflow",
			spec: &workflow.WorkflowSpec{
				Name:   "valid-workflow",
				RunsOn: "shell",
				Steps: []workflow.Step{
					{Name: "step1", Run: "echo test"},
				},
				Outputs: map[string]string{"output_file": "{{TMP_OUTPUT}}"},
			},
			expectValid: true,
		},
		{
			name: "missing name",
			spec: &workflow.WorkflowSpec{
				RunsOn: "shell",
				Steps: []workflow.Step{
					{Name: "step1", Run: "echo test"},
				},
			},
			expectValid: false,
		},
		{
			name: "no steps",
			spec: &workflow.WorkflowSpec{
				Name:   "no-steps",
				RunsOn: "shell",
				Steps:  []workflow.Step{},
			},
			expectValid: false,
		},
		{
			name: "invalid runs-on",
			spec: &workflow.WorkflowSpec{
				Name:   "invalid-runner",
				RunsOn: "kubernetes",
				Steps: []workflow.Step{
					{Name: "step1", Run: "echo test"},
				},
			},
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := tt.spec.Validate()
			isValid := len(errors) == 0

			if isValid != tt.expectValid {
				t.Errorf("Expected valid=%v, got valid=%v (errors: %v)",
					tt.expectValid, isValid, errors)
			}
		})
	}
}

func TestGetVariables(t *testing.T) {
	spec := &workflow.WorkflowSpec{
		Name:   "test",
		RunsOn: "shell",
		Env: map[string]string{
			"QUALITY": "{{QUALITY}}",
		},
		Steps: []workflow.Step{
			{
				Name: "step1",
				Run:  "convert {{INPUT_FILE}} {{OUTPUT_FILE}}",
			},
		},
		Outputs: map[string]string{
			"output_file": "{{TMP_OUTPUT}}",
		},
	}

	vars := spec.GetVariables()

	expectedVars := map[string]bool{
		"QUALITY":     true,
		"INPUT_FILE":  true,
		"OUTPUT_FILE": true,
		"TMP_OUTPUT":  true,
	}

	for _, v := range vars {
		if !expectedVars[v] {
			t.Errorf("Unexpected variable: %s", v)
		}
		delete(expectedVars, v)
	}

	if len(expectedVars) > 0 {
		t.Errorf("Missing variables: %v", expectedVars)
	}
}
