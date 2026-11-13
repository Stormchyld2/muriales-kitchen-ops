package main

import (
	"time"
)

// handoffReport is the manager handoff: the notes left for the next manager
// plus what is open in the kitchen right now. Callers hold s.mu.
func (s *store) handoffReport() *report {
	today := time.Now().Format("Monday, January 2, 2006")
	short := func(v string) string {
		if len(v) >= 10 {
			return v[:10]
		}
		return v
	}
	rep := &report{Title: "Manager Handoff", Subtitle: today}
	rows := [][]string{}
	for i := len(s.lib.Notes) - 1; i >= 0; i-- {
		n := s.lib.Notes[i]
		if !n.Done {
			rows = append(rows, []string{short(n.At), firstNonEmpty(n.By, "A manager"), n.Text})
		}
	}
	if len(rows) == 0 {
		rep.Sections = append(rep.Sections, reportSection{Heading: "Notes for the next manager", Note: "No open notes."})
	} else {
		rep.Sections = append(rep.Sections, reportSection{Heading: "Notes for the next manager", Cols: []string{"Date", "From", "Note"}, Rows: rows})
	}
	eq, is, _ := s.loadMaint()
	name := map[string]string{}
	for _, e := range eq {
		name[e.ID] = e.Name
	}
	oos := [][]string{}
	for _, e := range eq {
		if e.Status == "Out of Service" {
			oos = append(oos, []string{e.Name, e.Area})
		}
	}
	if len(oos) > 0 {
		rep.Sections = append(rep.Sections, reportSection{Heading: "Equipment out of service", Cols: []string{"Equipment", "Area"}, Rows: oos})
	}
	iss := [][]string{}
	for _, p := range is {
		if p.Status != "Verified" && p.Status != "Closed" {
			iss = append(iss, []string{name[p.EquipmentID], p.Title, p.Priority, p.Status, short(p.ReportedAt)})
		}
	}
	if len(iss) > 0 {
		rep.Sections = append(rep.Sections, reportSection{Heading: "Open maintenance issues", Cols: []string{"Equipment", "Issue", "Priority", "Status", "Reported"}, Rows: iss})
	}
	od := [][]string{}
	for _, v := range s.pmList() {
		if v.State == "Overdue" {
			od = append(od, []string{v.Name, firstNonEmpty(v.Area, v.Group), v.NextDue})
		}
	}
	if len(od) > 0 {
		rep.Sections = append(rep.Sections, reportSection{Heading: "Overdue maintenance and cleaning jobs", Cols: []string{"Job", "Area", "Was due"}, Rows: od})
	}
	s.syncMissed()
	ms := [][]string{}
	for _, r := range s.regSorted(regDefs["missed"]) {
		if r["status"] == "Open" || r["status"] == "In Progress" {
			ms = append(ms, []string{r["date"], r["area"], r["task"], r["reason"], r["status"]})
		}
	}
	if len(ms) > 0 {
		rep.Sections = append(rep.Sections, reportSection{Heading: "Missed and overdue cleaning", Cols: []string{"Date", "Area", "Task", "Reason", "Status"}, Rows: ms})
	}
	pend := [][]string{}
	for _, r := range s.regSorted(regDefs["cleanlog"]) {
		if r["verifyStatus"] == "Pending" {
			pend = append(pend, []string{r["date"], r["area"], r["task"], r["completedBy"]})
		}
	}
	if len(pend) > 0 {
		rep.Sections = append(rep.Sections, reportSection{Heading: "Cleaning waiting to be verified", Cols: []string{"Date", "Area", "Task", "Done by"}, Rows: pend})
	}
	return rep
}
