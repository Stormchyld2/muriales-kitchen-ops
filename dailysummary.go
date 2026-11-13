package main

// Daily summary and where it goes. The program cannot send mail by itself: it
// opens a ready-made draft (Outlook or the default mail program) and a person
// presses Send. Recipients are saved in Settings, comma separated; an
// email-to-text address works too. A second folder can be set for backup copies.

import (
	"fmt"
	"net/http"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type summarySettings struct {
	Recipients string `json:"recipients"` // comma separated
	CopyFolder string `json:"copyFolder"` // optional second place for the daily zip
}

func (s *store) summarySettings() summarySettings {
	var m summarySettings
	s.readMaint("summary-settings.json", &m)
	return m
}

func parseAddrs(in string) ([]string, error) {
	var out []string
	for _, a := range strings.Split(strings.NewReplacer(";", ",", "\n", ",").Replace(in), ",") {
		if a = strings.TrimSpace(a); a == "" {
			continue
		}
		p, err := mail.ParseAddress(a)
		if err != nil || strings.ContainsAny(a, "\r\n") {
			return nil, fmt.Errorf("%q is not a valid email address", a)
		}
		out = append(out, p.Address)
	}
	return out, nil
}

// dailySummaryReport is deliberately short: it is meant to be read on a phone.
func (s *store) dailySummaryReport() *report {
	d := s.dashboard()
	var b strings.Builder
	line := func(f string, a ...any) { fmt.Fprintf(&b, f+"\n", a...) }
	line("Overdue: %d. Due today: %d.", d["overdue"], d["dueToday"])
	for _, g := range d["gauges"].([]gauge) {
		if g.Pct >= 0 {
			line("%s %d%% (%s)", g.Label, g.Pct, g.Sub)
		}
	}
	for _, t := range d["tiles"].([]tile) {
		if t.Key == "issues" || t.Key == "oos" || t.Key == "ca" {
			line("%s: %s (%s)", t.Label, t.Value, t.Sub)
		}
	}
	if al := d["alerts"].([]dashAlert); len(al) > 0 {
		line("ALERTS:")
		for i, a := range al {
			if i == 6 {
				line("... and %d more", len(al)-6)
				break
			}
			line("- %s", a.Text)
		}
	} else {
		line("No alerts.")
	}
	if at := d["attention"].([]todoItem); len(at) > 0 {
		line("MOST URGENT:")
		for i, a := range at {
			if i == 5 {
				break
			}
			line("- %s (%s)", a.Title, a.State)
		}
	}
	return &report{Title: "Muriale's Daily Summary", Subtitle: time.Now().Format("Monday, January 2, 2006 3:04 PM"), Sections: []reportSection{{Text: strings.TrimSpace(b.String())}}}
}

func (s *store) dailySummaryRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/summary/settings", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		writeJSON(w, 200, s.summarySettings())
	})
	mux.HandleFunc("POST /api/summary/settings", func(w http.ResponseWriter, r *http.Request) {
		var in summarySettings
		if jsonDecode(r, &in) != nil {
			fail(w, 400, "Could not read that.")
			return
		}
		addrs, err := parseAddrs(in.Recipients)
		if err != nil {
			fail(w, 400, "Check the list: "+err.Error()+".")
			return
		}
		in.Recipients = strings.Join(addrs, ", ")
		in.CopyFolder = strings.TrimSpace(in.CopyFolder)
		if in.CopyFolder != "" {
			if !filepath.IsAbs(in.CopyFolder) {
				fail(w, 400, "Type the full folder path, for example C:\\Users\\Name\\OneDrive\\Kitchen Backups.")
				return
			}
			if err := os.MkdirAll(in.CopyFolder, 0o755); err != nil {
				fail(w, 400, "That backup folder could not be reached or created.")
				return
			}
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		if err := s.writeMaint("summary-settings.json", in); err != nil {
			fail(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, s.summarySettings())
	})
	// Opens a draft addressed to the saved list. It never sends.
	mux.HandleFunc("POST /api/summary/email", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		st := s.summarySettings()
		rep := s.dailySummaryReport()
		s.mu.Unlock()
		addrs, _ := parseAddrs(st.Recipients)
		if len(addrs) == 0 {
			fail(w, 400, "No one is on the daily summary list yet. Add addresses in Settings, Daily Summary & Backup Copy.")
			return
		}
		method, detail, cut := composeEmail(addrs, "Daily Summary - "+time.Now().Format("Jan 2"), rep.text())
		writeJSON(w, 200, map[string]any{"method": method, "detail": detail, "cut": cut, "to": addrs})
	})
}

// copyBackup puts a copy of today's zip in the second folder, if one is set.
// Callers hold s.mu. Failure is logged, never fatal.
func (s *store) copyBackup(name string) {
	dst := s.summarySettings().CopyFolder
	if dst == "" {
		return
	}
	if exists(filepath.Join(dst, name)) {
		return
	}
	data, err := os.ReadFile(filepath.Join(s.backupDir(), name))
	if err == nil {
		err = os.MkdirAll(dst, 0o755)
	}
	if err == nil {
		err = os.WriteFile(filepath.Join(dst, name), data, 0o644)
	}
	if err != nil {
		s.logf("backup copy failed: %v", err)
		return
	}
	prune(dst, "Daily Backup ", keepDailyZips)
}
