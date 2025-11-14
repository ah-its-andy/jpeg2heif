package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ah-its-andy/jpeg2heif/internal/converter"
	"github.com/ah-its-andy/jpeg2heif/internal/db"
)

// ConverterResponse represents a unified converter response (builtin or workflow)
type ConverterResponse struct {
	Name         string `json:"name"`
	Type         string `json:"type"` // "builtin" or "workflow"
	TargetFormat string `json:"target_format"`
	Enabled      bool   `json:"enabled"`
	Description  string `json:"description,omitempty"`
	WorkflowID   int64  `json:"workflow_id,omitempty"`
}

// handleConverters handles GET /api/converters - returns all converters (builtin + workflow)
func (s *Server) handleConverters(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var allConverters []ConverterResponse

	// Get builtin converters
	builtinConverters := converter.ListInfo()
	for _, c := range builtinConverters {
		// Skip workflow converters here, we'll add them separately
		if strings.HasPrefix(c.Name, "workflow:") {
			continue
		}
		allConverters = append(allConverters, ConverterResponse{
			Name:         c.Name,
			Type:         "builtin",
			TargetFormat: c.TargetFormat,
			Enabled:      c.Enabled,
		})
	}

	// Get workflow converters from database
	workflows, err := s.db.ListWorkflows(1000, 0)
	if err == nil {
		for _, wf := range workflows {
			allConverters = append(allConverters, ConverterResponse{
				Name:         "workflow:" + wf.Name,
				Type:         "workflow",
				TargetFormat: extractTargetFormat(wf.Name),
				Enabled:      wf.Enabled,
				Description:  wf.Description,
				WorkflowID:   wf.ID,
			})
		}
	}

	respondJSON(w, allConverters)
}

// handleConverterDetail handles PUT /api/converters/{name}
func (s *Server) handleConverterDetail(w http.ResponseWriter, r *http.Request) {
	// Extract name from path
	name := strings.TrimPrefix(r.URL.Path, "/api/converters/")
	if name == "" {
		http.Error(w, "Converter name required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPut:
		s.handleConverterUpdate(w, r, name)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleConverterUpdate handles enabling/disabling a converter
func (s *Server) handleConverterUpdate(w http.ResponseWriter, r *http.Request, name string) {
	var req struct {
		Enabled bool `json:"enabled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Check if it's a workflow converter
	if strings.HasPrefix(name, "workflow:") {
		workflowName := strings.TrimPrefix(name, "workflow:")

		// Find workflow by name
		workflows, err := s.db.ListWorkflows(1000, 0)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var workflow *db.Workflow
		for _, wf := range workflows {
			if wf.Name == workflowName {
				workflow = wf
				break
			}
		}

		if workflow == nil {
			http.Error(w, "Workflow not found", http.StatusNotFound)
			return
		}

		// Update workflow enabled status
		workflow.Enabled = req.Enabled
		if err := s.db.UpdateWorkflow(workflow); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Also update the converter registry
		if req.Enabled {
			converter.Enable(name)
		} else {
			converter.Disable(name)
		}

		respondJSON(w, map[string]interface{}{
			"name":    name,
			"enabled": req.Enabled,
			"type":    "workflow",
		})
		return
	}

	// Handle builtin converter
	if req.Enabled {
		if err := converter.Enable(name); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
	} else {
		if err := converter.Disable(name); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
	}

	respondJSON(w, map[string]interface{}{
		"name":    name,
		"enabled": req.Enabled,
		"type":    "builtin",
	})
}

// extractTargetFormat extracts target format from workflow name
func extractTargetFormat(name string) string {
	parts := strings.Split(name, "-to-")
	if len(parts) == 2 {
		return strings.TrimSpace(parts[1])
	}
	return "unknown"
}
