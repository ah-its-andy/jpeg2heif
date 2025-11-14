package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/ah-its-andy/jpeg2heif/internal/converter"
	"github.com/ah-its-andy/jpeg2heif/internal/db"
	"github.com/ah-its-andy/jpeg2heif/internal/workflow"
)

// handleWorkflows handles GET /api/workflows - list all workflows
func (s *Server) handleWorkflows(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		s.listWorkflows(w, r)
	} else if r.Method == http.MethodPost {
		s.createWorkflow(w, r)
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleWorkflowDetail handles GET/PUT/DELETE /api/workflows/{id}
func (s *Server) handleWorkflowDetail(w http.ResponseWriter, r *http.Request) {
	// Extract ID from path
	id := strings.TrimPrefix(r.URL.Path, "/api/workflows/")
	// Remove any suffix
	if idx := strings.Index(id, "/"); idx != -1 {
		id = id[:idx]
	}

	workflowID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		http.Error(w, "Invalid workflow ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.getWorkflow(w, r, workflowID)
	case http.MethodPut:
		s.updateWorkflow(w, r, workflowID)
	case http.MethodDelete:
		s.deleteWorkflow(w, r, workflowID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// listWorkflows lists all workflows with pagination
func (s *Server) listWorkflows(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	if limit <= 0 {
		limit = 100
	}

	workflows, err := s.db.ListWorkflows(limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, workflows)
}

// getWorkflow gets a single workflow
func (s *Server) getWorkflow(w http.ResponseWriter, r *http.Request, id int64) {
	wf, err := s.db.GetWorkflow(id)
	if err != nil {
		http.Error(w, "Workflow not found", http.StatusNotFound)
		return
	}

	respondJSON(w, wf)
}

// createWorkflow creates a new workflow
func (s *Server) createWorkflow(w http.ResponseWriter, r *http.Request) {
	var wf db.Workflow
	if err := json.NewDecoder(r.Body).Decode(&wf); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate workflow YAML
	spec, err := workflow.ParseWorkflow(wf.YAML)
	if err != nil {
		respondJSON(w, map[string]interface{}{
			"error":  "Invalid YAML",
			"detail": err.Error(),
		})
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if errors := spec.Validate(); len(errors) > 0 {
		respondJSON(w, map[string]interface{}{
			"error":  "Validation failed",
			"errors": errors,
		})
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Create workflow
	if err := s.db.CreateWorkflow(&wf); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// If enabled, load as converter
	if wf.Enabled {
		conv, err := converter.NewWorkflowConverter(&wf, s.db)
		if err == nil {
			converter.Register(conv)
		}
	}

	respondJSON(w, wf)
}

// updateWorkflow updates an existing workflow
func (s *Server) updateWorkflow(w http.ResponseWriter, r *http.Request, id int64) {
	var wf db.Workflow
	if err := json.NewDecoder(r.Body).Decode(&wf); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	wf.ID = id

	// Validate workflow YAML
	spec, err := workflow.ParseWorkflow(wf.YAML)
	if err != nil {
		respondJSON(w, map[string]interface{}{
			"error":  "Invalid YAML",
			"detail": err.Error(),
		})
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if errors := spec.Validate(); len(errors) > 0 {
		respondJSON(w, map[string]interface{}{
			"error":  "Validation failed",
			"errors": errors,
		})
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Update workflow
	if err := s.db.UpdateWorkflow(&wf); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, wf)
}

// deleteWorkflow deletes a workflow
func (s *Server) deleteWorkflow(w http.ResponseWriter, r *http.Request, id int64) {
	if err := s.db.DeleteWorkflow(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]string{"status": "deleted"})
}

// handleWorkflowValidate handles POST /api/workflows/{id}/validate
func (s *Server) handleWorkflowValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		YAML string `json:"yaml"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Parse and validate
	spec, err := workflow.ParseWorkflow(req.YAML)
	if err != nil {
		respondJSON(w, map[string]interface{}{
			"valid":  false,
			"error":  "Parse error",
			"detail": err.Error(),
		})
		return
	}

	errors := spec.Validate()
	respondJSON(w, map[string]interface{}{
		"valid":  len(errors) == 0,
		"errors": errors,
	})
}

// handleWorkflowRun handles POST /api/workflows/{id}/run
func (s *Server) handleWorkflowRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	workflowID, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		http.Error(w, "Invalid workflow ID", http.StatusBadRequest)
		return
	}

	// Parse request
	var req struct {
		FilePath  string            `json:"file_path"`
		Variables map[string]string `json:"variables"`
		DryRun    bool              `json:"dry_run"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Get workflow
	wf, err := s.db.GetWorkflow(workflowID)
	if err != nil {
		http.Error(w, "Workflow not found", http.StatusNotFound)
		return
	}

	if req.DryRun {
		// Dry run: just show what would be executed
		spec, err := workflow.ParseWorkflow(wf.YAML)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		respondJSON(w, map[string]interface{}{
			"dry_run":  true,
			"workflow": wf.Name,
			"steps":    spec.Steps,
			"message":  "Dry run - no actual execution performed",
		})
		return
	}

	// TODO: Implement actual workflow execution through worker queue
	// For now, return a placeholder
	respondJSON(w, map[string]interface{}{
		"status":  "queued",
		"message": fmt.Sprintf("Workflow '%s' queued for execution on %s", wf.Name, req.FilePath),
	})
}

// handleWorkflowRuns handles GET /api/workflows/{id}/runs
func (s *Server) handleWorkflowRuns(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	workflowID, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		http.Error(w, "Invalid workflow ID", http.StatusBadRequest)
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	if limit <= 0 {
		limit = 50
	}

	runs, err := s.db.ListWorkflowRuns(workflowID, limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, runs)
}

// handleWorkflowRunDetail handles GET /api/workflows/runs/{run_id}
func (s *Server) handleWorkflowRunDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract run ID
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	runID, err := strconv.ParseInt(parts[4], 10, 64)
	if err != nil {
		http.Error(w, "Invalid run ID", http.StatusBadRequest)
		return
	}

	run, err := s.db.GetWorkflowRun(runID)
	if err != nil {
		http.Error(w, "Run not found", http.StatusNotFound)
		return
	}

	respondJSON(w, run)
}
