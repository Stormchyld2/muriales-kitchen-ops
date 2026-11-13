package main

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (s *store) backupDir() string { return filepath.Join(filepath.Dir(s.dir), "Kitchen Backups") }

// backupPath reads the backup file name from the request and makes sure it
// names a zip file inside the backups folder and nothing else.
func (s *store) backupPath(w http.ResponseWriter, r *http.Request) (string, error) {
	var in struct {
		File string `json:"file"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&in); err != nil {
		return "", errors.New("the request could not be read")
	}
	if in.File == "" || in.File != filepath.Base(in.File) || !strings.EqualFold(filepath.Ext(in.File), ".zip") {
		return "", errors.New("choose a backup file from the list")
	}
	path := filepath.Join(s.backupDir(), in.File)
	if !exists(path) {
		return "", errors.New("that backup file is no longer in the backups folder")
	}
	return path, nil
}

type backupInfo struct {
	Products    int      `json:"products"`
	Removed     int      `json:"removed"`
	Sheets      int      `json:"sheets"`
	MissingPDFs []string `json:"missingPdfs"` // products in the backup whose PDF is not in it
}

// backupEntry says whether a file inside a zip belongs to a library backup,
// and returns its cleaned path. Anything else in the zip is ignored.
func backupEntry(name string) (string, bool) {
	name = strings.ReplaceAll(name, "\\", "/")
	if name == "library.json" {
		return name, true
	}
	for _, dir := range []string{"sheets/", "removed/"} {
		rest := strings.TrimPrefix(name, dir)
		if rest != name && rest != "" && !strings.Contains(rest, "/") && rest != "." && rest != ".." {
			return dir + rest, true
		}
	}
	return "", false
}

func inspectBackup(path string) (*backupInfo, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, errors.New("that file is not a readable backup")
	}
	defer zr.Close()
	var lib *Library
	files := map[string]bool{}
	for _, f := range zr.File {
		name, ok := backupEntry(f.Name)
		if !ok {
			continue
		}
		files[strings.ToLower(name)] = true
		if name == "library.json" {
			rc, err := f.Open()
			if err != nil {
				return nil, errors.New("the backup is damaged")
			}
			b, err := io.ReadAll(io.LimitReader(rc, 64<<20))
			rc.Close()
			lib = &Library{}
			if err != nil || json.Unmarshal(b, lib) != nil {
				return nil, errors.New("the product list inside the backup is damaged")
			}
		}
	}
	if lib == nil {
		return nil, errors.New("that zip file is not a Kitchen Operations backup (it has no library.json)")
	}
	info := &backupInfo{Products: len(lib.Products), Removed: len(lib.Removed), MissingPDFs: []string{}}
	for name := range files {
		if strings.HasPrefix(name, "sheets/") {
			info.Sheets++
		}
	}
	for _, p := range lib.Products {
		file := p.File
		if file == "" {
			file = p.ID + ".pdf"
		}
		if !files["sheets/"+strings.ToLower(file)] {
			info.MissingPDFs = append(info.MissingPDFs, p.Name)
		}
	}
	return info, nil
}

// restore replaces the library with the contents of a backup. The current
// library is not deleted: it is moved, whole, into a "Before restore" folder
// beside the backups. If anything goes wrong, including the restored library
// failing to open, everything is put back as it was. It returns the folder
// holding the previous library.
func (s *store) restore(zipPath string) (string, error) {
	if _, err := inspectBackup(zipPath); err != nil {
		return "", err
	}
	parts := []string{"library.json", "sheets", "removed"}
	stage := filepath.Join(s.backupDir(), ".restore-in-progress")
	if err := os.RemoveAll(stage); err != nil {
		return "", fmt.Errorf("the restore could not start: %v", err)
	}
	defer os.RemoveAll(stage)
	for _, d := range []string{"sheets", "removed"} {
		if err := os.MkdirAll(filepath.Join(stage, d), 0o755); err != nil {
			return "", fmt.Errorf("the restore could not start: %v", err)
		}
	}

	// 1. Unpack the backup beside the live library, without touching it.
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", errors.New("that file is not a readable backup")
	}
	for _, f := range zr.File {
		name, ok := backupEntry(f.Name)
		if !ok || f.FileInfo().IsDir() {
			continue
		}
		err = func() error {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer rc.Close()
			out, err := os.Create(filepath.Join(stage, filepath.FromSlash(name)))
			if err != nil {
				return err
			}
			_, err = io.Copy(out, io.LimitReader(rc, maxPDF))
			if cerr := out.Close(); err == nil {
				err = cerr
			}
			return err
		}()
		if err != nil {
			zr.Close()
			return "", fmt.Errorf("the backup could not be unpacked, so nothing was changed: %v", err)
		}
	}
	zr.Close()

	// 2. Move the current library aside, then move the unpacked one in.
	hold := filepath.Join(s.backupDir(), "Before restore "+time.Now().Format("2006-01-02 150405"))
	if err := os.MkdirAll(hold, 0o755); err != nil {
		return "", fmt.Errorf("the current library could not be set aside, so nothing was changed: %v", err)
	}
	var out, in []string
	undo := func() {
		for _, n := range in {
			_ = os.RemoveAll(filepath.Join(s.dir, n))
		}
		for _, n := range out {
			_ = os.Rename(filepath.Join(hold, n), filepath.Join(s.dir, n))
		}
		_ = os.Remove(hold)
	}
	for _, n := range parts {
		if !exists(filepath.Join(s.dir, n)) {
			continue
		}
		if err := os.Rename(filepath.Join(s.dir, n), filepath.Join(hold, n)); err != nil {
			undo()
			return "", fmt.Errorf("the current library could not be set aside, so nothing was changed: %v", err)
		}
		out = append(out, n)
	}
	for _, n := range parts {
		if !exists(filepath.Join(stage, n)) {
			continue
		}
		if err := os.Rename(filepath.Join(stage, n), filepath.Join(s.dir, n)); err != nil {
			undo()
			return "", fmt.Errorf("the backup could not be put in place, so nothing was changed: %v", err)
		}
		in = append(in, n)
	}

	// 3. Open the restored library exactly as the program does at start-up.
	previous := s.lib
	if err := s.load(); err != nil {
		s.lib = previous
		undo()
		return "", fmt.Errorf("the restored library did not open correctly, so the previous library was put back: %v", err)
	}
	return hold, nil
}

func zipDir(src, dst string) error {
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	err = zipTo(out, src)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	return err
}
