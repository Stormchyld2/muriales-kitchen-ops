package main

// Inspection packet: one report that gathers the records an inspector asks for
// into a fixed layout of parts and sub-parts, for a date range. Parts that are
// records (temperatures, cleaning, training given) honor the dates; parts that
// are a standing picture (matrix, SDS library, equipment) show how things stand
// today.

import (
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"
)

type packetPart struct {
	Key, Group, Title string
	Build             func(s *store, from, to string) []reportSection
}

func inRange(d, from, to string) bool {
	if len(d) > 10 {
		d = d[:10]
	}
	return d != "" && d >= from && d <= to
}

func snap(id string) func(s *store, from, to string) []reportSection {
	return func(s *store, _, _ string) []reportSection {
		if r, ok := s.buildReport(id, ""); ok {
			return r.Sections
		}
		return nil
	}
}

func regRange(coll, title, key string) func(s *store, from, to string) []reportSection {
	return func(s *store, from, to string) []reportSection {
		r := s.regReport(regDefs[coll], title, "", func(x rec) bool { return inRange(x[key], from, to) })
		for i := range r.Sections {
			if r.Sections[i].Heading == "" {
				r.Sections[i].Heading = title
			}
		}
		return r.Sections
	}
}

func packetTemps(badOnly bool) func(s *store, from, to string) []reportSection {
	return func(s *store, from, to string) []reportSection {
		rows := [][]string{}
		for _, v := range s.tempViews(from, to) {
			if badOnly && v.Status != "Too Warm" && v.Status != "Too Cold" {
				continue
			}
			rows = append(rows, []string{v.At, v.Point, fmt.Sprintf("%.1f", v.Temp), v.Limit, v.Status, v.By, v.Action})
		}
		title, note := "Temperature log", "Every reading in the date range, with its safe range."
		if badOnly {
			title, note = "Out-of-range temperatures and what was done", "Readings outside the safe range in the date range, with the corrective action recorded."
		}
		return []reportSection{{Heading: title, Note: note, Cols: []string{"When", "Point", "Temp", "Safe range", "Status", "By", "Corrective action"}, Rows: rows}}
	}
}

func packetCA(s *store, from, to string) []reportSection {
	rows := [][]string{}
	for _, c := range s.caForms() {
		if inRange(c.Date, from, to) {
			rows = append(rows, []string{c.ID, c.Date, c.Area, c.Priority, c.Issue, c.RootCause, c.Required, c.Owner, c.Target, c.Status})
		}
	}
	return []reportSection{{Heading: "Corrective actions opened in the date range", Cols: []string{"ID", "Date", "Area", "Priority", "Issue", "Root cause", "Action required", "Owner", "Target", "Status"}, Rows: rows}}
}

var packetParts []packetPart

func init() {
	packetParts = []packetPart{
		{"temps", "Food Safety", "Temperature log", packetTemps(false)},
		{"tempsbad", "Food Safety", "Out-of-range temperatures", packetTemps(true)},
		{"ready", "Food Safety", "Inspection-ready check", snap("inspection-ready")},
		{"walks", "Food Safety", "Daily readiness walks", regRange("walks", "Daily readiness walks", "date")},
		{"clean", "Cleaning & Sanitation", "Cleaning completion log", regRange("cleanlog", "Cleaning completion log", "date")},
		{"missed", "Cleaning & Sanitation", "Missed and overdue cleaning", snap("clean-missed")},
		{"cstd", "Cleaning & Sanitation", "Cleaning standards", snap("clean-standards")},
		{"mgr", "Cleaning & Sanitation", "Manager inspection log", snap("mgr-log")},
		{"tmatrix", "Employee Training", "Training matrix", snap("training-matrix")},
		{"troles", "Employee Training", "Required training by position", snap("training-roles")},
		{"tlog", "Employee Training", "Training records in the date range", regRange("training", "Training records", "completed")},
		{"tdue", "Employee Training", "Training due, expired or missing", snap("training-due")},
		{"sds", "Chemical Safety & SDS", "SDS library", snap("sds-library")},
		{"sdsr", "Chemical Safety & SDS", "SDS needing attention", snap("sds-review")},
		{"chem", "Chemical Safety & SDS", "Chemical incident log", snap("chem-log")},
		{"cref", "Chemical Safety & SDS", "Chemical and tool reference", snap("clean-chem")},
		{"equip", "Equipment & Maintenance", "Equipment list", snap("equipment")},
		{"pm", "Equipment & Maintenance", "Equipment PM schedule", snap("pm-equipment")},
		{"issues", "Equipment & Maintenance", "Open maintenance issues", snap("issues")},
		{"oos", "Equipment & Maintenance", "Out-of-service history", snap("oos")},
		{"loto", "Equipment & Maintenance", "Lockout/tagout log", regRange("lotolog", "Lockout/tagout log", "date")},
		{"lotop", "Equipment & Maintenance", "Lockout/tagout equipment procedures", snap("reg:lotoprocs")},
		{"ca", "Corrective Actions & Safety Events", "Corrective actions", packetCA},
		{"inc", "Corrective Actions & Safety Events", "Incidents and near misses", regRange("incidents", "Incidents and near misses", "date")},
		{"mtg", "Corrective Actions & Safety Events", "Safety meetings", regRange("meetings", "Safety meetings", "date")},
		{"binder", "Emergency Preparedness", "Emergency response binder", snap("binder-all")},
	}
}

