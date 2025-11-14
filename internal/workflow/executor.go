package workflow

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ExecutionContext holds runtime context for workflow execution
type ExecutionContext struct {
	WorkflowName string
	InputFile    string
	OutputFile   string
	TempDir      string
	Variables    map[string]string
	Quality      int
}

// ExecutionResult holds the result of workflow execution
type ExecutionResult struct {
	Success           bool
	ExitCode          int
	Duration          time.Duration
	Stdout            string
	Stderr            string
	Logs              string
	StepResults       []StepResult
	MetadataPreserved bool
	MetadataSummary   string
	OutputFiles       map[string]string
}

// StepResult holds the result of a single step
type StepResult struct {
	StepName  string
	Success   bool
	ExitCode  int
	Duration  time.Duration
	Stdout    string
	Stderr    string
	Error     string
	StartTime time.Time
	EndTime   time.Time
}

// Executor executes workflows
type Executor struct {
	spec    *WorkflowSpec
	ctx     context.Context
	execCtx *ExecutionContext
	result  *ExecutionResult
	logBuf  *bytes.Buffer
}

// NewExecutor creates a new workflow executor
func NewExecutor(spec *WorkflowSpec, ctx context.Context, execCtx *ExecutionContext) *Executor {
	return &Executor{
		spec:    spec,
		ctx:     ctx,
		execCtx: execCtx,
		result: &ExecutionResult{
			StepResults: []StepResult{},
			OutputFiles: make(map[string]string),
		},
		logBuf: &bytes.Buffer{},
	}
}

// Execute runs the workflow
func (e *Executor) Execute() (*ExecutionResult, error) {
	startTime := time.Now()
	e.log("=== Workflow Execution Started: %s ===\n", e.spec.Name)
	e.log("Workflow Type: YAML-based workflow converter\n")
	e.log("Start Time: %s\n", startTime.Format(time.RFC3339))
	e.log("Input File: %s\n", e.execCtx.InputFile)
	e.log("Output File: %s\n", e.execCtx.OutputFile)
	e.log("Temp Dir: %s\n", e.execCtx.TempDir)
	e.log("Quality: %d\n", e.execCtx.Quality)
	e.log("\n")

	// Setup context with global timeout
	ctx := e.ctx
	if e.spec.Timeout > 0 {
		e.log("Global timeout: %d seconds\n", e.spec.Timeout)
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(e.ctx, time.Duration(e.spec.Timeout)*time.Second)
		defer cancel()
	} else {
		e.log("No global timeout set\n")
	}
	e.log("\n")

	// Prepare variables
	e.log("=== Preparing Template Variables ===\n")
	if err := e.prepareVariables(); err != nil {
		e.log("❌ ERROR: Failed to prepare variables: %v\n", err)
		e.result.Success = false
		e.result.ExitCode = 1
		e.result.Logs = e.logBuf.String()
		e.result.Duration = time.Since(startTime)
		return e.result, err
	}
	e.log("✅ Variables prepared successfully\n\n")

	// Execute steps
	e.log("=== Executing Workflow Steps ===\n")
	e.log("Total steps: %d\n\n", len(e.spec.Steps))

	for i, step := range e.spec.Steps {
		e.log("╔════════════════════════════════════════════════════════════════\n")
		e.log("║ Step %d/%d: %s\n", i+1, len(e.spec.Steps), step.Name)
		e.log("╚════════════════════════════════════════════════════════════════\n")

		stepResult := e.executeStep(ctx, &step)
		e.result.StepResults = append(e.result.StepResults, stepResult)

		if stepResult.Success {
			e.log("✅ Step completed successfully\n")
		} else {
			e.log("❌ Step failed\n")
		}
		e.log("   Exit code: %d\n", stepResult.ExitCode)
		e.log("   Duration: %v\n", stepResult.Duration)
		if stepResult.Error != "" {
			e.log("   Error: %s\n", stepResult.Error)
		}
		e.log("\n")

		if !stepResult.Success {
			e.log("❌ ERROR: Step '%s' failed, aborting workflow\n", stepResult.StepName)
			e.log("   Exit code: %d\n", stepResult.ExitCode)
			e.log("   Duration: %v\n", stepResult.Duration)
			e.result.Success = false
			e.result.ExitCode = stepResult.ExitCode
			e.result.Logs = e.logBuf.String()
			e.result.Duration = time.Since(startTime)
			return e.result, fmt.Errorf("step '%s' failed with exit code %d: %s", stepResult.StepName, stepResult.ExitCode, stepResult.Error)
		}
	}

	// Handle outputs
	e.log("\n=== Processing Outputs ===\n")
	if err := e.handleOutputs(); err != nil {
		e.log("❌ ERROR: Failed to handle outputs: %v\n", err)
		e.result.Success = false
		e.result.ExitCode = 1
		e.result.Logs = e.logBuf.String()
		e.result.Duration = time.Since(startTime)
		return e.result, err
	}
	e.log("✅ Outputs processed successfully\n\n")

	// Extract metadata
	e.log("=== Extracting Metadata ===\n")
	e.extractMetadata()

	endTime := time.Now()
	e.log("\n=== Workflow Execution Completed Successfully ===\n")
	e.log("End Time: %s\n", endTime.Format(time.RFC3339))
	e.log("Total Duration: %v\n", time.Since(startTime))
	e.log("Status: SUCCESS ✅\n")

	e.result.Success = true
	e.result.ExitCode = 0
	e.result.Logs = e.logBuf.String()
	e.result.Duration = time.Since(startTime)

	return e.result, nil
}

