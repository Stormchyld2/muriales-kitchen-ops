package main

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

type ReadinessItem struct {
	N         int    `json:"n"`
	Area      string `json:"area"`
	Point     string `json:"point"`
	Status    string `json:"status"` // Ready, Needs Attention, Corrected, N/A
	Finding   string `json:"finding"`
	Corrected string `json:"corrected"` // Yes, No, N/A
	Priority  string `json:"priority"`
	Owner     string `json:"owner"`
	Target    string `json:"target"`
	CARef     string `json:"caRef"`
	Notes     string `json:"notes"`
}

type Readiness struct {
	ID         string          `json:"id"`
	Date       string          `json:"date"`
	Manager    string          `json:"manager"`
	Type       string          `json:"type"`
	WalkTime   string          `json:"walkTime"`
	Overall    string          `json:"overall"`  // Yes / No
	Critical   string          `json:"critical"` // Yes / No
	Photos     string          `json:"photos"`
	Recheck    string          `json:"recheck"`
	ReviewedBy string          `json:"reviewedBy"`
	Outcome    string          `json:"outcome"`
	Notes      string          `json:"notes"`
	Items      []ReadinessItem `json:"items"`
	Created    string          `json:"created"`
}

func (r *Readiness) open() int {
	n := 0
	for _, it := range r.Items {
		if it.Status == "Needs Attention" {
			n++
		}
	}
	return n
}

var readyStatuses = []string{"Ready", "Needs Attention", "Corrected", "N/A"}
var readyTypes = []string{"Internal Self-Inspection", "Pre-Health Inspection", "Follow-Up Recheck", "Other"}
var readyRoles = []string{"Kitchen Operations Manager", "Sous Chef", "Cook", "Prep", "Dish / Stewarding", "Management"}

// readyOwners are the roles plus the first name of everyone on the employee
// list, so a new hire can be named as the owner without touching the program.
func (s *store) readyOwners() []string {
	out := append([]string{}, readyRoles...)
	seen := map[string]bool{}
	for _, e := range s.regSorted(regDefs["employees"]) {
		if e["status"] == "Inactive" {
			continue
		}
		n := strings.Fields(e["name"])
		if len(n) > 0 && !seen[strings.ToLower(n[0])] {
			seen[strings.ToLower(n[0])] = true
			out = append(out, n[0])
		}
	}
	return append(out, "Other")
}

var readyPoints = [][2]string{
	{"Employee Hygiene", "Hair/beard restraints, clean uniforms, proper handwashing, no bare-hand contact where prohibited"},
	{"Employee Health", "Illness reporting awareness and no visibly ill employee working with food"},
	{"Hand Sinks", "Accessible, hot/cold water, soap, paper towels, signage"},
	{"Glove / Utensil Use", "Gloves/utensils used correctly and changed when contaminated"},
	{"Food Storage", "Food off floor, covered/protected, raw below ready-to-eat"},
	{"Date Marking", "Required foods labeled/dated and within use-by period"},
	{"Cold Holding", "TCS foods held at required cold temperature"},
	{"Hot Holding", "TCS foods held at required hot temperature"},
	{"Cooking / Reheat", "Cooking and reheating procedures/temperatures followed"},
	{"Cooling", "Cooling method, depth, containers and timing acceptable"},
	{"Thermometers", "Available, accurate/usable and properly stored"},
	{"Cross Contamination", "Separate tools/surfaces and safe handling practices"},
	{"Allergen Controls", "Knowledge, clean tools/surfaces and cross-contact prevention"},
	{"Food Contact Surfaces", "Clean, sanitized and in good repair"},
	{"Sanitizer", "Correct chemical, concentration, test strips and labeled containers"},
	{"Dish Machine", "Operating correctly, chemicals/temperature and clean/dirty separation"},
	{"Wiping Cloths", "Stored correctly in sanitizer when required"},
	{"Chemicals / SDS", "Chemicals labeled/stored away from food; SDS accessible"},
	{"Pest Control", "No signs of pests; doors/seals/gaps controlled"},
	{"Floors / Walls / Ceilings", "Clean enough, in repair, no major accumulation/leaks"},
	{"Drains / Plumbing", "No sewage backup, leaks or blocked drains"},
	{"Lighting / Shields", "Adequate lighting and protection where needed"},
	{"Equipment Condition", "Food equipment clean, in repair and safely installed"},
	{"Walk-Ins / Reach-Ins", "Clean, organized, gaskets/doors functional"},
	{"Dry Storage", "Clean, dry, organized, products protected"},
	{"Ice / Beverage", "Ice protected and scoop stored properly"},
	{"Trash / Grease", "Waste controlled, containers managed, area not attracting pests"},
	{"Receiving", "Receiving temperatures/condition checks and rejected product controls"},
	{"Records", "Temperature, cleaning, corrective action and training records available"},
	{"Manager Knowledge", "PIC/manager can explain key food-safety controls and corrective actions"},
}

