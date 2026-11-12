package main

// Monthly Summary: a one-page picture of the month for the owner or general
// manager. Everything is worked out from the work orders, service history and
// recurring jobs already stored. Only the three note boxes are saved, one set
// per month, in Maintenance Data\summary-notes.json.

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"
)

type summaryNotes struct {
	Highlights string `json:"highlights"`
	Concerns   string `json:"concerns"`
	Actions    string `json:"actions"`
	By         string `json:"by,omitempty"`
	At         string `json:"at,omitempty"`
}

type prioRow struct {
	Priority string  `json:"priority"`
	Count    int     `json:"count"`
	Cost     float64 `json:"cost"`
}

type repeatRow struct {
	Equipment string `json:"equipment"`
	Issues    int    `json:"issues"`
}

type majorRow struct {
	Date      string  `json:"date"`
	Equipment string  `json:"equipment"`
	What      string  `json:"what"`
	Cost      float64 `json:"cost"`
}

// prio2 reduces "P2 - Urgent" to "P2".
func prio2(p string) string {
	if len(p) >= 2 {
		return p[:2]
	}
	return p
}

func isOpenIssue(i *Issue) bool { return i.Status != "Verified" && i.Status != "Closed" }

func (s *store) monthSummary(month string) map[string]any {
	eq, is, _ := s.loadMaint()
	names := map[string]string{}
	for _, e := range eq {
		names[e.ID] = e.Name
	}
	name := func(id string) string {
		if n := names[id]; n != "" {
			return n
		}
		return "Not listed"
	}
	first, _ := time.Parse("2006-01", month)
	label := first.Format("January 2006")
	prior := []string{month, first.AddDate(0, -1, 0).Format("2006-01"), first.AddDate(0, -2, 0).Format("2006-01")}
	inMonth := func(d string) bool { return strings.HasPrefix(d, month) }

	opened, completed, openNow, hot := 0, 0, 0, 0
	byPrio := map[string]*prioRow{}
	repeats := map[string]int{}
	for _, i := range is {
		if inMonth(i.ReportedAt) {
			opened++
		}
		if isOpenIssue(i) {
			openNow++
			if k := prio2(i.Priority); k == "P1" || k == "P2" {
				hot++
			}
		}
		if inMonth(i.CompletedAt) {
			completed++
			r := byPrio[prio2(i.Priority)]
			if r == nil {
				r = &prioRow{Priority: i.Priority}
				byPrio[prio2(i.Priority)] = r
			}
			r.Count++
			r.Cost += i.PartsCost + i.LaborCost
		}
		for _, m := range prior {
			if i.EquipmentID != "" && strings.HasPrefix(i.ReportedAt, m) {
				repeats[i.EquipmentID]++
				break
			}
		}
	}
	prios := []prioRow{}
	for _, p := range priorities {
		if r := byPrio[prio2(p)]; r != nil {
			prios = append(prios, *r)
		} else {
			prios = append(prios, prioRow{Priority: p})
		}
	}
	rep := []repeatRow{}
	for id, n := range repeats {
		if n >= 2 {
			rep = append(rep, repeatRow{name(id), n})
		}
	}
	sort.Slice(rep, func(a, b int) bool {
		return rep[a].Issues > rep[b].Issues || rep[a].Issues == rep[b].Issues && rep[a].Equipment < rep[b].Equipment
	})

	var repairCost, prevCost, parts, downtime float64
	major := []majorRow{}
	for _, r := range s.serviceList() {
		if !inMonth(r.Date) {
			continue
		}
		if r.Type == "Repair" {
			repairCost += r.Cost
			parts += r.Parts
			downtime += r.Downtime
			if r.Cost > 0 {
				major = append(major, majorRow{r.Date, name(r.EquipmentID), r.Description, r.Cost})
			}
		} else {
			prevCost += r.Cost
		}
	}
	sort.Slice(major, func(a, b int) bool { return major[a].Cost > major[b].Cost })
	if len(major) > 5 {
		major = major[:5]
	}

	overdue, pmDone, pmDue := 0, 0, 0
	var tasks []*PMTask
	s.readMaint("pm.json", &tasks)
	for _, t := range tasks {
		for _, h := range t.History {
			if inMonth(h.At) && strings.HasPrefix(h.Text, "Done") {
				pmDone++
			}
		}
	}
	for _, t := range s.pmList() {
		if t.State == "Overdue" {
			overdue++
		}
		if t.NextDue != "" && inMonth(t.NextDue) {
			pmDue++
		}
	}

	notes := map[string]summaryNotes{}
	s.readMaint("summary-notes.json", &notes)
	return map[string]any{
		"month": month, "label": label,
		"opened": opened, "completed": completed, "openNow": openNow, "p1p2Open": hot,
		"overduePM": overdue, "pmDone": pmDone, "pmDueThisMonth": pmDue,
		"repairCost": repairCost, "partsCost": parts, "downtime": downtime, "preventiveCost": prevCost,
		"byPriority": prios, "repeat": rep, "major": major, "notes": notes[month],
	}
}

func (s *store) summaryRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/maint/summary", func(w http.ResponseWriter, r *http.Request) {
		m := r.URL.Query().Get("month")
		if _, err := time.Parse("2006-01", m); err != nil {
			m = time.Now().Format("2006-01")
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		writeJSON(w, 200, s.monthSummary(m))
	})
	mux.HandleFunc("POST /api/maint/summary-notes", func(w http.ResponseWriter, r *http.Request) {
		var in2 struct {
			Month string `json:"month"`
			summaryNotes
		}
		if json.NewDecoder(r.Body).Decode(&in2) != nil || strings.TrimSpace(in2.By) == "" {
			fail(w, 400, "Enter your name.")
			return
		}
		if _, err := time.Parse("2006-01", in2.Month); err != nil {
			fail(w, 400, "Choose a month.")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		notes := map[string]summaryNotes{}
		s.readMaint("summary-notes.json", &notes)
		n := in2.summaryNotes
		n.By, n.At = strings.TrimSpace(n.By), stamp()
		notes[in2.Month] = n
		if err := s.writeMaint("summary-notes.json", notes); err != nil {
			fail(w, 500, "The notes could not be saved: "+err.Error())
			return
		}
		writeJSON(w, 200, s.monthSummary(in2.Month))
	})
}
