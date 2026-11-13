package main

import (
	"strconv"
	"strings"
	"time"
)

var todoFreqs = []string{"Daily", "Weekly", "Monthly", "Quarterly", "Semi-Annually", "Yearly"}

// freqName puts a repeat interval, in days, into one of the six handout groups.
func freqName(days int) string {
	switch {
	case days <= 1:
		return "Daily"
	case days <= 27:
		return "Weekly"
	case days <= 89:
		return "Monthly"
	case days <= 135:
		return "Quarterly"
	case days < 360:
		return "Semi-Annually"
	}
	return "Yearly"
}

func (t *PMTask) days() int {
	d := t.Every
	switch t.Unit {
	case "weeks":
		d *= 7
	case "months":
		d *= 30
	case "years":
		d *= 365
	}
	return d
}

func recDays(r rec) int {
	d := freqDays[r["frequency"]]
	if n, err := strconv.Atoi(strings.TrimSpace(r["cadence"])); err == nil && n > 0 {
		d = n
	}
	return d
}

func stateOf(due, today string) string {
	switch {
	case due == "":
		return "Open"
	case due < today:
		return "Overdue"
	case due == today:
		return "Due Today"
	}
	return "Upcoming"
}

// freqItems lists every recurring job of the chosen frequencies, whether or not
// it is due yet, so a whole schedule can be printed and handed out.
// Callers hold s.mu.
func (s *store) freqItems(freqs, want map[string]bool) []todoItem {
	today := time.Now().Format(dayFmt)
	tt, _ := time.Parse(dayFmt, today)
	out := []todoItem{}
	for _, v := range s.pmList() {
		if !want[v.Group] || v.Unit == "asneeded" || v.Every <= 0 {
			continue
		}
		f := freqName(v.days())
		if !freqs[f] {
			continue
		}
		out = append(out, todoItem{Group: v.Group, Title: v.Name, Detail: strings.Join(nonEmpty(v.Area, v.Performer), " · "), Due: v.NextDue, State: stateOf(v.NextDue, today), Bucket: f, Kind: "task", ID: v.ID})
	}
	if want["Cleaning"] {
		for _, r := range s.regLoad(regDefs["cleantasks"]) {
			due, st := dueState(r, today)
			d := recDays(r)
			if st == "Inactive" || st == "As Needed" || d == 0 || !freqs[freqName(d)] {
				continue
			}
			if st == "Done Today" {
				st = "Upcoming"
			}
			out = append(out, todoItem{Group: "Cleaning", Title: r["area"] + ": " + r["task"], Detail: strings.Join(nonEmpty(r["role"], r["timing"]), " · "), Due: due, State: st, Bucket: freqName(d), Kind: "clean", ID: r["id"]})
		}
		for _, r := range s.regLoad(regDefs["deepclean"]) {
			due, st := deepState(r, tt)
			if st == "Inactive" || !freqs["Weekly"] {
				continue
			}
			out = append(out, todoItem{Group: "Cleaning", Title: r["day"] + " deep clean, " + r["area"] + ": " + r["task"], Detail: r["role"], Due: due, State: st, Bucket: "Weekly", Kind: "deep", ID: r["id"]})
		}
	}
	if want["Manager Check"] {
		for _, r := range s.regLoad(regDefs["mgrchecks"]) {
			due, st := dueState(r, today)
			d := recDays(r)
			if st == "Inactive" || st == "As Needed" || d == 0 || !freqs[freqName(d)] {
				continue
			}
			out = append(out, todoItem{Group: "Manager Check", Title: r["check"], Detail: strings.Join(nonEmpty(r["area"], r["owner"], r["timing"]), " · "), Due: due, State: st, Bucket: freqName(d), Kind: "mgr", ID: r["id"]})
		}
	}
	return out
}

// withFrequencies builds the full handout: the chosen recurring jobs plus
// everything else that is open right now.
func (s *store) withFrequencies(freqs, want map[string]bool) []todoItem {
	t0, _ := time.Parse(dayFmt, time.Now().Format(dayFmt))
	out := []todoItem{}
	for _, it := range s.todoItems(t0, t0.AddDate(0, 0, 364), want) {
		switch it.Kind {
		case "task", "clean", "cleanday", "mgr":
			continue
		}
		out = append(out, it)
	}
	return append(s.freqItems(freqs, want), out...)
}
