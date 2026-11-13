package main

// Employee training: who works here, what training each kind of job needs,
// and when each person last did it. Everything is kept in plain files in
// Maintenance Data (reg-employees.json, reg-trainingtypes.json,
// reg-training.json). The matrix shows at a glance who is current, due soon,
// expired or has never done a required training.

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

func init() {
	regDefs["employees"] = &collDef{Name: "employees", Title: "Employees", NewLabel: "Add Employee", Prefix: "EMP-", Digits: 3, Report: "training-employees",
		Intro:    "Everyone who needs training on record. Set someone to Inactive when they leave so they drop off the matrix; their records are kept.",
		ListCols: []string{"id", "name", "position", "department", "hireDate", "status"},
		Fields: []fieldDef{txt("name", "Name", true), txt("position", "Position", false), txt("department", "Department", false), dat("hireDate", "Hire date", false),
			sel("status", "Status", []string{"Active", "Inactive"}, true), lng("notes", "Notes", false)}}
	regDefs["trainingtypes"] = &collDef{Name: "trainingtypes", Title: "Training Types", NewLabel: "Add Training", Prefix: "TT-", Digits: 3, Report: "training-types", Starter: "training_types.json",
		Intro:    "The training the kitchen tracks. Who it applies to is All, or one or more department names or words from a position, separated by commas (for example Dish, Prep or Manager). Months until it expires is blank or 0 if it never does. Your health department and state decide what is actually required, so change these to match.",
		ListCols: []string{"id", "name", "appliesTo", "renewMonths", "active"},
		Fields:   []fieldDef{txt("name", "Training", true), lng("about", "What it covers", false), txt("appliesTo", "Applies to", false), txt("renewMonths", "Months until it expires (blank if never)", false), yn("active", "Active?")}}
	regDefs["training"] = &collDef{Name: "training", Title: "Training Records", NewLabel: "Record Training", Prefix: "TRN-", Digits: 5, Desc: true, Deletable: true, Report: "training-log",
		Intro:    "Each time someone completes a training. The expiry date is worked out from the training type when you leave it blank.",
		ListCols: []string{"id", "employeeName", "typeName", "completed", "expires", "trainer", "state"},
		Fields: []fieldDef{{Key: "employeeId", Label: "Employee", Type: "ref", Src: "employees", Req: true}, {Key: "typeId", Label: "Training", Type: "ref", Src: "trainingtypes", Req: true},
			dat("completed", "Date completed", true), dat("expires", "Expires (leave blank to calculate)", false), txt("trainer", "Trained by / provider", false), txt("cert", "Certificate or card number (optional)", false), lng("notes", "Notes", false)}}
}

// trainState says where a training stands by its expiry date.
func trainState(expires string, today time.Time) string {
	if expires == "" {
		return "Current"
	}
	e, err := time.Parse(dayFmt, expires)
	if err != nil {
		return "Current"
	}
	switch {
	case e.Before(today):
		return "Expired"
	case e.Before(today.AddDate(0, 0, 31)):
		return "Due Soon"
	}
	return "Current"
}

// trainingExpiry fills in the expiry date from the training type's renewal.
func (s *store) trainingExpiry(r rec) {
	if r["expires"] != "" {
		return
	}
	for _, t := range s.regLoad(regDefs["trainingtypes"]) {
		if t["id"] != r["typeId"] {
			continue
		}
		if m, err := strconv.Atoi(strings.TrimSpace(t["renewMonths"])); err == nil && m > 0 {
			if d, err := time.Parse(dayFmt, r["completed"]); err == nil {
				r["expires"] = d.AddDate(0, m, 0).Format(dayFmt)
			}
		}
	}
}

func (s *store) decorateTraining(rows []rec) {
	emp, typ := map[string]string{}, map[string]string{}
	for _, e := range s.regLoad(regDefs["employees"]) {
		emp[e["id"]] = e["name"]
	}
	for _, t := range s.regLoad(regDefs["trainingtypes"]) {
		typ[t["id"]] = t["name"]
	}
	today, _ := time.Parse(dayFmt, time.Now().Format(dayFmt))
	for _, r := range rows {
		r["employeeName"], r["typeName"] = firstNonEmpty(emp[r["employeeId"]], "(removed)"), firstNonEmpty(typ[r["typeId"]], "(removed)")
		r["state"] = trainState(r["expires"], today)
	}
}

func trainApplies(appliesTo string, e rec) bool {
	a := strings.ToLower(strings.TrimSpace(appliesTo))
	if a == "" || a == "all" || a == "all staff" || a == "everyone" {
		return true
	}
	dept, pos := strings.ToLower(e["department"]), strings.ToLower(e["position"])
	for _, w := range strings.Split(a, ",") {
		w = strings.TrimSpace(w)
		w2 := strings.TrimSuffix(w, "s")
		if w == "" {
			continue
		}
		if w == "all" || w == dept || w2 == strings.TrimSuffix(dept, "s") || (pos != "" && (strings.Contains(pos, w) || strings.Contains(pos, w2))) {
			return true
		}
	}
	return false
}

type trainCell struct {
	State     string `json:"state"` // Current, Due Soon, Expired, Missing, "" when it does not apply
	Completed string `json:"completed,omitempty"`
	Expires   string `json:"expires,omitempty"`
	RecordID  string `json:"recordId,omitempty"`
}

type trainRow struct {
	ID         string               `json:"id"`
	Name       string               `json:"name"`
	Position   string               `json:"position"`
	Department string               `json:"department"`
	Cells      map[string]trainCell `json:"cells"`
	Problems   int                  `json:"problems"`
}

type trainMatrix struct {
	Types []map[string]string `json:"types"`
	Rows  []trainRow          `json:"rows"`
	Count map[string]int      `json:"count"`
}