// executeStep executes a single step
func (e *Executor) executeStep(ctx context.Context, step *Step) StepResult {
	result := StepResult{
		StepName:  step.Name,
		StartTime: time.Now(),
	}

	e.log("Step: %s\n", step.Name)
	e.log("Start time: %s\n", result.StartTime.Format(time.RFC3339))

	// Apply step timeout
	if step.Timeout > 0 {
		e.log("Step timeout: %d seconds\n", step.Timeout)
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(step.Timeout)*time.Second)
		defer cancel()
	} else {
		e.log("No step timeout set\n")
	}

	// Replace variables in command
	command := e.replaceVariables(step.Run)
	e.log("\n📝 Command to execute:\n")
	e.log("─────────────────────────────────────────────────────────\n")
	e.log("%s\n", command)
	e.log("─────────────────────────────────────────────────────────\n")

	// Determine working directory
	workdir := e.execCtx.TempDir
	if step.Workdir != "" {
		workdir = e.replaceVariables(step.Workdir)
		// Remove shell escaping from path
		workdir = strings.Trim(workdir, "'")
	}
	e.log("Working directory: %s\n", workdir)

	// Ensure working directory exists
	if err := os.MkdirAll(workdir, 0755); err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to create working directory: %v", err)
		result.ExitCode = 1
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		e.log("❌ Failed to create working directory: %v\n", err)
		return result
	}
	e.log("✅ Working directory verified\n\n")

	// Log environment variables if any
	if len(e.spec.Env) > 0 || len(step.Env) > 0 {
		e.log("Environment variables:\n")
		for k, v := range e.spec.Env {
			e.log("  %s=%s\n", k, e.replaceVariables(v))
		}
		for k, v := range step.Env {
			e.log("  %s=%s\n", k, e.replaceVariables(v))
		}
		e.log("\n")
	}

	// Execute command
	e.log("🚀 Executing command...\n\n")
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)
	cmd.Dir = workdir

	// Merge environment variables
	cmd.Env = os.Environ()
	for k, v := range e.spec.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, e.replaceVariables(v)))
	}
	for k, v := range step.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, e.replaceVariables(v)))
	}

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run command
	err := cmd.Run()
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	result.Stdout = stdout.String()
	result.Stderr = stderr.String()

	// Log output
	e.log("📤 Command output:\n")
	if result.Stdout != "" {
		e.log("\n╔═══ STDOUT ═══════════════════════════════════════════════╗\n")
		e.log("%s", result.Stdout)
		if !strings.HasSuffix(result.Stdout, "\n") {
			e.log("\n")
		}
		e.log("╚══════════════════════════════════════════════════════════╝\n")
	} else {
		e.log("(no stdout output)\n")
	}

	if result.Stderr != "" {
		e.log("\n╔═══ STDERR ═══════════════════════════════════════════════╗\n")
		e.log("%s", result.Stderr)
		if !strings.HasSuffix(result.Stderr, "\n") {
			e.log("\n")
		}
		e.log("╚══════════════════════════════════════════════════════════╝\n")
	} else if result.Stdout == "" {
		e.log("(no stderr output)\n")
	}
	e.log("\n")

	// Check result
	if err != nil {
		result.Success = false
		result.Error = err.Error()
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = 1
		}
		e.log("❌ Command failed: %s\n", err.Error())
		if ctx.Err() == context.DeadlineExceeded {
			e.log("⏱️  Timeout exceeded\n")
		}
	} else {
		result.Success = true
		result.ExitCode = 0
		e.log("✅ Command succeeded\n")
	}

	e.log("End time: %s\n", result.EndTime.Format(time.RFC3339))
	e.log("Duration: %v\n", result.Duration)

	return result
}

