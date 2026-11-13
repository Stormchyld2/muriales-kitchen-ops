package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func (s *store) routes(mux *http.ServeMux) {
	s.moreRoutes(mux)
	s.fullBackupRoutes(mux)
	s.lockRoutes(mux)
	mux.HandleFunc("GET /api/library", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.scan() && !s.isReadOnly() {
			if err := s.save(); err != nil {
				log.Printf("library could not be saved after folder scan: %v", err)
			}
		}
		writeJSON(w, 200, map[string]any{
			"categories": s.lib.Categories,
			"locations":  s.lib.Locations,
			"catchAll":   catchAll,
			"products":   s.lib.Products,
			"removed":    s.lib.Removed,
			"binder":     s.lib.Binder,
			"lastBuild":  s.lib.LastBuild,
			"notes":      s.lib.Notes,
			"version":    appVersion,
			"integrity":  s.integrityProblems(),
			"dataFolder": s.dir,
			"readOnly":   s.isReadOnly(),
		})
	})

	mux.HandleFunc("POST /api/products", func(w http.ResponseWriter, r *http.Request) {
		in, tmp, err := s.readUpload(w, r)
		if err != nil {
			fail(w, 400, err.Error())
			return
		}
		if tmp == "" {
			fail(w, 400, "choose the SDS PDF for this product")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		p := &in.Product
		p.ID = newID()
		p.AddedAt, p.UpdatedAt, p.PDFUpdatedAt = now(), now(), now()
		p.Missing, p.NeedsReview, p.RemovedAt, p.ArchivedFile, p.SHA256 = false, false, "", "", ""
		p.SummaryVerifiedAt, p.SummaryVerifiedBy, p.SummarySourceRevision, p.SummaryNeedsCheck = "", "", "", false
		settleSummary(p, in, false, false)
		p.File = s.fileNameFor(p.Name)
		if err := os.Rename(tmp, s.pdfPath(p)); err != nil {
			os.Remove(tmp)
			fail(w, 500, "the PDF could not be saved")
			return
		}
		p.SHA256, _ = hashFile(s.pdfPath(p))
		s.lib.Products = append(s.lib.Products, p)
		if err := s.save(); err != nil {
			// Undo, so the folder and the library never disagree.
			s.lib.Products = s.lib.Products[:len(s.lib.Products)-1]
			os.Remove(s.pdfPath(p))
			fail(w, 500, "the library could not be saved, so nothing was added: "+err.Error())
			return
		}
		writeJSON(w, 200, p)
	})

	mux.HandleFunc("POST /api/products/{id}", func(w http.ResponseWriter, r *http.Request) {
		in, tmp, err := s.readUpload(w, r)
		if err != nil {
			fail(w, 400, err.Error())
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		p := s.find(r.PathValue("id"))
		if p == nil {
			if tmp != "" {
				os.Remove(tmp)
			}
			fail(w, 404, "that product is no longer in the library")
			return
		}
		// Replacing a PDF is done so that a failure at any step leaves the
		// product exactly as it was: the old sheet is copied to the "removed"
		// folder first, the new one then takes its place in a single step,
		// and if the library cannot be saved the old sheet is put back.
		before := *p
		replaced := tmp != ""
		var archive, dst string
		if replaced {
			dst = s.pdfPath(p)
			if exists(dst) {
				archive = filepath.Join(s.goneDir(), s.archiveName(p))
				if err := copyFile(dst, archive); err != nil {
					os.Remove(tmp)
					fail(w, 500, "the current PDF could not be archived, so nothing was changed: "+err.Error())
					return
				}
			} else {
				p.File = s.fileNameFor(in.Name)
				dst = s.pdfPath(p)
			}
			if err := os.Rename(tmp, dst); err != nil {
				os.Remove(tmp)
				if archive != "" {
					os.Remove(archive)
				}
				*p = before
				fail(w, 500, "the new PDF could not be put in place, so nothing was changed: "+err.Error())
				return
			}
			p.Pages = in.Pages
			p.PDFUpdatedAt = now()
			p.SHA256, _ = hashFile(dst)
		}
		oldSummary := p.summaryText()
		p.Name, p.Manufacturer, p.Category, p.Location = in.Name, in.Manufacturer, in.Category, in.Location
		p.Supplier, p.Tags, p.Aliases = in.Supplier, in.Tags, in.Aliases
		p.RevisionDate, p.Hazards, p.PPE = in.RevisionDate, in.Hazards, in.PPE
		p.Pictograms, p.SignalWord = in.Pictograms, in.SignalWord
		p.FirstAidEyes, p.FirstAidSkin = in.FirstAidEyes, in.FirstAidSkin
		p.FirstAidInhalation, p.FirstAidSwallowed = in.FirstAidInhalation, in.FirstAidSwallowed
		settleSummary(p, in, p.summaryText() != oldSummary, replaced)
		if p.NeedsReview && strings.TrimSpace(p.Manufacturer) != "" && p.RevisionDate != "" && strings.TrimSpace(p.Location) != "" {
			p.NeedsReview = false
		}
		p.UpdatedAt = now()
		if err := s.save(); err != nil {
			if replaced {
				if archive != "" {
					if cerr := copyFile(archive, dst); cerr == nil {
						os.Remove(archive)
					}
				} else {
					os.Remove(dst)
				}
			}
			*p = before
			fail(w, 500, "the library could not be saved, so nothing was changed: "+err.Error())
			return
		}
		writeJSON(w, 200, p)
	})

	// Removing keeps the product's details and its PDF so that a discontinued
	// product can be looked up or brought back later.
	mux.HandleFunc("DELETE /api/products/{id}", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		id := r.PathValue("id")
		p := s.find(id)
		if p == nil {
			writeJSON(w, 200, map[string]bool{"ok": true})
			return
		}
		src, archived := s.pdfPath(p), ""
		if exists(src) {
			archived = s.archiveName(p)
			if err := os.Rename(src, filepath.Join(s.goneDir(), archived)); err != nil {
				fail(w, 500, "the PDF could not be moved to the removed folder, so nothing was changed: "+err.Error())
				return
			}
			_ = os.Chtimes(filepath.Join(s.goneDir(), archived), time.Now(), time.Now()) // the 30 days count from today
		}
		oldProducts, oldRemoved, before := s.lib.Products, s.lib.Removed, *p
		kept := make([]*Product, 0, len(oldProducts))
		for _, q := range oldProducts {
			if q.ID != id {
				kept = append(kept, q)
			}
		}
		p.RemovedAt, p.ArchivedFile, p.File, p.Missing = now(), archived, "", false
		s.lib.Products, s.lib.Removed = kept, append(append([]*Product{}, oldRemoved...), p)
		if err := s.save(); err != nil {
			*p = before
			s.lib.Products, s.lib.Removed = oldProducts, oldRemoved
			if archived != "" {
				_ = os.Rename(filepath.Join(s.goneDir(), archived), src)
			}
			fail(w, 500, "the library could not be saved, so nothing was changed: "+err.Error())
			return
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
	})

	mux.HandleFunc("POST /api/removed/{id}/restore", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		id := r.PathValue("id")
		idx := -1
		for i, q := range s.lib.Removed {
			if q.ID == id {
				idx = i
			}
		}
		if idx < 0 {
			fail(w, 404, "that product is not in the removed list")
			return
		}
		p := s.lib.Removed[idx]
		before := *p
		oldProducts, oldRemoved := s.lib.Products, s.lib.Removed
		src := filepath.Join(s.goneDir(), p.ArchivedFile)
		moved := false
		p.File = ""
		if p.ArchivedFile != "" && exists(src) {
			p.File = s.fileNameFor(p.Name)
			if err := os.Rename(src, s.pdfPath(p)); err != nil {
				*p = before
				fail(w, 500, "the PDF could not be moved back, so nothing was changed: "+err.Error())
				return
			}
			moved = true
		} else {
			p.File = s.fileNameFor(p.Name) // its PDF is gone; the product comes back flagged "PDF missing"
		}
		p.RemovedAt, p.ArchivedFile, p.UpdatedAt = "", "", now()
		rest := append(append([]*Product{}, oldRemoved[:idx]...), oldRemoved[idx+1:]...)
		s.lib.Removed, s.lib.Products = rest, append(append([]*Product{}, oldProducts...), p)
		if err := s.save(); err != nil {
			if moved {
				_ = os.Rename(s.pdfPath(p), src)
			}
			*p = before
			s.lib.Products, s.lib.Removed = oldProducts, oldRemoved
			fail(w, 500, "the library could not be saved, so nothing was changed: "+err.Error())
			return
		}
		writeJSON(w, 200, p)
	})

	mux.HandleFunc("GET /pdf-removed/{id}", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		file := ""
		for _, q := range s.lib.Removed {
			if q.ID == r.PathValue("id") {
				file = q.ArchivedFile
			}
		}
		s.mu.Unlock()
		if file == "" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		http.ServeFile(w, r, filepath.Join(s.goneDir(), filepath.Base(file)))
	})

	// Records that a book was assembled. This is not the same as printing it.
	mux.HandleFunc("POST /api/binder/generated", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Scope    string   `json:"scope"`
			Pages    int      `json:"pages"`
			Complete bool     `json:"complete"`
			Missing  []string `json:"missing"`
			IDs      []string `json:"ids"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil {
			fail(w, 400, "the request could not be read")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		rec := &BuildRecord{At: now(), Scope: in.Scope, Pages: in.Pages, Complete: in.Complete && len(in.Missing) == 0, Missing: in.Missing, Items: map[string]BinderItem{}}
		for _, id := range in.IDs {
			if p := s.find(id); p != nil {
				rec.Items[id] = snapshot(p)
			}
		}
		if rec.Scope == "all" && len(rec.Items) != len(s.lib.Products) {
			rec.Scope = "selected" // not every product is in it, so it cannot replace the whole binder
		}
		if rec.Scope != "all" && rec.Scope != "selected" && rec.Scope != "index" {
			fail(w, 400, "unknown kind of book")
			return
		}
		old := s.lib.LastBuild
		s.lib.LastBuild = rec
		if err := s.save(); err != nil {
			s.lib.LastBuild = old
			fail(w, 500, "the library could not be saved: "+err.Error())
			return
		}
		writeJSON(w, 200, rec)
	})

	// A manager confirms that the last complete book was printed and the
	// paper binder updated. Only then does the binder record change.
	mux.HandleFunc("POST /api/binder/confirm", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		lb := s.lib.LastBuild
		if lb == nil || !lb.Complete {
			fail(w, 400, "there is no complete book to confirm. Print the book again first")
			return
		}
		oldBinder, oldBuild := s.lib.Binder, *lb
		nb := &Binder{Items: map[string]BinderItem{}}
		if oldBinder != nil {
			*nb = *oldBinder
			nb.Items = map[string]BinderItem{}
			for k, v := range oldBinder.Items {
				nb.Items[k] = v
			}
		}
		switch lb.Scope {
		case "all":
			nb.Items = map[string]BinderItem{}
			for k, v := range lb.Items {
				nb.Items[k] = v
			}
			nb.Pages = lb.Pages
		case "selected":
			for k, v := range lb.Items {
				nb.Items[k] = v
			}
		case "index":
			if oldBinder == nil {
				fail(w, 400, "print and confirm the entire book first; the index alone cannot start a binder record")
				return
			}
			for k, v := range lb.Items {
				if cur, ok := nb.Items[k]; ok {
					v.PDFUpdatedAt = cur.PDFUpdatedAt
					nb.Items[k] = v
				}
			}
		}
		nb.UpdatedAt, nb.GeneratedAt = now(), lb.At
		lb.Confirmed, lb.ConfirmedAt = true, now()
		s.lib.Binder = nb
		if err := s.save(); err != nil {
			s.lib.Binder, *lb = oldBinder, oldBuild
			fail(w, 500, "the library could not be saved: "+err.Error())
			return
		}
		writeJSON(w, 200, nb)
	})

	// Sheets that were taken out of the paper binder after removal.
	mux.HandleFunc("POST /api/binder/pulled", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.lib.Binder == nil {
			writeJSON(w, 200, map[string]bool{"ok": true})
			return
		}
		old := s.lib.Binder
		nb := *old
		nb.Items = map[string]BinderItem{}
		for id, it := range old.Items {
			if s.find(id) != nil {
				nb.Items[id] = it
			}
		}
		nb.UpdatedAt = now()
		s.lib.Binder = &nb
		if err := s.save(); err != nil {
			s.lib.Binder = old
			fail(w, 500, "the library could not be saved: "+err.Error())
			return
		}
		writeJSON(w, 200, nb)
	})

	mux.HandleFunc("GET /api/backups", func(w http.ResponseWriter, r *http.Request) {
		type item struct {
			File    string `json:"file"`
			Created string `json:"created"`
			Size    int64  `json:"size"`
			Kind    string `json:"kind"`
		}
		list := []item{}
		entries, _ := os.ReadDir(s.backupDir())
		for _, e := range entries {
			if info, err := e.Info(); err == nil && !e.IsDir() && !strings.HasPrefix(e.Name(), ".") && strings.EqualFold(filepath.Ext(e.Name()), ".zip") {
				list = append(list, item{e.Name(), info.ModTime().Format("2006-01-02T15:04:05"), info.Size(), backupKind(e.Name())})
			}
		}
		sort.Slice(list, func(i, j int) bool { return list[i].Created > list[j].Created })
		writeJSON(w, 200, map[string]any{"folder": s.backupDir(), "backups": list})
	})

	mux.HandleFunc("POST /api/backups/inspect", func(w http.ResponseWriter, r *http.Request) {
		path, err := s.backupPath(w, r)
		if err != nil {
			fail(w, 400, err.Error())
			return
		}
		info, err := inspectBackup(path)
		if err != nil {
			fail(w, 400, err.Error())
			return
		}
		writeJSON(w, 200, info)
	})

	mux.HandleFunc("POST /api/restore", func(w http.ResponseWriter, r *http.Request) {
		path, err := s.backupPath(w, r)
		if err != nil {
			fail(w, 400, err.Error())
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		kept, err := s.restore(path)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"products": len(s.lib.Products), "previousKeptIn": kept})
	})

	mux.HandleFunc("GET /pdf/{id}", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		p := s.find(r.PathValue("id"))
		s.mu.Unlock()
		if p == nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		http.ServeFile(w, r, s.pdfPath(p))
	})

	mux.HandleFunc("POST /api/categories", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Categories []string          `json:"categories"`
			Renames    map[string]string `json:"renames"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil {
			fail(w, 400, "the categories could not be read")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		seen := map[string]bool{}
		var cats []string
		for _, c := range in.Categories {
			c = strings.TrimSpace(c)
			if c != "" && c != catchAll && !seen[strings.ToLower(c)] {
				seen[strings.ToLower(c)] = true
				cats = append(cats, c)
			}
		}
		cats = append(cats, catchAll)
		valid := map[string]bool{}
		for _, c := range cats {
			valid[c] = true
		}
		for _, p := range s.lib.Products {
			if to, ok := in.Renames[p.Category]; ok {
				p.Category = strings.TrimSpace(to)
			}
			if !valid[p.Category] {
				p.Category = catchAll
			}
		}
		s.lib.Categories = cats
		if err := s.save(); err != nil {
			fail(w, 500, "the library could not be saved: "+err.Error())
			return
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
	})

	mux.HandleFunc("POST /api/backup", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		dir := s.backupDir()
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fail(w, 500, "the backup folder could not be created")
			return
		}
		name := filepath.Join(dir, "Kitchen Backup "+time.Now().Format("2006-01-02 150405")+".zip")
		if err := zipDir(s.dir, name); err != nil {
			os.Remove(name)
			fail(w, 500, "the backup could not be written: "+err.Error())
			return
		}
		if exists(s.maintDir()) {
			// The maintenance files are small; keep a copy of them with every backup.
			mname := filepath.Join(dir, "Maintenance Backup "+time.Now().Format("2006-01-02 150405")+".zip")
			if err := zipDir(s.maintDir(), mname); err != nil {
				os.Remove(mname)
				log.Printf("maintenance backup failed: %v", err)
			}
		}
		openFolder(dir)
		writeJSON(w, 200, map[string]string{"file": name})
	})

	mux.HandleFunc("POST /api/data-folder", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&in); err != nil {
			fail(w, 400, "the request could not be read")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		old := s.dir
		copied, err := s.relocate(in.Path)
		if err != nil {
			fail(w, 400, err.Error())
			return
		}
		log.Printf("library folder changed from %s to %s (copied=%v)", old, s.dir, copied)
		writeJSON(w, 200, map[string]any{"folder": s.dir, "copied": copied, "previous": old, "products": len(s.lib.Products)})
	})

	mux.HandleFunc("POST /api/open-folder", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("which") == "backups" {
			_ = os.MkdirAll(s.backupDir(), 0o755)
			openFolder(s.backupDir())
		} else if r.URL.Query().Get("which") == "reports" {
			_ = os.MkdirAll(s.reportsDir(), 0o755)
			openFolder(s.reportsDir())
		} else {
			openFolder(s.dir)
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
	})
}
