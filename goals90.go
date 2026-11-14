package main

// 90-day introductory goals: nine signed short-term goals with an owner, target
// date, status and evidence, six ongoing objectives for the leadership
// scorecard, and a weekly progress review. Lives under Maintenance.

import (
	"fmt"
	"net/http"
	"time"
)

var g90Statuses = []string{"Not Started", "In Progress", "Blocked", "Done"}

func init() {
	regDefs["goals90"] = &collDef{Name: "goals90", Title: "90-Day Goals", NewLabel: "Add Goal", Prefix: "G90-", Digits: 3, Deletable: true, Starter: "goals90.json", Report: "g90",
		Intro:    "The nine signed short-term goals. Set an owner and target date, keep Status and Next action current, and write what proves it is finished in Completion evidence.",
		ListCols: []string{"id", "title", "owner", "target", "status", "nextAction"},
		Fields:   []fieldDef{txt("title", "Goal", true), lng("scope", "What it covers", false), txt("owner", "Owner", false), dat("target", "Target date", false), sel("status", "Status", g90Statuses, true), lng("nextAction", "Next action / blocker", false), lng("evidence", "Completion evidence / notes", false)}}
	regDefs["obj90"] = &collDef{Name: "obj90", Title: "Leadership Scorecard", NewLabel: "Add Objective", Prefix: "OB90-", Digits: 2, Deletable: true, Starter: "obj90.json", Report: "g90",
		Intro:    "Ongoing objectives for the introductory period. Rate each one at the weekly review.",
		ListCols: []string{"id", "objective", "rating", "notes"},
		Fields:   []fieldDef{lng("objective", "Objective", true), sel("rating", "Rating", []string{"On Track", "Needs Attention", "Behind"}, true), lng("notes", "Notes", false)}}
	regDefs["rev90"] = &collDef{Name: "rev90", Title: "Weekly Progress Reviews", NewLabel: "Add Weekly Review", Prefix: "WR-", Digits: 3, Desc: true, Deletable: true, Report: "g90",
		Intro:    "One review a week: what got finished, what is outstanding or blocked, and what the leader needs to decide.",
		ListCols: []string{"id", "date", "done", "outstanding", "decision"},
		Fields:   []fieldDef{dat("date", "Review date", true), lng("done", "Completed this week", false), lng("outstanding", "Outstanding / blocker", false), lng("decision", "Leader decision needed", false)}}
}

type g90Settings struct {
	Start string `json:"start"` // first day of the 90 days
}

func (s *store) g90Settings() g90Settings {
	var g g90Settings
	s.readMaint("goals90-settings.json", &g)
	if _, err := time.Parse(dayFmt, g.Start); err != nil {
		g.Start = ""
	}
	return g
}

var g90Phases = []string{"Week 1: baseline, owners, supplies, budget needs, dependencies", "Weeks 2-4: quick wins and published operating standards", "Weeks 5-8: verify use in real operations; train and correct gaps", "Weeks 9-12: audit performance, close actions, document results"}

func (s *store) g90View(now time.Time) map[string]any {
	st := s.g90Settings()
	out := map[string]any{"start": st.Start, "phases": g90Phases, "day": 0, "week": 0, "phase": -1}
	if st.Start != "" {
		a, _ := time.ParseInLocation(dayFmt, st.Start, time.Local)
		day := int(now.Sub(a).Hours()/24) + 1
		if day < 1 {
			day = 0
		}
		out["day"] = day
		if day >= 1 {
			wk := (day-1)/7 + 1
			out["week"] = wk
			ph := 3
			switch {
			case wk <= 1:
				ph = 0
			case wk <= 4:
				ph = 1
			case wk <= 8:
				ph = 2
			}
			out["phase"] = ph
		}
	}
	counts := map[string]int{}
	goals := s.regSorted(regDefs["goals90"])
	for _, g := range goals {
		counts[firstNonEmpty(g["status"], "Not Started")]++
	}
	out["goals"], out["counts"], out["total"] = goals, counts, len(goals)
	out["objectives"] = s.regSorted(regDefs["obj90"])
	revs := s.regSorted(regDefs["rev90"])
	if len(revs) > 5 {
		revs = revs[:5]
	}
	out["reviews"] = revs
	mon := mondayOf(now).Format(dayFmt)
	reviewed := false
	for _, r := range revs {
		if r["date"] >= mon {
			reviewed = true
		}
	}
	out["reviewedThisWeek"] = reviewed
	return out
}

func (s *store) g90Attention(today string) []todoItem {
	out := []todoItem{}
	for _, g := range s.regLoad(regDefs["goals90"]) {
		if g["status"] == "Done" {
			continue
		}
		state := ""
		switch {
		case g["status"] == "Blocked":
			state = "Overdue"
		case g["target"] != "" && g["target"] < today:
			state = "Overdue"
		case g["target"] == today:
			state = "Due Today"
		}
		if state == "" {
			continue
		}
		out = append(out, todoItem{Group: "90-Day Goals", Title: "90-day goal: " + clipText(g["title"], 60), Detail: firstNonEmpty(g["nextAction"], g["status"]), Due: g["target"], State: state, Kind: "goal90", ID: g["id"]})
	}
	return out
}

func (s *store) g90Report() *report {
	v := s.g90View(time.Now())
	rep := &report{Title: "90-Day Introductory Goals", Subtitle: "Implementation & Progress Tracker"}
	rows := [][]string{}
	for n, g := range v["goals"].([]rec) {
		rows = append(rows, []string{fmt.Sprint(n+1) + ". " + g["title"], g["owner"], g["target"], g["status"], g["nextAction"], g["evidence"]})
	}
	rep.Sections = append(rep.Sections, reportSection{Heading: "Short-term goals", Cols: []string{"Goal", "Owner", "Target", "Status", "Next action / blocker", "Completion evidence"}, Rows: rows})
	rows = [][]string{}
	for _, o := range v["objectives"].([]rec) {
		rows = append(rows, []string{o["objective"], o["rating"], o["notes"]})
	}
	rep.Sections = append(rep.Sections, reportSection{Heading: "Leadership scorecard", Cols: []string{"Objective", "Rating", "Notes"}, Rows: rows})
	rows = [][]string{}
	for _, r := range s.regSorted(regDefs["rev90"]) {
		rows = append(rows, []string{r["date"], r["done"], r["outstanding"], r["decision"]})
	}
	for len(rows) < 5 {
		rows = append(rows, []string{"", "", "", ""})
	}
	rep.Sections = append(rep.Sections, reportSection{Heading: "Weekly progress review", Cols: []string{"Review date", "Completed this week", "Outstanding / blocker", "Leader decision needed"}, Rows: rows})
	return rep
}

func (s *store) g90Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/g90", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		writeJSON(w, 200, s.g90View(time.Now()))
	})
	mux.HandleFunc("POST /api/g90/start", func(w http.ResponseWriter, r *http.Request) {
		var in g90Settings
		if jsonDecode(r, &in) != nil {
			fail(w, 400, "Could not read that.")
			return
		}
		if _, err := time.Parse(dayFmt, in.Start); err != nil {
			fail(w, 400, "The start date is not a valid date.")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.isReadOnly() {
			fail(w, 423, errReadOnly.Error())
			return
		}
		if err := s.writeMaint("goals90-settings.json", in); err != nil {
			fail(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, s.g90View(time.Now()))
	})
}
