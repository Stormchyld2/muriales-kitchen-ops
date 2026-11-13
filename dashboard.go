package main

// The dashboard: one screen that says how the kitchen stands right now.
// Everything is worked out live from the records; nothing extra is stored.

import (
	"net/http"
	"sort"
	"strings"
	"time"
)

type gauge struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Pct   int    `json:"pct"` // -1 = not set up yet
	Sub   string `json:"sub"`
	View  string `json:"view"`
}

type tile struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Value string `json:"value"`
	Sub   string `json:"sub"`
	Tone  string `json:"tone"` // good, warn, bad, ""
	View  string `json:"view"`
}

func pctOf(good, total int) int {
	if total <= 0 {
		return 100
	}
	return good * 100 / total
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return itoa(n) + " " + many
}

// dashboard callers hold s.mu.
func (s *store) dashboard() map[string]any {
	now := time.Now()
	today := now.Format(dayFmt)
	g := []gauge{}
	t := []tile{}

	// 1. inspection readiness: share of the live checks that read READY
	rep := s.inspectionReadyReport()
	ready, total := 0, 0
	if len(rep.Sections) > 0 {
		for _, r := range rep.Sections[0].Rows {
			total++
			if r[1] == "READY" {
				ready++
			}
		}
	}
	g = append(g, gauge{"ready", "Inspection Ready", pctOf(ready, total), itoa(ready) + " of " + itoa(total) + " checks ready", "ready"})

	// 2. temperatures taken today
	pts := s.tempPoints()
	seen := map[string]bool{}
	for _, r := range s.tempViews(today, today) {
		seen[strings.ToLower(r.Point)] = true
	}
	done := 0
	for _, p := range pts {
		if seen[strings.ToLower(p.Name)] {
			done++
		}
	}
	if len(pts) == 0 {
		g = append(g, gauge{"temps", "Temperatures Today", -1, "No temperature points set up", "tpoints"})
	} else {
		g = append(g, gauge{"temps", "Temperatures Today", pctOf(done, len(pts)), itoa(done) + " of " + itoa(len(pts)) + " points taken", "temps"})
	}

	// 3. cleaning today
	ct := s.cleanToday()
	cdone, cdue := 0, 0
	if list, ok := ct["tasks"].([]rec); ok {
		for _, r := range list {
			switch r["state"] {
			case "Done Today":
				cdone++
			case "Overdue", "Due Today":
				cdue++
			}
		}
	}
	g = append(g, gauge{"clean", "Cleaning Today", pctOf(cdone, cdone+cdue), itoa(cdone) + " done, " + itoa(cdue) + " still due", "cltoday"})

	// 4. maintenance jobs on schedule
	pmTotal, pmLate := 0, 0
	for _, v := range s.pmList() {
		pmTotal++
		if v.State == "Overdue" {
			pmLate++
		}
	}
	g = append(g, gauge{"pm", "Maintenance On Time", pctOf(pmTotal-pmLate, pmTotal), plural(pmLate, "job overdue", "jobs overdue"), "pm"})

	// 5. training current
	tm := s.trainingMatrix()
	cells := tm.Count["Current"] + tm.Count["Due Soon"] + tm.Count["Expired"] + tm.Count["Missing"]
	if cells == 0 {
		g = append(g, gauge{"train", "Training Current", -1, "No employees on the training list", "tremp"})
	} else {
		g = append(g, gauge{"train", "Training Current", pctOf(tm.Count["Current"], cells), itoa(tm.Count["Expired"]+tm.Count["Missing"]) + " expired or missing, " + itoa(tm.Count["Due Soon"]) + " due soon", "trmatrix"})
	}

	// 6. station checklists finished today (Closing counts from 2 PM)
	{
		today, late := time.Now().Format(dayFmt), time.Now().Hour() >= 14
		fin := map[string]bool{}
		for _, l := range s.regLoad(regDefs["stchecklog"]) {
			if l["date"] == today {
				fin[l["station"]+"|"+l["kind"]] = true
			}
		}
		due, ok := 0, 0
		for _, c := range s.regLoad(regDefs["stchecklists"]) {
			if c["active"] == "No" || (c["kind"] == "Closing" && !late) {
				continue
			}
			due++
			if fin[c["station"]+"|"+c["kind"]] {
				ok++
			}
		}
		if due == 0 {
			g = append(g, gauge{"stchk", "Station Checklists", -1, "No checklists due yet", "stchk"})
		} else {
			g = append(g, gauge{"stchk", "Station Checklists", pctOf(ok, due), itoa(ok) + " of " + itoa(due) + " finished today", "stchk"})
		}
	}

	// tiles
	all := map[string]bool{}
	for _, k := range todoSources {
		all[k] = true
	}
	items := s.todoItems(now, now, all)
	over, dueToday := 0, 0
	for _, it := range items {
		switch it.State {
		case "Overdue":
			over++
		case "Due Today":
			dueToday++
		}
	}
	tone := func(n int, bad string) string {
		if n == 0 {
			return "good"
		}
		return bad
	}
	t = append(t, tile{"over", "Overdue", itoa(over), plural(dueToday, "more due today", "more due today"), tone(over, "bad"), "todo"})

	eq, is, _ := s.loadMaint()
	openIssues, urgent := 0, 0
	for _, p := range is {
		if p.Status == "Verified" || p.Status == "Closed" {
			continue
		}
		openIssues++
		if p.Priority == "High" || p.Priority == "Urgent" {
			urgent++
		}
	}
	t = append(t, tile{"issues", "Open Issues", itoa(openIssues), itoa(urgent) + " high or urgent", tone(urgent, "bad"), "issues"})

	caOpen, caLate := 0, 0
	for _, c := range s.caForms() {
		if c.Status == "Closed" {
			continue
		}
		caOpen++
		if c.Target != "" && c.Target < today {
			caLate++
		}
	}
	t = append(t, tile{"ca", "Corrective Actions", itoa(caOpen), itoa(caLate) + " past target date", tone(caLate, "bad"), "calog"})

	oos := 0
	for _, e := range eq {
		if e.Status == "Out of Service" {
			oos++
		}
	}
	t = append(t, tile{"oos", "Out of Service", itoa(oos), "pieces of equipment", tone(oos, "warn"), "equip"})

	bname, bwhen := s.lastBackup()
	bval, bsub, btone := "None yet", "A backup is made each day", "warn"
	if bname != "" {
		days := int(now.Sub(bwhen).Hours() / 24)
		switch {
		case bwhen.Format(dayFmt) == today:
			bval, bsub, btone = "Today", bwhen.Format("3:04 PM"), "good"
		case days <= 1:
			bval, bsub, btone = "Yesterday", bwhen.Format("Jan 2, 3:04 PM"), "good"
		default:
			bval, bsub, btone = plural(days, "day ago", "days ago"), bwhen.Format("Jan 2, 3:04 PM"), "bad"
		}
	}
	t = append(t, tile{"backup", "Last Backup", bval, bsub, btone, "settings"})

	// what needs attention: overdue and due today first
	rank := map[string]int{"Overdue": 0, "Due Today": 1}
	need := []todoItem{}
	for _, it := range items {
		if _, ok := rank[it.State]; ok {
			need = append(need, it)
		}
	}
	sort.SliceStable(need, func(i, j int) bool {
		if rank[need[i].State] != rank[need[j].State] {
			return rank[need[i].State] < rank[need[j].State]
		}
		return need[i].Due < need[j].Due
	})
	if len(need) > 40 {
		need = need[:40]
	}

	// open handoff notes, newest first
	notes := []*Note{}
	for _, n := range s.lib.Notes {
		if !n.Done {
			notes = append(notes, n)
		}
	}
	sort.SliceStable(notes, func(i, j int) bool { return notes[i].At > notes[j].At })
	if len(notes) > 20 {
		notes = notes[:20]
	}

	recent, _ := s.historyRows("", 8)

	return map[string]any{"gauges": g, "tiles": t, "attention": need, "notes": notes, "recent": recent,
		"overdue": over, "dueToday": dueToday, "today": today, "version": appVersion}
}

func (s *store) dashboardRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/dashboard", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		writeJSON(w, 200, s.dashboard())
	})
}
