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
	"regexp"
	"strings"
	"time"
)

type summarySettings struct {
	Recipients string `json:"recipients"` // comma separated
	CopyFolder string `json:"copyFolder"` // optional second place for the daily zip
	AutoSend   string `json:"autoSend"`   // HH:MM to send the summary by itself, blank for off
	LastSent   string `json:"lastSent,omitempty"`
	LastTry    string `json:"lastTry,omitempty"`   // RFC 3339
	LastError  string `json:"lastError,omitempty"` // why the last automatic send failed
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
		in.AutoSend = strings.TrimSpace(in.AutoSend)
		if in.AutoSend != "" && !autoRe.MatchString(in.AutoSend) {
			fail(w, 400, "Type the send time like 15:30, or leave it blank to turn it off.")
			return
		}
		if in.AutoSend != "" && len(addrs) == 0 {
			fail(w, 400, "Add at least one address before turning on the daily send.")
			return
		}
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
		old := s.summarySettings()
		in.LastSent, in.LastTry, in.LastError = old.LastSent, old.LastTry, ""
		if err := s.writeMaint("summary-settings.json", in); err != nil {
			fail(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, s.summarySettings())
	})
	// Sends it now through Outlook, no draft shown. Used to try the setup.
	mux.HandleFunc("POST /api/summary/send", func(w http.ResponseWriter, r *http.Request) {
		if err := s.sendSummary(sendOutlookMail); err != nil {
			fail(w, 400, "It could not be sent from here: "+err.Error()+". The Email Daily Summary button opens a draft you can send yourself.")
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
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

var autoRe = regexp.MustCompile(`^([01]?[0-9]|2[0-3]):[0-5][0-9]$`)

// sendSummary sends the summary through Outlook with no window. Called without s.mu.
func (s *store) sendSummary(send func(to []string, subject, body string) error) error {
	s.mu.Lock()
	st := s.summarySettings()
	rep := s.dailySummaryReport()
	s.mu.Unlock()
	addrs, _ := parseAddrs(st.Recipients)
	if len(addrs) == 0 {
		return fmt.Errorf("no one is on the daily summary list")
	}
	return send(addrs, "Daily Summary - "+time.Now().Format("Jan 2"), rep.text())
}

// autoSendTick sends the summary once a day at the set time, from the one
// computer that is editing, and tries again after 30 minutes if it fails.
func (s *store) autoSendTick(now time.Time, send func(to []string, subject, body string) error) {
	s.mu.Lock()
	st := s.summarySettings()
	due := st.AutoSend != "" && !s.isReadOnly() && st.LastSent != now.Format(dayFmt)
	if due {
		hm := st.AutoSend
		if len(hm) == 4 {
			hm = "0" + hm
		}
		due = now.Format("15:04") >= hm
	}
	if due && st.LastTry != "" {
		if t, err := time.Parse(time.RFC3339, st.LastTry); err == nil && now.Sub(t) < 30*time.Minute {
			due = false
		}
	}
	s.mu.Unlock()
	if !due {
		return
	}
	err := s.sendSummary(send)
	s.mu.Lock()
	defer s.mu.Unlock()
	st = s.summarySettings()
	st.LastTry = now.Format(time.RFC3339)
	if err != nil {
		st.LastError = err.Error()
		s.logf("daily summary not sent: %v", err)
	} else {
		st.LastSent, st.LastError = now.Format(dayFmt), ""
	}
	_ = s.writeMaint("summary-settings.json", st)
}

func (s *store) autoSendLoop() {
	for range time.Tick(time.Minute) {
		s.autoSendTick(time.Now(), sendOutlookMail)
	}
}
