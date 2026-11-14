package main

import (
	"net/http"
)

// Menu visibility: which tabs and subtabs the kitchen has hidden. Kept in a
// file so every computer using the program sees the same menu.

type menuPrefs struct {
	Hidden []string `json:"hidden"`
}

func (s *store) menuRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/menu", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		m := menuPrefs{Hidden: []string{}}
		s.readMaint("menu-hidden.json", &m)
		if m.Hidden == nil {
			m.Hidden = []string{}
		}
		writeJSON(w, 200, m)
	})
	mux.HandleFunc("POST /api/menu", func(w http.ResponseWriter, r *http.Request) {
		var in menuPrefs
		if jsonDecode(r, &in) != nil {
			fail(w, 400, "Could not read that.")
			return
		}
		clean := []string{}
		seen := map[string]bool{}
		for _, h := range in.Hidden {
			if (len(h) > 4 && (h[:4] == "mod:" || h[:4] == "sub:")) && len(h) < 40 && !seen[h] {
				seen[h] = true
				clean = append(clean, h)
			}
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		if err := s.writeMaint("menu-hidden.json", menuPrefs{Hidden: clean}); err != nil {
			fail(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, menuPrefs{Hidden: clean})
	})
}