// prepareVariables prepares all template variables
func (e *Executor) prepareVariables() error {
	vars := e.execCtx.Variables
	if vars == nil {
		vars = make(map[string]string)
		e.execCtx.Variables = vars
	}

	// Calculate file MD5
	md5Hash, err := calculateFileMD5(e.execCtx.InputFile)
	if err != nil {
		return fmt.Errorf("failed to calculate MD5: %w", err)
	}

	// Standard variables
	inputDir := filepath.Dir(e.execCtx.InputFile)
	parentDir := filepath.Dir(inputDir)
	basename := filepath.Base(e.execCtx.InputFile)
	ext := strings.ToLower(filepath.Ext(basename))
	if len(ext) > 0 && ext[0] == '.' {
		ext = ext[1:]
	}
	basenameNoExt := strings.TrimSuffix(basename, filepath.Ext(basename))

	tmpOutput := filepath.Join(e.execCtx.TempDir, filepath.Base(e.execCtx.OutputFile))

	vars["INPUT_FILE"] = e.execCtx.InputFile
	vars["INPUT_DIR"] = inputDir
	vars["INPUT_BASENAME"] = basenameNoExt
	vars["INPUT_FILE_EXT"] = ext
	vars["PARENT_DIR"] = parentDir
	vars["OUTPUT_FILE"] = e.execCtx.OutputFile
	vars["TMP_DIR"] = e.execCtx.TempDir
	vars["TMP_OUTPUT"] = tmpOutput
	vars["FILE_MD5"] = md5Hash
	vars["TIMESTAMP"] = fmt.Sprintf("%d", time.Now().Unix())
	vars["QUALITY"] = fmt.Sprintf("%d", e.execCtx.Quality)
	vars["CONVERT_QUALITY"] = fmt.Sprintf("%d", e.execCtx.Quality)

	e.log("Variables:\n")
	for k, v := range vars {
		e.log("  %s = %s\n", k, v)
	}
	e.log("\n")

	return nil
}

// replaceVariables replaces {{VAR}} with actual values
func (e *Executor) replaceVariables(text string) string {
	result := text

	// Use regex to find and replace variables
	re := regexp.MustCompile(`\{\{([A-Z_][A-Z0-9_]*)\}\}`)
	result = re.ReplaceAllStringFunc(result, func(match string) string {
		varName := match[2 : len(match)-2] // Remove {{ and }}
		if value, ok := e.execCtx.Variables[varName]; ok {
			// Shell-escape the value
			return shellEscape(value)
		}
		return match // Keep original if not found
	})

	return result
}

