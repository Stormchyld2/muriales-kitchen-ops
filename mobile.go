package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Phone access: an optional second listener on the kitchen network that serves
// the phone page (/m/) and only the few APIs it needs. The full program stays
// on this computer; everything else is refused.

const phonePort = 47800

type phoneCfg struct {
	On bool `json:"on"`
}

var phone struct {
	mu      sync.Mutex
	srv     *http.Server
	port    int
	err     string
	inner   http.Handler
	cfgPath string
	log     []phoneEvent
	dir     string
	seen    map[string]bool // queued saves already received, so a retry never doubles an entry
	seenLog []string
	stamp   int // counts saves from phones, so the computer screen can refresh itself
}

// phoneEvent is one thing a phone sent: saves, and anything that failed.
type phoneEvent struct {
	At     string `json:"at"`
	From   string `json:"from"`
	Method string `json:"method"`
	Path   string `json:"path"`
	Code   int    `json:"code"`
	Note   string `json:"note,omitempty"`
}

type recWriter struct {
	http.ResponseWriter
	code int
	body bytes.Buffer
}

func (w *recWriter) WriteHeader(c int) { w.code = c; w.ResponseWriter.WriteHeader(c) }
func (w *recWriter) Write(b []byte) (int, error) {
	if w.code == 0 {
		w.code = 200
	}
	if w.code >= 400 && w.body.Len() < 300 {
		w.body.Write(b)
	}
	return w.ResponseWriter.Write(b)
}

func phoneRecord(r *http.Request, rw *recWriter) {
	if r.Method == "GET" && rw.code < 400 {
		return
	}
	note := ""
	if rw.code >= 400 {
		var m map[string]any
		if json.Unmarshal(rw.body.Bytes(), &m) == nil {
			note, _ = m["error"].(string)
		} else {
			note = strings.TrimSpace(rw.body.String())
		}
	}
	h, _, _ := net.SplitHostPort(r.RemoteAddr)
	phone.mu.Lock()
	if r.Method == "POST" && rw.code < 400 {
		phone.stamp++
	}
	phone.log = append(phone.log, phoneEvent{At: nowT().Format("1/2 3:04:05 PM"), From: h, Method: r.Method, Path: r.URL.Path, Code: rw.code, Note: note})
	if len(phone.log) > 30 {
		phone.log = phone.log[len(phone.log)-30:]
	}
	phone.mu.Unlock()
}

func phoneSeenPath() string { return filepath.Join(phone.dir, "phone-queue-ids.txt") }

func phoneSeen(id string) bool {
	phone.mu.Lock()
	defer phone.mu.Unlock()
	if phone.seen == nil {
		phone.seen = map[string]bool{}
		if b, err := os.ReadFile(phoneSeenPath()); err == nil {
			for _, l := range strings.Fields(string(b)) {
				phone.seen[l] = true
				phone.seenLog = append(phone.seenLog, l)
			}
		}
	}
	return phone.seen[id]
}

func phoneRemember(id string) {
	phone.mu.Lock()
	defer phone.mu.Unlock()
	phone.seen[id] = true
	phone.seenLog = append(phone.seenLog, id)
	if len(phone.seenLog) > 1000 {
		for _, o := range phone.seenLog[:500] {
			delete(phone.seen, o)
		}
		phone.seenLog = phone.seenLog[500:]
	}
	if phone.dir != "" {
		_ = os.WriteFile(phoneSeenPath(), []byte(strings.Join(phone.seenLog, "\n")), 0o644)
	}
}

func (s *store) phoneCfgPath() string {
	if phone.cfgPath != "" {
		return phone.cfgPath
	}
	if os.Getenv("SDSBOOK_DATA") != "" {
		return filepath.Join(s.dir, "phone-access.json")
	}
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), "Kitchen Ops phone.json")
	}
	return ""
}

func (s *store) phoneEnabled() bool {
	var c phoneCfg
	if b, err := os.ReadFile(s.phoneCfgPath()); err == nil {
		_ = json.Unmarshal(b, &c)
	}
	return c.On
}

func privateAddr(a string) bool {
	h, _, err := net.SplitHostPort(a)
	if err != nil {
		h = a
	}
	ip := net.ParseIP(h)
	return ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast())
}

func lanAddresses() []string {
	out := []string{}
	ifs, _ := net.Interfaces()
	for _, i := range ifs {
		if i.Flags&net.FlagUp == 0 || i.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := i.Addrs()
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil && n.IP.IsPrivate() {
				out = append(out, n.IP.String())
			}
		}
	}
	return out
}

type phoneRule struct {
	method string
	re     *regexp.Regexp
}

