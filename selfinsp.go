package main

// Weekly full self-inspection from the Health Inspection Success Program:
// each area is scored Compliant, Corrected on Spot, Major or Critical; the
// internal score is 100 minus the deductions, with a green / yellow / red band.
// A monthly performance review is worked out from the weekly scores.

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

func init() {
	regDefs["selfareas"] = &collDef{Name: "selfareas", Title: "Self-Inspection Areas", NewLabel: "Add Inspection Area", Prefix: "SIA-", Digits: 3, Deletable: true, Starter: "selfareas.json", Report: "selfinsp-blank",
		Intro:    "The areas scored on the weekly self-inspection and the standard for each. Add, reword or turn off areas here; the score is worked out from however many are active.",
		ListCols: []string{"id", "area", "standard", "active"},
		Fields:   []fieldDef{txt("area", "Inspection area", true), lng("standard", "Standard", true), yn("active", "Active?")}}
	regDefs["selfinsp"] = &collDef{Name: "selfinsp", Title: "Weekly Self-Inspections", NewLabel: "New Self-Inspection", Prefix: "SIN-", Digits: 4, Desc: true, Deletable: true, Report: "selfinsp-month",
		ListCols: []string{"id", "date", "by", "score", "band", "critical", "major", "cos"},
		Fields:   []fieldDef{dat("date", "Date", true), txt("by", "Inspector / manager", true), lng("results", "Results", false), dat("recheck", "Recheck date", false), yn("critResolved", "Critical findings corrected and verified?"), lng("notes", "Notes", false)}}
}

var selfRatings = map[string]int{"C": 0, "COS": 1, "M": 3, "CR": 5, "NA": 0}

type selfLine struct {
	AreaID, Area, Rating, Note string
}

// parseSelf reads "SIA-001|M|note" lines.
func parseSelf(v string) []selfLine {
	out := []selfLine{}
	for _, ln := range strings.Split(strings.ReplaceAll(v, "\r", ""), "\n") {
		p := strings.SplitN(ln, "|", 3)
		if len(p) < 2 || strings.TrimSpace(p[0]) == "" {
			continue
		}
		l := selfLine{AreaID: strings.TrimSpace(p[0]), Rating: strings.ToUpper(strings.TrimSpace(p[1]))}
		if len(p) == 3 {
			l.Note = strings.TrimSpace(p[2])
		}
		out = append(out, l)
	}
	return out
}

type selfScore struct {
	Score                   int
	Band                    string
	Crit, Major, COS, Rated int
}

// scoreSelf works out the internal score. Not-applicable areas are left out.
func scoreSelf(lines []selfLine, critResolved bool, repeatMajor bool) selfScore {
	var r selfScore
	ded := 0
	for _, l := range lines {
		pts, ok := selfRatings[l.Rating]
		if !ok || l.Rating == "NA" {
			continue
		}
		r.Rated++
		ded += pts
		switch l.Rating {
		case "CR":
			r.Crit++
		case "M":
			r.Major++
		case "COS":
			r.COS++
		}
	}
	r.Score = 100
	if r.Rated > 0 {
		r.Score = 100 - ded*100/(5*r.Rated)
	}
	switch {
	case r.Score < 90 || (r.Crit > 0 && !critResolved):
		r.Band = "Red"
	case r.Score >= 95 && !repeatMajor:
		r.Band = "Green"
	default:
		r.Band = "Yellow"
	}
	return r
}

func (s *store) selfAreaNames() map[string]rec {
	m := map[string]rec{}
	for _, a := range s.regLoad(regDefs["selfareas"]) {
		m[a["id"]] = a
	}
	return m
}

func (s *store) decorateSelf(rows []rec) {
	areas := s.selfAreaNames()
	list := s.regSorted(regDefs["selfinsp"])
	for _, r := range rows {
		lines := parseSelf(r["results"])
		prev := s.prevSelfMajors(list, r)
		rep := false
		for _, l := range lines {
			if (l.Rating == "M" || l.Rating == "CR") && prev[l.AreaID] {
				rep = true
			}
		}
		sc := scoreSelf(lines, r["critResolved"] == "Yes", rep)
		r["score"], r["band"] = fmt.Sprint(sc.Score), sc.Band
		r["critical"], r["major"], r["cos"] = fmt.Sprint(sc.Crit), fmt.Sprint(sc.Major), fmt.Sprint(sc.COS)
		_ = areas
	}
}

// prevSelfMajors are the areas that were Major or Critical on the inspection before this one.
func (s *store) prevSelfMajors(list []rec, cur rec) map[string]bool {
	out := map[string]bool{}
	var prev rec
	for _, r := range list {
		if r["id"] == cur["id"] || r["date"] > cur["date"] || (r["date"] == cur["date"] && r["id"] > cur["id"]) {
			continue
		}
		if prev == nil || r["date"] > prev["date"] || (r["date"] == prev["date"] && r["id"] > prev["id"]) {
			prev = r
		}
	}
	if prev != nil {
		for _, l := range parseSelf(prev["results"]) {
			if l.Rating == "M" || l.Rating == "CR" {
				out[l.AreaID] = true
			}
		}
	}
	return out
}

