package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// removedKeepDays is how long a removed product and its PDF are kept before being deleted for good.
const removedKeepDays = 30

func removedAge(p *Product) (time.Duration, bool) {
	t, err := time.ParseInLocation("2006-01-02T15:04:05", p.RemovedAt, time.Local)
	if err != nil {
		return 0, false
	}
	return time.Since(t), true
}

// purgeOld permanently deletes removed products, and PDFs left in the removed folder, that are older
// than removedKeepDays. The caller holds s.mu. It does nothing in a read-only copy.
func (s *store) purgeOld() (products, files int) {
	if s.isReadOnly() {
		return
	}
	limit := time.Duration(removedKeepDays) * 24 * time.Hour
	var keep []*Product
	var doomed []string
	for _, p := range s.lib.Removed {
		if age, ok := removedAge(p); ok && age > limit {
			if p.ArchivedFile != "" {
				doomed = append(doomed, filepath.Join(s.goneDir(), p.ArchivedFile))
			}
			products++
			continue
		}
		keep = append(keep, p)
	}
	linked := map[string]bool{}
	for _, p := range keep {
		linked[p.ArchivedFile] = true
	}
	if entries, err := os.ReadDir(s.goneDir()); err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".pdf") || linked[e.Name()] {
				continue
			}
			if info, err := e.Info(); err == nil && time.Since(info.ModTime()) > limit {
				doomed = append(doomed, filepath.Join(s.goneDir(), e.Name()))
			}
		}
	}
	if products > 0 {
		old := s.lib.Removed
		s.lib.Removed = keep
		if err := s.save(); err != nil {
			s.lib.Removed = old
			return 0, 0
		}
	}
	for _, f := range doomed {
		if os.Remove(f) == nil {
			files++
		}
	}
	if products+files > 0 {
		s.logf("deleted for good after %d days: %d removed products, %d PDFs", removedKeepDays, products, files)
	}
	return
}

func (s *store) purgeRoutes(mux *http.ServeMux) {
	// A manager deletes one removed product for good, before its 30 days are up.
	mux.HandleFunc("DELETE /api/removed/{id}", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		id := r.PathValue("id")
		var keep []*Product
		var gone *Product
		for _, p := range s.lib.Removed {
			if p.ID == id {
				gone = p
			} else {
				keep = append(keep, p)
			}
		}
		if gone == nil {
			writeJSON(w, 200, map[string]bool{"ok": true})
			return
		}
		old := s.lib.Removed
		s.lib.Removed = keep
		if err := s.save(); err != nil {
			s.lib.Removed = old
			fail(w, 500, "the library could not be saved, so nothing was deleted: "+err.Error())
			return
		}
		if gone.ArchivedFile != "" {
			_ = os.Remove(filepath.Join(s.goneDir(), filepath.Base(gone.ArchivedFile)))
		}
		s.logf("deleted for good: %s", gone.Name)
		writeJSON(w, 200, map[string]bool{"ok": true})
	})
}
