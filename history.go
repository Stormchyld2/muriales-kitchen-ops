package main

// The edit history. Every successful change made through the program - an
// entry added, edited, deleted, marked done - is appended to one plain file,
// "Edit History.jsonl", in the Maintenance Data folder. Each line says when,
// who (the name typed on the form), what kind of record, which one, and for
// edits to the list-type records what changed. Deleted records keep a copy of
// everything they held, so nothing is lost without a trace.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type histRow struct {
	At     string `json:"at"`
	Who    string `json:"who"`
	Action string `json:"action"`
	Kind   string `json:"kind"`
	Label  string `json:"label"`
	ID     string `json:"id,omitempty"`
	Detail string `json:"detail,omitempty"`
}

var histMu sync.Mutex

const histMaxBytes = 8 << 20

func (s *store) historyFile() string { return filepath.Join(s.maintDir(), "Edit History.jsonl") }

func (s *store) addHistory(h histRow) {
	histMu.Lock()
	defer histMu.Unlock()
	if os.MkdirAll(s.maintDir(), 0o755) != nil {
		return
	}
	path := s.historyFile()
	if st, err := os.Stat(path); err == nil && st.Size() > histMaxBytes {
		_ = os.Rename(path, filepath.Join(s.maintDir(), "Edit History "+time.Now().Format("2006-01-02")+".jsonl"))
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	b, _ := json.Marshal(h)
	_, _ = f.Write(append(b, '\n'))
}

// historyRows returns the newest rows first, filtered by a search word.
func (s *store) historyRows(q string, limit int) ([]histRow, int) {
	histMu.Lock()
	defer histMu.Unlock()
	f, err := os.Open(s.historyFile())
	if err != nil {
		return []histRow{}, 0
	}
	defer f.Close()
	q = strings.ToLower(strings.TrimSpace(q))
	all := []histRow{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 4<<20)
	for sc.Scan() {
		var h histRow
		if json.Unmarshal(sc.Bytes(), &h) != nil {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(h.Who+" "+h.Action+" "+h.Kind+" "+h.Label+" "+h.ID+" "+h.Detail+" "+h.At), q) {
			continue
		}
		all = append(all, h)
	}
	for i, j := 0, len(all)-1; i < j; i, j = i+1, j-1 { // later lines first when two share a second
		all[i], all[j] = all[j], all[i]
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].At > all[j].At })
	total := len(all)
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	return all, total
}

var histWhoKeys = []string{"by", "doneBy", "createdBy", "reportedBy", "identifiedBy", "editedBy", "requestedBy", "approvedBy", "verifiedBy", "manager", "foundBy", "removedBy", "clearedBy", "reviewedBy", "closedBy", "completedBy", "owner"}
var histLabelKeys = []string{"name", "title", "issue", "check", "task", "point", "product", "equipment", "description", "text", "type", "area", "standard"}

// Paths that change nothing worth recording.
var histSkip = []string{"/api/ping", "/api/closing", "/api/lock", "/api/extract", "/api/temps/sheet-preview", "/api/temps/import/preview",
	"/api/backups/inspect", "/api/maint/open-portal", "/api/open-folder", "/api/temp-reports", "/api/binder/pulled", "/api/binder/generated", "/api/materials"}

var histNouns = [][2]string{
	{"/api/maint/equipment", "Equipment"}, {"/api/maint/issues", "Maintenance issue"}, {"/api/maint/pm", "Maintenance task"},
	{"/api/maint/service", "Service record"}, {"/api/maint/findings", "Inspection item"}, {"/api/maint/calendar", "Calendar event"},
	{"/api/maint/todos", "To-do item"}, {"/api/maint/settings", "Maintenance settings"}, {"/api/maint/summary-notes", "Monthly summary notes"},
	{"/api/maint/import", "Import"}, {"/api/inspections", "PM inspection"}, {"/api/temps/points", "Temperature point"},
	{"/api/temps/settings", "Temperature settings"}, {"/api/temps/import", "Temperature import"}, {"/api/temps", "Temperature reading"},
	{"/api/corrective", "Corrective action"}, {"/api/readiness", "Readiness review"}, {"/api/chem", "Chemical incident"},
	{"/api/clean", "Cleaning"}, {"/api/mgr", "Manager inspection"}, {"/api/replacements", "Replacement request"},
	{"/api/products", "Safety data sheet"}, {"/api/categories", "SDS categories"}, {"/api/locations", "SDS locations"},
	{"/api/notes", "Handoff note"}, {"/api/backup", "Backup"}, {"/api/restore", "Restore"}, {"/api/data-folder", "Data folder"},
	{"/api/removed", "Removed SDS"}, {"/api/import", "Import"}, {"/api/integrity", "SDS file check"}, {"/api/binder", "Binder record"},
	{"/api/recovery", "Recovery"}, {"/api/reports/email", "Report emailed"},
}

var histVerbs = map[string]string{"done": "Marked done", "clear": "Cleared", "verify": "Verified", "tag": "Tagged", "out-of-service": "Put out of service",
	"tag-details": "Tag details saved", "restore": "Restored", "accept": "Accepted", "confirm": "Confirmed", "corrective": "Corrective action made",
	"deepdone": "Marked done", "starter": "Starter list added", "sync": "Synced", "xlsx": "Exported", "email": "Emailed", "full": "Full backup made",
	"restore-other": "Restored", "reopen": "Reopened"}

