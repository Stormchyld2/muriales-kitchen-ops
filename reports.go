package main

// Reports: every printable report the program can produce, grouped by module.
// A report is built on request from the stored data and shown as a plain
// page that prints well. A report can also be emailed: the program saves it
// (as a PDF when Edge can make one, otherwise as a web page) in the
// "Kitchen Reports" folder and opens a draft in Outlook with it attached. The
// draft is never sent by the program; the manager reads it and presses Send.

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/mail"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type reportSection struct {
	Heading string
	Note    string
	Cols    []string
	Rows    [][]string
}

type report struct {
	Title    string
	Subtitle string
	Sections []reportSection
}

type reportDef struct {
	ID     string `json:"id"`
	Module string `json:"module"`
	Title  string `json:"title"`
	About  string `json:"about"`
	Month  bool   `json:"month,omitempty"` // takes a month
}

var reportCatalog = []reportDef{
	{"handoff", "Maintenance", "Manager Handoff", "The notes left for the next manager plus what is open: out-of-service equipment, issues, overdue jobs and missed cleaning.", false},
	{"equipment", "Maintenance", "Equipment List", "Every piece of equipment with make, model, serial number, status and open issues.", false},
	{"issues", "Maintenance", "Open Maintenance Issues", "Work orders not yet verified, with priority, who it is assigned to and the MaintainX number.", false},
	{"assets", "Maintenance", "Asset Costs and Replacement Advice", "Spending on upkeep for each unit, last 12 months, and the repair-or-replace suggestion.", false},
	{"oos", "Maintenance", "Out-of-Service History", "Every time equipment was taken out of service: why, who, and who cleared it.", false},
	{"findings", "Maintenance", "Inspection Items", "Things found on walk-throughs that still need doing.", false},
	{"inspection:blank", "Maintenance", "Blank PM Inspection Sheet", "The 16-point checklist to print and fill in by hand.", false},
	{"summary", "Maintenance", "Monthly Maintenance Summary", "The month at a glance for management, with the notes saved for that month.", true},
	{"procedures", "Maintenance", "Maintenance Procedures", "How the department works: the steps, the review schedule and what each priority means.", false},
	{"temps-week", "Food Safety", "Temperature Log: Last 7 Days", "Every temperature reading from the last week with its safe range and status.", false},
	{"temps-bad", "Food Safety", "Out-of-Range Temperatures", "Readings outside the safe range in the last 30 days, with the corrective action taken.", false},
	{"ca-open", "Corrective Actions", "Open Corrective Actions", "Actions not yet closed, with owner, target date and whether they are overdue.", false},
	{"ca-log", "Corrective Actions", "Corrective Action Log", "Every corrective action form on file.", false},
	{"chem-response", "Chemical Safety", "Spill / Exposure Response Sheet", "The immediate-response steps and exposure quick reference to post or keep at hand.", false},
	{"chem-blank", "Chemical Safety", "Blank Chemical Incident Form", "Fill in by hand during an incident, then enter it in the log.", false},
	{"chem-log", "Chemical Safety", "Chemical Incident Log", "Every chemical spill or exposure on record.", false},
	{"inspection-ready", "Food Safety", "Inspection-Ready Checklist", "A live check of the records (temperatures, cleaning, maintenance, corrective actions, SDS) plus the walk-through points and a sign-off line.", false},
	{"weekly-summary", "Summary", "Weekly Summary", "The last 7 days at a glance: temperatures, cleaning, maintenance, corrective actions and what is still open.", false},
	{"readiness-blank", "Food Safety", "Blank Inspection Readiness Checklist", "The 30-point self-inspection to print and carry on a walk-through.", false},
	{"readiness-log", "Food Safety", "Inspection Readiness History", "Every readiness review with the result and open actions.", false},
	{"replace-log", "Maintenance", "Equipment Replacement History", "Every replacement request with the decision and cost.", false},
	{"clean-schedule", "Cleaning & Sanitation", "Cleaning Schedule", "Every cleaning task with frequency, timing, role and standard.", false},
	{"clean-weekly", "Cleaning & Sanitation", "Weekly Cleaning Schedule (to post)", "The week's deep-clean jobs with spaces for assignment, initials and verification.", false},
	{"linecook-weekly", "Cleaning & Sanitation", "Line Cook Weekly Cleaning Schedule", "The nightly close and day-by-day detail cleaning for line cooks.", false},
	{"clean-standards", "Cleaning & Sanitation", "Cleaning Standards", "What clean means for each job: steps, sanitizing and verification.", false},
	{"clean-chem", "Cleaning & Sanitation", "Chemical & Tool Reference", "Approved chemicals and tools, dilution, PPE and storage.", false},
	{"clean-log", "Cleaning & Sanitation", "Cleaning Completion Log", "Cleaning completed in the last 30 days and who verified it.", false},
	{"clean-missed", "Cleaning & Sanitation", "Missed & Overdue Cleaning Log", "Cleaning missed or failed, and how it was corrected.", false},
	{"mgr-schedule", "Cleaning & Sanitation", "Manager Inspection Schedule", "The recurring manager checks and when each is due.", false},
	{"mgr-log", "Cleaning & Sanitation", "Manager Inspection Log", "Manager checks done in the last 30 days and their results.", false},
	{"pm-equipment", "Schedule", "Equipment PM Schedule", "Recurring equipment maintenance jobs, when each was last done and when it is next due.", false},
	{"pm-cleaning", "Schedule", "Cleaning Schedule", "Daily, weekly, monthly and yearly cleaning tasks with last done and next due.", false},
	{"pm-facility", "Schedule", "Grounds and Facility Schedule", "Building and grounds jobs with last done and next due.", false},
	{"overdue", "Schedule", "Overdue Jobs", "Every recurring job that is past due, all groups together.", false},
	{"todo-today", "Schedule", "To-Do List: Today", "Everything due today plus anything overdue, from all areas.", false},
	{"todo-week", "Schedule", "To-Do List: This Week", "Everything due in the next 7 days plus anything overdue, from all areas.", false},
	{"sds-library", "SDS Manager", "SDS Library", "Every product with manufacturer, category, location, sheet date and signal word.", false},
	{"sds-review", "SDS Manager", "SDS Needing Attention", "Sheets that are missing, changed, or still need their details checked.", false},
}

