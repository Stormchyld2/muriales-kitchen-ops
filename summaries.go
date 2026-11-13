package main

// Two reports worked out live from everything the program holds:
//   - the Inspection-Ready Checklist: what the records show right now, plus
//     the walk-through points to check by eye, with a sign-off line;
//   - the Weekly Summary: the last seven days at a glance.
// Callers hold s.mu.

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

func within(v, from string) bool { return len(v) >= 10 && v[:10] >= from }

func okOr(c bool, a, b string) string {
	if c {
		return a
	}
	return b
}

func okText(ok bool, good, bad string) string {
	if ok {
		return good
	}
	return bad
}

func (s *store) inspectionReadyReport() *report {
	now := time.Now()
	today := now.Format(dayFmt)
	week := now.AddDate(0, 0, -6).Format(dayFmt)
	eq, is, _ := s.loadMaint()
	rows := [][]string{}
	add := func(check, result, detail string) { rows = append(rows, []string{check, result, detail}) }

	// temperatures
	pts := s.tempPoints()
	readings := s.tempViews(week, "")
	seenToday := map[string]bool{}
	bad, noAction := 0, 0
	for _, r := range readings {
		if within(r.At, today) {
			seenToday[r.Point] = true
		}
		if r.Status == "Too Warm" || r.Status == "Too Cold" {
			bad++
			if strings.TrimSpace(r.Action) == "" {
				noAction++
			}
		}
	}
	if len(pts) == 0 {
		add("Temperatures taken today", "SET UP", "No temperature points are set up yet.")
	} else {
		missing := []string{}
		for _, p := range pts {
			if !seenToday[p.Name] {
				missing = append(missing, p.Name)
			}
		}
		detail := fmt.Sprintf("%d of %d points have a reading today.", len(pts)-len(missing), len(pts))
		if len(missing) > 0 && len(missing) <= 6 {
			detail += " Not yet: " + strings.Join(missing, ", ") + "."
		}
		add("Temperatures taken today", okText(len(missing) == 0, "READY", "CHECK"), detail)
	}
	add("Out-of-range temperatures have a corrective action (last 7 days)", okText(noAction == 0, "READY", "FIX"),
		fmt.Sprintf("%d out of range, %d with no action written.", bad, noAction))

	// corrective actions
	open, late := 0, 0
	for _, c := range s.caForms() {
		if c.Status == "Closed" {
			continue
		}
		open++
		if c.Target != "" && c.Target < today {
			late++
		}
	}
	add("Corrective actions closed or on time", okText(late == 0, "READY", "FIX"), fmt.Sprintf("%d open, %d past their target date.", open, late))

	// cleaning and manager checks
	cl, mg, dp := 0, 0, 0
	for _, o := range s.overdueItems() {
		switch o.src {
		case "Cleaning Schedule":
			cl++
		case "Manager Inspection":
			mg++
		case "Weekly Deep Clean":
			dp++
		}
	}
	add("Cleaning schedule up to date", okText(cl == 0, "READY", "FIX"), fmt.Sprintf("%d cleaning tasks overdue.", cl))
	add("Weekly deep clean up to date", okText(dp == 0, "READY", "FIX"), fmt.Sprintf("%d deep-clean jobs overdue.", dp))
	add("Manager inspections up to date", okText(mg == 0, "READY", "FIX"), fmt.Sprintf("%d manager checks overdue.", mg))
	pend := 0
	for _, r := range s.regLoad(regDefs["cleanlog"]) {
		if r["verifyStatus"] == "Pending" {
			pend++
		}
	}
	add("Completed cleaning has been verified", okText(pend == 0, "READY", "CHECK"), fmt.Sprintf("%d entries waiting to be verified.", pend))

	// equipment
	oos := 0
	for _, e := range eq {
		if e.Status == "Out of Service" {
			oos++
		}
	}
	add("No equipment out of service", okText(oos == 0, "READY", "CHECK"), fmt.Sprintf("%d units out of service.", oos))
	od := 0
	for _, v := range s.pmList() {
		if v.State == "Overdue" {
			od++
		}
	}
	add("Maintenance jobs up to date", okText(od == 0, "READY", "FIX"), fmt.Sprintf("%d jobs overdue (equipment, grounds and cleaning).", od))
	urgent, openIssues := 0, 0
	for _, p := range is {
		if p.Status == "Verified" || p.Status == "Closed" {
			continue
		}
		openIssues++
		if p.Priority == "High" || p.Priority == "Urgent" {
			urgent++
		}
	}
	add("No urgent maintenance issues open", okText(urgent == 0, "READY", "FIX"), fmt.Sprintf("%d open issues, %d high or urgent.", openIssues, urgent))

	// SDS
	badSDS := s.integrityProblems()
	needs := 0
	for _, p := range s.lib.Products {
		if p.Missing || badSDS[p.ID] == "changed" || p.NeedsReview || p.SummaryNeedsCheck {
			needs++
		}
	}
	add("Safety data sheets complete and checked", okText(needs == 0, "READY", "FIX"), fmt.Sprintf("%d products on file, %d need attention.", len(s.lib.Products), needs))

	// staff training
	tm := s.trainingMatrix()
	if len(tm.Rows) == 0 {
		add("Staff training current", "SET UP", "No employees are on the training list yet.")
	} else {
		bad := tm.Count["Expired"] + tm.Count["Missing"]
		add("Staff training current", okText(bad == 0 && tm.Count["Due Soon"] == 0, "READY", okOr(bad > 0, "FIX", "CHECK")),
			fmt.Sprintf("%d expired, %d never done, %d due within 30 days.", tm.Count["Expired"], tm.Count["Missing"], tm.Count["Due Soon"]))
	}

	// last readiness walk
	last := ""
	list := s.readinessList()
	sort.SliceStable(list, func(i, j int) bool { return list[i].Date > list[j].Date })
	if len(list) == 0 {
		add("Self-inspection walk-through done recently", "CHECK", "No walk-through has been recorded yet.")
	} else {
		r := list[0]
		last = r.Date
		recent := r.Date >= now.AddDate(0, 0, -30).Format(dayFmt)
		add("Self-inspection walk-through done recently", okText(recent && r.open() == 0, "READY", "CHECK"),
			fmt.Sprintf("Last walk %s by %s, %d points still need attention.", r.Date, firstNonEmpty(r.Manager, "unknown"), r.open()))
	}
	_ = last

	ready, fix := 0, 0
	for _, r := range rows {
		switch r[1] {
		case "READY":
			ready++
		case "FIX", "CHECK", "SET UP":
			fix++
		}
	}
	rep := &report{Title: "Inspection-Ready Checklist", Subtitle: now.Format("Monday, January 2, 2006 3:04 PM")}
	rep.Sections = append(rep.Sections, reportSection{
		Heading: "What the records show right now",
		Note:    fmt.Sprintf("%d of %d checks are ready, %d need attention. READY = nothing outstanding. FIX = something is overdue or missing. CHECK = look at it before the inspector does.", ready, len(rows), fix),
		Cols:    []string{"Check", "Result", "Detail"}, Rows: rows})

	walk := [][]string{}
	for i, p := range readyPoints {
		walk = append(walk, []string{"[  ]", strconv.Itoa(i + 1), p[0], p[1], ""})
	}
	rep.Sections = append(rep.Sections, reportSection{
		Heading: "Walk-through: tick each point as you check it",
		Note:    "Records cannot show these. Walk the kitchen and correct what you can on the spot. Record anything unresolved in a Corrective Action.",
		Cols:    []string{"Done", "#", "Area", "Inspection Point", "Finding / Fix"}, Rows: walk})
	rep.Sections = append(rep.Sections, reportSection{
		Heading: "Sign-off",
		Note:    "Manager: ______________________    Date: ______________    Ready for inspection?  Yes  /  No"})
	return rep
}

