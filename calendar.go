package main

// The calendar draws everything that recurs or happens on a date: cleaning,
// equipment preventive maintenance, grounds and facility jobs, jobs that were
// done, and issues as they were reported. Nothing is stored for it; the
// events are worked out from the task list each time it is opened.

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
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
	Src     string `json:"src,omitempty"`     // list a cleaning or inspection event comes from
	EventID string `json:"eventId,omitempty"` // a hand-added event
	Notes   string `json:"notes,omitempty"`
}

// CalEvent is an event added by hand to the calendar.
type CalEvent struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Date   string `json:"date"`
	Group  string `json:"group"` // Cleaning, Equipment PM, Grounds / Facility, Other
	Area   string `json:"area"`
	Notes  string `json:"notes"`
	Repeat string `json:"repeat"` // None, Daily, Weekly, Monthly, Quarterly, Semi-Annually, Yearly
	By     string `json:"by"`
}

var calGroups = []string{"Cleaning", "Equipment PM", "Grounds / Facility", "Other"}
var calRepeats = []string{"None", "Daily", "Weekly", "Monthly", "Quarterly", "Semi-Annually", "Yearly"}

func stepRepeat(d time.Time, r string) time.Time {
	switch r {
	case "Daily":
		return d.AddDate(0, 0, 1)
	case "Weekly":
		return d.AddDate(0, 0, 7)
	case "Monthly":
		return d.AddDate(0, 1, 0)
	case "Quarterly":
		return d.AddDate(0, 3, 0)
	case "Semi-Annually":
		return d.AddDate(0, 6, 0)
	case "Yearly":
		return d.AddDate(1, 0, 0)
	}
	return d.AddDate(100, 0, 0)
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
	// Hand-added events, repeating ones carried forward through the period.
	var custom []*CalEvent
	s.readMaint("calendar-events.json", &custom)
	for _, c := range custom {
		d, err := time.Parse(dayFmt, c.Date)
		if err != nil {
			continue
		}
		for n := 0; !d.After(to) && n < 3000; n++ {
			if in(d) {
				out = append(out, calEvent{Date: d.Format(dayFmt), Kind: "event", State: "OK", Title: c.Title, Group: c.Group, Area: c.Area, By: c.By, EventID: c.ID, Notes: c.Notes, Bucket: map[bool]string{true: "", false: c.Repeat}[c.Repeat == "None" || c.Repeat == ""]})
			}
			d = stepRepeat(d, c.Repeat)
		}
	}
	// Cleaning and inspection schedules (the daily ones would fill every day, so
	// they stay on Today's Checklist).
	addSched := func(group, src, id, title, area, who string, days int, due string) {
		d, err := time.Parse(dayFmt, due)
		if err != nil || days < 7 {
			return
		}
		b := freqName(days)
		for n := 0; !d.After(to) && n < 400; n++ {
			if in(d) {
				st := "OK"
				if d.Before(today) {
					st = "Overdue"
				} else if d.Sub(today).Hours()/24 <= 7 {
					st = "Due Soon"
				}
				out = append(out, calEvent{Date: d.Format(dayFmt), Kind: "due", State: st, Title: title, Group: group, Bucket: b, Area: area, By: who, Src: src, TaskID: id})
			}
			if d.Before(today) {
				d = today.AddDate(0, 0, days)
				continue
			}
			d = d.AddDate(0, 0, days)
		}
	}
	for _, r := range s.regLoad(regDefs["cleantasks"]) {
		if due, st := dueState(r, today.Format(dayFmt)); st != "Inactive" && st != "As Needed" {
			addSched("Cleaning", "cleantasks", r["id"], r["area"]+": "+r["task"], r["area"], r["role"], recDays(r), due)
		}
	}
	for _, r := range s.regLoad(regDefs["mgrchecks"]) {
		if due, st := dueState(r, today.Format(dayFmt)); st != "Inactive" && st != "As Needed" {
			addSched("Cleaning", "mgrchecks", r["id"], "Manager check: "+r["check"], r["area"], r["owner"], recDays(r), due)
		}
	}
	for _, r := range s.regLoad(regDefs["deepclean"]) {
		if due, st := deepState(r, today); st != "Inactive" && st != "As Needed" && due != "" {
			addSched("Cleaning", "deepclean", r["id"], r["day"]+" deep clean: "+r["task"], r["area"], r["role"], 7, due)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out
}

func (s *store) calendarRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/maint/calendar/events", func(w http.ResponseWriter, r *http.Request) {
		var in2 CalEvent
		if json.NewDecoder(r.Body).Decode(&in2) != nil {
			fail(w, 400, "Could not read that.")
			return
		}
		in2.Title, in2.Area, in2.Notes, in2.By = strings.TrimSpace(in2.Title), strings.TrimSpace(in2.Area), strings.TrimSpace(in2.Notes), strings.TrimSpace(in2.By)
		if in2.Title == "" || in2.By == "" {
			fail(w, 400, "Enter what the event is and your name.")
			return
		}
		if _, err := time.Parse(dayFmt, in2.Date); err != nil {
			fail(w, 400, "Choose the date.")
			return
		}
		if in2.Group == "" {
			in2.Group = "Other"
		}
		if in2.Repeat == "" {
			in2.Repeat = "None"
		}
		if !oneOf(calGroups, in2.Group) || !oneOf(calRepeats, in2.Repeat) || len(in2.Title) > 200 || len(in2.Notes) > 2000 {
			fail(w, 400, "Check the type and how often it repeats.")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		var all []*CalEvent
		s.readMaint("calendar-events.json", &all)
		in2.ID = newID()
		if err := s.writeMaint("calendar-events.json", append(all, &in2)); err != nil {
			s.maintFail(w, err)
			return
		}
		writeJSON(w, 200, map[string]string{"id": in2.ID})
	})
	mux.HandleFunc("DELETE /api/maint/calendar/events/{id}", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		var all, keep []*CalEvent
		s.readMaint("calendar-events.json", &all)
		for _, c := range all {
			if c.ID != r.PathValue("id") {
				keep = append(keep, c)
			}
		}
		if keep == nil {
			keep = []*CalEvent{}
		}
		if err := s.writeMaint("calendar-events.json", keep); err != nil {
			s.maintFail(w, err)
			return
		}
		writeJSON(w, 200, map[string]int{"left": len(keep)})
	})
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
