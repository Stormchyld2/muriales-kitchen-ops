package main

import "strings"

// deptWords are extra words that put an item in one of the standard
// departments. A department added in Settings matches on its own name.
var deptWords = map[string][]string{
	"Front Line":          {"front line", "cook line", "line cook", "grill", "sauté", "saute"},
	"Back Line":           {"back line"},
	"Prep":                {"prep"},
	"Salad":               {"salad"},
	"Dish":                {"dish", "stewarding", "pot sink", "warewash"},
	"Coolers & Freezers":  {"cooler", "freezer", "walk-in", "walk in", "reach-in", "lowboy"},
	"Receiving & Storage": {"receiving", "dry storage", "storage"},
}

// deptOf finds the department for an item from its area and wording.
func deptOf(depts []string, text string) string {
	t := strings.ToLower(text)
	for _, d := range depts {
		if d == "General" {
			continue
		}
		words := append([]string{strings.ToLower(d)}, deptWords[d]...)
		for _, w := range words {
			if w != "" && strings.Contains(t, w) {
				return d
			}
		}
	}
	return "General"
}

// byDept labels every item with its department and, when one department is
// asked for, keeps only that department's items. Callers hold s.mu.
func (s *store) byDept(items []todoItem, only string) []todoItem {
	_, _, st := s.loadMaint()
	depts := st.departments()
	out := make([]todoItem, 0, len(items))
	for _, it := range items {
		it.Dept = deptOf(depts, it.Title+" "+it.Detail)
		if only == "" || strings.EqualFold(only, it.Dept) {
			out = append(out, it)
		}
	}
	return out
}