// trainingMatrix works out every active employee against every active
// training that applies to them. Callers hold s.mu.
func (s *store) trainingMatrix() trainMatrix {
	today, _ := time.Parse(dayFmt, time.Now().Format(dayFmt))
	m := trainMatrix{Types: []map[string]string{}, Rows: []trainRow{}, Count: map[string]int{"Current": 0, "Due Soon": 0, "Expired": 0, "Missing": 0}}
	var types []rec
	for _, t := range s.regSorted(regDefs["trainingtypes"]) {
		if t["active"] != "No" {
			types = append(types, t)
			m.Types = append(m.Types, map[string]string{"id": t["id"], "name": t["name"], "appliesTo": t["appliesTo"]})
		}
	}
	latest := map[string]rec{}
	for _, r := range s.regLoad(regDefs["training"]) {
		k := r["employeeId"] + "|" + r["typeId"]
		if old, ok := latest[k]; !ok || r["completed"] > old["completed"] || (r["completed"] == old["completed"] && r["id"] > old["id"]) {
			latest[k] = r
		}
	}
	emps := s.regSorted(regDefs["employees"])
	sort.SliceStable(emps, func(i, j int) bool { return strings.ToLower(emps[i]["name"]) < strings.ToLower(emps[j]["name"]) })
	for _, e := range emps {
		if e["status"] == "Inactive" {
			continue
		}
		row := trainRow{ID: e["id"], Name: e["name"], Position: e["position"], Department: e["department"], Cells: map[string]trainCell{}}
		for _, t := range types {
			if !trainApplies(t["appliesTo"], e) {
				continue
			}
			c := trainCell{State: "Missing"}
			if r, ok := latest[e["id"]+"|"+t["id"]]; ok {
				c = trainCell{State: trainState(r["expires"], today), Completed: r["completed"], Expires: r["expires"], RecordID: r["id"]}
			}
			row.Cells[t["id"]] = c
			m.Count[c.State]++
			if c.State != "Current" {
				row.Problems++
			}
		}
		m.Rows = append(m.Rows, row)
	}
	return m
}

func (s *store) trainingRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/training/matrix", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		writeJSON(w, 200, s.trainingMatrix())
	})
}

// trainingReport builds the printable training reports. Callers hold s.mu.
func (s *store) trainingReport(id string) (*report, bool) {
	today := time.Now().Format("January 2, 2006")
	switch id {
	case "training-matrix":
		m := s.trainingMatrix()
		cols := []string{"Employee", "Position"}
		for _, t := range m.Types {
			cols = append(cols, t["name"])
		}
		rows := [][]string{}
		for _, r := range m.Rows {
			row := []string{r.Name, r.Position}
			for _, t := range m.Types {
				c, ok := r.Cells[t["id"]]
				switch {
				case !ok:
					row = append(row, "n/a")
				case c.State == "Missing":
					row = append(row, "MISSING")
				case c.Expires != "":
					row = append(row, c.State+" · exp "+c.Expires)
				default:
					row = append(row, c.State+" · "+c.Completed)
				}
			}
			rows = append(rows, row)
		}
		note := "Current: " + strconv.Itoa(m.Count["Current"]) + " · Due soon (30 days): " + strconv.Itoa(m.Count["Due Soon"]) + " · Expired: " + strconv.Itoa(m.Count["Expired"]) + " · Missing: " + strconv.Itoa(m.Count["Missing"])
		return &report{Title: "Employee Training Matrix", Subtitle: today, Sections: []reportSection{{Note: note, Cols: cols, Rows: rows}}}, true
	case "training-due":
		m := s.trainingMatrix()
		name := map[string]string{}
		for _, t := range m.Types {
			name[t["id"]] = t["name"]
		}
		rows := [][]string{}
		for _, state := range []string{"Expired", "Missing", "Due Soon"} {
			for _, r := range m.Rows {
				for _, t := range m.Types {
					if c, ok := r.Cells[t["id"]]; ok && c.State == state {
						rows = append(rows, []string{state, r.Name, r.Department, name[t["id"]], c.Completed, c.Expires})
					}
				}
			}
		}
		return &report{Title: "Training Due, Expired or Missing", Subtitle: today, Sections: []reportSection{{Note: "Everything that needs attention first: expired, then never done, then due within 30 days.", Cols: []string{"Status", "Employee", "Department", "Training", "Last done", "Expires"}, Rows: rows}}}, true
	case "training-log":
		rows := [][]string{}
		recs := s.regSorted(regDefs["training"])
		s.decorateTraining(recs)
		for _, r := range recs {
			rows = append(rows, []string{r["id"], r["completed"], r["employeeName"], r["typeName"], r["expires"], r["trainer"], r["state"]})
		}
		return &report{Title: "Training Records", Subtitle: today, Sections: []reportSection{{Cols: []string{"ID", "Completed", "Employee", "Training", "Expires", "Trained by", "Status"}, Rows: rows}}}, true
	case "training-employees":
		return s.regReport(regDefs["employees"], "Employee List", "", nil), true
	case "training-types":
		return s.regReport(regDefs["trainingtypes"], "Training Types", "", nil), true
	case "training-signin":
		rows := [][]string{}
		for i := 0; i < 20; i++ {
			rows = append(rows, []string{"", "", "", ""})
		}
		return &report{Title: "Training Sign-In Sheet", Subtitle: "Training: ______________________________   Date: ______________   Trainer: ______________________   Length: ________",
			Sections: []reportSection{{Note: "Topics covered: ____________________________________________________________________________", Cols: []string{"Name (print)", "Position", "Signature", "Initials"}, Rows: rows},
				{Note: "Trainer signature: ______________________________   Manager review: ______________________________   Date: ______________"}}}, true
	}
	return nil, false
}
