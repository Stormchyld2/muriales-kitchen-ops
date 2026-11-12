package main

// PM inspection sheets: the 16-point checklist done with a preventive
// maintenance job. Each sheet records the condition of the equipment, pass or
// fail for every point, readings, work done, parts used and what still needs
// doing. A failed point must be followed by a maintenance issue before the PM
// job can be marked done, as in the procedures. Sheets are kept in
// Maintenance Data\inspections.json and can be printed.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var inspectItems = [][2]string{
	{"general", "General condition"}, {"clean", "Cleanliness / buildup"}, {"electrical", "Electrical / cord / plug"}, {"controls", "Controls / switches"},
	{"safety", "Safety devices / guards"}, {"fasteners", "Fasteners / hardware"}, {"seals", "Seals / gaskets / doors"}, {"moving", "Moving parts"},
	{"airflow", "Airflow / ventilation"}, {"water", "Water / drain / plumbing"}, {"gas", "Gas / heat source"}, {"temp", "Temperature / performance"},
	{"noise", "Noise / vibration / odor"}, {"lube", "Lubrication / service point"}, {"specific", "Asset-specific PM"}, {"final", "Final operational test"},
}

var inspectConditions = []string{"Good", "Fair", "Poor", "Unsafe"}

type InspectResult struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Result string `json:"result"` // Pass, Fail, N/A
	Note   string `json:"note"`
}

type Inspection struct {
	ID          string          `json:"id"`
	Number      int             `json:"number"`
	TaskID      string          `json:"taskId"`
	TaskName    string          `json:"taskName"`
	EquipmentID string          `json:"equipmentId"`
	Date        string          `json:"date"`
	By          string          `json:"by"`
	Condition   string          `json:"condition"`
	Items       []InspectResult `json:"items"`
	Readings    string          `json:"readings"`
	Work        string          `json:"work"`
	Parts       string          `json:"parts"`
	Corrective  string          `json:"corrective"` // what still needs to be done
	Priority    string          `json:"priority"`
	IssueID     string          `json:"issueId,omitempty"`
	PMDone      bool            `json:"pmDone"`
	Fails       int             `json:"fails"`
}

func (s *store) inspectionList() []*Inspection {
	out := []*Inspection{}
	s.readMaint("inspections.json", &out)
	return out
}

func (s *store) inspectionRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/inspections", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		items := []map[string]string{}
		for _, it := range inspectItems {
			items = append(items, map[string]string{"key": it[0], "label": it[1]})
		}
		writeJSON(w, 200, map[string]any{"inspections": s.inspectionList(), "items": items, "conditions": inspectConditions})
	})
	mux.HandleFunc("POST /api/inspections", func(w http.ResponseWriter, r *http.Request) {
		var in2 struct {
			Inspection
			Results     map[string]InspectResult `json:"results"`
			CreateIssue bool                     `json:"createIssue"`
			MarkDone    bool                     `json:"markDone"`
		}
		if json.NewDecoder(r.Body).Decode(&in2) != nil {
			fail(w, 400, "Could not read that.")
			return
		}
		x := in2.Inspection
		x.By = strings.TrimSpace(x.By)
		if x.By == "" {
			fail(w, 400, "Enter your name.")
			return
		}
		if !oneOf(inspectConditions, x.Condition) {
			fail(w, 400, "Choose the condition of the equipment.")
			return
		}
		if x.Date == "" {
			x.Date = time.Now().Format(dayFmt)
		}
		if d, err := time.Parse(dayFmt, x.Date); err != nil || d.After(time.Now().AddDate(0, 0, 1)) {
			fail(w, 400, "Enter the date of the inspection (today or earlier).")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		eq, is, _ := s.loadMaint()
		var tasks []*PMTask
		s.readMaint("pm.json", &tasks)
		var task *PMTask
		for _, t := range tasks {
			if t.ID == x.TaskID && x.TaskID != "" {
				task = t
				x.TaskName = t.Name
				if x.EquipmentID == "" {
					x.EquipmentID = t.EquipmentID
				}
			}
		}
		var e *Equipment
		for _, q := range eq {
			if q.ID == x.EquipmentID {
				e = q
			}
		}
		if e == nil {
			fail(w, 400, "Choose the equipment that was inspected.")
			return
		}
		failed := []string{}
		x.Items = nil
		for _, it := range inspectItems {
			r := in2.Results[it[0]]
			res := r.Result
			if res != "Pass" && res != "Fail" && res != "N/A" {
				res = "Pass"
			}
			if res == "Fail" {
				failed = append(failed, it[1])
			}
			x.Items = append(x.Items, InspectResult{it[0], it[1], res, strings.TrimSpace(r.Note)})
		}
		x.Fails = len(failed)
		x.Corrective = strings.TrimSpace(x.Corrective)
		if (x.Fails > 0 || x.Condition == "Poor" || x.Condition == "Unsafe") && x.Corrective == "" {
			fail(w, 400, "Describe what needs to be done. Something failed or the condition is Poor or Unsafe.")
			return
		}
		if x.Fails > 0 && in2.MarkDone && !in2.CreateIssue {
			fail(w, 400, "Create a maintenance issue for the failed items before marking the PM job done, or leave it open.")
			return
		}
		if x.Priority = fixPriority(x.Priority); !oneOf(priorities, x.Priority) {
			x.Priority = priorities[2]
			if x.Condition == "Unsafe" {
				x.Priority = priorities[0]
			}
		}
		all := s.inspectionList()
		for _, q := range all {
			if q.Number >= x.Number {
				x.Number = q.Number + 1
			}
		}
		if x.Number == 0 {
			x.Number = 1
		}
		x.ID = newID()
		stampNow := stamp()
		if in2.CreateIssue && x.Corrective != "" {
			title := "PM inspection: " + strings.Join(failed, ", ")
			if len(failed) == 0 {
				title = "PM inspection: " + x.Condition + " condition"
			}
			p := &Issue{ID: newID(), EquipmentID: e.ID, Title: title, Description: x.Corrective, Priority: x.Priority, Status: "Open", ReportedBy: x.By, ReportedAt: stampNow,
				History: []Entry{{stampNow, x.By, fmt.Sprintf("Found on PM inspection #%d.", x.Number)}}}
			for _, q := range is {
				if q.Number >= p.Number {
					p.Number = q.Number + 1
				}
			}
			if p.Number == 0 {
				p.Number = 1
			}
			is = append(is, p)
			if e.Status == "Operational" {
				e.Status = "Needs Attention"
			}
			x.IssueID = p.ID
			if err := s.writeMaint("issues.json", is); err != nil {
				s.maintFail(w, err)
				return
			}
			if err := s.writeMaint("equipment.json", eq); err != nil {
				s.maintFail(w, err)
				return
			}
		}
		if in2.MarkDone && task != nil {
			task.History = append(task.History, Entry{x.Date, x.By, fmt.Sprintf("Done. Inspection #%d: %s condition.", x.Number, x.Condition)})
			if x.Date > task.LastDone {
				task.LastDone = x.Date
			}
			x.PMDone = true
			if err := s.writeMaint("pm.json", tasks); err != nil {
				s.maintFail(w, err)
				return
			}
		}
		all = append(all, &x)
		if err := s.writeMaint("inspections.json", all); err != nil {
			s.maintFail(w, err)
			return
		}
		s.writeLog(eq, is)
		out := s.maintPayload()
		out["inspection"] = x
		writeJSON(w, 200, out)
	})
}

