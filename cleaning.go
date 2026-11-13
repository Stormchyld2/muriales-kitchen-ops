package main

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

var freqDays = map[string]int{"Each Shift": 1, "Daily": 1, "Weekly": 7, "Biweekly": 14, "Monthly": 30, "Quarterly": 90, "Semi-Annual": 182, "Annual": 365}

// dueState says when a recurring item is next due. Items with no schedule
// ("As Needed") have no due date.
func dueState(r rec, today string) (due, state string) {
	if r["active"] == "No" {
		return "", "Inactive"
	}
	days := freqDays[r["frequency"]]
	if n, err := strconv.Atoi(strings.TrimSpace(r["cadence"])); err == nil && n > 0 {
		days = n
	}
	if days == 0 {
		return "", "As Needed"
	}
	last := r["lastDone"]
	if last == "" {
		return today, "Due Today"
	}
	t, err := time.Parse(dayFmt, last)
	if err != nil {
		return today, "Due Today"
	}
	due = t.AddDate(0, 0, days).Format(dayFmt)
	switch {
	case due < today:
		return due, "Overdue"
	case due == today:
		return due, "Due Today"
	case last == today:
		return due, "Done Today"
	}
	return due, "Upcoming"
}

func isDaily(r rec) bool {
	d := freqDays[r["frequency"]]
	if n, err := strconv.Atoi(strings.TrimSpace(r["cadence"])); err == nil && n > 0 {
		d = n
	}
	return d == 1
}

func (s *store) cleanToday() map[string]any {
	s.syncMissed()
	today := time.Now().Format(dayFmt)
	view := func(c *collDef) []rec {
		out := []rec{}
		for _, r := range s.regLoad(c) {
			due, st := dueState(r, today)
			if st == "Inactive" || st == "As Needed" {
				continue
			}
			v := rec{"due": due, "state": st}
			for k, x := range r {
				v[k] = x
			}
			out = append(out, v)
		}
		rank := map[string]int{"Overdue": 0, "Due Today": 1, "Upcoming": 2, "Done Today": 3}
		sort.SliceStable(out, func(i, j int) bool { return rank[out[i]["state"]] < rank[out[j]["state"]] })
		return out
	}
	pending := []rec{}
	for _, r := range s.regSorted(regDefs["cleanlog"]) {
		if r["verifyStatus"] == "Pending" {
			pending = append(pending, r)
		}
	}
	return map[string]any{"tasks": view(regDefs["cleantasks"]), "checks": view(regDefs["mgrchecks"]), "deep": s.deepToday(), "pending": pending, "today": today}
}

func (s *store) deepToday() []rec {
	tt, _ := time.Parse(dayFmt, time.Now().Format(dayFmt))
	out := []rec{}
	for _, r := range s.regLoad(regDefs["deepclean"]) {
		due, st := deepState(r, tt)
		if st == "Overdue" || st == "Due Today" || st == "Done Today" {
			v := rec{"due": due, "state": st}
			for k, x := range r {
				v[k] = x
			}
			out = append(out, v)
		}
	}
	return out
}

func (s *store) cleanTodo(from, to time.Time, want map[string]bool) []todoItem {
	today := time.Now().Format(dayFmt)
	out := []todoItem{}
	left := 0
	if want["Cleaning"] {
		for _, r := range s.regLoad(regDefs["cleantasks"]) {
			due, st := dueState(r, today)
			if st == "Inactive" || st == "As Needed" || st == "Done Today" {
				continue
			}
			if isDaily(r) {
				left++
				continue
			}
			if d, _ := time.Parse(dayFmt, due); st == "Upcoming" && d.After(to) {
				continue
			}
			state := st
			if st == "Upcoming" {
				state = "Upcoming"
			}
			out = append(out, todoItem{Group: "Cleaning", Title: r["area"] + ": " + r["task"], Detail: strings.Join(nonEmpty(r["frequency"], r["role"], r["timing"]), " · "), Due: due, State: state, Kind: "clean", ID: r["id"]})
		}
		s.syncMissed()
		for _, r := range s.regSorted(regDefs["missed"]) {
			if (r["status"] != "Open" && r["status"] != "In Progress") || r["key"] != "" {
				continue // overdue items already show as themselves
			}
			st := "Open"
			if strings.HasPrefix(r["priority"], "P1") || strings.HasPrefix(r["priority"], "P2") || strings.Contains(r["reason"], "Overdue") {
				st = "Overdue"
			}
			out = append(out, todoItem{Group: "Cleaning", Title: "Fix: " + r["area"] + ": " + r["task"], Detail: strings.Join(nonEmpty(r["reason"], r["assignedTo"], r["priority"]), " · "), Due: r["date"], State: st, Kind: "missed", ID: r["id"]})
		}
		if left > 0 {
			out = append(out, todoItem{Group: "Cleaning", Title: "Daily cleaning checklist: " + strconv.Itoa(left) + " task" + map[bool]string{true: "", false: "s"}[left == 1] + " to do", Detail: "Open Cleaning & Sanitation, Today's Checklist", Due: today, State: "Due Today", Kind: "cleanday", ID: "today"})
		}
	}
	if want["Manager Check"] {
		for _, r := range s.regLoad(regDefs["mgrchecks"]) {
			due, st := dueState(r, today)
			if st == "Inactive" || st == "As Needed" || st == "Done Today" {
				continue
			}
			if d, _ := time.Parse(dayFmt, due); st == "Upcoming" && d.After(to) {
				continue
			}
			out = append(out, todoItem{Group: "Manager Check", Title: r["check"], Detail: strings.Join(nonEmpty(r["frequency"], r["area"], r["timing"]), " · "), Due: due, State: st, Kind: "mgr", ID: r["id"]})
		}
	}
	return out
}