func (s *store) readinessList() []*Readiness {
	out := []*Readiness{}
	s.readMaint("readiness.json", &out)
	return out
}

func (s *store) readinessReport(id string) (*report, bool) {
	today := time.Now().Format("January 2, 2006")
	switch id {
	case "readiness-blank":
		var rows [][]string
		for i, p := range readyPoints {
			rows = append(rows, []string{strconv.Itoa(i + 1), p[0], p[1], "", "", "", "", "", ""})
		}
		return &report{Title: "Food Safety Inspection Readiness Checklist", Subtitle: "Date: ____________   Manager: ____________________   Walk time: ________   Overall ready? Yes / No   Critical finding? Yes / No",
			Sections: []reportSection{{Note: "Status: Ready / Needs Attention / Corrected / N/A. Correct findings immediately when possible and document unresolved corrective actions.", Cols: []string{"#", "Area", "Inspection Point", "Status", "Finding", "Corrected Now?", "Priority", "Owner", "Target Date"}, Rows: rows}}}, true
	case "readiness-log":
		list := s.readinessList()
		sort.SliceStable(list, func(i, j int) bool { return list[i].ID > list[j].ID })
		rows := [][]string{}
		for _, r := range list {
			rows = append(rows, []string{r.ID, r.Date, r.Manager, r.Type, r.Overall, r.Critical, strconv.Itoa(r.open()), r.Recheck, r.ReviewedBy, r.Outcome})
		}
		return &report{Title: "Inspection Readiness History", Subtitle: today, Sections: []reportSection{{Cols: []string{"Review ID", "Date", "Manager", "Type", "Overall Ready?", "Critical Finding?", "Open Actions", "Recheck Date", "Reviewed By", "Outcome"}, Rows: rows}}}, true
	}
	if strings.HasPrefix(id, "readiness:") {
		for _, r := range s.readinessList() {
			if r.ID != strings.TrimPrefix(id, "readiness:") {
				continue
			}
			var rows [][]string
			for _, it := range r.Items {
				rows = append(rows, []string{strconv.Itoa(it.N), it.Area, it.Point, it.Status, it.Finding, it.Corrected, it.Priority, it.Owner, it.Target, it.CARef})
			}
			head := fmt.Sprintf("%s · %s · Manager: %s · %s · Walk time %s", r.ID, r.Date, r.Manager, r.Type, r.WalkTime)
			return &report{Title: "Inspection Readiness Review", Subtitle: head, Sections: []reportSection{
				{Note: fmt.Sprintf("Overall ready: %s · Critical finding: %s · Open actions: %d · Photos: %s · Recheck: %s · Reviewed by: %s · Outcome: %s", r.Overall, r.Critical, r.open(), r.Photos, r.Recheck, r.ReviewedBy, r.Outcome)},
				{Cols: []string{"#", "Area", "Inspection Point", "Status", "Finding", "Corrected Now?", "Priority", "Owner", "Target", "CA Ref"}, Rows: rows},
				{Heading: "Notes", Note: r.Notes},
			}}, true
		}
	}
	return nil, false
}

