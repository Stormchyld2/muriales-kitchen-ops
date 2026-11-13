package main

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func (s *store) moreRoutes(mux *http.ServeMux) {
	s.extractRoutes(mux)
	s.locationRoutes(mux)
	s.purgeRoutes(mux)
	s.maintRoutes(mux)
	// ---- handoff notes
	mux.HandleFunc("POST /api/notes", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			By   string `json:"by"`
			Text string `json:"text"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&in); err != nil {
			fail(w, 400, "the note could not be read")
			return
		}
		text := strings.TrimSpace(in.Text)
		if text == "" || len(text) > 2000 {
			fail(w, 400, "write a note of up to 2000 characters")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		old := s.lib.Notes
		n := &Note{ID: newID(), At: now(), By: strings.TrimSpace(in.By), Text: text}
		s.lib.Notes = append(append([]*Note{}, old...), n)
		if err := s.save(); err != nil {
			s.lib.Notes = old
			fail(w, 500, "the library could not be saved: "+err.Error())
			return
		}
		writeJSON(w, 200, n)
	})
	mux.HandleFunc("POST /api/notes/{id}/{what}", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			By string `json:"by"`
		}
		_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&in)
		s.mu.Lock()
		defer s.mu.Unlock()
		var n *Note
		for _, q := range s.lib.Notes {
			if q.ID == r.PathValue("id") {
				n = q
			}
		}
		if n == nil {
			fail(w, 404, "that note no longer exists")
			return
		}
		before := *n
		switch r.PathValue("what") {
		case "done":
			n.Done, n.DoneAt, n.DoneBy = true, now(), strings.TrimSpace(in.By)
		case "reopen":
			n.Done, n.DoneAt, n.DoneBy = false, "", ""
		default:
			fail(w, 404, "unknown action")
			return
		}
		if err := s.save(); err != nil {
			*n = before
			fail(w, 500, "the library could not be saved: "+err.Error())
			return
		}
		writeJSON(w, 200, n)
	})

	// ---- document integrity
	mux.HandleFunc("GET /api/integrity", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		writeJSON(w, 200, map[string]any{"rows": s.integrity(r.URL.Query().Get("full") == "1"), "checked": now()})
	})
	// A manager accepts the file as it is now. The binder then shows that
	// sheet as needing to be replaced, since the paper copy came from the old file.
	mux.HandleFunc("POST /api/integrity/{id}/accept", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		p := s.find(r.PathValue("id"))
		if p == nil {
			fail(w, 404, "that product is no longer in the library")
			return
		}
		sum, err := hashFile(s.pdfPath(p))
		if err != nil {
			fail(w, 400, "the PDF cannot be read, so it cannot be accepted")
			return
		}
		before := *p
		p.SHA256, p.PDFUpdatedAt, p.UpdatedAt = sum, now(), now()
		if err := s.save(); err != nil {
			*p = before
			fail(w, 500, "the library could not be saved: "+err.Error())
			return
		}
		writeJSON(w, 200, p)
	})
	mux.HandleFunc("POST /api/products/{id}/pages", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Pages int `json:"pages"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&in); err != nil || in.Pages < 1 || in.Pages > 5000 {
			fail(w, 400, "that is not a page count")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		if p := s.find(r.PathValue("id")); p != nil && p.Pages != in.Pages {
			old := p.Pages
			p.Pages = in.Pages
			if err := s.save(); err != nil {
				p.Pages = old
			}
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
	})

	// ---- export
	mux.HandleFunc("GET /api/export/products.csv", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		download(w, "text/csv; charset=utf-8", "Muriales SDS Products "+time.Now().Format("2006-01-02")+".csv")
		_, _ = io.WriteString(w, "\xEF\xBB\xBF") // so Excel reads accents correctly
		cw := csv.NewWriter(w)
		_ = cw.Write([]string{"Product Name", "Also Known As", "Manufacturer", "Supplier", "Category", "Also Used For", "Storage Location",
			"SDS Revision Date", "Signal Word", "Summary Verified On", "Summary Verified By", "PDF File", "Pages", "SHA-256"})
		for _, p := range s.lib.Products {
			_ = cw.Write([]string{csvCell(p.Name), csvCell(p.Aliases), csvCell(p.Manufacturer), csvCell(p.Supplier), csvCell(p.Category), csvCell(p.Tags),
				csvCell(p.Location), p.RevisionDate, p.SignalWord, p.SummaryVerifiedAt, csvCell(p.SummaryVerifiedBy), filepath.Base(s.pdfPath(p)),
				fmt.Sprint(p.Pages), p.SHA256})
		}
		cw.Flush()
	})
	mux.HandleFunc("GET /api/export/library.zip", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		download(w, "application/zip", "Muriales SDS Library "+time.Now().Format("2006-01-02")+".zip")
		if err := zipTo(w, s.dir); err != nil {
			s.logf("library export failed: %v", err)
		}
	})

	// ---- import
	mux.HandleFunc("POST /api/import/pdfs", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<30)
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			fail(w, 400, "the files could not be read; they may be too large")
			return
		}
		files := r.MultipartForm.File["pdf"]
		if len(files) == 0 || len(files) > 500 {
			fail(w, 400, "choose between 1 and 500 PDF files")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		known := map[string]string{}
		for _, p := range s.lib.Products {
			if p.SHA256 != "" {
				known[p.SHA256] = p.Name
			}
		}
		var made []*Product
		var written []string
		type skip struct {
			Name   string `json:"name"`
			Reason string `json:"reason"`
		}
		skipped := []skip{}
		for _, fh := range files {
			name := filepath.Base(fh.Filename)
			f, err := fh.Open()
			if err != nil {
				skipped = append(skipped, skip{name, "could not be read"})
				continue
			}
			tmp, err := os.CreateTemp(s.pdfDir(), "import-*.tmp")
			if err != nil {
				f.Close()
				skipped = append(skipped, skip{name, "could not be saved"})
				continue
			}
			h := sha256.New()
			head := make([]byte, 1024)
			n, _ := io.ReadFull(f, head)
			isPDF := strings.Contains(string(head[:n]), "%PDF")
			_, _ = tmp.Write(head[:n])
			h.Write(head[:n])
			_, cerr := io.Copy(io.MultiWriter(tmp, h), f)
			f.Close()
			tmp.Close()
			sum := hex.EncodeToString(h.Sum(nil))
			switch {
			case cerr != nil:
				os.Remove(tmp.Name())
				skipped = append(skipped, skip{name, "could not be saved"})
			case !isPDF:
				os.Remove(tmp.Name())
				skipped = append(skipped, skip{name, "is not a PDF"})
			case known[sum] != "":
				os.Remove(tmp.Name())
				skipped = append(skipped, skip{name, "is already in the library as " + known[sum]})
			default:
				title := titleFromFile(name)
				draft := draftFile(tmp.Name())
				if draft != nil && draft.Name != "" {
					title = draft.Name
				}
				p := &Product{ID: newID(), Name: title, Category: catchAll, NeedsReview: true, SHA256: sum,
					AddedAt: now(), UpdatedAt: now(), PDFUpdatedAt: now()}
				if draft != nil {
					applyDraft(p, draft)
				}
				p.File = s.fileNameFor(title)
				if err := os.Rename(tmp.Name(), s.pdfPath(p)); err != nil {
					os.Remove(tmp.Name())
					skipped = append(skipped, skip{name, "could not be saved"})
					continue
				}
				known[sum] = title
				written = append(written, s.pdfPath(p))
				made = append(made, p)
			}
		}
		if len(made) > 0 {
			old := s.lib.Products
			s.lib.Products = append(append([]*Product{}, old...), made...)
			if err := s.save(); err != nil {
				s.lib.Products = old
				for _, f := range written {
					os.Remove(f)
				}
				fail(w, 500, "the library could not be saved, so nothing was added: "+err.Error())
				return
			}
		}
		names := []string{}
		for _, p := range made {
			names = append(names, p.Name)
		}
		writeJSON(w, 200, map[string]any{"added": names, "skipped": skipped})
	})
	// A backup zip from anywhere (an export, a USB drive) is placed with the
	// other backups; restoring it is then the same checked, undoable restore.
	mux.HandleFunc("POST /api/import/backup", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 2<<30)
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			fail(w, 400, "the file could not be read; it may be too large")
			return
		}
		f, fh, err := r.FormFile("zip")
		if err != nil {
			fail(w, 400, "choose a library zip file")
			return
		}
		defer f.Close()
		if err := os.MkdirAll(s.backupDir(), 0o755); err != nil {
			fail(w, 500, "the backups folder could not be created")
			return
		}
		base := strings.TrimSuffix(strings.Join(strings.Fields(badFileChars.ReplaceAllString(filepath.Base(fh.Filename), " ")), " "), ".zip")
		if len(base) > 60 {
			base = base[:60]
		}
		name := "Imported " + time.Now().Format("2006-01-02 150405") + " " + strings.TrimSpace(base) + ".zip"
		path := filepath.Join(s.backupDir(), name)
		out, err := os.Create(path)
		if err == nil {
			_, err = io.Copy(out, f)
			if cerr := out.Close(); err == nil {
				err = cerr
			}
		}
		if err != nil {
			os.Remove(path)
			fail(w, 500, "the file could not be saved: "+err.Error())
			return
		}
		if _, err := inspectBackup(path); err != nil {
			os.Remove(path)
			fail(w, 400, err.Error())
			return
		}
		writeJSON(w, 200, map[string]string{"file": name})
	})

	// ---- diagnostics
	mux.HandleFunc("GET /api/diagnostics", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		writeJSON(w, 200, s.diagnostics())
	})

	// ---- recovery
	mux.HandleFunc("GET /api/recovery", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		out := map[string]any{"backups": s.backupDir()}
		prev := map[string]any{"exists": false}
		if info, err := os.Stat(s.file() + ".bak"); err == nil {
			prev["exists"], prev["modified"] = true, info.ModTime().Format("2006-01-02T15:04:05")
			if b, err := os.ReadFile(s.file() + ".bak"); err == nil {
				var lib Library
				if json.Unmarshal(b, &lib) == nil {
					prev["products"], prev["removed"] = len(lib.Products), len(lib.Removed)
				} else {
					prev["damaged"] = true
				}
			}
		}
		out["previousSave"] = prev
		linked := map[string]bool{}
		for _, p := range s.lib.Removed {
			linked[p.ArchivedFile] = true
		}
		unlinked := []map[string]any{}
		if entries, err := os.ReadDir(s.goneDir()); err == nil {
			for _, e := range entries {
				if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".pdf") || linked[e.Name()] {
					continue
				}
				info, _ := e.Info()
				unlinked = append(unlinked, map[string]any{"file": e.Name(), "title": titleFromFile(archiveSuffix.ReplaceAllString(strings.TrimSuffix(e.Name(), ".pdf"), "")), "modified": info.ModTime().Format("2006-01-02T15:04:05")})
			}
		}
		sort.Slice(unlinked, func(i, j int) bool { return unlinked[i]["modified"].(string) > unlinked[j]["modified"].(string) })
		out["unlinked"] = unlinked
		held := []string{}
		if entries, err := os.ReadDir(s.backupDir()); err == nil {
			for _, e := range entries {
				if e.IsDir() && strings.HasPrefix(e.Name(), "Before restore ") {
					held = append(held, e.Name())
				}
			}
		}
		out["beforeRestore"] = held
		writeJSON(w, 200, out)
	})
	mux.HandleFunc("POST /api/recovery/previous-save", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		keep, err := s.recoverPreviousSave()
		if err != nil {
			fail(w, 400, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"products": len(s.lib.Products), "keptAs": filepath.Base(keep)})
	})
	mux.HandleFunc("POST /api/recovery/rebuild", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		before := len(s.lib.Products)
		changed := s.scan()
		if changed {
			if err := s.save(); err != nil {
				fail(w, 500, "the library could not be saved: "+err.Error())
				return
			}
		}
		missing := 0
		for _, p := range s.lib.Products {
			if p.Missing {
				missing++
			}
		}
		writeJSON(w, 200, map[string]int{"added": len(s.lib.Products) - before, "missing": missing, "products": len(s.lib.Products)})
	})
	mux.HandleFunc("POST /api/recovery/unlinked", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			File string `json:"file"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&in); err != nil || in.File == "" || in.File != filepath.Base(in.File) {
			fail(w, 400, "choose a file from the list")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		src := filepath.Join(s.goneDir(), in.File)
		if !exists(src) {
			fail(w, 404, "that file is no longer in the removed folder")
			return
		}
		sum, err := hashFile(src)
		if err != nil {
			fail(w, 500, "the file could not be read")
			return
		}
		title := titleFromFile(archiveSuffix.ReplaceAllString(strings.TrimSuffix(in.File, ".pdf"), ""))
		p := &Product{ID: newID(), Name: title, Category: catchAll, NeedsReview: true, SHA256: sum, AddedAt: now(), UpdatedAt: now(), PDFUpdatedAt: now()}
		p.File = s.fileNameFor(title)
		if err := os.Rename(src, s.pdfPath(p)); err != nil {
			fail(w, 500, "the file could not be moved back: "+err.Error())
			return
		}
		old := s.lib.Products
		s.lib.Products = append(append([]*Product{}, old...), p)
		if err := s.save(); err != nil {
			s.lib.Products = old
			_ = os.Rename(s.pdfPath(p), src)
			fail(w, 500, "the library could not be saved: "+err.Error())
			return
		}
		writeJSON(w, 200, p)
	})
}