func (s *store) buildReport(id, month string) (*report, bool) {
	if id == "handoff" {
		return s.handoffReport(), true
	}
	if id == "inspection-ready" {
		return s.inspectionReadyReport(), true
	}
	if id == "weekly-summary" {
		return s.weeklySummaryReport(), true
	}
	if strings.HasPrefix(id, "inspection:") {
		return s.inspectionReport(strings.TrimPrefix(id, "inspection:"))
	}
	if strings.HasPrefix(id, "clean-") || strings.HasPrefix(id, "mgr-") || id == "linecook-weekly" {
		return s.cleanReport(id)
	}
	if strings.HasPrefix(id, "replace") {
		return s.replReport(id)
	}
	if strings.HasPrefix(id, "chem") {
		return s.chemReport(id)
	}
	if strings.HasPrefix(id, "readiness") {
		return s.readinessReport(id)
	}
	eq, is, _ := s.loadMaint()
	_, _, st := s.loadMaint()
	names := map[string]string{}
	for _, e := range eq {
		names[e.ID] = e.Name
	}
	nm := func(id string) string {
		if n := names[id]; n != "" {
			return n
		}
		return ""
	}
	d := func(v string) string {
		if len(v) >= 10 {
			if t, err := time.Parse(dayFmt, v[:10]); err == nil {
				return t.Format("1/2/2006")
			}
		}
		return v
	}
	money := func(f float64) string { return fmt.Sprintf("$%.2f", f) }
	today := time.Now().Format("January 2, 2006")
	pmRows := func(group string) [][]string {
		rows := [][]string{}
		list := s.pmList()
		sort.SliceStable(list, func(i, j int) bool { return list[i].NextDue < list[j].NextDue })
		for _, t := range list {
			if group != "" && t.Group != group {
				continue
			}
			rep := t.FreqLabel
			if rep == "" {
				rep = t.Bucket
			}
			where := strings.Join(nonEmpty(nm(t.EquipmentID), t.Area), " · ")
			rows = append(rows, []string{t.Name, where, rep, d(t.LastDone), d(t.NextDue), t.State, t.Performer})
		}
		return rows
	}
	pmCols := []string{"Job", "Equipment / Area", "Repeats", "Last Done", "Next Due", "Status", "Done By"}

	switch id {
	case "equipment":
		r := &report{Title: "Equipment List", Subtitle: today}
		rows := [][]string{}
		for _, e := range eq {
			open := 0
			for _, p := range is {
				if p.EquipmentID == e.ID && p.Status != "Verified" && p.Status != "Closed" {
					open++
				}
			}
			rows = append(rows, []string{e.Name, e.Area, e.Type, strings.TrimSpace(e.Make + " " + e.Model), e.Serial, e.Status, fmt.Sprint(open)})
		}
		sort.Slice(rows, func(a, b int) bool { return strings.ToLower(rows[a][0]) < strings.ToLower(rows[b][0]) })
		r.Sections = []reportSection{{Cols: []string{"Equipment", "Area", "Type", "Make / Model", "Serial Number", "Status", "Open Issues"}, Rows: rows}}
		return r, true
	case "issues":
		rows := [][]string{}
		for _, p := range is {
			if p.Status == "Verified" || p.Status == "Closed" {
				continue
			}
			rows = append(rows, []string{"#" + itoa(p.Number), nm(p.EquipmentID), p.Title, p.Priority, p.Status, d(p.ReportedAt), p.AssignedTo, p.WorkOrder})
		}
		sort.SliceStable(rows, func(a, b int) bool { return rows[a][3] < rows[b][3] })
		return &report{Title: "Open Maintenance Issues", Subtitle: today, Sections: []reportSection{{Cols: []string{"#", "Equipment", "Issue", "Priority", "Status", "Reported", "Assigned To", "Work Order"}, Rows: rows}}}, true
	case "assets":
		a := s.assetSummaries(eq, st.ReplacePercent)
		rows := [][]string{}
		for _, e := range eq {
			x := a[e.ID]
			pct := ""
			if x.Percent >= 0 {
				pct = fmt.Sprintf("%d%%", x.Percent)
			}
			age := ""
			if x.AgeYears > 0 {
				age = fmt.Sprintf("%.1f yr", x.AgeYears)
			}
			rows = append(rows, []string{e.Name, strings.TrimSpace(e.Make + " " + e.Model), e.Serial, age, money(x.Total), money(x.Last12), pct, x.Advice})
		}
		sort.Slice(rows, func(i, j int) bool {
			return rows[i][7] > rows[j][7] || rows[i][7] == rows[j][7] && rows[i][0] < rows[j][0]
		})
		return &report{Title: "Asset Costs and Replacement Advice", Subtitle: today, Sections: []reportSection{{Note: fmt.Sprintf("A unit is flagged when upkeep reaches %d%% of the cost of a new unit. This is a rule of thumb: also consider downtime, energy use and parts availability.", st.ReplacePercent),
			Cols: []string{"Equipment", "Make / Model", "Serial", "Age", "Spent on Upkeep", "Last 12 Months", "% of New Unit", "Suggestion"}, Rows: rows}}}, true
	case "oos":
		rows := [][]string{}
		all := s.oosList()
		for i := len(all) - 1; i >= 0; i-- {
			x := all[i]
			cleared := ""
			if x.ClearedAt != "" {
				cleared = d(x.ClearedAt) + " by " + x.ClearedBy
			} else {
				cleared = "STILL OUT OF SERVICE"
			}
			rows = append(rows, []string{nm(x.EquipmentID), x.Reason, d(x.RemovedAt) + " by " + x.RemovedBy, cleared, x.ClearNote, daysOut(x)})
		}
		return &report{Title: "Out-of-Service History", Subtitle: today, Sections: []reportSection{{Cols: []string{"Equipment", "Reason", "Removed", "Cleared", "What Was Checked or Fixed", "Time Out"}, Rows: rows}}}, true
	case "findings":
		rows := [][]string{}
		for _, f := range s.findingList() {
			if f.Status == "Done" {
				continue
			}
			rows = append(rows, []string{"#" + itoa(f.Number), f.What, strings.Join(nonEmpty(f.Area, nm(f.EquipmentID)), " · "), f.Category, f.Priority, f.AssignedTo, d(f.Due), f.Status})
		}
		return &report{Title: "Open Inspection Items", Subtitle: today, Sections: []reportSection{{Cols: []string{"#", "What Needs Doing", "Where", "Category", "Priority", "Assigned To", "Due", "Status"}, Rows: rows}}}, true
	case "summary":
		if _, err := time.Parse("2006-01", month); err != nil {
			month = time.Now().Format("2006-01")
		}
		m := s.monthSummary(month)
		num := func(k string) string { return fmt.Sprint(m[k]) }
		r := &report{Title: "Monthly Maintenance Summary", Subtitle: fmt.Sprint(m["label"])}
		r.Sections = append(r.Sections, reportSection{Heading: "At a glance", Cols: []string{"Measure", "Value"}, Rows: [][]string{
			{"Work orders opened", num("opened")}, {"Work orders completed", num("completed")}, {"Open now", num("openNow")}, {"P1 / P2 open now", num("p1p2Open")},
			{"Equipment out of service now", num("outNow")}, {"Times equipment was taken out of service", num("outEvents")},
			{"PM jobs overdue now", num("overduePM")}, {"PM jobs completed", num("pmDone")}, {"PM jobs due this month", num("pmDueThisMonth")},
			{"Repair cost", money(m["repairCost"].(float64))}, {"Downtime (hours)", fmt.Sprint(m["downtime"])}}})
		pr := [][]string{}
		for _, x := range m["byPriority"].([]prioRow) {
			pr = append(pr, []string{x.Priority, fmt.Sprint(x.Count), money(x.Cost)})
		}
		r.Sections = append(r.Sections, reportSection{Heading: "Completed repairs by priority", Cols: []string{"Priority", "Completed", "Cost"}, Rows: pr})
		rp := [][]string{}
		for _, x := range m["repeat"].([]repeatRow) {
			rp = append(rp, []string{x.Equipment, fmt.Sprint(x.Issues)})
		}
		r.Sections = append(r.Sections, reportSection{Heading: "Equipment with repeat problems (2 or more issues in 3 months)", Cols: []string{"Equipment", "Issues"}, Rows: rp})
		mj := [][]string{}
		for _, x := range m["major"].([]majorRow) {
			mj = append(mj, []string{d(x.Date), x.Equipment, x.What, money(x.Cost)})
		}
		r.Sections = append(r.Sections, reportSection{Heading: "Largest repairs", Cols: []string{"Date", "Equipment", "Repair", "Cost"}, Rows: mj})
		n := m["notes"].(summaryNotes)
		r.Sections = append(r.Sections, reportSection{Heading: "Management notes", Cols: []string{"", ""}, Rows: [][]string{{"Highlights", n.Highlights}, {"Concerns", n.Concerns}, {"Actions for next month", n.Actions}}})
		return r, true
	case "procedures":
		p := s.loadProcedures()
		rep := &report{Title: "Maintenance Procedures", Subtitle: today}
		steps := [][]string{}
		for _, x := range p.Steps {
			steps = append(steps, []string{x.Title, x.Text})
		}
		rep.Sections = append(rep.Sections, reportSection{Heading: "Procedures", Cols: []string{"Step", "What happens"}, Rows: steps})
		pr := [][]string{}
		for _, x := range p.Priorities {
			pr = append(pr, []string{x.Priority, x.Use, x.Response})
		}
		rep.Sections = append(rep.Sections, reportSection{Heading: "Priorities", Cols: []string{"Priority", "Use", "Response"}, Rows: pr})
		cd := [][]string{}
		for _, x := range p.Cadence {
			cd = append(cd, []string{x.When, x.Task, x.Owner, x.Backup})
		}
		rep.Sections = append(rep.Sections, reportSection{Heading: "Review schedule", Cols: []string{"When", "Review", "Owner", "Backup"}, Rows: cd})
		return rep, true
	case "temps-week", "temps-bad":
		days, title := 7, "Temperature Log: Last 7 Days"
		if id == "temps-bad" {
			days, title = 30, "Out-of-Range Temperatures"
		}
		rows := [][]string{}
		for _, v := range s.tempViews(time.Now().AddDate(0, 0, -days).Format(dayFmt), "") {
			bad := v.Status == "Too Warm" || v.Status == "Too Cold"
			if id == "temps-bad" && !bad {
				continue
			}
			at := v.At
			if t, err := time.Parse("2006-01-02 15:04", v.At); err == nil {
				at = t.Format("1/2/2006 3:04 PM")
			}
			rows = append(rows, []string{at, v.Point, fmt.Sprintf("%g°F", v.Temp), v.Limit, v.Status, v.By, strings.TrimSpace(v.Notes + " " + v.Action)})
		}
		return &report{Title: title, Subtitle: today, Sections: []reportSection{{Note: "Safe ranges follow the FDA Food Code unless a different range was set for the point.", Cols: []string{"When", "Point", "Temperature", "Safe Range", "Status", "By", "Notes / Corrective Action"}, Rows: rows}}}, true
	case "ca-open", "ca-log":
		return s.caReport(id == "ca-open"), true
	case "pm-equipment":
		return &report{Title: "Equipment PM Schedule", Subtitle: today, Sections: []reportSection{{Cols: pmCols, Rows: pmRows("Equipment PM")}}}, true
	case "pm-cleaning":
		return &report{Title: "Cleaning Schedule", Subtitle: today, Sections: []reportSection{{Cols: pmCols, Rows: pmRows("Cleaning")}}}, true
	case "pm-facility":
		return &report{Title: "Grounds and Facility Schedule", Subtitle: today, Sections: []reportSection{{Cols: pmCols, Rows: pmRows("Grounds / Facility")}}}, true
	case "overdue":
		rows := [][]string{}
		for _, t := range s.pmList() {
			if t.State == "Overdue" {
				rows = append(rows, []string{t.Group, t.Name, strings.Join(nonEmpty(nm(t.EquipmentID), t.Area), " · "), d(t.LastDone), d(t.NextDue), fmt.Sprintf("%d days", -t.Days), t.Performer})
			}
		}
		return &report{Title: "Overdue Jobs", Subtitle: today, Sections: []reportSection{{Cols: []string{"Group", "Job", "Equipment / Area", "Last Done", "Was Due", "Overdue By", "Done By"}, Rows: rows}}}, true
	case "todo-today", "todo-week":
		from, _ := time.Parse(dayFmt, time.Now().Format(dayFmt))
		to, title := from, "To-Do List: Today"
		if id == "todo-week" {
			to, title = from.AddDate(0, 0, 6), "To-Do List: This Week"
		}
		want := map[string]bool{}
		for _, g := range todoSources {
			want[g] = true
		}
		r := &report{Title: title, Subtitle: today}
		for _, g := range todoSources {
			rows := [][]string{}
			for _, it := range s.todoItems(from, to, want) {
				if it.Group == g {
					rows = append(rows, []string{"☐", it.Title, it.Detail, d(it.Due), it.State})
				}
			}
			if len(rows) > 0 {
				r.Sections = append(r.Sections, reportSection{Heading: g, Cols: []string{"", "Item", "Details", "Due", "Status"}, Rows: rows})
			}
		}
		if len(r.Sections) == 0 {
			r.Sections = []reportSection{{Note: "Nothing is due."}}
		}
		return r, true
	case "sds-library", "sds-review":
		bad := s.integrityProblems()
		rows := [][]string{}
		prods := append([]*Product{}, s.lib.Products...)
		sort.Slice(prods, func(i, j int) bool { return strings.ToLower(prods[i].Name) < strings.ToLower(prods[j].Name) })
		for _, p := range prods {
			state := ""
			switch {
			case p.Missing:
				state = "PDF missing"
			case bad[p.ID] == "changed":
				state = "PDF changed since added"
			case p.NeedsReview:
				state = "Needs review"
			case p.SummaryNeedsCheck:
				state = "Summary needs checking"
			}
			if id == "sds-review" && state == "" {
				continue
			}
			if id == "sds-library" {
				rows = append(rows, []string{p.Name, p.Manufacturer, p.Category, p.Location, d(p.RevisionDate), p.SignalWord, state})
			} else {
				rows = append(rows, []string{p.Name, p.Manufacturer, p.Location, state})
			}
		}
		if id == "sds-review" {
			return &report{Title: "SDS Needing Attention", Subtitle: today, Sections: []reportSection{{Cols: []string{"Product", "Manufacturer", "Location", "Problem"}, Rows: rows}}}, true
		}
		return &report{Title: "SDS Library", Subtitle: fmt.Sprintf("%s · %d products", today, len(rows)), Sections: []reportSection{{Cols: []string{"Product", "Manufacturer", "Category", "Location", "Sheet Date", "Signal Word", "Check"}, Rows: rows}}}, true
	}
	return nil, false
}

