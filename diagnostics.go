package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

type check struct {
	Name   string `json:"name"`
	Status string `json:"status"` // ok, warn or bad
	Detail string `json:"detail"`
}

func (s *store) newestBackup() (string, time.Time) {
	entries, _ := os.ReadDir(s.backupDir())
	var name string
	var when time.Time
	for _, e := range entries {
		if info, err := e.Info(); err == nil && !e.IsDir() && strings.EqualFold(filepath.Ext(e.Name()), ".zip") && info.ModTime().After(when) {
			name, when = e.Name(), info.ModTime()
		}
	}
	return name, when
}

func logTail(path string, lines int) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	if len(b) > 16<<10 {
		b = b[len(b)-16<<10:]
	}
	all := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(all) > lines {
		all = all[len(all)-lines:]
	}
	return strings.Join(all, "\n")
}

func (s *store) diagnostics() map[string]any {
	var checks []check
	add := func(name, status, detail string) { checks = append(checks, check{name, status, detail}) }

	add("Library file", "ok", fmt.Sprintf("library.json opened (format %d): %d products, %d removed.", s.lib.Schema, len(s.lib.Products), len(s.lib.Removed)))
	probe := filepath.Join(s.dir, ".write-test")
	if err := os.WriteFile(probe, []byte("ok"), 0o644); err != nil {
		add("Data folder", "bad", "The program cannot save files in the data folder: "+err.Error())
	} else {
		os.Remove(probe)
		add("Data folder", "ok", "The data folder can be written to.")
	}
	if free := freeSpace(s.dir); free >= 0 {
		gb := float64(free) / (1 << 30)
		st := "ok"
		if free < 1<<30 {
			st = "warn"
		}
		add("Free disk space", st, fmt.Sprintf("%.1f GB free on the drive holding the library.", gb))
	}
	missing, changed := 0, 0
	for _, r := range s.integrity(false) {
		switch r.Status {
		case "missing":
			missing++
		case "changed":
			changed++
		}
	}
	if missing > 0 {
		add("Sheets present", "bad", fmt.Sprintf("%d product(s) have no PDF in the sheets folder.", missing))
	} else {
		add("Sheets present", "ok", "Every product has its PDF.")
	}
	if changed > 0 {
		add("Document integrity", "bad", fmt.Sprintf("%d PDF(s) differ from the fingerprint taken when they were added.", changed))
	} else {
		add("Document integrity", "ok", "Every PDF matches the fingerprint taken when it was added.")
	}
	review, unverified := 0, 0
	for _, p := range s.lib.Products {
		if p.NeedsReview {
			review++
		}
		if !p.hasSummary() || p.SummaryNeedsCheck || p.SummaryVerifiedAt == "" {
			unverified++
		}
	}
	st := "ok"
	if review > 0 {
		st = "warn"
	}
	add("Needs review", st, fmt.Sprintf("%d product(s) are waiting for their details.", review))
	st = "ok"
	if unverified > 0 {
		st = "warn"
	}
	add("Emergency summaries", st, fmt.Sprintf("%d product(s) have no verified emergency summary.", unverified))
	if name, when := s.newestBackup(); name == "" {
		add("Backups", "warn", "No backup has been made yet.")
	} else if age := time.Since(when); age > 30*24*time.Hour {
		add("Backups", "warn", fmt.Sprintf("The newest backup is %d days old (%s).", int(age.Hours()/24), name))
	} else {
		add("Backups", "ok", fmt.Sprintf("Newest backup: %s.", name))
	}
	if exists(s.file() + ".bak") {
		add("Previous save", "ok", "A copy of the previous save is kept (library.json.bak).")
	} else {
		add("Previous save", "warn", "No previous-save copy exists yet.")
	}
	open := 0
	for _, n := range s.lib.Notes {
		if !n.Done {
			open++
		}
	}
	add("Handoff notes", "ok", fmt.Sprintf("%d open note(s).", open))

	exe, _ := os.Executable()
	tail := logTail(filepath.Join(s.dir, "sdsbook.log"), 30)
	var rep strings.Builder
	fmt.Fprintf(&rep, "Muriale's Kitchen Operations diagnostics\nVersion: %s\nChecked: %s\nSystem: %s/%s, %s\nProgram: %s\nLibrary folder: %s\nBackups folder: %s\n\n",
		appVersion, time.Now().Format("2006-01-02 15:04:05"), runtime.GOOS, runtime.GOARCH, runtime.Version(), exe, s.dir, s.backupDir())
	for _, c := range checks {
		fmt.Fprintf(&rep, "[%s] %s: %s\n", strings.ToUpper(c.Status), c.Name, c.Detail)
	}
	if tail != "" {
		fmt.Fprintf(&rep, "\nRecent program log:\n%s\n", tail)
	}
	return map[string]any{"version": appVersion, "checks": checks, "report": rep.String(), "program": exe, "folder": s.dir, "backups": s.backupDir()}
}

// recoverPreviousSave puts back the copy of library.json kept from the save
// before the current one. The library being replaced is kept beside it.
func (s *store) recoverPreviousSave() (string, error) {
	bak := s.file() + ".bak"
	b, err := os.ReadFile(bak)
	if err != nil {
		return "", errors.New("there is no previous-save copy to go back to")
	}
	var lib Library
	if err := json.Unmarshal(b, &lib); err != nil {
		return "", errors.New("the previous-save copy is damaged, so it was not used")
	}
	keep := s.file() + ".replaced-" + time.Now().Format("2006-01-02-150405")
	if exists(s.file()) {
		if err := copyFile(s.file(), keep); err != nil {
			return "", fmt.Errorf("the current library could not be kept safe, so nothing was changed: %v", err)
		}
	}
	tmp := s.file() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, s.file()); err != nil {
		return "", err
	}
	previous := s.lib
	if err := s.load(); err != nil {
		s.lib = previous
		if exists(keep) {
			_ = copyFile(keep, s.file())
		}
		return "", fmt.Errorf("the previous save did not open correctly, so it was put back as it was: %v", err)
	}
	return keep, nil
}

var archiveSuffix = regexp.MustCompile(`_\d{4}-\d{2}-\d{2}_\d{6}_[0-9a-f]{6}$`)

// ------------------------------------------------------------------ routes
