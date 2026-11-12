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
	Code        string  `json:"code"`        // the job's ID in the maintenance workbook, e.g. PM-001
	AssetCode   string  `json:"assetCode"`   // the asset ID in the workbook, e.g. K-001
	Performer   string  `json:"performer"`   // who does it: a person, role or vendor
	FreqLabel   string  `json:"freqLabel"`   // the frequency as originally written, e.g. "3x a week"
	MaintainXID string  `json:"maintainxId"` // optional reference in MaintainX
	Name        string  `json:"name"`
	Every       int     `json:"every"`
	Unit        string  `json:"unit"`     // days, weeks, months, years, or asneeded (not scheduled)
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
	if t.Unit == "asneeded" {
		return ""
	}
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
	if t.Unit == "asneeded" {
		return "As Needed"
	}
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
		if t.Unit == "asneeded" {
			v.State = "As Needed"
		} else if d, err := time.Parse(dayFmt, v.NextDue); err == nil {
			v.Days = int(d.Sub(today).Hours() / 24)
			if v.Days < 0 {
				v.State = "Overdue"
			} else if v.Days <= 7 {
				v.State = "Due Soon"
			}
		}
		out = append(out, v)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if (out[i].NextDue == "") != (out[j].NextDue == "") {
			return out[j].NextDue == ""
		}
		return out[i].NextDue < out[j].NextDue
	})
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
	unitOK := func(u string) bool {
		return u == "days" || u == "weeks" || u == "months" || u == "years" || u == "asneeded"
	}

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
		if in2.Group == "Equipment PM" && in2.EquipmentID == "" && strings.TrimSpace(in2.Area) == "" {
			fail(w, 400, "Choose the equipment, or say what it is on.")
			return
		}
		if !unitOK(in2.Unit) || (in2.Unit != "asneeded" && in2.Every < 1) {
			fail(w, 400, "Enter how often it is done, for example every 3 months.")
			return
		}
		if in2.Unit == "asneeded" {
			in2.Every, in2.FirstDue = 0, ""
		} else if _, err := time.Parse(dayFmt, in2.FirstDue); err != nil {
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
				if t.Group == "Cleaning" {
					s.logPMCleaning(t, strings.TrimSpace(in2.By), in2.Date, strings.TrimSpace(in2.Note))
				}
				if save(w, tasks) {
					s.syncMissed()
					reply(w)
				}
				return
			}
		}
		fail(w, 404, "That job was not found.")
	})
}

// importSchedule loads the recurring jobs from the maintenance workbooks. They
// ship inside the program (starter/schedule_import.json). Jobs already
// loaded, matched by their workbook ID, are left alone, so it is safe to run
// twice. pastDates says what to do with due dates that have already gone by:
// by default they move forward along the job's own schedule; "keep" keeps the
// date from the workbook.
func (s *store) importRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/maint/import-schedule", func(w http.ResponseWriter, r *http.Request) {
		var in2 struct {
			PastDates string `json:"pastDates"`
		}
		_ = json.NewDecoder(r.Body).Decode(&in2)
		raw, err := starterFiles.ReadFile("starter/schedule_import.json")
		if err != nil {
			fail(w, 500, "The schedule file is missing from this build.")
			return
		}
		var src struct {
			Tasks []PMTask `json:"tasks"`
		}
		if json.Unmarshal(raw, &src) != nil {
			fail(w, 500, "The schedule file could not be read.")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		var have []*PMTask
		s.readMaint("pm.json", &have)
		known := map[string]bool{}
		for _, t := range have {
			if t.Code != "" {
				known[t.Code] = true
			}
		}
		today := time.Now().Format(dayFmt)
		added, skipped := 0, 0
		for i := range src.Tasks {
			t := src.Tasks[i]
			if known[t.Code] {
				skipped++
				continue
			}
			t.ID, t.History, t.LastDone = newID(), []Entry{}, ""
			switch {
			case t.Unit == "asneeded":
				t.FirstDue = ""
			case t.FirstDue == "":
				t.FirstDue = today
			case in2.PastDates != "keep" && t.FirstDue < today:
				// Move a date that has gone by forward along the job's own
				// schedule, so monthly and quarterly jobs stay spread out
				// instead of all landing on today.
				d, err := time.Parse(dayFmt, t.FirstDue)
				for n := 0; err == nil && d.Format(dayFmt) < today && n < 5000; n++ {
					d = t.step(d)
				}
				if err != nil {
					d = time.Now()
				}
				t.FirstDue = d.Format(dayFmt)
			}
			have = append(have, &t)
			added++
		}
		if added > 0 {
			if err := s.writeMaint("pm.json", have); err != nil {
				s.maintFail(w, err)
				return
			}
		}
		writeJSON(w, 200, map[string]int{"added": added, "skipped": skipped})
	})
}