// selfMonth is the monthly performance review worked out from the weekly scores.
func (s *store) selfMonth(month string) map[string]any {
	rows := s.decorateSelfList()
	areas := s.selfAreaNames()
	n, sum, low, crit, major := 0, 0, 101, 0, 0
	repeats := map[string]int{}
	for _, r := range rows {
		if !strings.HasPrefix(r["date"], month) {
			continue
		}
		var sc int
		fmt.Sscan(r["score"], &sc)
		n++
		sum += sc
		if sc < low {
			low = sc
		}
		var c, m int
		fmt.Sscan(r["critical"], &c)
		fmt.Sscan(r["major"], &m)
		crit += c
		major += m
		for _, l := range parseSelf(r["results"]) {
			if l.Rating == "M" || l.Rating == "CR" {
				repeats[l.AreaID]++
			}
		}
	}
	type kv struct {
		id string
		n  int
	}
	var kvs []kv
	for id, c := range repeats {
		kvs = append(kvs, kv{id, c})
	}
	sort.Slice(kvs, func(i, j int) bool { return kvs[i].n > kvs[j].n || (kvs[i].n == kvs[j].n && kvs[i].id < kvs[j].id) })
	top := []string{}
	for i, k := range kvs {
		if i == 3 {
			break
		}
		name := k.id
		if a, ok := areas[k.id]; ok {
			name = a["area"]
		}
		top = append(top, fmt.Sprintf("%s (%dx)", name, k.n))
	}
	overdue := 0
	for _, f := range s.caForms() {
		if f.overdue() {
			overdue++
		}
	}
	out := map[string]any{"month": month, "count": n, "criticals": crit, "majors": major, "top": top, "overdueCA": overdue, "avg": 0, "low": 0}
	if n > 0 {
		out["avg"], out["low"] = sum/n, low
	}
	return out
}

func (s *store) decorateSelfList() []rec {
	rows := s.regSorted(regDefs["selfinsp"])
	s.decorateSelf(rows)
	return rows
}

func (s *store) selfAttention(now time.Time) []todoItem {
	rows := s.regSorted(regDefs["selfinsp"])
	if len(rows) == 0 {
		return nil
	}
	last := ""
	for _, r := range rows {
		if r["date"] > last {
			last = r["date"]
		}
	}
	if t, err := time.ParseInLocation(dayFmt, last, time.Local); err == nil && now.Sub(t) > 8*24*time.Hour {
		return []todoItem{{Group: "Self-Inspection", Title: "Weekly self-inspection overdue", Detail: "last done " + last, Due: last, State: "Overdue", Kind: "selfinsp", ID: "selfinsp"}}
	}
	return nil
}

func (s *store) selfReport(id string) (*report, bool) {
	today := time.Now().Format("January 2, 2006")
	areas := s.regSorted(regDefs["selfareas"])
	switch {
	case id == "selfinsp-blank":
		rows := [][]string{}
		for _, a := range areas {
			if a["active"] != "No" {
				rows = append(rows, []string{a["area"], a["standard"], "☐", "☐", "☐", ""})
			}
		}
		return &report{Title: "Weekly Full Self-Inspection", Subtitle: "Score each item C = Compliant, COS = Corrected on Spot, M = Major, CR = Critical. Record every M or CR item in corrective action.", Sections: []reportSection{
			{Cols: []string{"Inspection area", "Standard", "C", "COS", "M/CR", "Notes / CA ref"}, Rows: rows},
			{Note: "Internal Score: ______ / 100    Critical Findings: ______    Corrected on Spot: ______    Major Findings: ______\nInspector/Manager: ______________________    Date: ____________    Recheck Date: ____________"}}}, true
	case strings.HasPrefix(id, "selfinsp:"):
		names := s.selfAreaNames()
		for _, r := range s.decorateSelfList() {
			if r["id"] != strings.TrimPrefix(id, "selfinsp:") {
				continue
			}
			rows := [][]string{}
			for _, l := range parseSelf(r["results"]) {
				a := names[l.AreaID]
				rows = append(rows, []string{a["area"], l.Rating, l.Note})
			}
			return &report{Title: "Weekly Self-Inspection " + r["id"], Subtitle: r["date"] + " - " + r["by"], Sections: []reportSection{
				{Note: fmt.Sprintf("Internal score %s / 100 (%s). Critical %s, Major %s, Corrected on spot %s.", r["score"], r["band"], r["critical"], r["major"], r["cos"]), Cols: []string{"Inspection area", "Rating", "Notes"}, Rows: rows}}}, true
		}
		return nil, false
	case id == "selfinsp-month":
		m := s.selfMonth(time.Now().Format("2006-01"))
		top := strings.Join(m["top"].([]string), "; ")
		rows := [][]string{{"Month", time.Now().Format("January 2006")}, {"Weekly inspections done", fmt.Sprint(m["count"])}, {"Average internal inspection score", fmt.Sprint(m["avg"])}, {"Lowest weekly score", fmt.Sprint(m["low"])}, {"Number of critical findings", fmt.Sprint(m["criticals"])}, {"Number of major findings", fmt.Sprint(m["majors"])}, {"Top repeat findings", top}, {"Overdue corrective actions", fmt.Sprint(m["overdueCA"])},
			{"Station / shift of concern", ""}, {"Maintenance / facility risk", ""}, {"Training completed", ""}, {"Retraining needed", ""}, {"Program / SOP change needed", ""}, {"Priority #1 for next month", ""}, {"Priority #2 for next month", ""}, {"Priority #3 for next month", ""}, {"Reviewed by / date", ""}}
		return &report{Title: "Monthly Inspection Performance Review", Subtitle: today, Sections: []reportSection{{Cols: []string{"Item", "Result"}, Rows: rows}}}, true
	}
	return nil, false
}

