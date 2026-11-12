package main

import (
	"fmt"
	"html"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// CAForm is one Corrective Action Form. Forms are kept in the log and never deleted.
type CAForm struct {
	ID           string `json:"id"`
	Date         string `json:"date"`
	IdentifiedBy string `json:"identifiedBy"`
	Area         string `json:"area"`
	Priority     string `json:"priority"`
	Issue        string `json:"issue"`
	Risk         string `json:"risk"`
	Immediate    string `json:"immediate"`
	RootCause    string `json:"rootCause"`
	Required     string `json:"required"`
	Owner        string `json:"owner"`
	Target       string `json:"target"`
	Training     string `json:"training"`
	WorkOrder    string `json:"workOrder"`
	Vendor       string `json:"vendor"`
	SOP          string `json:"sop"`
	FollowUpNeed string `json:"followUpNeeded"`
	Evidence     string `json:"evidence"`
	DoneDate     string `json:"doneDate"`
	DoneBy       string `json:"doneBy"`
	VerifyDate   string `json:"verifyDate"`
	VerifiedBy   string `json:"verifiedBy"`
	VerifyResult string `json:"verifyResult"`
	Status       string `json:"status"` // Open, In Progress, Closed
	ClosedDate   string `json:"closedDate"`
	ClosedBy     string `json:"closedBy"`
	Repeat       string `json:"repeat"`
	FollowUpDate string `json:"followUpDate"`
	FollowUpRes  string `json:"followUpResult"`
	Notes        string `json:"notes"`
	Related      string `json:"related"`
	Created      string `json:"created"`
	Updated      string `json:"updated"`
}

var caAreas = []string{"Food Safety", "Sanitation", "Safety", "Maintenance", "Training", "Receiving", "Inventory", "Facility", "Chemical / Janitorial", "Other"}
var caPriorities = []string{"P1 - Critical", "P2 - Urgent", "P3 - Routine"}
var caStatuses = []string{"Open", "In Progress", "Closed"}

func (s *store) caForms() []*CAForm {
	out := []*CAForm{}
	s.readMaint("corrective-actions.json", &out)
	return out
}

func (s *store) caDir() string { return filepath.Join(filepath.Dir(s.dir), "Corrective Action Forms") }

func (f *CAForm) overdue() bool {
	return f.Status != "Closed" && f.Target != "" && f.Target < time.Now().Format(dayFmt)
}

func (f *CAForm) page(autoPrint bool) string {
	e := func(v string) string { return strings.ReplaceAll(html.EscapeString(v), "\n", "<br>") }
	d := func(v string) string {
		if t, err := time.Parse(dayFmt, v); err == nil {
			return t.Format("1/2/2006")
		}
		return v
	}
	cell := func(label, val string) string {
		return `<td><div class="l">` + e(label) + `</div><div class="v">` + e(val) + `&nbsp;</div></td>`
	}
	block := func(label, val string) string {
		return `<tr><td colspan="4"><div class="l">` + e(label) + `</div><div class="v tall">` + e(val) + `&nbsp;</div></td></tr>`
	}
	var b strings.Builder
	b.WriteString(`<!doctype html><html><head><meta charset="utf-8"><title>` + e(f.ID) + ` Corrective Action</title><style>
@page{margin:10mm}body{font:12px/1.35 Segoe UI,Arial,sans-serif;color:#111;margin:20px}.org{color:#7a1f2b;font-weight:700;font-size:12px;letter-spacing:.04em;text-transform:uppercase}h1{margin:0 0 6px;font-size:21px}
h2{font-size:13px;background:#7a1f2b;color:#fff;margin:12px 0 0;padding:3px 8px}table{border-collapse:collapse;width:100%;table-layout:fixed}td{border:1px solid #999;padding:3px 7px;vertical-align:top}
.l{font-size:10px;color:#555;text-transform:uppercase}.v{min-height:16px}.tall{min-height:42px}.btn{margin:8px 0;padding:6px 14px}@media print{.noprint{display:none}body{margin:0}tr{page-break-inside:avoid}}
</style></head><body><div class="noprint"><button class="btn" onclick="window.print()">Print</button></div>
<div class="org">Muriale's Italian Kitchen</div><h1>Corrective Action Form</h1>
<h2>Issue Information</h2><table><tr>` + cell("Corrective Action ID", f.ID) + cell("Date Identified", d(f.Date)) + cell("Identified By", f.IdentifiedBy) + cell("Area / System", f.Area) + `</tr><tr>` + cell("Priority", f.Priority) + `<td colspan="3"></td></tr>` +
		block("Issue / Finding", f.Issue) + block("Immediate Risk / Impact", f.Risk) + block("Immediate Correction Taken", f.Immediate) + `</table>
<h2>Root Cause / Required Action</h2><table>` + block("Root Cause / Contributing Factors", f.RootCause) + block("Required Corrective Action", f.Required) + `<tr>` + cell("Assigned Owner", f.Owner) + cell("Target Date", d(f.Target)) + cell("Training Required?", f.Training) + cell("Work Order / MaintainX Needed?", f.WorkOrder) + `</tr><tr>` +
		cell("Vendor Needed?", f.Vendor) + cell("SOP / Procedure Change Needed?", f.SOP) + cell("Follow-Up Inspection Needed?", f.FollowUpNeed) + cell("Evidence / Photo Ref", f.Evidence) + `</tr><tr>` +
		cell("Completion Date", d(f.DoneDate)) + cell("Completed By", f.DoneBy) + cell("Verification Date", d(f.VerifyDate)) + cell("Verified By", f.VerifiedBy) + `</tr>` + block("Verification Result", f.VerifyResult) + `</table>
<h2>Closure</h2><table><tr>` + cell("Status", f.Status) + cell("Closed Date", d(f.ClosedDate)) + cell("Closed By", f.ClosedBy) + cell("Repeat Issue?", f.Repeat) + `</tr><tr>` + cell("Follow-Up Date", d(f.FollowUpDate)) + `<td colspan="3">` + `<div class="l">Follow-Up Result</div><div class="v">` + e(f.FollowUpRes) + `&nbsp;</div></td></tr>` +
		block("Management Notes", f.Notes) + block("Related Records", f.Related) + `</table>
<p style="margin-top:14px">Manager signature: ______________________________ &nbsp;&nbsp; Date: ______________</p>
<div style="color:#666;font-size:10px">Printed ` + time.Now().Format("1/2/2006 3:04 PM") + `</div>`)
	if autoPrint {
		b.WriteString(`<script>onload=function(){setTimeout(function(){print()},300)}</script>`)
	}
	b.WriteString(`</body></html>`)
	return b.String()
}

func (s *store) caSaveFile(f *CAForm) {
	if os.MkdirAll(s.caDir(), 0o755) == nil {
		_ = os.WriteFile(filepath.Join(s.caDir(), f.ID+".html"), []byte(f.page(false)), 0o644)
	}
}

func (s *store) caReport(openOnly bool) *report {
	forms := s.caForms()
	sort.SliceStable(forms, func(i, j int) bool { return forms[i].ID > forms[j].ID })
	rows := [][]string{}
	for _, f := range forms {
		if openOnly && f.Status == "Closed" {
			continue
		}
		st := f.Status
		if f.overdue() {
			st += " (overdue)"
		}
		rows = append(rows, []string{f.ID, f.Date, f.Area, f.Priority, f.Issue, f.Owner, f.Target, st, f.ClosedDate})
	}
	title := "Corrective Action Log"
	if openOnly {
		title = "Open Corrective Actions"
	}
	return &report{Title: title, Subtitle: time.Now().Format("January 2, 2006"), Sections: []reportSection{{Cols: []string{"ID", "Date", "Area", "Priority", "Issue", "Owner", "Target", "Status", "Closed"}, Rows: rows}}}
}

func (s *store) caTodo() []todoItem {
	out := []todoItem{}
	for _, f := range s.caForms() {
		if f.Status == "Closed" {
			continue
		}
		state := "Open"
		if f.overdue() {
			state = "Overdue"
		} else if f.Target == time.Now().Format(dayFmt) {
			state = "Due Today"
		}
		due := f.Target
		if due == "" {
			due = f.Date
		}
		out = append(out, todoItem{Group: "Corrective Action", Title: f.ID + " " + f.Issue, Detail: strings.Join(nonEmpty(f.Area, f.Priority, "Owner: "+f.Owner), " · "), Due: due, State: state, Kind: "ca", ID: f.ID})
	}
	return out
}

func (s *store) correctiveRoutes(mux *http.ServeMux) {
	payload := func() map[string]any {
		forms := s.caForms()
		sort.SliceStable(forms, func(i, j int) bool { return forms[i].ID > forms[j].ID })
		return map[string]any{"forms": forms, "areas": caAreas, "priorities": caPriorities, "statuses": caStatuses, "folder": s.caDir()}
	}
	mux.HandleFunc("GET /api/corrective", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		writeJSON(w, 200, payload())
	})
	mux.HandleFunc("POST /api/corrective", func(w http.ResponseWriter, r *http.Request) {
		var in2 CAForm
		if jsonDecode(r, &in2) != nil {
			fail(w, 400, "Could not read that form.")
			return
		}
		for _, v := range []*string{&in2.IdentifiedBy, &in2.Issue} {
			*v = strings.TrimSpace(*v)
		}
		switch {
		case in2.IdentifiedBy == "":
			fail(w, 400, "Enter who identified the issue.")
			return
		case !oneOf(caAreas, in2.Area):
			fail(w, 400, "Choose the area or system.")
			return
		case !oneOf(caPriorities, in2.Priority):
			fail(w, 400, "Choose a priority.")
			return
		case in2.Issue == "":
			fail(w, 400, "Describe the issue or finding.")
			return
		}
		if in2.Status == "" {
			in2.Status = "Open"
		}
		if !oneOf(caStatuses, in2.Status) {
			fail(w, 400, "Choose a status.")
			return
		}
		if in2.Status == "Closed" {
			if strings.TrimSpace(in2.ClosedBy) == "" {
				fail(w, 400, "Enter who is closing this action.")
				return
			}
			if strings.TrimSpace(in2.Required) == "" {
				fail(w, 400, "Describe the corrective action before closing it.")
				return
			}
			if in2.ClosedDate == "" {
				in2.ClosedDate = time.Now().Format(dayFmt)
			}
		} else {
			in2.ClosedDate, in2.ClosedBy = "", ""
		}
		if in2.Date == "" {
			in2.Date = time.Now().Format(dayFmt)
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		forms := s.caForms()
		now := time.Now().Format("2006-01-02 15:04")
		var saved *CAForm
		for i, f := range forms {
			if in2.ID != "" && f.ID == in2.ID {
				in2.Created, in2.Updated = f.Created, now
				forms[i] = &in2
				saved = &in2
			}
		}
		if saved == nil {
			max := 0
			for _, f := range forms {
				var n int
				if _, err := fmt.Sscanf(f.ID, "CA-%d", &n); err == nil && n > max {
					max = n
				}
			}
			in2.ID = fmt.Sprintf("CA-%04d", max+1)
			in2.Created, in2.Updated = now, now
			forms = append(forms, &in2)
			saved = &in2
		}
		if err := s.writeMaint("corrective-actions.json", forms); err != nil {
			fail(w, 500, "The form could not be saved: "+err.Error())
			return
		}
		s.caSaveFile(saved)
		p := payload()
		p["saved"] = saved
		writeJSON(w, 200, p)
	})
	mux.HandleFunc("GET /api/corrective/{id}/print", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		forms := s.caForms()
		s.mu.Unlock()
		for _, f := range forms {
			if f.ID == r.PathValue("id") {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				_, _ = w.Write([]byte(f.page(r.URL.Query().Get("print") == "1")))
				return
			}
		}
		fail(w, 404, "That form was not found.")
	})
}
