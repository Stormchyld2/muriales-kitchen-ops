package main

// The master to-do list. It is not stored: it is gathered on request from the
// recurring jobs, open maintenance issues, equipment worth a second look, SDS
// follow-ups, and any one-off items typed in by hand (todos.json).

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Todo struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Due       string `json:"due"`
	Notes     string `json:"notes"`
	CreatedBy string `json:"createdBy"`
	Done      bool   `json:"done"`
	DoneBy    string `json:"doneBy"`
	DoneAt    string `json:"doneAt"`
}

type todoItem struct {
	Group  string `json:"group"` // Cleaning, Equipment PM, Grounds / Facility, Issue, SDS, Equipment, Other
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Due    string `json:"due"`
	State  string `json:"state"` // Overdue, Due Today, Upcoming, Open
	Bucket string `json:"bucket,omitempty"`
	Kind   string `json:"kind"` // task, issue, product, equipment, todo
	ID     string `json:"id"`
}

var todoSources = []string{"Cleaning", "Equipment PM", "Grounds / Facility", "Issue", "Inspection", "Temperature", "SDS", "Equipment", "Other"}

func (s *store) todoItems(from, to time.Time, want map[string]bool) []todoItem {
	today, _ := time.Parse(dayFmt, time.Now().Format(dayFmt))
	out := []todoItem{}
	eq, issues, st := s.loadMaint()
	name := map[string]string{}
	for _, e := range eq {
		name[e.ID] = e.Name
	}
	// Recurring jobs: everything overdue, plus what falls inside the period.
	for _, e := range s.calendarEvents(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), to) {
		if e.Kind != "due" || !want[e.Group] {
			continue
		}
		d, _ := time.Parse(dayFmt, e.Date)
		if e.State != "Overdue" && d.Before(from) {
			continue
		}
		state := "Upcoming"
		if e.State == "Overdue" {
			state = "Overdue"
		} else if d.Equal(today) {
			state = "Due Today"
		}
		where := e.Area
		if e.Unit != "" {
			if where != "" {
				where += " · "
			}
			where += e.Unit
		}
		out = append(out, todoItem{Group: e.Group, Title: e.Title, Detail: strings.TrimSpace(where + " · " + e.Bucket), Due: e.Date, State: state, Bucket: e.Bucket, Kind: "task", ID: e.TaskID})
	}
	if want["Issue"] {
		for _, p := range issues {
			if p.Status == "Verified" || p.Status == "Closed" {
				continue
			}
			detail := name[p.EquipmentID] + " · " + p.Priority + " priority · " + p.Status
			if p.Status == "Repaired" {
				detail += " · needs a manager to verify it is ready for food production"
			}
			state := "Open"
			if strings.HasPrefix(p.Priority, "P1") || strings.HasPrefix(p.Priority, "P2") {
				state = "Overdue"
			}
			out = append(out, todoItem{Group: "Issue", Title: "#" + itoa(p.Number) + " " + p.Title, Detail: detail, Due: p.ReportedAt[:10], State: state, Kind: "issue", ID: p.ID})
		}
	}
	if want["Inspection"] {
		for _, f := range s.findingList() {
			if f.Status == "Done" || (f.Due != "" && f.Due > to.Format(dayFmt)) {
				continue
			}
			state := "Open"
			if f.Due != "" {
				if d, err := time.Parse(dayFmt, f.Due); err == nil {
					if d.Before(today) {
						state = "Overdue"
					} else if d.Equal(today) {
						state = "Due Today"
					} else {
						state = "Upcoming"
					}
				}
			} else if strings.HasPrefix(f.Priority, "P1") || strings.HasPrefix(f.Priority, "P2") {
				state = "Overdue"
			}
			detail := strings.TrimSpace(strings.Join(nonEmpty(f.Area, name[f.EquipmentID], f.Category, f.Priority, f.AssignedTo), " · "))
			out = append(out, todoItem{Group: "Inspection", Title: "#" + itoa(f.Number) + " " + f.What, Detail: detail, Due: f.Due, State: state, Kind: "finding", ID: f.ID})
		}
	}
	if want["Temperature"] {
		for _, v := range s.tempsNeedingAction() {
			out = append(out, todoItem{Group: "Temperature", Title: v.Point + " " + strconv.FormatFloat(v.Temp, 'f', -1, 64) + "°F (" + strings.ToLower(v.Status) + ")", Detail: "Safe range " + v.Limit + " · " + v.At + " · write the corrective action", Due: v.At[:10], State: "Overdue", Kind: "temp", ID: v.ID})
		}
	}
	if want["Equipment"] {
		a := s.assetSummaries(eq, st.ReplacePercent)
		for _, e := range eq {
			if e.Status == "Out of Service" {
				why := "A manager must clear it before it is used"
				for _, r := range s.oosList() {
					if r.EquipmentID == e.ID && r.ClearedAt == "" {
						why = r.Reason + " · " + why
					}
				}
				out = append(out, todoItem{Group: "Equipment", Title: "Out of service: " + e.Name, Detail: why, State: "Overdue", Kind: "equipment", ID: e.ID})
			}
			if a[e.ID].Advice == "Consider Replacing" {
				out = append(out, todoItem{Group: "Equipment", Title: "Decide: repair or replace " + e.Name, Detail: a[e.ID].Reason, State: "Open", Kind: "equipment", ID: e.ID})
			}
		}
	}
	if want["SDS"] {
		bad := s.integrityProblems()
		for _, p := range s.lib.Products {
			switch {
			case p.Missing:
				out = append(out, todoItem{Group: "SDS", Title: p.Name, Detail: "The SDS PDF is missing from the sheets folder", State: "Open", Kind: "product", ID: p.ID})
			case bad[p.ID] == "changed":
				out = append(out, todoItem{Group: "SDS", Title: p.Name, Detail: "The PDF has changed since it was added. Check it is the right sheet", State: "Open", Kind: "product", ID: p.ID})
			case p.NeedsReview:
				out = append(out, todoItem{Group: "SDS", Title: p.Name, Detail: "Added from the folder. Check the details and fill in manufacturer, location and date", State: "Open", Kind: "product", ID: p.ID})
			}
		}
	}
	if want["Other"] {
		var all []*Todo
		s.readMaint("todos.json", &all)
		for _, t := range all {
			if t.Done || (t.Due != "" && t.Due > to.Format(dayFmt)) {
				continue
			}
			state := "Open"
			if t.Due != "" {
				if d, err := time.Parse(dayFmt, t.Due); err == nil {
					if d.Before(today) {
						state = "Overdue"
					} else if d.Equal(today) {
						state = "Due Today"
					} else {
						state = "Upcoming"
					}
				}
			}
			out = append(out, todoItem{Group: "Other", Title: t.Title, Detail: t.Notes, Due: t.Due, State: state, Kind: "todo", ID: t.ID})
		}
	}
	rank := map[string]int{"Overdue": 0, "Due Today": 1, "Open": 2, "Upcoming": 3}
	sort.SliceStable(out, func(i, j int) bool {
		if rank[out[i].State] != rank[out[j].State] {
			return rank[out[i].State] < rank[out[j].State]
		}
		if out[i].Due != out[j].Due {
			return out[i].Due < out[j].Due
		}
		return out[i].Title < out[j].Title
	})
	return out
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func (s *store) todoRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/todo", func(w http.ResponseWriter, r *http.Request) {
		from, e1 := time.Parse(dayFmt, r.URL.Query().Get("from"))
		to, e2 := time.Parse(dayFmt, r.URL.Query().Get("to"))
		if e1 != nil || e2 != nil || to.Before(from) || to.Sub(from).Hours()/24 > maxCalendarDays {
			fail(w, 400, "Choose a date range of up to a year.")
			return
		}
		want := map[string]bool{}
		for _, g := range strings.Split(r.URL.Query().Get("sources"), ",") {
			if oneOf(todoSources, g) {
				want[g] = true
			}
		}
		if len(want) == 0 {
			for _, g := range todoSources {
				want[g] = true
			}
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		writeJSON(w, 200, map[string]any{"items": s.todoItems(from, to, want), "generated": time.Now().Format("2006-01-02T15:04:05")})
	})

	mux.HandleFunc("POST /api/maint/todos", func(w http.ResponseWriter, r *http.Request) {
		var in2 Todo
		if json.NewDecoder(r.Body).Decode(&in2) != nil || strings.TrimSpace(in2.Title) == "" || strings.TrimSpace(in2.CreatedBy) == "" {
			fail(w, 400, "Enter the item and your name.")
			return
		}
		if in2.Due != "" {
			if _, err := time.Parse(dayFmt, in2.Due); err != nil {
				fail(w, 400, "Enter a valid due date, or leave it blank.")
				return
			}
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		var all []*Todo
		s.readMaint("todos.json", &all)
		in2.ID, in2.Done, in2.DoneBy, in2.DoneAt = newID(), false, "", ""
		in2.Title, in2.CreatedBy = strings.TrimSpace(in2.Title), strings.TrimSpace(in2.CreatedBy)
		if err := s.writeMaint("todos.json", append(all, &in2)); err != nil {
			s.maintFail(w, err)
			return
		}
		writeJSON(w, 200, map[string]string{"id": in2.ID})
	})

	mux.HandleFunc("POST /api/maint/todos/{id}/done", func(w http.ResponseWriter, r *http.Request) {
		var in2 struct{ By string }
		if json.NewDecoder(r.Body).Decode(&in2) != nil || strings.TrimSpace(in2.By) == "" {
			fail(w, 400, "Enter your name.")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		var all []*Todo
		s.readMaint("todos.json", &all)
		for _, t := range all {
			if t.ID == r.PathValue("id") {
				t.Done, t.DoneBy, t.DoneAt = true, strings.TrimSpace(in2.By), time.Now().Format("2006-01-02T15:04:05")
				if err := s.writeMaint("todos.json", all); err != nil {
					s.maintFail(w, err)
					return
				}
				writeJSON(w, 200, map[string]string{"id": t.ID})
				return
			}
		}
		fail(w, 404, "That item was not found.")
	})
}

func nonEmpty(v ...string) []string {
	out := []string{}
	for _, x := range v {
		if strings.TrimSpace(x) != "" {
			out = append(out, x)
		}
	}
	return out
}
