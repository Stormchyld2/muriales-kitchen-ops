package main

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Photos taken on a phone during a walkthrough are saved as ordinary picture
// files in a "Walk Photos" folder next to the program.

var photoName = regexp.MustCompile(`^[A-Za-z0-9._-]{1,80}$`)

func (s *store) photosDir() string { return filepath.Join(filepath.Dir(s.dir), "Walk Photos") }

// cleanPhotos keeps only well-formed picture names that exist in the folder.
func (s *store) cleanPhotos(in []string) []string {
	out := []string{}
	for _, n := range in {
		if photoName.MatchString(n) && len(out) < 6 {
			if _, err := os.Stat(filepath.Join(s.photosDir(), n)); err == nil {
				out = append(out, n)
			}
		}
	}
	return out
}

func (s *store) photoRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/photo", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Data string `json:"data"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 9<<20)).Decode(&in); err != nil {
			fail(w, 400, "The photo could not be read. Try a smaller one.")
			return
		}
		ext := ""
		switch {
		case strings.HasPrefix(in.Data, "data:image/jpeg;base64,"):
			ext, in.Data = ".jpg", strings.TrimPrefix(in.Data, "data:image/jpeg;base64,")
		case strings.HasPrefix(in.Data, "data:image/png;base64,"):
			ext, in.Data = ".png", strings.TrimPrefix(in.Data, "data:image/png;base64,")
		default:
			fail(w, 400, "That is not a picture.")
			return
		}
		b, err := base64.StdEncoding.DecodeString(in.Data)
		if err != nil || len(b) == 0 || len(b) > 6<<20 {
			fail(w, 400, "The photo could not be read. Try a smaller one.")
			return
		}
		if err := os.MkdirAll(s.photosDir(), 0o755); err != nil {
			fail(w, 500, "The Walk Photos folder could not be made next to the program.")
			return
		}
		name := "walk-" + nowT().Format("20060102-150405") + "-" + newID()[:6] + ext
		if err := os.WriteFile(filepath.Join(s.photosDir(), name), b, 0o644); err != nil {
			fail(w, 500, "The photo could not be saved: "+err.Error())
			return
		}
		writeJSON(w, 200, map[string]string{"name": name})
	})
	mux.HandleFunc("GET /api/photo/{name}", func(w http.ResponseWriter, r *http.Request) {
		n := r.PathValue("name")
		if !photoName.MatchString(n) {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "private, max-age=86400")
		http.ServeFile(w, r, filepath.Join(s.photosDir(), n))
	})
}
