package workflow

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// WorkflowSpec represents the YAML workflow specification
type WorkflowSpec struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	RunsOn      string            `yaml:"runs-on"`     // shell or docker
	Timeout     int               `yaml:"timeout"`     // seconds, global timeout
	CanConvert  *CanConvertSpec   `yaml:"can_convert"` // Optional: check if file is supported
	Env         map[string]string `yaml:"env"`
	Steps       []Step            `yaml:"steps"`
	Outputs     map[string]string `yaml:"outputs"`
}

// CanConvertSpec defines how to check if a file can be converted
type CanConvertSpec struct {
	Extensions []string `yaml:"extensions"` // File extensions (e.g., [".jpg", ".jpeg"])
	Run        string   `yaml:"run"`        // Script to execute (exitcode 0 = supported)
	Timeout    int      `yaml:"timeout"`    // Timeout in seconds (only for run)
}

// Step represents a single workflow step
type Step struct {
	Name    string            `yaml:"name"`
	Run     string            `yaml:"run"`
	Env     map[string]string `yaml:"env"`
	Workdir string            `yaml:"workdir"`
	Timeout int               `yaml:"timeout"` // seconds, step-level timeout
}

// ParseWorkflow parses YAML into WorkflowSpec
func ParseWorkflow(yamlContent string) (*WorkflowSpec, error) {
	spec := &WorkflowSpec{}
	if err := yaml.Unmarshal([]byte(yamlContent), spec); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return spec, nil
}

// Validate validates the workflow specification
func (spec *WorkflowSpec) Validate() []string {
	var errors []string

	// Validate required fields
	if spec.Name == "" {
		errors = append(errors, "workflow name is required")
	}

	if !isValidName(spec.Name) {
		errors = append(errors, "workflow name must be alphanumeric with hyphens/underscores")
	}

	if spec.RunsOn == "" {
		spec.RunsOn = "shell" // Default
	}

	if spec.RunsOn != "shell" && spec.RunsOn != "docker" {
		errors = append(errors, "runs-on must be 'shell' or 'docker'")
	}

	if spec.Timeout < 0 {
		errors = append(errors, "timeout must be non-negative")
	}

	// Validate can_convert if present
	if spec.CanConvert != nil {
		hasExtensions := len(spec.CanConvert.Extensions) > 0
		hasRun := spec.CanConvert.Run != ""

		if !hasExtensions && !hasRun {
			errors = append(errors, "can_convert: must specify either 'extensions' or 'run'")
		}

		if hasExtensions && hasRun {
			errors = append(errors, "can_convert: cannot specify both 'extensions' and 'run', choose one")
		}

		if hasExtensions {
			for i, ext := range spec.CanConvert.Extensions {
				if !strings.HasPrefix(ext, ".") {
					errors = append(errors, fmt.Sprintf("can_convert: extensions[%d] should start with '.' (got '%s')", i, ext))
				}
			}
		}

		if hasRun {
			if spec.CanConvert.Timeout < 0 {
				errors = append(errors, "can_convert: timeout must be non-negative")
			}
			if spec.CanConvert.Timeout == 0 {
				spec.CanConvert.Timeout = 10 // Default 10 seconds
			}
		}
	}

	// Validate steps
	if len(spec.Steps) == 0 {
		errors = append(errors, "at least one step is required")
	}

	for i, step := range spec.Steps {
		if step.Name == "" {
			errors = append(errors, fmt.Sprintf("step %d: name is required", i))
		}

		if step.Run == "" {
			errors = append(errors, fmt.Sprintf("step %d (%s): run command is required", i, step.Name))
		}

		if step.Timeout < 0 {
			errors = append(errors, fmt.Sprintf("step %d (%s): timeout must be non-negative", i, step.Name))
		}
	}

	// Validate template variables in outputs
	for key, value := range spec.Outputs {
		if !strings.Contains(value, "{{") {
			errors = append(errors, fmt.Sprintf("output '%s': value should contain template variable", key))
		}
	}

	return errors
}

// isValidName checks if name is valid (alphanumeric, hyphens, underscores)
func isValidName(name string) bool {
	match, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, name)
	return match
}

// GetVariables returns all template variables used in the workflow
func (spec *WorkflowSpec) GetVariables() []string {
	vars := make(map[string]bool)

	// Scan env
	for _, v := range spec.Env {
		extractVariables(v, vars)
	}

	// Scan steps
	for _, step := range spec.Steps {
		extractVariables(step.Run, vars)
		extractVariables(step.Workdir, vars)
		for _, v := range step.Env {
			extractVariables(v, vars)
		}
	}

	// Scan outputs
	for _, v := range spec.Outputs {
		extractVariables(v, vars)
	}

	// Convert to slice
	result := []string{}
	for k := range vars {
		result = append(result, k)
	}

	return result
}

// extractVariables finds all {{VAR}} patterns in a string
func extractVariables(text string, vars map[string]bool) {
	re := regexp.MustCompile(`\{\{([A-Z_][A-Z0-9_]*)\}\}`)
	matches := re.FindAllStringSubmatch(text, -1)
	for _, match := range matches {
		if len(match) > 1 {
			vars[match[1]] = true
		}
	}
}
