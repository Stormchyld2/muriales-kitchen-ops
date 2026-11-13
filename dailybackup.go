package main

// Daily backup: once a day the program zips both data folders (the SDS library
// and the Maintenance Data) into "Kitchen Backups" as "Daily Backup <date>.zip",
// and keeps the last two weeks. It checks at start-up and then every hour, so a
// kitchen that leaves the program open overnight still gets one each day.

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

const keepDailyZips = 14

func (s *store) dailyBackup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.isReadOnly() || !exists(s.file()) {
		return
	}
	dir := s.backupDir()
	name := "Daily Backup " + time.Now().Format(dayFmt) + ".zip"
	if exists(filepath.Join(dir, name)) {
		s.copyBackup(name) // a copy folder set after today's zip was made
		return
	}
	if _, err := s.fullBackupAs(name); err != nil {
		s.logf("daily backup failed: %v", err)
		return
	}
	prune(dir, "Daily Backup ", keepDailyZips)
	s.copyBackup(name)
}

func (s *store) dailyBackupLoop() {
	s.dailyBackup()
	for range time.Tick(time.Hour) {
		s.dailyBackup()
	}
}

// lastBackup describes the newest backup of any kind, for the dashboard.
func (s *store) lastBackup() (name string, when time.Time) {
	entries, _ := os.ReadDir(s.backupDir())
	for _, e := range entries {
		if info, err := e.Info(); err == nil && !e.IsDir() && strings.EqualFold(filepath.Ext(e.Name()), ".zip") &&
			!strings.Contains(e.Name(), "Before") && info.ModTime().After(when) {
			name, when = e.Name(), info.ModTime()
		}
	}
	return
}