func (s *store) cleanReport(id string) (*report, bool) {
	today := time.Now().Format(dayFmt)
	switch id {
	case "clean-schedule":
		return s.regReport(regDefs["cleantasks"], "Cleaning Schedule", "", nil), true
	case "clean-standards":
		return s.regReport(regDefs["standards"], "Cleaning Standards", "", nil), true
	case "clean-chem":
		return s.regReport(regDefs["cleanref"], "Cleaning Chemical & Tool Reference", "", nil), true
	case "mgr-schedule":
		return s.regReport(regDefs["mgrchecks"], "Manager Inspection Schedule", "", nil), true
	case "clean-log":
		from := time.Now().AddDate(0, 0, -30).Format(dayFmt)
		return s.regReport(regDefs["cleanlog"], "Cleaning Completion Log (last 30 days)", "", func(r rec) bool { return r["date"] >= from }), true
	case "clean-missed":
		s.syncMissed()
		return s.regReport(regDefs["missed"], "Missed & Overdue Cleaning Log", "", nil), true
	case "clean-areas":
		return s.regReport(regDefs["cleanareas"], "Area Cleaning Matrix", "", nil), true
	case "mgr-log":
		from := time.Now().AddDate(0, 0, -30).Format(dayFmt)
		return s.regReport(regDefs["mgrlog"], "Manager Inspection Log (last 30 days)", "", func(r rec) bool { return r["date"] >= from }), true
	case "clean-weekly":
		rows := [][]string{}
		for _, w := range s.regLoad(regDefs["deepclean"]) {
			if w["active"] == "No" {
				continue
			}
			rows = append(rows, []string{w["day"], w["area"], w["task"], w["role"], "", "", "", ""})
			rows = append(rows, []string{w["day"], "Extra deep-clean job", "", "", "", "", "", ""})
		}
		return &report{Title: "Weekly Cleaning Schedule", Subtitle: "Week of: ______________   Kitchen manager: ______________   Posted: ____________   Week complete / verified by: ______________",
			Sections: []reportSection{{Note: "Post this on the line each week. Initial each job when it is done; the manager verifies at the end of the week.", Cols: []string{"Day", "Area / Equipment", "Cleaning Task", "Assigned To", "Completed", "Initials", "Verified By", "Notes"}, Rows: rows}}}, true
	case "linecook-weekly":
		return lineCookReport(today), true
	}
	return nil, false
}

