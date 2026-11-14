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
	phone.log = append(phone.log, phoneEvent{At: time.Now().Format("1/2 3:04:05 PM"), From: h, Method: r.Method, Path: r.URL.Path, Code: rw.code, Note: note})
	if len(phone.log) > 30 {
		phone.log = phone.log[len(phone.log)-30:]
	}
	phone.mu.Unlock()
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
	mux.HandleFunc("GET /api/phone", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, s.phoneStatus())
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
