package main

// Dashboard extras: today's task progress, operational alerts and 30 day key
// metrics. All worked out live from the records. Callers hold s.mu.

import (
	"strings"
	"time"
)

type dashAlert struct {
	Level string `json:"level"` // bad, warn
	Text  string `json:"text"`
	Sub   string `json:"sub"`
	View  string `json:"view"`
}

type dashMetric struct {
	Label string `json:"label"`
	Now   int    `json:"now"`
	Prev  int    `json:"prev"`
	View  string `json:"view"`
}

type dashPart struct {
	Label string `json:"label"`
	Done  int    `json:"done"`
	Total int    `json:"total"`
	View  string `json:"view"`
}

func (s *store) dashAlerts(today string, bwhen time.Time, hasBackup bool) []dashAlert {
	out := []dashAlert{}
	eq, is, _ := s.loadMaint()
	for _, e := range eq {
		if e.Status == "Out of Service" {
			out = append(out, dashAlert{"bad", e.Name + " is out of service", strings.TrimSpace(e.Area), "equip"})
		}
	}
	for _, p := range is {
		if p.Status == "Verified" || p.Status == "Closed" {
			continue
		}
		if p.Priority == "High" || p.Priority == "Urgent" {
			out = append(out, dashAlert{"bad", p.Title, p.Priority + " priority issue, " + p.Status, "issues"})
		}
	}
	for _, r := range s.tempViews(today, today) {
		if r.Status == "Too Warm" || r.Status == "Too Cold" {
			out = append(out, dashAlert{"bad", r.Point + " is " + strings.ToLower(r.Status), "Today's temperature check", "temps"})
		}
	}
	for _, c := range s.caForms() {
		if c.Status != "Closed" && c.Target != "" && c.Target < today {
			out = append(out, dashAlert{"warn", "Corrective action " + c.ID + " is past its target date", firstNonEmpty(c.Area, c.Owner), "calog"})
		}
	}
	tm := s.trainingMatrix()
	if n := tm.Count["Expired"] + tm.Count["Missing"]; n > 0 {
		out = append(out, dashAlert{"warn", plural(n, "training is expired or missing", "trainings are expired or missing"), "Employee Training Matrix", "trmatrix"})
	}
	if !hasBackup || time.Since(bwhen) > 48*time.Hour {
		out = append(out, dashAlert{"warn", "No recent backup", "A backup is made each day the program runs", "settings"})
	}
	if len(out) > 12 {
		out = out[:12]
	}
	return out
}

// dashMetrics counts the last 30 days against the 30 before.
func (s *store) dashMetrics(now time.Time) []dashMetric {
	d := func(n int) string { return now.AddDate(0, 0, -n).Format(dayFmt) }
	from, mid, to := d(59), d(29), now.Format(dayFmt)
	split := func(date string) (cur, prev int) {
		date = firstNonEmpty(date, "")
		if len(date) > 10 {
			date = date[:10]
		}
		switch {
		case date >= mid && date <= to:
			return 1, 0
		case date >= from && date < mid:
			return 0, 1
		}
		return 0, 0
	}
	count := func(coll, key string) (int, int) {
		a, b := 0, 0
		for _, r := range s.regLoad(regDefs[coll]) {
			c, p := split(r[key])
			a, b = a+c, b+p
		}
		return a, b
	}
	m := []dashMetric{}
	add := func(label, view string, n, p int) { m = append(m, dashMetric{label, n, p, view}) }
	n, p := count("cleanlog", "date")
	add("Cleaning tasks done", "cllog", n, p)
	tn, tp := 0, 0
	for _, r := range s.tempViews(from, to) {
		if r.Status == "Not Checked" {
			continue
		}
		c, pv := split(r.At)
		tn, tp = tn+c, tp+pv
	}
	add("Temperatures logged", "temps", tn, tp)
	_, is, _ := s.loadMaint()
	in, ip := 0, 0
	for _, x := range is {
		c, pv := split(x.ReportedAt)
		in, ip = in+c, ip+pv
	}
	add("Problems reported", "issues", in, ip)
	cn, cp := 0, 0
	for _, c := range s.caForms() {
		a, b := split(c.Date)
		cn, cp = cn+a, cp+b
	}
	add("Corrective actions", "calog", cn, cp)
	n, p = count("incidents", "date")
	add("Incidents & near misses", "inc", n, p)
	n, p = count("meetings", "date")
	add("Safety meetings held", "mtg", n, p)
	n, p = count("training", "completed")
	add("Trainings recorded", "trrec", n, p)
	n, p = count("mealplan", "date")
	add("Employee meals logged", "mplan", n, p)
	return m
}
