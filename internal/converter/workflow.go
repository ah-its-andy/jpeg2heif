package converter

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ah-its-andy/jpeg2heif/internal/db"
	"github.com/ah-its-andy/jpeg2heif/internal/workflow"
)

// WorkflowConverter implements converter using YAML workflows
type WorkflowConverter struct {
	workflow *db.Workflow
	spec     *workflow.WorkflowSpec
	database *db.DB
}

// NewWorkflowConverter creates a new workflow converter
func NewWorkflowConverter(wf *db.Workflow, database *db.DB) (*WorkflowConverter, error) {
	// Parse workflow YAML
	spec, err := workflow.ParseWorkflow(wf.YAML)
	if err != nil {
		return nil, fmt.Errorf("failed to parse workflow: %w", err)
	}

	// Validate workflow
	if errors := spec.Validate(); len(errors) > 0 {
		return nil, fmt.Errorf("workflow validation failed: %v", errors)
	}

	return &WorkflowConverter{
		workflow: wf,
		spec:     spec,
		database: database,
	}, nil
}

// Name returns the converter name
func (c *WorkflowConverter) Name() string {
	return fmt.Sprintf("workflow:%s", c.workflow.Name)
}

// TargetFormat returns the target format (extracted from workflow name or defaults)
func (c *WorkflowConverter) TargetFormat() string {
	// Try to extract format from workflow name (e.g., "jpeg-to-heic" -> "heic")
	parts := strings.Split(c.workflow.Name, "-to-")
	if len(parts) == 2 {
		return strings.TrimSpace(parts[1])
	}

	// Check outputs for clues
	if outputFile, ok := c.spec.Outputs["output_file"]; ok {
		if strings.Contains(outputFile, ".heic") || strings.Contains(outputFile, "HEIC") {
			return "heic"
		}
		if strings.Contains(outputFile, ".avif") || strings.Contains(outputFile, "AVIF") {
			return "avif"
		}
	}

	return "unknown"
}

// CanConvert checks if this converter can handle the input file
func (c *WorkflowConverter) CanConvert(srcPath string, srcMime string) bool {
	// Create a temporary execution context for checking
	tmpDir, err := os.MkdirTemp("", "workflow-check-*")
	if err != nil {
		return false
	}
	defer os.RemoveAll(tmpDir)

	execCtx := &workflow.ExecutionContext{
		WorkflowName: c.workflow.Name,
		InputFile:    srcPath,
		OutputFile:   filepath.Join(tmpDir, "output"),
		TempDir:      tmpDir,
		Quality:      80, // Default quality for check
		Variables:    make(map[string]string),
	}

	// Create executor and perform can_convert check
	executor := workflow.NewExecutor(c.spec, context.Background(), execCtx)
	canConvert, err := executor.CanConvertCheck(srcPath)
	if err != nil {
		// Log error but return false
		fmt.Printf("Warning: can_convert check failed for workflow '%s': %v\n", c.workflow.Name, err)
		return false
	}

	return canConvert
}

// Convert performs the conversion
func (c *WorkflowConverter) Convert(ctx context.Context, srcPath, dstPath string, opts ConvertOptions) (MetaResult, error) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp(opts.TempDir, "workflow-*")
	if err != nil {
		return MetaResult{}, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create execution context
	execCtx := &workflow.ExecutionContext{
		WorkflowName: c.workflow.Name,
		InputFile:    srcPath,
		OutputFile:   dstPath,
		TempDir:      tmpDir,
		Quality:      opts.Quality,
		Variables:    make(map[string]string),
	}

	// Add any custom variables from opts (future extension)
	// For now, standard variables are auto-populated

	// Create workflow run record
	run := &db.WorkflowRun{
		WorkflowID:   c.workflow.ID,
		WorkflowName: c.workflow.Name,
		FilePath:     srcPath,
		Status:       "running",
		StartTime:    time.Now(),
		JobParams:    fmt.Sprintf(`{"quality": %d}`, opts.Quality),
	}

	if err := c.database.CreateWorkflowRun(run); err != nil {
		return MetaResult{}, fmt.Errorf("failed to create workflow run: %w", err)
	}

	// Execute workflow
	executor := workflow.NewExecutor(c.spec, ctx, execCtx)
	result, execErr := executor.Execute()

	// Update run record
	endTime := time.Now()
	run.EndTime = &endTime
	run.DurationMs = result.Duration.Milliseconds()
	run.Stdout = combineStepOutputs(result.StepResults, true)
	run.Stderr = combineStepOutputs(result.StepResults, false)
	run.Logs = result.Logs
	run.MetadataPreserved = result.MetadataPreserved
	run.MetadataSummary = result.MetadataSummary

	if execErr != nil {
		run.Status = "failed"
		exitCode := result.ExitCode
		run.ExitCode = &exitCode
	} else {
		run.Status = "success"
		exitCode := 0
		run.ExitCode = &exitCode
	}

	if err := c.database.UpdateWorkflowRun(run); err != nil {
		// Log error but don't fail conversion
		fmt.Printf("Warning: failed to update workflow run: %v\n", err)
	}

	// Return result
	metaResult := MetaResult{
		MetadataPreserved: result.MetadataPreserved,
		MetadataSummary:   result.MetadataSummary,
		ConversionLog:     result.Logs,
	}

	if execErr != nil {
		return metaResult, execErr
	}

	return metaResult, nil
}

// combineStepOutputs combines stdout or stderr from all steps
func combineStepOutputs(steps []workflow.StepResult, stdout bool) string {
	var builder strings.Builder
	for _, step := range steps {
		if stdout && step.Stdout != "" {
			builder.WriteString(fmt.Sprintf("=== %s (stdout) ===\n%s\n", step.StepName, step.Stdout))
		} else if !stdout && step.Stderr != "" {
			builder.WriteString(fmt.Sprintf("=== %s (stderr) ===\n%s\n", step.StepName, step.Stderr))
		}
	}
	return builder.String()
}

// LoadWorkflowConverters sets up the database for workflow converter lookup
func LoadWorkflowConverters(database *db.DB) error {
	// Set the database for runtime workflow lookup
	SetDatabase(database)

	// Validate that workflows can be loaded
	workflows, err := database.ListWorkflows(1000, 0)
	if err != nil {
		return fmt.Errorf("failed to list workflows: %w", err)
	}

	// Log available workflows
	enabledCount := 0
	for _, wf := range workflows {
		if wf.Enabled {
			enabledCount++
			fmt.Printf("Available workflow converter: workflow:%s (enabled)\n", wf.Name)
		} else {
			fmt.Printf("Available workflow converter: workflow:%s (disabled)\n", wf.Name)
		}
	}

	fmt.Printf("Loaded %d enabled workflow converters from database\n", enabledCount)
	return nil
}