func (r *report) html(plain bool) string {
	var b strings.Builder
	e := html.EscapeString
	b.WriteString(`<!doctype html><html><head><meta charset="utf-8"><title>` + e(r.Title) + `</title><style>
body{font:13px/1.4 Segoe UI,Arial,sans-serif;color:#111;margin:28px}h1{margin:0;font-size:22px}.sub{color:#555;margin-bottom:6px}.org{color:#7a1f2b;font-weight:700;font-size:12px;letter-spacing:.04em;text-transform:uppercase}
h2{font-size:15px;margin:20px 0 4px;border-bottom:1px solid #999;padding-bottom:2px}table{border-collapse:collapse;width:100%;margin:6px 0}th{background:#eee;text-align:left}th,td{border:1px solid #bbb;padding:4px 7px;vertical-align:top}
.note{color:#444;margin:4px 0}.empty{color:#666;font-style:italic;padding:8px 0}.btn{margin:10px 0;padding:6px 14px}@media print{.noprint{display:none}body{margin:12mm}tr{page-break-inside:avoid}thead{display:table-header-group}}
</style></head><body>`)
	if !plain {
		b.WriteString(`<div class="noprint"><button class="btn" onclick="window.print()">Print</button></div>`)
	}
	b.WriteString(`<div class="org">Muriale's Italian Kitchen</div><h1>` + e(r.Title) + `</h1><div class="sub">` + e(r.Subtitle) + `</div>`)
	for _, sec := range r.Sections {
		if sec.Heading != "" {
			b.WriteString("<h2>" + e(sec.Heading) + "</h2>")
		}
		if sec.Note != "" {
			b.WriteString(`<div class="note">` + strings.ReplaceAll(e(sec.Note), "\n", "<br>") + `</div>`)
		}
		if len(sec.Cols) == 0 {
			continue
		}
		if len(sec.Rows) == 0 {
			b.WriteString(`<div class="empty">Nothing to show.</div>`)
			continue
		}
		b.WriteString("<table><thead><tr>")
		for _, c := range sec.Cols {
			b.WriteString("<th>" + e(c) + "</th>")
		}
		b.WriteString("</tr></thead><tbody>")
		for _, row := range sec.Rows {
			b.WriteString("<tr>")
			for _, c := range row {
				b.WriteString("<td>" + strings.ReplaceAll(e(c), "\n", "<br>") + "</td>")
			}
			b.WriteString("</tr>")
		}
		b.WriteString("</tbody></table>")
	}
	b.WriteString(`<div class="sub" style="margin-top:18px">Printed ` + time.Now().Format("1/2/2006 3:04 PM") + `</div></body></html>`)
	return b.String()
}