func lineCookReport(string) *report {
	nightly := [][]string{
		{"☐", "Clean, degrease, and sanitize all work surfaces, cutting surfaces, and stainless at the station."},
		{"☐", "Clean cooking-equipment surfaces used during the shift, including tops, fronts, sides, handles, knobs, splash areas, and drip/crumb areas."},
		{"☐", "Empty the station rail of food as required; wipe and sanitize the rail, lids, ledges, and surrounding surfaces. Send removable pans/parts to dish."},
		{"☐", "Wipe lowboy/reach-in interiors where food or spills are present; clean door handles and gaskets; leave shelves organized."},
		{"☐", "Clean shelves, speed racks, seasoning/spice areas, heat-lamp/ticket-rail areas, and small equipment at the station."},
		{"☐", "Scrape/sweep all food debris from under and around the station; sweep and mop the station floor, including reachable areas under equipment."},
		{"☐", "Empty station trash as required and replace liners. Return chemicals/tools to the proper storage area."},
		{"☐", "Restock and organize the station only after cleaning is complete. No food, towels, pans, or utensils left on dirty surfaces."},
	}
	week := [][]string{
		{"MONDAY", "Normal Close / Detail Check", "• Complete the full nightly station close.\n• Wipe shelf fronts, container exteriors, handles, knobs, and other high-touch areas.\n• Reset and organize the station so Tuesday deep cleaning can be completed without clutter.", ""},
		{"TUESDAY", "Pull-Out & Under-Equipment Cleaning", "• Pull movable line equipment and lowboys only where it can be done safely.\n• Degrease and clean walls, stainless, casters/legs, sides of equipment, and floor underneath/behind.\n• Remove built-up food debris from corners and hard-to-reach floor edges before mopping.\n• Return equipment to its correct position and leave walkways clear.", ""},
		{"WEDNESDAY", "Hot-Line Equipment Detail", "• Detail range, sauté, grill/charbroiler, fryer, and hot-line surfaces used by the station.\n• Clean burner wells/top surfaces, splash guards, drip/crumb areas, fryer exteriors, baskets, and equipment fronts/sides.\n• Detail oven doors, handles, exterior surfaces, and accessible interior spills or debris.\n• Degrease the wall/stainless directly behind and beside hot-line equipment.", ""},
		{"THURSDAY", "Cold-Side / Refrigeration Detail", "• Empty and clean line rails, ledges, corners, lids, and removable components.\n• Detail lowboy/reach-in shelves, interior walls, door gaskets, handles, and exterior surfaces.\n• Clean underneath cutting-board edges/supports and other food-contact-adjacent areas missed during normal wiping.\n• Reorganize product neatly after cleaning; discard or report anything that should not remain in the station.", ""},
		{"FRIDAY", "Normal Close / Weekend Readiness", "• Complete the full nightly station close.\n• Give extra attention to high-touch areas, visible grease, spills, and floor edges.\n• Leave stations fully organized and ready for weekend volume; do not leave deep-clean work for the next crew.", ""},
		{"SATURDAY", "Shelves, Walls & Station Structure", "• Clean and degrease open shelving, shelf uprights, brackets, ledges, and rack surfaces on the line.\n• Wipe wall surfaces, stainless panels, corners, and vertical surfaces around each station.\n• Detail heat-lamp housings/exteriors, ticket rails, spice/seasoning racks, and other fixed station fixtures.\n• Clean exposed equipment legs/casters and floor edges after the upper surfaces are complete.", ""},
		{"SUNDAY", "Weekly Reset & Missed-Area Check", "• Complete the nightly close, then inspect the entire line for anything missed during the week.\n• Correct buildup behind/under accessible equipment, on shelves, refrigeration interiors, walls, and equipment sides/fronts.\n• Remove unnecessary containers, cardboard, towels, utensils, and clutter from line storage areas.\n• Leave every station clean, organized, stocked, and ready to begin the new week.", ""},
	}
	return &report{Title: "Line Cook Weekly Cleaning Schedule", Subtitle: "Line-cook responsibilities only • Complete during close unless directed otherwise", Sections: []reportSection{
		{Note: "STANDARD: Each cook is responsible for leaving their station clean, sanitized, organized, and ready for the next shift. Weekly assignments are in addition to the normal nightly close. Tasks assigned separately to Kitchen Operations or Maintenance are not part of this schedule."},
		{Heading: "Every Night: Each Line Cook / Station", Cols: []string{"", "Task"}, Rows: nightly},
		{Heading: "Weekly Detail Schedule", Cols: []string{"Day", "Focus", "Line Cook Responsibilities", "Init."}, Rows: week},
		{Heading: "Cleaning Expectations", Note: "• Cleaning is part of closing the station, not something left for the opening crew.\n• Use the correct chemical and follow label directions. Food and clean utensils must be protected before spraying or degreasing.\n• Do not move, disassemble, or service equipment beyond normal safe cleaning. Report damaged, unsafe, or malfunctioning equipment instead of attempting repairs.\n• If a scheduled task cannot be completed because of volume or an operational issue, notify the closing manager before leaving."},
	}}
}

