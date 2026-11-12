package main

// The maintenance procedures: how the department works, the review schedule
// and what each priority means. They start from the workbook's guide
// (starter/procedures.json) and are copied into Maintenance Data\procedures.json
// the first time they are needed, so the wording can be edited in any text editor.

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
)

type procedures struct {
	Cadence []struct {
		When   string `json:"when"`
		Task   string `json:"task"`
		Owner  string `json:"owner"`
		Backup string `json:"backup"`
	} `json:"cadence"`
	Steps []struct {
		Title string `json:"title"`
		Text  string `json:"text"`
	} `json:"steps"`
	Priorities []struct {
		Priority string `json:"priority"`
		Use      string `json:"use"`
		Response string `json:"response"`
	} `json:"priorities"`
}

func (s *store) loadProcedures() procedures {
	var p procedures
	path := filepath.Join(s.maintDir(), "procedures.json")
	if b, err := os.ReadFile(path); err == nil && json.Unmarshal(b, &p) == nil && len(p.Steps) > 0 {
		return p
	}
	b, err := starterFiles.ReadFile("starter/procedures.json")
	if err == nil {
		_ = json.Unmarshal(b, &p)
		if !s.isReadOnly() {
			_ = os.MkdirAll(s.maintDir(), 0o755)
			_ = os.WriteFile(path, b, 0o644)
		}
	}
	return p
}

func (s *store) procedureRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/procedures", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		writeJSON(w, 200, s.loadProcedures())
	})
}
