package main

// Preventive maintenance: recurring jobs on a piece of equipment (change the
// filter, calibrate the thermometer, deep service). Kept in pm.json in the
// Maintenance Data folder. A manager records each time a job is done and the
// next due date is worked out from that.

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Groups of recurring jobs. They all work the same way: how often, when first
// due, when last done. The calendar draws them in different colours.
var taskGroups = []string{"Equipment PM", "Cleaning", "Grounds / Facility"}

type PMTask struct {
	ID          string  `json:"id"`
	Group       string  `json:"group"`
	Area        string  `json:"area"`
	EquipmentID string  `json:"equipmentId"`
	Name        string  `json:"name"`
	Every       int     `json:"every"`
	Unit        string  `json:"unit"`     // days, weeks, months, years
	FirstDue    string  `json:"firstDue"` // used until the first time it is done
	LastDone    string  `json:"lastDone"`
	Notes       string  `json:"notes"`
	History     []Entry `json:"history"`
}

type pmView struct {
	PMTask
	NextDue string `json:"nextDue"`
	Bucket  string `json:"bucket"` // Daily, Weekly, Monthly, Quarterly / Semi-Annual, Yearly
	State   string `json:"state"`  // Overdue, Due Soon, OK
	Days    int    `json:"days"`   // days until due (negative when overdue)
}

const dayFmt = "2006-01-02"

func (t *PMTask) next() string {
	if t.LastDone == "" {
		return t.FirstDue
	}
	d, err := time.Parse(dayFmt, t.LastDone)
	if err != nil {
		return t.FirstDue
	}
	switch t.Unit {
	case "years":
		d = d.AddDate(t.Every, 0, 0)
	case "weeks":
		d = d.AddDate(0, 0, 7*t.Every)
	case "months":
		d = d.AddDate(0, t.Every, 0)
	default:
		d = d.AddDate(0, 0, t.Every)
	}
	return d.Format(dayFmt)
}

// step moves a date forward by one interval of the task.
func (t *PMTask) step(d time.Time) time.Time {
	switch t.Unit {
	case "years":
		return d.AddDate(t.Every, 0, 0)
	case "weeks":
		return d.AddDate(0, 0, 7*t.Every)
	case "months":
		return d.AddDate(0, t.Every, 0)
	}
	return d.AddDate(0, 0, t.Every)
}

// bucket names how often a task repeats, for the calendar's views.
func (t *PMTask) bucket() string {
	days := t.Every
	switch t.Unit {
	case "weeks":
		days *= 7
	case "months":
		days *= 30
	case "years":
		days *= 365
	}
	switch {
	case days <= 1:
		return "Daily"
	case days <= 27:
		return "Weekly"
	case days <= 89:
		return "Monthly"
	case days < 360:
		return "Quarterly / Semi-Annual"
	}
	return "Yearly"
}

func (s *store) pmList() []pmView {
	var tasks []*PMTask
	s.readMaint("pm.json", &tasks)
	today, _ := time.Parse(dayFmt, time.Now().Format(dayFmt))
	out := []pmView{}
	for _, t := range tasks {
		if t.Group == "" {
			t.Group = "Equipment PM"
		}
		v := pmView{PMTask: *t, NextDue: t.next(), State: "OK", Bucket: t.bucket()}
		if d, err := time.Parse(dayFmt, v.NextDue); err == nil {
			v.Days = int(d.Sub(today).Hours() / 24)
			if v.Days < 0 {
				v.State = "Overdue"
			} else if v.Days <= 7 {
				v.State = "Due Soon"
			}
		}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].NextDue < out[j].NextDue })
	return out
}

func (s *store) pmRoutes(mux *http.ServeMux, reply func(http.ResponseWriter)) {
	load := func() (tasks []*PMTask) { s.readMaint("pm.json", &tasks); return }
	save := func(w http.ResponseWriter, tasks []*PMTask) bool {
		if tasks == nil {
			tasks = []*PMTask{}
		}
		if err := s.writeMaint("pm.json", tasks); err != nil {
			s.maintFail(w, err)
			return false
		}
		return true
	}
	unitOK := func(u string) bool { return u == "days" || u == "weeks" || u == "months" || u == "years" }

	mux.HandleFunc("POST /api/maint/pm", func(w http.ResponseWriter, r *http.Request) {
		var in2 PMTask
		if json.NewDecoder(r.Body).Decode(&in2) != nil {
			fail(w, 400, "Could not read that.")
			return
		}
		if in2.Group == "" {
			in2.Group = "Equipment PM"
		}
		if strings.TrimSpace(in2.Name) == "" || !oneOf(taskGroups, in2.Group) {
			fail(w, 400, "Enter what needs doing.")
			return
		}
		if in2.Group == "Equipment PM" && in2.EquipmentID == "" {
			fail(w, 400, "Choose the equipment.")
			return
		}
		if in2.Every < 1 || !unitOK(in2.Unit) {
			fail(w, 400, "Enter how often it is done, for example every 3 months.")
			return
		}
		if _, err := time.Parse(dayFmt, in2.FirstDue); err != nil {
			fail(w, 400, "Enter the date it is first due.")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		tasks := load()
		in2.Name = strings.TrimSpace(in2.Name)
		found := false
		for i, t := range tasks {
			if t.ID == in2.ID && in2.ID != "" {
				in2.LastDone, in2.History = t.LastDone, t.History // done-dates are only changed by Mark Done
				tasks[i], found = &in2, true
			}
		}
		if !found {
			in2.ID, in2.LastDone, in2.History = newID(), "", []Entry{}
			tasks = append(tasks, &in2)
		}
		if save(w, tasks) {
			reply(w)
		}
	})

	mux.HandleFunc("DELETE /api/maint/pm/{id}", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		out := []*PMTask{}
		for _, t := range load() {
			if t.ID != r.PathValue("id") {
				out = append(out, t)
			}
		}
		if save(w, out) {
			reply(w)
		}
	})

	mux.HandleFunc("POST /api/maint/pm/{id}/done", func(w http.ResponseWriter, r *http.Request) {
		var in2 struct {
			By, Date, Note string
			Cost           float64
		}
		if json.NewDecoder(r.Body).Decode(&in2) != nil || strings.TrimSpace(in2.By) == "" {
			fail(w, 400, "Enter your name.")
			return
		}
		if in2.Date == "" {
			in2.Date = time.Now().Format(dayFmt)
		}
		d, err := time.Parse(dayFmt, in2.Date)
		if err != nil || d.After(time.Now().AddDate(0, 0, 1)) {
			fail(w, 400, "Enter the date it was done (today or earlier).")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		tasks := load()
		for _, t := range tasks {
			if t.ID == r.PathValue("id") {
				t.History = append(t.History, Entry{in2.Date, strings.TrimSpace(in2.By), strings.TrimSpace("Done. " + in2.Note)})
				if in2.Cost > 0 && t.EquipmentID != "" {
					s.addService(ServiceRecord{EquipmentID: t.EquipmentID, PMID: t.ID, Date: in2.Date, Type: "Preventive", Description: t.Name, Cost: in2.Cost, By: strings.TrimSpace(in2.By)})
				}
				if in2.Date > t.LastDone {
					t.LastDone = in2.Date
				}
				if save(w, tasks) {
					reply(w)
				}
				return
			}
		}
		fail(w, 404, "That job was not found.")
	})
}