// shellEscape escapes a string for safe use in shell commands
func shellEscape(s string) string {
	// Simple escape: wrap in single quotes and escape single quotes
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// handleOutputs processes workflow outputs
func (e *Executor) handleOutputs() error {
	for key, value := range e.spec.Outputs {
		outputPath := e.replaceVariables(value)
		// Remove shell escaping from path
		outputPath = strings.Trim(outputPath, "'")

		e.log("Output '%s': %s\n", key, outputPath)
		e.result.OutputFiles[key] = outputPath

		// For primary output, copy to final destination
		if key == "output_file" {
			if err := copyFile(outputPath, e.execCtx.OutputFile); err != nil {
				return fmt.Errorf("failed to copy output file: %w", err)
			}
			e.log("Copied output to: %s\n", e.execCtx.OutputFile)
		}
	}

	return nil
}

// extractMetadata extracts metadata from the output file
func (e *Executor) extractMetadata() {
	// Try to extract EXIF DateTimeOriginal using exiftool
	cmd := exec.Command("exiftool", "-DateTimeOriginal", "-s", "-s", "-s", e.execCtx.OutputFile)
	output, err := cmd.Output()
	if err == nil && len(output) > 0 {
		dateTime := strings.TrimSpace(string(output))
		e.result.MetadataPreserved = true
		e.result.MetadataSummary = fmt.Sprintf("DateTimeOriginal: %s", dateTime)
		e.log("Metadata preserved: %s\n", e.result.MetadataSummary)
	} else {
		e.result.MetadataPreserved = false
		e.result.MetadataSummary = "No EXIF metadata found"
		e.log("Metadata: %s\n", e.result.MetadataSummary)
	}
}

// log writes to the execution log
func (e *Executor) log(format string, args ...interface{}) {
	fmt.Fprintf(e.logBuf, format, args...)
}

// calculateFileMD5 calculates MD5 hash of a file
func calculateFileMD5(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	// Ensure destination directory exists
	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return err
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	return dstFile.Sync()
}

// CanConvertCheck checks if the workflow can convert the given file
func (e *Executor) CanConvertCheck(filePath string) (bool, error) {
	if e.spec.CanConvert == nil {
		// No can_convert specified, assume it can convert
		return true, nil
	}

	// Method 1: Check by extensions
	if len(e.spec.CanConvert.Extensions) > 0 {
		ext := strings.ToLower(filepath.Ext(filePath))
		for _, allowedExt := range e.spec.CanConvert.Extensions {
			if strings.ToLower(allowedExt) == ext {
				return true, nil
			}
		}
		return false, nil
	}

	// Method 2: Execute run script
	if e.spec.CanConvert.Run != "" {
		// Prepare variables for can_convert check
		if err := e.prepareVariables(); err != nil {
			return false, fmt.Errorf("failed to prepare variables: %w", err)
		}

		// Apply timeout
		timeout := 10 * time.Second
		if e.spec.CanConvert.Timeout > 0 {
			timeout = time.Duration(e.spec.CanConvert.Timeout) * time.Second
		}

		ctx, cancel := context.WithTimeout(e.ctx, timeout)
		defer cancel()

		// Replace variables in command
		command := e.replaceVariables(e.spec.CanConvert.Run)

		// Execute command
		cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)
		cmd.Dir = e.execCtx.TempDir

		// Set environment
		cmd.Env = os.Environ()
		for k, v := range e.spec.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, e.replaceVariables(v)))
		}

		// Run and check exit code
		err := cmd.Run()
		if err != nil {
			if _, ok := err.(*exec.ExitError); ok {
				// Non-zero exit code means not supported
				return false, nil
			}
			// Other errors (timeout, not found, etc.)
			return false, fmt.Errorf("failed to execute can_convert check: %w", err)
		}

		// Exit code 0 means supported
		return true, nil
	}

	// No check method specified, assume can convert
	return true, nil
}