// inspectionReport is the printed sheet: a blank one for paper, or a finished one.
func (s *store) inspectionReport(id string) (*report, bool) {
	eq, _, _ := s.loadMaint()
	names := map[string]string{}
	for _, e := range eq {
		names[e.ID] = e.Name
	}
	if id == "blank" {
		rows := [][]string{}
		for i, it := range inspectItems {
			rows = append(rows, []string{fmt.Sprint(i + 1), it[1], "☐ Pass   ☐ Fail   ☐ N/A", ""})
		}
		return &report{Title: "PM Inspection Sheet", Subtitle: "Equipment: ______________________   Asset ID: __________   Date: __________   Inspected by: ______________________", Sections: []reportSection{
			{Cols: []string{"#", "Inspection point", "Result", "Notes"}, Rows: rows},
			{Heading: "Condition and follow-up", Cols: []string{"", ""}, Rows: [][]string{
				{"Condition", "☐ Good   ☐ Fair   ☐ Poor   ☐ Unsafe"}, {"Readings", "\n"}, {"Work performed", "\n"}, {"Parts used", "\n"},
				{"Corrective action needed", "\n\n"}, {"Priority", "☐ P1   ☐ P2   ☐ P3   ☐ P4"}, {"Work order / issue #", ""}}},
		}}, true
	}
	for _, q := range s.inspectionList() {
		if q.ID != id {
			continue
		}
		rows := [][]string{}
		for i, it := range q.Items {
			rows = append(rows, []string{fmt.Sprint(i + 1), it.Label, it.Result, it.Note})
		}
		d := q.Date
		if t, err := time.Parse(dayFmt, q.Date); err == nil {
			d = t.Format("1/2/2006")
		}
		return &report{Title: fmt.Sprintf("PM Inspection #%d", q.Number), Subtitle: strings.Join(nonEmpty(names[q.EquipmentID], q.TaskName, d, "by "+q.By), " · "), Sections: []reportSection{
			{Cols: []string{"#", "Inspection point", "Result", "Notes"}, Rows: rows},
			{Heading: "Condition and follow-up", Cols: []string{"", ""}, Rows: [][]string{
				{"Condition", q.Condition}, {"Readings", q.Readings}, {"Work performed", q.Work}, {"Parts used", q.Parts},
				{"Corrective action needed", q.Corrective}, {"Priority", q.Priority}, {"PM job marked done", map[bool]string{true: "Yes", false: "No"}[q.PMDone]}}},
		}}, true
	}
	return nil, false
}
