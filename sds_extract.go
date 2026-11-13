package main

import (
	"io"
	"net/http"
	"os"
	"strings"
)

// applyDraft fills the empty fields of a product from a drafted summary. It never overwrites what a
// manager typed, and never marks anything verified.
func applyDraft(p *Product, d *Draft) {
	fill := func(dst *string, v string) {
		if strings.TrimSpace(*dst) == "" {
			*dst = v
		}
	}
	fill(&p.Manufacturer, d.Manufacturer)
	fill(&p.RevisionDate, d.RevisionDate)
	fill(&p.SignalWord, d.SignalWord)
	fill(&p.Hazards, d.Hazards)
	fill(&p.FirstAidEyes, d.FirstAidEyes)
	fill(&p.FirstAidSkin, d.FirstAidSkin)
	fill(&p.FirstAidInhalation, d.FirstAidInhalation)
	fill(&p.FirstAidSwallowed, d.FirstAidSwallowed)
	fill(&p.PPE, d.PPE)
	if len(p.Pictograms) == 0 {
		p.Pictograms = d.Pictograms
	}
}

// draftFile reads a PDF on disk and drafts its summary; nil when it cannot be read.
func draftFile(path string) *Draft {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	d, err := extractDraft(b)
	if err != nil {
		return nil
	}
	return d
}

func (s *store) extractRoutes(mux *http.ServeMux) {
	reply := func(w http.ResponseWriter, b []byte) {
		d, err := extractDraft(b)
		if err != nil {
			writeJSON(w, 200, map[string]any{"unreadable": "The summary could not be read from this PDF: " + err.Error() + ". Type it from Sections 2, 4 and 8."})
			return
		}
		writeJSON(w, 200, d)
	}
	// A PDF that has not been saved yet (the Add screen).
	mux.HandleFunc("POST /api/extract", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			fail(w, 400, "the file could not be read")
			return
		}
		f, _, err := r.FormFile("pdf")
		if err != nil {
			fail(w, 400, "choose a PDF first")
			return
		}
		defer f.Close()
		b, err := io.ReadAll(io.LimitReader(f, 64<<20))
		if err != nil {
			fail(w, 400, "the file could not be read")
			return
		}
		reply(w, b)
	})
	// The PDF already stored for a product.
	mux.HandleFunc("POST /api/products/{id}/extract", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		p := s.find(r.PathValue("id"))
		var path string
		if p != nil {
			path = s.pdfPath(p)
		}
		s.mu.Unlock()
		if p == nil {
			fail(w, 404, "that product was not found")
			return
		}
		b, err := os.ReadFile(path)
		if err != nil {
			fail(w, 404, "the PDF for this product is missing")
			return
		}
		reply(w, b)
	})
}