var phoneAllowed = []phoneRule{
	{"GET", regexp.MustCompile(`^/api/temps$`)},
	{"POST", regexp.MustCompile(`^/api/temps$`)},
	{"GET", regexp.MustCompile(`^/api/parcheck$`)},
	{"POST", regexp.MustCompile(`^/api/parcheck$`)},
	{"GET", regexp.MustCompile(`^/api/dw/today$`)},
	{"POST", regexp.MustCompile(`^/api/dw/tick$`)},
	{"GET", regexp.MustCompile(`^/api/reg/breakage$`)},
	{"POST", regexp.MustCompile(`^/api/reg/breakage$`)},
	{"POST", regexp.MustCompile(`^/api/notes$`)},
	// cleaning and the to-do list: see what is due and mark it done (verifying stays on the computer)
	{"GET", regexp.MustCompile(`^/api/clean/today$`)},
	{"POST", regexp.MustCompile(`^/api/clean/done$`)},
	{"POST", regexp.MustCompile(`^/api/clean/deepdone$`)},
	{"POST", regexp.MustCompile(`^/api/mgr/done$`)},
	{"GET", regexp.MustCompile(`^/api/todo$`)},
	{"GET", regexp.MustCompile(`^/api/phone/options$`)},
	{"GET", regexp.MustCompile(`^/api/reg/walkchecks$`)}, // facility walkthrough: the checks, and finishing the walk
	{"POST", regexp.MustCompile(`^/api/walk$`)},
	{"GET", regexp.MustCompile(`^/api/phone/attention$`)}, // what needs attention, SDS lookup, guides and station checklists
	{"GET", regexp.MustCompile(`^/api/phone/sds$`)},
	{"GET", regexp.MustCompile(`^/api/phone/sds/[^/]+/pdf$`)},
	{"GET", regexp.MustCompile(`^/api/stchecklists$`)},
	{"POST", regexp.MustCompile(`^/api/stchecklog/finish$`)},
	{"GET", regexp.MustCompile(`^/api/guides$`)},
	{"GET", regexp.MustCompile(`^/api/guides/[^/]+$`)},
	{"POST", regexp.MustCompile(`^/api/notes/[^/]+/done$`)}, // a manager marks a handoff note done
	{"GET", regexp.MustCompile(`^/api/phone/notes$`)},
	{"POST", regexp.MustCompile(`^/api/photo$`)}, // walkthrough photos
	{"GET", regexp.MustCompile(`^/api/photo/[A-Za-z0-9._-]+$`)},
	{"POST", regexp.MustCompile(`^/api/maint/pm/[^/]+/done$`)},
	{"POST", regexp.MustCompile(`^/api/maint/todos$`)},
	{"POST", regexp.MustCompile(`^/api/maint/todos/[^/]+/done$`)},
}

// phoneGate refuses anything that is not on the allowed list, and anything
// that does not come from a private (kitchen) network address.
func phoneGate(inner http.Handler) http.Handler {
	static, _ := fs.Sub(uiFiles, "ui")
	files := http.FileServer(http.FS(static))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !privateAddr(r.RemoteAddr) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		p := r.URL.Path
		if strings.HasPrefix(p, "/api/") {
			ok := false
			for _, ru := range phoneAllowed {
				if ru.method == r.Method && ru.re.MatchString(p) {
					ok = true
				}
			}
			if !ok {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			if r.Method == "POST" && p == "/api/reg/breakage" { // a phone adds entries, it never edits one
				b, _ := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<16))
				var m map[string]any
				if json.Unmarshal(b, &m) != nil || m["id"] != nil {
					http.Error(w, "forbidden", http.StatusForbidden)
					return
				}
				r.Body = io.NopCloser(bytes.NewReader(b))
			}
			rw := &recWriter{ResponseWriter: w}
			if qid := r.Header.Get("X-Queue-Id"); r.Method == "POST" && qid != "" && len(qid) <= 64 {
				replayMu.Lock()
				defer replayMu.Unlock()
				if phoneSeen(qid) {
					writeJSON(rw, 200, map[string]any{"duplicate": true})
					return
				}
				if ms, err := strconv.ParseInt(r.Header.Get("X-Queued-At"), 10, 64); err == nil {
					t := time.UnixMilli(ms)
					if t.Before(time.Now()) && time.Since(t) < 7*24*time.Hour && time.Since(t) > 20*time.Second {
						clockAt.Store(&t)
						defer clockAt.Store(nil)
					}
				}
				inner.ServeHTTP(rw, r)
				if rw.code < 400 {
					phoneRemember(qid)
				}
				phoneRecord(r, rw)
				return
			}
			inner.ServeHTTP(rw, r)
			phoneRecord(r, rw)
			return
		}
		switch {
		case p == "/" || p == "/m":
			http.Redirect(w, r, "/m/", http.StatusFound)
		case strings.HasPrefix(p, "/m/") || p == "/logo.png" || p == "/favicon.png" || p == "/app-icon.png":
			files.ServeHTTP(w, r)
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	})
}

