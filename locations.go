package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

// locationRoutes lets a manager keep the list of storage areas.
func (s *store) locationRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/locations", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Locations []string          `json:"locations"`
			Renames   map[string]string `json:"renames"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil {
			fail(w, 400, "the storage areas could not be read")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		seen := map[string]bool{}
		var list []string
		for _, l := range in.Locations {
			l = strings.Join(strings.Fields(l), " ")
			if l != "" && len(l) <= 60 && !seen[strings.ToLower(l)] {
				seen[strings.ToLower(l)] = true
				list = append(list, l)
			}
		}
		// Work out each product's new area first, so a refused change touches nothing.
		next := map[*Product]string{}
		inUse := map[string]int{}
		for _, p := range s.lib.Products {
			cur := strings.TrimSpace(p.Location)
			if to, ok := in.Renames[cur]; ok {
				cur = strings.Join(strings.Fields(to), " ")
				if cur != "" && !seen[strings.ToLower(cur)] {
					seen[strings.ToLower(cur)] = true
					list = append(list, cur)
				}
			}
			if cur != "" && !seen[strings.ToLower(cur)] {
				inUse[cur]++
			}
			next[p] = cur
		}
		if len(inUse) > 0 {
			var names []string
			for n, c := range inUse {
				names = append(names, fmt.Sprintf("%s (%d)", n, c))
			}
			sort.Strings(names)
			fail(w, 409, "These storage areas still have products in them: "+strings.Join(names, ", ")+". Move or rename them first.")
			return
		}
		oldList, old := s.lib.Locations, map[*Product]string{}
		for p, loc := range next {
			if p.Location != loc {
				old[p] = p.Location
				p.Location = loc
			}
		}
		s.lib.Locations = list
		if err := s.save(); err != nil {
			s.lib.Locations = oldList
			for p, loc := range old {
				p.Location = loc
			}
			fail(w, 500, "the library could not be saved: "+err.Error())
			return
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
	})
}