// text is a short plain-text version, used as the body of an email draft.
func (r *report) text() string {
	var b strings.Builder
	b.WriteString(r.Title + "\r\n" + r.Subtitle + "\r\n")
	for _, sec := range r.Sections {
		if sec.Heading != "" {
			b.WriteString("\r\n" + sec.Heading + "\r\n")
		}
		if sec.Note != "" {
			b.WriteString(sec.Note + "\r\n")
		}
		n := 0
		for _, row := range sec.Rows {
			if n++; n > 25 {
				b.WriteString("... and more. See the attached report.\r\n")
				break
			}
			b.WriteString("- " + strings.Join(nonEmpty(row...), " | ") + "\r\n")
		}
	}
	return b.String()
}

func jsonDecode(r *http.Request, v any) error { return json.NewDecoder(r.Body).Decode(v) }

var safeName = regexp.MustCompile(`[^A-Za-z0-9 _.-]+`)

func (s *store) reportsDir() string { return filepath.Join(filepath.Dir(s.dir), "Kitchen Reports") }

func (s *store) reportRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/reports", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"reports": reportCatalog})
	})
	mux.HandleFunc("GET /api/reports/{id}", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		rep, ok := s.buildReport(r.PathValue("id"), r.URL.Query().Get("month"))
		s.mu.Unlock()
		if !ok {
			fail(w, 404, "That report does not exist.")
			return
		}
		page := rep.html(false)
		if r.URL.Query().Get("print") == "1" {
			page = strings.Replace(page, "</body>", `<script>onload=function(){setTimeout(function(){print()},300)}</script></body>`, 1)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(page))
	})
	// Opens an Outlook draft with the report attached. It never sends.
	mux.HandleFunc("POST /api/reports/email", func(w http.ResponseWriter, r *http.Request) {
		var in2 struct {
			ID, Month, To, Subject, Message string
		}
		if jsonDecode(r, &in2) != nil {
			fail(w, 400, "Could not read that.")
			return
		}
		to := strings.NewReplacer(",", ";", "\n", ";").Replace(in2.To)
		var addrs []string
		for _, a := range strings.Split(to, ";") {
			if a = strings.TrimSpace(a); a == "" {
				continue
			}
			p, err := mail.ParseAddress(a)
			if err != nil || strings.ContainsAny(a, "\r\n") {
				fail(w, 400, "“"+a+"” is not a valid email address.")
				return
			}
			addrs = append(addrs, p.Address)
		}
		if len(addrs) == 0 {
			fail(w, 400, "Enter who to send it to.")
			return
		}
		s.mu.Lock()
		rep, ok := s.buildReport(in2.ID, in2.Month)
		s.mu.Unlock()
		if !ok {
			fail(w, 404, "That report does not exist.")
			return
		}
		subject := strings.TrimSpace(strings.NewReplacer("\r", " ", "\n", " ").Replace(in2.Subject))
		if subject == "" {
			subject = rep.Title + " - " + rep.Subtitle
		}
		if len(subject) > 200 || len(in2.Message) > 4000 {
			fail(w, 400, "The subject or message is too long.")
			return
		}
		dir := s.reportsDir()
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fail(w, 500, "The reports folder could not be made: "+err.Error())
			return
		}
		base := strings.TrimSpace(safeName.ReplaceAllString(rep.Title+" "+time.Now().Format("2006-01-02 1504"), ""))
		file := filepath.Join(dir, base+".html")
		if err := os.WriteFile(file, []byte(rep.html(true)), 0o644); err != nil {
			fail(w, 500, "The report could not be saved: "+err.Error())
			return
		}
		attach := file
		if pdf := file[:len(file)-5] + ".pdf"; htmlToPDF(file, pdf) == nil {
			attach = pdf
		}
		body := strings.TrimSpace(in2.Message)
		if body != "" {
			body += "\r\n\r\n"
		}
		body += rep.text()
		method, detail := composeEmail(addrs, subject, body, attach, dir)
		if detail != "" {
			s.logf("Outlook draft failed: %v", detail)
		}
		short := body
		if len(short) > 6000 {
			short = short[:6000]
		}
		writeJSON(w, 200, map[string]any{"method": method, "detail": detail, "text": short, "file": filepath.Base(attach), "folder": dir})
	})
}
