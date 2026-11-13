package main

import "strings"

// deptWords are built-in extra words that put an item in a department, keyed
// by the department name in lower case with any trailing "s" removed.
var deptWords = map[string][]string{
	"front line":          {"cook line", "line cook", "grill", "sauté", "saute", "fryer", "flat top", "griddle", "hot holding", "heat lamp", "steam table", "bain-marie"},
	"back line":           {},
	"prep":                {"food processor", "mixer", "slicer", "cutting board"},
	"salad":               {"garde manger", "salad station"},
	"dish":                {"stewarding", "pot sink", "warewash", "dish machine"},
	"coolers & freezer":   {"cooler", "freezer", "walk-in", "walk in", "reach-in", "lowboy", "refrigeration"},
	"receiving & storage": {"receiving", "dry storage", "storage"},
}

// splitDeptLine reads one line from the Departments setting: the name, then
// optionally "|" and extra words that send items to it.
func splitDeptLine(l string) (string, []string) {
	name, rest, _ := strings.Cut(l, "|")
	var words []string
	for _, w := range strings.Split(rest, ",") {
		if w = strings.ToLower(strings.TrimSpace(w)); w != "" {
			words = append(words, w)
		}
	}
	return strings.TrimSpace(name), words
}

func deptKey(name string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(name)), "s")
}

// deptOf finds the department for an item from its area and wording.
func deptOf(lines []string, text string) string {
	t := strings.ToLower(text)
	for _, l := range lines {
		name, words := splitDeptLine(l)
		if name == "" || strings.EqualFold(name, "General") {
			continue
		}
		cands := append([]string{strings.ToLower(name), deptKey(name)}, words...)
		cands = append(cands, deptWords[deptKey(name)]...)
		for _, w := range cands {
			if w != "" && strings.Contains(t, w) {
				return name
			}
		}
	}
	return "General"
}

// byDept labels every item with its department and, when one department is
// asked for, keeps only that department's items. Callers hold s.mu.
func (s *store) byDept(items []todoItem, only string) []todoItem {
	_, _, st := s.loadMaint()
	depts := st.deptLines()
	out := make([]todoItem, 0, len(items))
	for _, it := range items {
		if it.Dept == "" {
			it.Dept = deptOf(depts, it.Title+" "+it.Detail)
		}
		if only == "" || strings.EqualFold(only, it.Dept) {
			out = append(out, it)
		}
	}
	return out
}
