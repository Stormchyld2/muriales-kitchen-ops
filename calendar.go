package main

// The calendar draws everything that recurs or happens on a date: cleaning,
// equipment preventive maintenance, grounds and facility jobs, jobs that were
// done, and issues as they were reported. Nothing is stored for it; the
// events are worked out from the task list each time it is opened.

import (
	"net/http"
	"sort"
	"time"
)

type calEvent struct {
	Date    string `json:"date"`
	Kind    string `json:"kind"`  // due, done, issue
	State   string `json:"state"` // Overdue, Due Soon, OK, Done, or the issue status
	TaskID  string `json:"taskId,omitempty"`
	IssueID string `json:"issueId,omitempty"`
	Title   string `json:"title"`
	Group   string `json:"group"`
	Bucket  string `json:"bucket,omitempty"`
	Area    string `json:"area,omitempty"`
	Unit    string `json:"unit,omitempty"` // equipment name
	By      string `json:"by,omitempty"`
}

const maxCalendarDays = 400

func (s *store) calendarEvents(from, to time.Time) []calEvent {
	today, _ := time.Parse(dayFmt, time.Now().Format(dayFmt))
	eq, issues, _ := s.loadMaint()
	names := map[string]string{}
	for _, e := range eq {
		names[e.ID] = e.Name
	}
	var tasks []*PMTask
	s.readMaint("pm.json", &tasks)
	out := []calEvent{}
	in := func(d time.Time) bool { return !d.Before(from) && !d.After(to) }
	for _, t := range tasks {
		if t.Group == "" {
			t.Group = "Equipment PM"
		}
		if t.Every < 1 {
			continue
		}
		base := calEvent{TaskID: t.ID, Title: t.Name, Group: t.Group, Bucket: t.bucket(), Area: t.Area, Unit: names[t.EquipmentID]}
		next, err := time.Parse(dayFmt, t.next())
		if err == nil {
			// A job that was missed shows once, at its due date, as overdue;
			// the schedule then carries on from today.
			for d, n := next, 0; !d.After(to) && n < 2000; d, n = t.step(d), n+1 {
				if d.Before(today) {
					if n == 0 && in(d) {
						e := base
						e.Date, e.Kind, e.State = d.Format(dayFmt), "due", "Overdue"
						out = append(out, e)
					}
					continue
				}
				if in(d) {
					e := base
					e.Date, e.Kind, e.State = d.Format(dayFmt), "due", "OK"
					if d.Sub(today).Hours()/24 <= 7 {
						e.State = "Due Soon"
					}
					out = append(out, e)
				}
			}
		}
		for _, h := range t.History {
			if d, err := time.Parse(dayFmt, h.At); err == nil && in(d) {
				e := base
				e.Date, e.Kind, e.State, e.By = h.At, "done", "Done", h.By
				out = append(out, e)
			}
		}
	}
	for _, p := range issues {
		if len(p.ReportedAt) < 10 {
			continue
		}
		if d, err := time.Parse(dayFmt, p.ReportedAt[:10]); err == nil && in(d) {
			out = append(out, calEvent{Date: p.ReportedAt[:10], Kind: "issue", State: p.Status, IssueID: p.ID, Title: p.Title, Group: "Issue", Unit: names[p.EquipmentID], By: p.ReportedBy})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out
}

func (s *store) calendarRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/maint/calendar", func(w http.ResponseWriter, r *http.Request) {
		from, e1 := time.Parse(dayFmt, r.URL.Query().Get("from"))
		to, e2 := time.Parse(dayFmt, r.URL.Query().Get("to"))
		if e1 != nil || e2 != nil || to.Before(from) || to.Sub(from).Hours()/24 > maxCalendarDays {
			fail(w, 400, "Choose a date range of up to a year.")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		writeJSON(w, 200, map[string]any{"events": s.calendarEvents(from, to)})
	})
}