func (s *store) history(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if r.Method == http.MethodGet || r.Method == http.MethodHead || !strings.HasPrefix(path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		for _, p := range histSkip {
			if strings.HasPrefix(path, p) {
				next.ServeHTTP(w, r)
				return
			}
		}
		if strings.HasSuffix(path, "/extract") || strings.HasSuffix(path, "/pages") {
			next.ServeHTTP(w, r)
			return
		}
		var body map[string]any
		ct := r.Header.Get("Content-Type")
		if !strings.Contains(ct, "multipart") && r.ContentLength != 0 && r.ContentLength < 2<<20 {
			if b, err := io.ReadAll(r.Body); err == nil {
				r.Body = io.NopCloser(bytes.NewReader(b))
				_ = json.Unmarshal(b, &body)
			}
		}
		parts := strings.Split(strings.Trim(path, "/"), "/")
		var coll *collDef
		var before rec
		regID := ""
		if len(parts) >= 3 && parts[1] == "reg" {
			coll = regDefs[parts[2]]
			if len(parts) >= 4 {
				regID = parts[3]
			} else if body != nil {
				regID, _ = body["id"].(string)
			}
			if coll != nil && regID != "" && (r.Method == http.MethodDelete || r.Method == http.MethodPost) {
				s.mu.Lock()
				for _, x := range s.regLoad(coll) {
					if x["id"] == regID {
						before = x
					}
				}
				s.mu.Unlock()
			}
		}
		rw := &statusRW{ResponseWriter: w}
		next.ServeHTTP(rw, r)
		if rw.code >= 400 {
			return
		}
		h := histRow{At: time.Now().Format("2006-01-02T15:04:05")}
		str := func(m map[string]any, keys []string) string {
			for _, k := range keys {
				if v, ok := m[k].(string); ok && strings.TrimSpace(v) != "" {
					return strings.TrimSpace(v)
				}
			}
			return ""
		}
		if body != nil {
			h.Who = str(body, histWhoKeys)
			h.Label = str(body, histLabelKeys)
			h.ID, _ = body["id"].(string)
		}
		last := parts[len(parts)-1]
		verb := histVerbs[last]
		switch {
		case coll != nil:
			h.Kind = coll.Title
			h.ID = regID
			switch {
			case r.Method == http.MethodDelete:
				h.Action = "Deleted"
				if before != nil {
					b, _ := json.Marshal(before)
					h.Detail = string(b)
					h.Label = firstNonEmpty(before["name"], before["title"], before["task"], before["check"], before["issue"], before["area"], regID)
				}
			case before != nil:
				h.Action = "Changed"
				var ch []string
				for _, f := range coll.Fields {
					nv, _ := body[f.Key].(string)
					if before[f.Key] != nv {
						ch = append(ch, fmt.Sprintf("%s: %s → %s", f.Label, histClip(before[f.Key], 60), histClip(nv, 60)))
					}
				}
				h.Detail = strings.Join(ch, "; ")
				if len(ch) == 0 {
					h.Detail = "saved with no changes"
				}
			default:
				h.Action = "Added"
			}
		default:
			for _, n := range histNouns {
				if strings.HasPrefix(path, n[0]) {
					h.Kind = n[1]
					break
				}
			}
			if h.Kind == "" {
				h.Kind = strings.ReplaceAll(strings.TrimPrefix(path, "/api/"), "/", " ")
			}
			switch {
			case r.Method == http.MethodDelete:
				h.Action = "Deleted"
				h.ID = last
			case verb != "":
				h.Action = verb
			case r.Method == http.MethodPut || h.ID != "":
				h.Action = "Changed"
			default:
				h.Action = "Saved"
			}
		}
		if strings.HasPrefix(path, "/api/products/") && len(parts) >= 3 {
			h.ID = parts[2]
			s.mu.Lock()
			if p := s.find(h.ID); p != nil {
				h.Label = p.Name
			}
			s.mu.Unlock()
		}
		if h.Label == "" {
			h.Label = h.ID
		}
		s.addHistory(h)
	})
}

func histClip(v string, n int) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "(blank)"
	}
	if r := []rune(v); len(r) > n {
		return string(r[:n]) + "…"
	}
	return v
}

type statusRW struct {
	http.ResponseWriter
	code int
}

func (w *statusRW) WriteHeader(c int) { w.code = c; w.ResponseWriter.WriteHeader(c) }
func (w *statusRW) Write(b []byte) (int, error) {
	if w.code == 0 {
		w.code = 200
	}
	return w.ResponseWriter.Write(b)
}
func (w *statusRW) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (s *store) historyRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/history", func(w http.ResponseWriter, r *http.Request) {
		limit := 400
		if n, err := fmt.Sscanf(r.URL.Query().Get("limit"), "%d", &limit); n == 0 || err != nil || limit <= 0 || limit > 5000 {
			limit = 400
		}
		rows, total := s.historyRows(r.URL.Query().Get("q"), limit)
		writeJSON(w, 200, map[string]any{"rows": rows, "total": total, "file": s.historyFile()})
	})
}
