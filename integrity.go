package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"time"
)

type hashEntry struct {
	size int64
	mod  time.Time
	sum  string
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func sumBytes(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// fileStatus compares a product's PDF with the fingerprint taken when it was
// added. The program never writes to a stored PDF, so anything other than
// "ok" means the file was changed or damaged outside the program.
func (s *store) fileStatus(p *Product, full bool) string {
	path := s.pdfPath(p)
	info, err := os.Stat(path)
	if err != nil {
		return "missing"
	}
	if p.SHA256 == "" {
		return "ok"
	}
	if s.hc == nil {
		s.hc = map[string]hashEntry{}
	}
	e, ok := s.hc[path]
	if !full && ok && e.size == info.Size() && e.mod.Equal(info.ModTime()) {
		if e.sum == p.SHA256 {
			return "ok"
		}
		return "changed"
	}
	sum, err := hashFile(path)
	if err != nil {
		return "missing"
	}
	s.hc[path] = hashEntry{info.Size(), info.ModTime(), sum}
	if sum == p.SHA256 {
		return "ok"
	}
	return "changed"
}

type integrityRow struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	File   string `json:"file"`
	Status string `json:"status"` // ok, changed or missing
}

func (s *store) integrity(full bool) []integrityRow {
	rows := make([]integrityRow, 0, len(s.lib.Products))
	for _, p := range s.lib.Products {
		rows = append(rows, integrityRow{p.ID, p.Name, filepath.Base(s.pdfPath(p)), s.fileStatus(p, full)})
	}
	return rows
}

// integrityProblems lists only the products whose file is not as it was added.
func (s *store) integrityProblems() map[string]string {
	out := map[string]string{}
	for _, r := range s.integrity(false) {
		if r.Status != "ok" {
			out[r.ID] = r.Status
		}
	}
	return out
}

// ----------------------------------------------------------------- helpers