func (s *store) cleanRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/clean/today", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		writeJSON(w, 200, s.cleanToday())
	})
	// Marks a cleaning task done and logs it. An issue found also goes in the missed log.
	mux.HandleFunc("POST /api/clean/done", func(w http.ResponseWriter, r *http.Request) {
		var in2 struct {
			ID, By, Shift, Notes string
			Issue                bool
		}
		if jsonDecode(r, &in2) != nil || strings.TrimSpace(in2.By) == "" {
			fail(w, 400, "Enter your name or initials.")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		tc := regDefs["cleantasks"]
		tasks := s.regLoad(tc)
		var task rec
		for _, t := range tasks {
			if t["id"] == in2.ID {
				task = t
			}
		}
		if task == nil {
			fail(w, 404, "That task was not found.")
			return
		}
		now := time.Now()
		vs := "Not required"
		if task["verify"] == "Yes" {
			vs = "Pending"
		}
		issue := "No"
		if in2.Issue {
			issue = "Yes"
		}
		shift := strings.TrimSpace(in2.Shift)
		if shift == "" {
			shift = task["timing"]
		}
		if _, err := s.regAdd(regDefs["cleanlog"], rec{"date": now.Format(dayFmt), "shift": shift, "taskId": task["id"], "area": task["area"], "task": task["task"], "completedBy": strings.TrimSpace(in2.By), "time": now.Format("3:04 PM"), "issue": issue, "verifyStatus": vs, "notes": in2.Notes}); err != nil {
			fail(w, 500, err.Error())
			return
		}
		if in2.Issue {
			reason := strings.TrimSpace(in2.Notes)
			if reason == "" {
				reason = "Issue found while cleaning."
			}
			if _, err := s.regAdd(regDefs["missed"], rec{"date": now.Format(dayFmt), "taskId": task["id"], "area": task["area"], "task": task["task"], "reason": reason, "priority": task["priority"], "assignedTo": task["role"], "status": "Open"}); err != nil {
				fail(w, 500, err.Error())
				return
			}
		}
		task["lastDone"] = now.Format(dayFmt)
		if err := s.regSave(tc, tasks); err != nil {
			fail(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, s.cleanToday())
	})
	mux.HandleFunc("POST /api/clean/verify", func(w http.ResponseWriter, r *http.Request) {
		var in2 struct {
			ID, By, Notes string
			OK            bool
		}
		if jsonDecode(r, &in2) != nil || strings.TrimSpace(in2.By) == "" {
			fail(w, 400, "Enter who is verifying.")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		lc := regDefs["cleanlog"]
		rows := s.regLoad(lc)
		found := false
		for _, x := range rows {
			if x["id"] != in2.ID {
				continue
			}
			found = true
			x["verifiedBy"], x["verifyTime"] = strings.TrimSpace(in2.By), time.Now().Format("3:04 PM")
			x["verifyStatus"] = map[bool]string{true: "Verified", false: "Failed"}[in2.OK]
			if in2.Notes != "" {
				x["notes"] = strings.TrimSpace(x["notes"] + " " + in2.Notes)
			}
			if !in2.OK {
				reason := in2.Notes
				if reason == "" {
					reason = "Failed verification."
				}
				if _, err := s.regAdd(regDefs["missed"], rec{"date": time.Now().Format(dayFmt), "taskId": x["taskId"], "area": x["area"], "task": x["task"], "reason": reason, "priority": "P2 - Urgent", "assignedTo": x["completedBy"], "status": "Open"}); err != nil {
					fail(w, 500, err.Error())
					return
				}
			}
		}
		if !found {
			fail(w, 404, "That log entry was not found.")
			return
		}
		if err := s.regSave(lc, rows); err != nil {
			fail(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, s.cleanToday())
	})
	mux.HandleFunc("POST /api/mgr/done", func(w http.ResponseWriter, r *http.Request) {
		var in2 struct{ ID, By, Result, Notes, FollowUp string }
		if jsonDecode(r, &in2) != nil || strings.TrimSpace(in2.By) == "" {
			fail(w, 400, "Enter your name.")
			return
		}
		if !oneOf([]string{"Pass", "Needs Attention", "N/A"}, in2.Result) {
			fail(w, 400, "Choose the result.")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		mc := regDefs["mgrchecks"]
		rows := s.regLoad(mc)
		var chk rec
		for _, x := range rows {
			if x["id"] == in2.ID {
				chk = x
			}
		}
		if chk == nil {
			fail(w, 404, "That inspection was not found.")
			return
		}
		ca := "No"
		if in2.Result == "Needs Attention" {
			ca = "Yes"
		}
		if _, err := s.regAdd(regDefs["mgrlog"], rec{"date": time.Now().Format(dayFmt), "inspectionId": chk["id"], "area": chk["area"], "check": chk["check"], "result": in2.Result, "by": strings.TrimSpace(in2.By), "caNeeded": ca, "followUp": strings.TrimSpace(in2.FollowUp), "notes": in2.Notes}); err != nil {
			fail(w, 500, err.Error())
			return
		}
		chk["lastDone"] = time.Now().Format(dayFmt)
		if err := s.regSave(mc, rows); err != nil {
			fail(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, s.cleanToday())
	})
}
