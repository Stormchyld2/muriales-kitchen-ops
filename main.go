// Muriale's SDS Book - a small desktop program for keeping the restaurant's
// Safety Data Sheet library and printing the paper SDS binder.
//
// The program runs a private server that only this computer can reach and
// shows its screens in an application window. All sheets are stored as normal
// PDF files in the "SDS Data" folder next to the program.
package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

//go:embed ui
var uiFiles embed.FS

// starter holds the sheets that ship with the program. Each one is copied
// into the library once, the first time a build that contains it is run.
//
//go:embed starter
var starterFiles embed.FS

var lastPing atomic.Int64

// guard only lets this computer's own SDS Book window talk to the server.
func guard(port int, next http.Handler) http.Handler {
	hosts := map[string]bool{
		fmt.Sprintf("127.0.0.1:%d", port): true,
		fmt.Sprintf("localhost:%d", port): true,
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !hosts[r.Host] {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if o := r.Header.Get("Origin"); o != "" && !hosts[strings.TrimPrefix(o, "http://")] {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func alreadyRunning(port int) bool {
	if os.Getenv("SDSBOOK_SECOND") != "" { // testing: pretend to be a copy on another computer
		return false
	}
	c := http.Client{Timeout: 2 * time.Second}
	resp, err := c.Get(fmt.Sprintf("http://127.0.0.1:%d/api/ping", port))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 100))
	return strings.TrimSpace(string(b)) == appID
}

// dataDir works out where the library lives: a folder a manager chose in
// Settings if there is one, otherwise "SDS Data" next to the program. It also
// returns the path of the settings file that remembers the choice.
func dataDir() (dir, cfg string, chosen bool) {
	if d := os.Getenv("SDSBOOK_DATA"); d != "" {
		return d, "", false
	}
	exe, err := os.Executable()
	if err != nil {
		fatal("The program could not find its own location: " + err.Error())
	}
	cfg = filepath.Join(filepath.Dir(exe), "Kitchen Ops settings.json")
	if b, err := os.ReadFile(cfg); err == nil {
		var st settings
		if json.Unmarshal(b, &st) == nil && st.DataFolder != "" {
			return st.DataFolder, cfg, true
		}
	}
	return filepath.Join(filepath.Dir(exe), "SDS Data"), cfg, false
}

func main() {
	// Find a free port, or hand over to a copy that is already open.
	var ln net.Listener
	port := basePort
	for ; port < basePort+10; port++ {
		l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			ln = l
			break
		}
		if alreadyRunning(port) {
			_, _ = openWindow(fmt.Sprintf("http://127.0.0.1:%d/", port))
			return
		}
	}
	if ln == nil {
		fatal("Muriale's SDS Book could not start. Restart the computer and try again.")
	}

	dir, cfg, chosen := dataDir()
	// A chosen folder that has gone missing usually means a network or
	// OneDrive folder is offline. Stop here instead of quietly starting an
	// empty library somewhere else.
	if chosen && !exists(filepath.Join(dir, "library.json")) {
		fatal("Muriale's Kitchen Operations cannot reach its library folder:\n\n" + dir +
			"\n\nCheck that the network drive or OneDrive folder is connected, then open the program again." +
			"\n\nTo go back to the folder next to the program instead, delete the file\n\"Kitchen Ops settings.json\" beside the program.")
	}
	s := &store{dir: dir, cfg: cfg}
	s.startLock()
	if err := s.load(); err != nil {
		fatal("Muriale's SDS Book could not open its data folder:\n\n" + s.dir + "\n\n" + err.Error() +
			"\n\nMove the program to a folder you can save files in, such as Documents, and try again.")
	}
	if lf, err := os.OpenFile(filepath.Join(s.dir, "sdsbook-"+logName()+".log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644); err == nil {
		log.SetOutput(lf)
	}
	sort.SliceStable(s.lib.Products, func(i, j int) bool {
		return strings.ToLower(s.lib.Products[i].Name) < strings.ToLower(s.lib.Products[j].Name)
	})

	s.mu.Lock()
	s.purgeOld()
	s.mu.Unlock()
	go s.dailyBackupLoop()
	go s.autoSendLoop()
	go func() {
		for range time.Tick(6 * time.Hour) {
			s.mu.Lock()
			s.purgeOld()
			s.mu.Unlock()
		}
	}()

	quit := make(chan struct{})
	var quitOnce sync.Once
	stop := func() { quitOnce.Do(func() { close(quit) }) }
	mux := http.NewServeMux()
	s.routes(mux)
	mux.HandleFunc("GET /api/ping", func(w http.ResponseWriter, r *http.Request) {
		lastPing.Store(time.Now().UnixNano())
		fmt.Fprint(w, appID)
	})
	// The window says so as it closes. If no page checks in again within a
	// few seconds (a reload does), the program ends.
	mux.HandleFunc("GET /api/closing", func(w http.ResponseWriter, r *http.Request) {
		asked := time.Now().UnixNano()
		go func() {
			time.Sleep(12 * time.Second)
			if lastPing.Load() < asked {
				stop()
			}
		}()
		w.WriteHeader(http.StatusNoContent)
	})
	sub, _ := fs.Sub(uiFiles, "ui")
	fs := http.FileServer(http.FS(sub))
	mux.Handle("GET /", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		fs.ServeHTTP(w, r)
	}))

	go func() {
		if err := http.Serve(ln, guard(port, s.gate(s.history(mux)))); err != nil {
			log.Printf("server stopped: %v", err)
		}
	}()

	url := fmt.Sprintf("http://127.0.0.1:%d/", port)
	log.Printf("Muriale's SDS Book running at %s, data in %s", url, s.dir)
	started := time.Now()

	if win, err := openWindow(url); err != nil {
		log.Printf("could not open a window: %v", err)
	} else if win != nil {
		// When the application window is closed, the program ends.
		go func() {
			_ = win.Wait()
			if time.Since(started) > 8*time.Second {
				stop()
			}
		}()
	}

	// Safety net: the window checks in every few seconds. If it has been
	// silent for a few minutes the window is gone, so the program ends.
	go func() {
		for range time.Tick(20 * time.Second) {
			last := lastPing.Load()
			if last == 0 {
				if time.Since(started) > 5*time.Minute {
					stop()
					return
				}
				continue
			}
			if time.Since(time.Unix(0, last)) > 90*time.Second {
				stop()
				return
			}
		}
	}()

	go func() {
		for range time.Tick(beatEvery) {
			s.beat()
		}
	}()

	<-quit
	s.mu.Lock()
	finalDir := s.dir
	s.mu.Unlock()
	s.unlock(finalDir)
	os.Exit(0)
}

func logName() string {
	h, _ := os.Hostname()
	h = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' {
			return r
		}
		return '_'
	}, h)
	if h == "" {
		h = "computer"
	}
	return h
}

func (s *store) logf(format string, args ...any) { log.Printf(format, args...) }