func packetKeys() []string {
	k := []string{}
	for _, p := range packetParts {
		k = append(k, p.Key)
	}
	return k
}

// packetReport reads ids shaped packet:<from>:<to>[:<keys, comma separated>].
func (s *store) packetReport(id string) (*report, bool) {
	f := strings.SplitN(strings.TrimPrefix(id, "packet:"), ":", 3)
	if len(f) < 2 {
		return nil, false
	}
	from, to := f[0], f[1]
	if _, e1 := time.Parse(dayFmt, from); e1 != nil {
		return nil, false
	}
	if _, e2 := time.Parse(dayFmt, to); e2 != nil || to < from {
		return nil, false
	}
	want := map[string]bool{}
	if len(f) == 3 && f[2] != "" {
		for _, k := range strings.Split(f[2], ",") {
			want[strings.TrimSpace(k)] = true
		}
	} else {
		for _, k := range packetKeys() {
			want[k] = true
		}
	}
	rep := &report{Title: "Health Inspection Packet", Subtitle: "Records from " + from + " to " + to + " · prepared " + time.Now().Format("January 2, 2006")}
	// the contents page lists what is inside, by part
	groups, order := map[string][]string{}, []string{}
	for _, p := range packetParts {
		if want[p.Key] {
			if _, ok := groups[p.Group]; !ok {
				order = append(order, p.Group)
			}
			groups[p.Group] = append(groups[p.Group], p.Title)
		}
	}
	var toc strings.Builder
	toc.WriteString(`<table class="toc">`)
	for i, g := range order {
		toc.WriteString(fmt.Sprintf("<tr><td><b>Part %d</b></td><td><b>%s</b></td></tr>", i+1, html.EscapeString(g)))
		for j, t := range groups[g] {
			toc.WriteString(fmt.Sprintf("<tr><td></td><td>%d.%d &nbsp; %s</td></tr>", i+1, j+1, html.EscapeString(t)))
		}
	}
	toc.WriteString(`</table><p><b>Prepared by:</b> ______________________ &nbsp; <b>Date:</b> ____________ &nbsp; <b>Reviewed by:</b> ______________________</p>`)
	rep.Sections = append(rep.Sections, reportSection{Heading: "Contents", Note: "Records that carry dates (temperatures, cleaning, training given, incidents, meetings) cover the range above. The rest shows how things stand on the day this was prepared.", HTML: toc.String()})
	cur, gi, pi := "", 0, 0
	for _, p := range packetParts {
		if !want[p.Key] {
			continue
		}
		if p.Group != cur {
			cur, gi, pi = p.Group, gi+1, 0
			rep.Sections = append(rep.Sections, reportSection{Heading: fmt.Sprintf("Part %d: %s", gi, p.Group), Group: true})
		}
		pi++
		secs := p.Build(s, from, to)
		if len(secs) == 0 {
			secs = []reportSection{{Note: "Nothing on record."}}
		}
		for k, sec := range secs {
			if k == 0 {
				sec.Heading = fmt.Sprintf("%d.%d  %s", gi, pi, p.Title)
			} else if sec.Heading != "" {
				sec.Heading = "   " + sec.Heading
			}
			rep.Sections = append(rep.Sections, sec)
		}
	}
	return rep, true
}

func (s *store) packetRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/packet/parts", func(w http.ResponseWriter, r *http.Request) {
		out := []map[string]any{}
		for _, p := range packetParts {
			out = append(out, map[string]any{"key": p.Key, "group": p.Group, "title": p.Title, "dated": p.Group != ""})
		}
		writeJSON(w, 200, map[string]any{"parts": out})
	})
}