func (s *store) weeklySummaryReport() *report {
	now := time.Now()
	from := now.AddDate(0, 0, -6).Format(dayFmt)
	today := now.Format(dayFmt)
	eq, is, _ := s.loadMaint()
	name := map[string]string{}
	for _, e := range eq {
		name[e.ID] = e.Name
	}
	rep := &report{Title: "Weekly Summary", Subtitle: from + " to " + today}
	s.syncMissed()

	// temperatures
	readings := s.tempViews(from, "")
	bad, acted := 0, 0
	var badRows [][]string
	for _, r := range readings {
		if r.Status == "Too Warm" || r.Status == "Too Cold" {
			bad++
			if strings.TrimSpace(r.Action) != "" {
				acted++
			}
			badRows = append(badRows, []string{when12ish(r.At), r.Point, fmt.Sprintf("%.1f°F", r.Temp), r.Limit, r.Status, r.By, strings.TrimSpace(r.Notes + " " + r.Action)})
		}
	}
	// cleaning
	done, pend := 0, 0
	for _, r := range s.regLoad(regDefs["cleanlog"]) {
		if within(r["date"], from) {
			done++
			if r["verifyStatus"] == "Pending" {
				pend++
			}
		}
	}
	missedNew, missedOpen := 0, 0
	for _, r := range s.regLoad(regDefs["missed"]) {
		if within(r["date"], from) {
			missedNew++
		}
		if r["status"] == "Open" || r["status"] == "In Progress" {
			missedOpen++
		}
	}
	// maintenance
	pmDone, pmOver := 0, 0
	for _, v := range s.pmList() {
		if within(v.LastDone, from) {
			pmDone++
		}
		if v.State == "Overdue" {
			pmOver++
		}
	}
	opened, closed, stillOpen := 0, 0, 0
	for _, p := range is {
		if within(p.ReportedAt, from) {
			opened++
		}
		if within(p.CompletedAt, from) || within(p.VerifiedAt, from) {
			closed++
		}
		if p.Status != "Verified" && p.Status != "Closed" {
			stillOpen++
		}
	}
	oos := 0
	for _, e := range eq {
		if e.Status == "Out of Service" {
			oos++
		}
	}
	// corrective actions and chemicals
	caNew, caClosed, caOpen := 0, 0, 0
	for _, c := range s.caForms() {
		if within(c.Date, from) {
			caNew++
		}
		if c.Status == "Closed" && within(c.ClosedDate, from) {
			caClosed++
		}
		if c.Status != "Closed" {
			caOpen++
		}
	}
	chem := 0
	for _, c := range s.chemIncidents() {
		if within(c.At, from) {
			chem++
		}
	}
	walks := 0
	for _, r := range s.readinessList() {
		if within(r.Date, from) {
			walks++
		}
	}
	tm := s.trainingMatrix()
	trainDone := 0
	for _, r := range s.regLoad(regDefs["training"]) {
		if within(r["completed"], from) {
			trainDone++
		}
	}
	n := func(i int) string { return strconv.Itoa(i) }
	rep.Sections = append(rep.Sections, reportSection{Heading: "The week at a glance", Cols: []string{"Area", "Measure", "Count"}, Rows: [][]string{
		{"Food safety", "Temperature readings taken", n(len(readings))},
		{"Food safety", "Readings out of range", n(bad)},
		{"Food safety", "Out-of-range readings with a corrective action written", n(acted)},
		{"Food safety", "Corrective action forms opened", n(caNew)},
		{"Food safety", "Corrective action forms closed", n(caClosed)},
		{"Food safety", "Corrective actions still open now", n(caOpen)},
		{"Food safety", "Chemical incidents", n(chem)},
		{"Food safety", "Self-inspection walk-throughs", n(walks)},
		{"Cleaning", "Cleaning entries completed", n(done)},
		{"Cleaning", "Completed entries waiting to be verified", n(pend)},
		{"Cleaning", "Missed or overdue items logged this week", n(missedNew)},
		{"Cleaning", "Missed or overdue items still open now", n(missedOpen)},
		{"Maintenance", "Recurring jobs completed", n(pmDone)},
		{"Maintenance", "Recurring jobs overdue now", n(pmOver)},
		{"Maintenance", "Issues reported", n(opened)},
		{"Maintenance", "Issues repaired or verified", n(closed)},
		{"Maintenance", "Issues still open now", n(stillOpen)},
		{"Maintenance", "Equipment out of service now", n(oos)},
		{"Training", "Trainings completed this week", n(trainDone)},
		{"Training", "Expired or never done (required)", n(tm.Count["Expired"] + tm.Count["Missing"])},
		{"Training", "Due within 30 days", n(tm.Count["Due Soon"])},
	}})
	if len(badRows) > 0 {
		rep.Sections = append(rep.Sections, reportSection{Heading: "Out-of-range temperatures this week", Cols: []string{"When", "Point", "Temp", "Safe Range", "Status", "By", "Notes / Action"}, Rows: badRows})
	}
	var od [][]string
	for _, v := range s.pmList() {
		if v.State == "Overdue" {
			od = append(od, []string{v.Group, v.Name, firstNonEmpty(name[v.EquipmentID], v.Area), v.NextDue})
		}
	}
	if len(od) > 0 {
		rep.Sections = append(rep.Sections, reportSection{Heading: "Overdue jobs", Cols: []string{"Group", "Job", "Equipment / Area", "Was due"}, Rows: od})
	}
	var oi [][]string
	for _, p := range is {
		if p.Status != "Verified" && p.Status != "Closed" {
			oi = append(oi, []string{name[p.EquipmentID], p.Title, p.Priority, p.Status, p.ReportedAt})
		}
	}
	if len(oi) > 0 {
		rep.Sections = append(rep.Sections, reportSection{Heading: "Open maintenance issues", Cols: []string{"Equipment", "Issue", "Priority", "Status", "Reported"}, Rows: oi})
	}
	rep.Sections = append(rep.Sections, reportSection{Heading: "Manager notes", Note: "Highlights: \n\nConcerns: \n\nActions for next week: "})
	return rep
}

func when12ish(at string) string {
	if t, err := time.Parse("2006-01-02 15:04", at); err == nil {
		return t.Format("Mon Jan 2, 3:04 PM")
	}
	return at
}

// weeklyReviewReport is the Monday report: where the kitchen stands against
// the inspection checks, the week just finished, and the notes still open.
func (s *store) weeklyReviewReport() *report {
	now := time.Now()
	rep := &report{Title: "Weekly Review", Subtitle: now.Format("Monday, January 2, 2006")}
	ready := s.inspectionReadyReport()
	if len(ready.Sections) > 0 {
		sec := ready.Sections[0]
		sec.Heading = "Inspection readiness: what the records show right now"
		rep.Sections = append(rep.Sections, sec)
	}
	for _, sec := range s.weeklySummaryReport().Sections {
		rep.Sections = append(rep.Sections, sec)
	}
	for _, sec := range s.handoffReport().Sections {
		rep.Sections = append(rep.Sections, sec)
	}
	rep.Sections = append(rep.Sections, reportSection{Heading: "Reviewed by",
		Note: "Manager: ______________________    Date: ______________"})
	return rep
}