// phoneApply starts or stops the phone listener to match the setting.
func (s *store) phoneApply(on bool) {
	phone.mu.Lock()
	defer phone.mu.Unlock()
	if phone.srv != nil {
		_ = phone.srv.Close()
		phone.srv, phone.port = nil, 0
	}
	phone.err = ""
	if !on || phone.inner == nil {
		return
	}
	for p := phonePort; p < phonePort+10; p++ {
		ln, err := net.Listen("tcp4", fmt.Sprintf("0.0.0.0:%d", p))
		if err != nil {
			continue
		}
		srv := &http.Server{Handler: phoneGate(phone.inner)}
		phone.srv, phone.port = srv, p
		go func() { _ = srv.Serve(ln) }()
		return
	}
	phone.err = "The phone page could not start because its ports are in use."
}

func phoneActivity() []phoneEvent {
	out := make([]phoneEvent, 0, len(phone.log))
	for i := len(phone.log) - 1; i >= 0 && len(out) < 15; i-- {
		out = append(out, phone.log[i])
	}
	return out
}

func (s *store) phoneStatus() map[string]any {
	phone.mu.Lock()
	defer phone.mu.Unlock()
	urls := []string{}
	if phone.srv != nil {
		for _, a := range lanAddresses() {
			urls = append(urls, fmt.Sprintf("http://%s:%d/", a, phone.port))
		}
	}
	return map[string]any{"on": s.phoneEnabled(), "running": phone.srv != nil, "port": phone.port, "urls": urls, "error": phone.err, "activity": phoneActivity()}
}

func (s *store) phoneRoutes(mux *http.ServeMux) {
	phone.dir = s.dir
	mux.HandleFunc("GET /api/phone", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, s.phoneStatus())
	})
	mux.HandleFunc("GET /api/phone/options", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		_, _, st := s.loadMaint()
		writeJSON(w, 200, map[string]any{"areas": st.areas(), "categories": findingCategories, "priorities": priorities, "portal": st.PortalURL})
	})
	mux.HandleFunc("GET /api/phone/attention", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		d := s.dashboard()
		writeJSON(w, 200, map[string]any{"items": d["attention"], "alerts": d["alerts"], "overdue": d["overdue"], "dueToday": d["dueToday"], "notes": d["notes"]})
	})
	mux.HandleFunc("GET /api/phone/notes", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		open := []*Note{}
		for i := len(s.lib.Notes) - 1; i >= 0 && len(open) < 30; i-- {
			if !s.lib.Notes[i].Done {
				open = append(open, s.lib.Notes[i])
			}
		}
		writeJSON(w, 200, map[string]any{"notes": open})
	})
	mux.HandleFunc("GET /api/phone/sds", func(w http.ResponseWriter, r *http.Request) {
		q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
		s.mu.Lock()
		defer s.mu.Unlock()
		out := []map[string]any{}
		for _, p := range s.lib.Products {
			hay := strings.ToLower(p.Name + " " + p.Manufacturer + " " + p.Aliases + " " + p.Tags + " " + p.Category + " " + p.Location)
			if q != "" && !strings.Contains(hay, q) {
				continue
			}
			if len(out) >= 40 {
				break
			}
			out = append(out, map[string]any{"id": p.ID, "name": p.Name, "manufacturer": p.Manufacturer, "category": p.Category, "location": p.Location, "signalWord": p.SignalWord, "hazards": p.Hazards,
				"eyes": p.FirstAidEyes, "skin": p.FirstAidSkin, "inhalation": p.FirstAidInhalation, "swallowed": p.FirstAidSwallowed, "ppe": p.PPE, "missing": p.Missing})
		}
		writeJSON(w, 200, map[string]any{"products": out})
	})
	mux.HandleFunc("GET /api/phone/sds/{id}/pdf", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		p := s.find(r.PathValue("id"))
		s.mu.Unlock()
		if p == nil || p.Missing {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		http.ServeFile(w, r, s.pdfPath(p))
	})
	mux.HandleFunc("GET /api/phone/stamp", func(w http.ResponseWriter, r *http.Request) {
		phone.mu.Lock()
		n := phone.stamp
		phone.mu.Unlock()
		writeJSON(w, 200, map[string]int{"n": n})
	})
	mux.HandleFunc("POST /api/phone", func(w http.ResponseWriter, r *http.Request) {
		var in phoneCfg
		if jsonDecode(r, &in) != nil {
			fail(w, 400, "Could not read that.")
			return
		}
		b, _ := json.Marshal(in)
		if p := s.phoneCfgPath(); p == "" || os.WriteFile(p, b, 0o644) != nil {
			fail(w, 500, "The setting could not be saved next to the program.")
			return
		}
		s.phoneApply(in.On)
		writeJSON(w, 200, s.phoneStatus())
	})
}