func (s *store) readinessRoutes(mux *http.ServeMux) {
	payload := func() map[string]any {
		list := s.readinessList()
		sort.SliceStable(list, func(i, j int) bool { return list[i].ID > list[j].ID })
		pts := []map[string]any{}
		for i, p := range readyPoints {
			pts = append(pts, map[string]any{"n": i + 1, "area": p[0], "point": p[1]})
		}
		type row struct {
			*Readiness
			Open int `json:"open"`
		}
		rows := []row{}
		for _, r := range list {
			rows = append(rows, row{r, r.open()})
		}
		return map[string]any{"reviews": rows, "points": pts, "statuses": readyStatuses, "types": readyTypes, "owners": s.readyOwners(), "priorities": caPriorities}
	}
	mux.HandleFunc("GET /api/readiness", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		writeJSON(w, 200, payload())
	})
	mux.HandleFunc("POST /api/readiness", func(w http.ResponseWriter, r *http.Request) {
		var in2 Readiness
		if jsonDecode(r, &in2) != nil {
			fail(w, 400, "Could not read that review.")
			return
		}
		in2.Manager = strings.TrimSpace(in2.Manager)
		if _, err := time.Parse(dayFmt, in2.Date); err != nil {
			fail(w, 400, "Choose the date of the review.")
			return
		}
		if in2.Manager == "" {
			fail(w, 400, "Enter the manager doing the review.")
			return
		}
		if !oneOf(readyTypes, in2.Type) {
			in2.Type = readyTypes[0]
		}
		if (in2.Overall != "Yes" && in2.Overall != "No") || (in2.Critical != "Yes" && in2.Critical != "No") {
			fail(w, 400, "Answer Overall Ready and Critical Finding.")
			return
		}
		if len(in2.Items) != len(readyPoints) {
			fail(w, 400, "The checklist is incomplete. Close it and open it again.")
			return
		}
		for i := range in2.Items {
			it := &in2.Items[i]
			it.N, it.Area, it.Point = i+1, readyPoints[i][0], readyPoints[i][1]
			if !oneOf(readyStatuses, it.Status) {
				it.Status = "Ready"
			}
			if it.Status == "Needs Attention" {
				if strings.TrimSpace(it.Finding) == "" {
					fail(w, 400, fmt.Sprintf("Item %d (%s) needs attention. Write what was found.", it.N, it.Area))
					return
				}
				if !oneOf(caPriorities, it.Priority) {
					it.Priority = "P3 - Routine"
				}
			}
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		list := s.readinessList()
		found := false
		for i, x := range list {
			if in2.ID != "" && x.ID == in2.ID {
				in2.Created = x.Created
				for j := range in2.Items { // keep corrective action links already made
					if in2.Items[j].CARef == "" && j < len(x.Items) {
						in2.Items[j].CARef = x.Items[j].CARef
					}
				}
				list[i], found = &in2, true
			}
		}
		if !found {
			max := 0
			for _, x := range list {
				var n int
				if _, err := fmt.Sscanf(x.ID, "IR-%d", &n); err == nil && n > max {
					max = n
				}
			}
			in2.ID = fmt.Sprintf("IR-%04d", max+1)
			in2.Created = time.Now().Format("2006-01-02 15:04")
			list = append(list, &in2)
		}
		if err := s.writeMaint("readiness.json", list); err != nil {
			fail(w, 500, "The review could not be saved: "+err.Error())
			return
		}
		p := payload()
		p["saved"] = in2.ID
		writeJSON(w, 200, p)
	})
	// Makes Corrective Action forms from the items that need attention.
	mux.HandleFunc("POST /api/readiness/{id}/corrective", func(w http.ResponseWriter, r *http.Request) {
		var in2 struct{ N int } // 0 means every item that still needs a form
		_ = jsonDecode(r, &in2)
		s.mu.Lock()
		defer s.mu.Unlock()
		list := s.readinessList()
		var rv *Readiness
		for _, x := range list {
			if x.ID == r.PathValue("id") {
				rv = x
			}
		}
		if rv == nil {
			fail(w, 404, "That review was not found.")
			return
		}
		forms := s.caForms()
		max := 0
		for _, f := range forms {
			var n int
			if _, err := fmt.Sscanf(f.ID, "CA-%d", &n); err == nil && n > max {
				max = n
			}
		}
		now := time.Now().Format("2006-01-02 15:04")
		var made []*CAForm
		for i := range rv.Items {
			it := &rv.Items[i]
			if it.Status != "Needs Attention" || it.CARef != "" || (in2.N != 0 && in2.N != it.N) {
				continue
			}
			max++
			f := &CAForm{ID: fmt.Sprintf("CA-%04d", max), Date: rv.Date, IdentifiedBy: rv.Manager, Area: "Food Safety", Priority: it.Priority,
				Issue: it.Area + ": " + it.Finding, Risk: it.Point, Owner: it.Owner, Target: it.Target, Status: "Open",
				Related: fmt.Sprintf("Inspection Readiness %s, item %d", rv.ID, it.N), Created: now, Updated: now}
			if it.Corrected == "Yes" {
				f.Immediate = "Corrected during the walk-through."
			}
			it.CARef = f.ID
			forms = append(forms, f)
			made = append(made, f)
		}
		if len(made) == 0 {
			fail(w, 400, "No items are waiting for a corrective action form.")
			return
		}
		if err := s.writeMaint("corrective-actions.json", forms); err != nil {
			fail(w, 500, err.Error())
			return
		}
		if err := s.writeMaint("readiness.json", list); err != nil {
			fail(w, 500, err.Error())
			return
		}
		for _, f := range made {
			s.caSaveFile(f)
		}
		p := payload()
		p["made"] = len(made)
		writeJSON(w, 200, p)
	})
}
