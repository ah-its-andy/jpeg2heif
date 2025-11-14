package tests

import (
	"testing"

	"github.com/ah-its-andy/jpeg2heif/internal/workflow"
)

func TestCanConvertExtensions(t *testing.T) {
	yamlContent := `
name: test-workflow
description: Test workflow with extensions
runs-on: shell
timeout: 60

can_convert:
  extensions: [".jpg", ".jpeg"]

steps:
  - name: test-step
    run: echo "test"

outputs:
  output_file: "{{TMP_OUTPUT}}"
`

	spec, err := workflow.ParseWorkflow(yamlContent)
	if err != nil {
		t.Fatalf("Failed to parse workflow: %v", err)
	}

	// Validate
	errors := spec.Validate()
	if len(errors) > 0 {
		t.Fatalf("Validation failed: %v", errors)
	}

	// Check structure
	if spec.CanConvert == nil {
		t.Fatal("CanConvert should not be nil")
	}

	if len(spec.CanConvert.Extensions) != 2 {
		t.Fatalf("Expected 2 extensions, got %d", len(spec.CanConvert.Extensions))
	}

	if spec.CanConvert.Extensions[0] != ".jpg" {
		t.Errorf("Expected first extension to be .jpg, got %s", spec.CanConvert.Extensions[0])
	}

	if spec.CanConvert.Extensions[1] != ".jpeg" {
		t.Errorf("Expected second extension to be .jpeg, got %s", spec.CanConvert.Extensions[1])
	}
}

func TestCanConvertRun(t *testing.T) {
	yamlContent := `
name: test-workflow
description: Test workflow with run script
runs-on: shell
timeout: 60

can_convert:
  run: |
    file_type=$(file -b --mime-type "{{INPUT_FILE}}")
    [[ "$file_type" == "image/jpeg" ]] && exit 0 || exit 1
  timeout: 5

steps:
  - name: test-step
    run: echo "test"

outputs:
  output_file: "{{TMP_OUTPUT}}"
`

	spec, err := workflow.ParseWorkflow(yamlContent)
	if err != nil {
		t.Fatalf("Failed to parse workflow: %v", err)
	}

	// Validate
	errors := spec.Validate()
	if len(errors) > 0 {
		t.Fatalf("Validation failed: %v", errors)
	}

	// Check structure
	if spec.CanConvert == nil {
		t.Fatal("CanConvert should not be nil")
	}

	if spec.CanConvert.Run == "" {
		t.Fatal("Run script should not be empty")
	}

	if spec.CanConvert.Timeout != 5 {
		t.Errorf("Expected timeout to be 5, got %d", spec.CanConvert.Timeout)
	}
}

func TestCanConvertValidation(t *testing.T) {
	tests := []struct {
		name          string
		yaml          string
		expectErrors  bool
		errorContains string
	}{
		{
			name: "both extensions and run",
			yaml: `
name: test
runs-on: shell
can_convert:
  extensions: [".jpg"]
  run: echo "test"
steps:
  - name: test
    run: echo "test"
outputs:
  output_file: "{{TMP_OUTPUT}}"
`,
			expectErrors:  true,
			errorContains: "cannot specify both",
		},
		{
			name: "neither extensions nor run",
			yaml: `
name: test
runs-on: shell
can_convert:
  timeout: 5
steps:
  - name: test
    run: echo "test"
outputs:
  output_file: "{{TMP_OUTPUT}}"
`,
			expectErrors:  true,
			errorContains: "must specify either",
		},
		{
			name: "extension without dot",
			yaml: `
name: test
runs-on: shell
can_convert:
  extensions: ["jpg", ".png"]
steps:
  - name: test
    run: echo "test"
outputs:
  output_file: "{{TMP_OUTPUT}}"
`,
			expectErrors:  true,
			errorContains: "should start with '.'",
		},
		{
			name: "valid extensions",
			yaml: `
name: test
runs-on: shell
can_convert:
  extensions: [".jpg", ".png"]
steps:
  - name: test
    run: echo "test"
outputs:
  output_file: "{{TMP_OUTPUT}}"
`,
			expectErrors: false,
		},
		{
			name: "valid run with timeout",
			yaml: `
name: test
runs-on: shell
can_convert:
  run: echo "test"
  timeout: 10
steps:
  - name: test
    run: echo "test"
outputs:
  output_file: "{{TMP_OUTPUT}}"
`,
			expectErrors: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, err := workflow.ParseWorkflow(tt.yaml)
			if err != nil {
				t.Fatalf("Failed to parse workflow: %v", err)
			}

			errors := spec.Validate()
			hasErrors := len(errors) > 0

			if hasErrors != tt.expectErrors {
				t.Errorf("Expected errors: %v, got errors: %v (%v)", tt.expectErrors, hasErrors, errors)
			}

			if tt.expectErrors && tt.errorContains != "" {
				found := false
				for _, errMsg := range errors {
					if contains(errMsg, tt.errorContains) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorContains, errors)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
