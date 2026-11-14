package main

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Policies & Reference: a folder of reference documents (the FDA Food Code,
// the state rule, the kitchen's own policies) that sits next to the program.
// The first run copies the documents that came with the program in.

func (s *store) referenceDir() string {
	return filepath.Join(filepath.Dir(s.dir), "Policies & Reference")
}

// seedReference copies the bundled documents once, so one the kitchen later
// deletes does not come back. Caller holds s.mu.
func (s *store) seedReference(tag string) {
	flag := filepath.Join(s.maintDir(), "seed-"+tag+".flag")
	if _, err := os.Stat(flag); err == nil {
		return
	}
	dir := s.referenceDir()
	if os.MkdirAll(dir, 0o755) != nil {
		return
	}
	ents, _ := starterFiles.ReadDir("starter/reference")
	for _, e := range ents {
		dst := filepath.Join(dir, e.Name())
		if _, err := os.Stat(dst); err == nil {
			continue
		}
		if b, err := starterFiles.ReadFile("starter/reference/" + e.Name()); err == nil {
			_ = os.WriteFile(dst, b, 0o644)
		}
	}
	_ = os.WriteFile(flag, []byte(time.Now().Format(time.RFC3339)), 0o644)
}

func (s *store) referenceRoutes(mux *http.ServeMux) {
	s.mu.Lock()
	s.seedReference("reference-240")
	s.mu.Unlock()
	mux.HandleFunc("GET /api/reference", func(w http.ResponseWriter, r *http.Request) {
		dir := s.referenceDir()
		_ = os.MkdirAll(dir, 0o755)
		ents, _ := os.ReadDir(dir)
		out := []map[string]any{}
		for _, e := range ents {
			if e.IsDir() || strings.HasPrefix(e.Name(), ".") || strings.HasPrefix(e.Name(), "~$") {
				continue
			}
			if fi, err := e.Info(); err == nil {
				out = append(out, map[string]any{"name": e.Name(), "size": fi.Size(), "modified": fi.ModTime().Format("2006-01-02")})
			}
		}
		sort.Slice(out, func(i, j int) bool {
			return strings.ToLower(out[i]["name"].(string)) < strings.ToLower(out[j]["name"].(string))
		})
		writeJSON(w, 200, map[string]any{"files": out, "folder": dir})
	})
	mux.HandleFunc("POST /api/reference/open", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Name string }
		if jsonDecode(r, &in) != nil || in.Name == "" || filepath.Base(in.Name) != in.Name {
			fail(w, 400, "Could not read that.")
			return
		}
		p := filepath.Join(s.referenceDir(), in.Name)
		if _, err := os.Stat(p); err != nil {
			fail(w, 404, "That file is no longer in the Policies & Reference folder.")
			return
		}
		openFile(p)
		writeJSON(w, 200, map[string]any{"ok": true})
	})
	mux.HandleFunc("POST /api/reference/folder", func(w http.ResponseWriter, r *http.Request) {
		_ = os.MkdirAll(s.referenceDir(), 0o755)
		openFolder(s.referenceDir())
		writeJSON(w, 200, map[string]any{"ok": true})
	})
}
