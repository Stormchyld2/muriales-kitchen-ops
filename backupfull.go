package main

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Full backups hold both data folders in one zip: "SDS Data/..." and
// "Maintenance Data/...". Maintenance backups hold just the maintenance files.

func backupKind(name string) string {
	switch {
	case strings.HasPrefix(name, "Full Backup"):
		return "full"
	case strings.HasPrefix(name, "Maintenance Backup"):
		return "maint"
	}
	return "library"
}

func zipTree(zw *zip.Writer, src, prefix string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if strings.HasSuffix(path, ".tmp") || strings.HasSuffix(path, ".log") || strings.HasSuffix(path, ".lock") {
			return nil
		}
		rel, _ := filepath.Rel(src, path)
		zf, err := zw.Create(prefix + filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(zf, f)
		return err
	})
}

func (s *store) fullBackup() (string, error) {
	dir := s.backupDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", errors.New("the backup folder could not be created")
	}
	name := filepath.Join(dir, "Full Backup "+time.Now().Format("2006-01-02 150405")+".zip")
	out, err := os.Create(name)
	if err != nil {
		return "", err
	}
	zw := zip.NewWriter(out)
	err = zipTree(zw, s.dir, "SDS Data/")
	if err == nil && exists(s.maintDir()) {
		err = zipTree(zw, s.maintDir(), "Maintenance Data/")
	}
	if cerr := zw.Close(); err == nil {
		err = cerr
	}
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(name)
		return "", err
	}
	return name, nil
}

// restoreMaint replaces the maintenance files with those in a zip. The current
// files are zipped first into the backups folder.
func (s *store) restoreMaint(zipPath string) (string, error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", errors.New("that file is not a readable backup")
	}
	defer zr.Close()
	kind := backupKind(filepath.Base(zipPath))
	var files []*zip.File
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		n := filepath.ToSlash(f.Name)
		if kind == "full" {
			if !strings.HasPrefix(n, "Maintenance Data/") {
				continue
			}
			n = strings.TrimPrefix(n, "Maintenance Data/")
		}
		if n == "" || strings.HasPrefix(n, "/") || strings.Contains(n, "..") || strings.Contains(n, ":") {
			continue
		}
		files = append(files, f)
	}
	if len(files) == 0 {
		return "", errors.New("that backup has no maintenance files in it")
	}
	keep := ""
	if exists(s.maintDir()) {
		keep = filepath.Join(s.backupDir(), "Maintenance Before Restore "+time.Now().Format("2006-01-02 150405")+".zip")
		_ = os.MkdirAll(s.backupDir(), 0o755)
		if err := zipDir(s.maintDir(), keep); err != nil {
			return "", fmt.Errorf("the current files could not be saved first: %v", err)
		}
	}
	for _, f := range files {
		n := filepath.ToSlash(f.Name)
		if kind == "full" {
			n = strings.TrimPrefix(n, "Maintenance Data/")
		}
		dst := filepath.Join(s.maintDir(), filepath.FromSlash(n))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return keep, err
		}
		rc, err := f.Open()
		if err != nil {
			return keep, err
		}
		tmp := dst + ".tmp"
		o, err := os.Create(tmp)
		if err == nil {
			_, err = io.Copy(o, rc)
			if cerr := o.Close(); err == nil {
				err = cerr
			}
		}
		rc.Close()
		if err != nil {
			os.Remove(tmp)
			return keep, err
		}
		if err := os.Rename(tmp, dst); err != nil {
			return keep, err
		}
	}
	return keep, nil
}

// restoreFull restores both folders from a Full Backup zip.
func (s *store) restoreFull(zipPath string) (string, error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", errors.New("that file is not a readable backup")
	}
	tmp := filepath.Join(s.backupDir(), ".full-sds-part.zip")
	out, err := os.Create(tmp)
	if err != nil {
		zr.Close()
		return "", err
	}
	defer os.Remove(tmp)
	zw := zip.NewWriter(out)
	n := 0
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || !strings.HasPrefix(f.Name, "SDS Data/") {
			continue
		}
		w, err := zw.Create(strings.TrimPrefix(f.Name, "SDS Data/"))
		if err == nil {
			var rc io.ReadCloser
			if rc, err = f.Open(); err == nil {
				_, err = io.Copy(w, rc)
				rc.Close()
			}
		}
		if err != nil {
			zr.Close()
			out.Close()
			return "", err
		}
		n++
	}
	zr.Close()
	if cerr := zw.Close(); err == nil {
		err = cerr
	}
	out.Close()
	if n > 0 {
		if _, err := s.restore(tmp); err != nil {
			return "", err
		}
	}
	return s.restoreMaint(zipPath)
}

func (s *store) fullBackupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/backup/full", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		name, err := s.fullBackup()
		if err != nil {
			fail(w, 500, "the backup could not be written: "+err.Error())
			return
		}
		openFolder(s.backupDir())
		writeJSON(w, 200, map[string]string{"file": filepath.Base(name)})
	})
	mux.HandleFunc("POST /api/backup/restore-other", func(w http.ResponseWriter, r *http.Request) {
		path, err := s.backupPath(w, r)
		if err != nil {
			fail(w, 400, err.Error())
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		var keep string
		if backupKind(filepath.Base(path)) == "full" {
			keep, err = s.restoreFull(path)
		} else {
			keep, err = s.restoreMaint(path)
		}
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]string{"previousKeptIn": keep})
	})
}