func (s *store) selfRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/selfinsp", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		areas := []rec{}
		for _, a := range s.regSorted(regDefs["selfareas"]) {
			if a["active"] != "No" {
				areas = append(areas, a)
			}
		}
		writeJSON(w, 200, map[string]any{"areas": areas, "rows": s.decorateSelfList(), "month": s.selfMonth(time.Now().Format("2006-01"))})
	})
	mux.HandleFunc("GET /api/selfinsp/month", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		m := r.URL.Query().Get("month")
		if _, err := time.Parse("2006-01", m); err != nil {
			m = time.Now().Format("2006-01")
		}
		writeJSON(w, 200, s.selfMonth(m))
	})
	mux.HandleFunc("POST /api/selfinsp", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			ID, Date, By, Results, Recheck, CritResolved, Notes string
			MakeCA                                              bool
		}
		if jsonDecode(r, &in) != nil {
			fail(w, 400, "Could not read that.")
			return
		}
		in.By = strings.TrimSpace(in.By)
		if in.By == "" {
			fail(w, 400, "Enter who did the inspection.")
			return
		}
		if _, err := time.Parse(dayFmt, in.Date); err != nil {
			in.Date = time.Now().Format(dayFmt)
		}
		lines := parseSelf(in.Results)
		for _, l := range lines {
			if _, ok := selfRatings[l.Rating]; !ok {
				fail(w, 400, "Every area needs a rating (C, COS, M, CR or NA).")
				return
			}
		}
		if len(lines) == 0 {
			fail(w, 400, "Score at least one area.")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.isReadOnly() {
			fail(w, 423, errReadOnly.Error())
			return
		}
		c := regDefs["selfinsp"]
		row := rec{"date": in.Date, "by": in.By, "results": in.Results, "recheck": in.Recheck, "critResolved": in.CritResolved, "notes": in.Notes}
		var id string
		if in.ID != "" {
			rows := s.regLoad(c)
			found := false
			for _, x := range rows {
				if x["id"] == in.ID {
					for k, v := range row {
						x[k] = v
					}
					found = true
				}
			}
			if !found {
				fail(w, 404, "That inspection no longer exists.")
				return
			}
			if err := s.regSave(c, rows); err != nil {
				fail(w, 500, err.Error())
				return
			}
			id = in.ID
		} else {
			saved, err := s.regAdd(c, row)
			if err != nil {
				fail(w, 500, err.Error())
				return
			}
			id = saved["id"]
		}
		made := 0
		if in.MakeCA && in.ID == "" {
			names := s.selfAreaNames()
			forms := s.caForms()
			next := 0
			for _, f := range forms {
				var n int
				if _, err := fmt.Sscanf(f.ID, "CA-%d", &n); err == nil && n > next {
					next = n
				}
			}
			now := time.Now().Format("2006-01-02 15:04")
			for _, l := range lines {
				if l.Rating != "M" && l.Rating != "CR" {
					continue
				}
				next++
				a := names[l.AreaID]
				pri := "P2 - Urgent"
				if l.Rating == "CR" {
					pri = "P1 - Critical"
				}
				f := &CAForm{ID: fmt.Sprintf("CA-%04d", next), Date: in.Date, IdentifiedBy: in.By, Area: "Food Safety", Priority: pri, Issue: a["area"] + ": " + firstNonEmpty(l.Note, a["standard"]), Status: "Open", Related: "Weekly self-inspection " + id, Created: now, Updated: now}
				forms = append(forms, f)
				s.caSaveFile(f)
				made++
			}
			if made > 0 {
				if err := s.writeMaint("corrective-actions.json", forms); err != nil {
					fail(w, 500, err.Error())
					return
				}
			}
		}
		writeJSON(w, 200, map[string]any{"id": id, "made": made})
	})
}
