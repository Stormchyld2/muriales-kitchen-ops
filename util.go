package main

import (
	"archive/zip"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// copyFile copies src to dst and makes sure the copy has reached the disk
// before reporting success.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err = io.Copy(out, in); err == nil {
		err = out.Sync()
	}
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(dst)
	}
	return err
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func now() string { return time.Now().Format("2006-01-02T15:04:05") }

var titleCleaner = strings.NewReplacer("_", " ", "-", " ")

func titleFromFile(name string) string {
	t := strings.Join(strings.Fields(titleCleaner.Replace(strings.TrimSuffix(name, filepath.Ext(name)))), " ")
	if t == "" {
		t = "Unnamed SDS"
	}
	return t
}

// csvCell stops a spreadsheet from running a cell as a formula.
func csvCell(v string) string {
	if v != "" && strings.ContainsAny(v[:1], "=+-@\t\r") {
		return "'" + v
	}
	return v
}

func zipTo(w io.Writer, src string) error {
	zw := zip.NewWriter(w)
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if strings.HasSuffix(path, ".tmp") || strings.HasSuffix(path, ".log") {
			return nil
		}
		rel, _ := filepath.Rel(src, path)
		zf, err := zw.Create(filepath.ToSlash(rel))
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
	if cerr := zw.Close(); err == nil {
		err = cerr
	}
	return err
}

func download(w http.ResponseWriter, contentType, name string) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
}
