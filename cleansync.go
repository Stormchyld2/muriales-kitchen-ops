package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

var weekdayNames = []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}

// deepState says where a weekly deep-clean job stands. Each job belongs to one
// weekday: it is due on the most recent occurrence of that day until it is done.
func deepState(r rec, today time.Time) (due, state string) {
	if r["active"] == "No" {
		return "", "Inactive"
	}
	wd := -1
	for i, n := range weekdayNames {
		if strings.EqualFold(strings.TrimSpace(r["day"]), n) {
			wd = i
		}
	}
	if wd < 0 {
		return "", "As Needed"
	}
	back := (int(today.Weekday()) - wd + 7) % 7
	occ := today.AddDate(0, 0, -back).Format(dayFmt)
	t := today.Format(dayFmt)
	last := r["lastDone"]
	switch {
	case last >= occ && last != "":
		if last == t {
			return today.AddDate(0, 0, 7-back).Format(dayFmt), "Done Today"
		}
		return today.AddDate(0, 0, 7-back).Format(dayFmt), "Upcoming"
	case back == 0:
		return occ, "Due Today"
	case last == "":
		return today.AddDate(0, 0, 7-back).Format(dayFmt), "Upcoming"
	}
	return occ, "Overdue"
}

// decorate adds the computed due date and status to the schedule lists.
func (s *store) decorate(c *collDef, rows []rec) []rec {
	if c.Name == "training" {
		s.decorateTraining(rows)
		return rows
	}
	if c.Name == "prices" || c.Name == "specials" {
		s.decorateCosting(c, rows)
		return rows
	}
	if c.Name == "trainreq" {
		s.decorateTrainReq(rows)
		return rows
	}
	if c.Name == "mealrecipes" || c.Name == "mealplan" {
		s.decorateMeals(c, rows)
		return rows
	}
	today := time.Now().Format(dayFmt)
	tt, _ := time.Parse(dayFmt, today)
	for _, r := range rows {
		switch c.Name {
		case "cleantasks", "mgrchecks":
			r["due"], r["state"] = dueState(r, today)
		case "lotolog":
			r["state"] = map[bool]string{true: "Locked Out", false: "Removed"}[r["removed"] == ""]
		case "newhire":
			done := 0
			for _, l := range strings.Split(r["progress"], "\n") {
				if strings.TrimSpace(l) != "" {
					done++
				}
			}
			tot := 0
			for _, it := range s.regLoad(regDefs["newhireitems"]) {
				if it["active"] != "No" {
					tot++
				}
			}
			r["state"] = r["status"]
			if r["status"] != "Complete" {
				r["state"] = fmt.Sprintf("%d of %d done", done, tot)
			}
		case "deepclean":
			r["due"], r["state"] = deepState(r, tt)
		}
	}
	return rows
}

type overdueItem struct{ key, src, id, area, task, priority, who, due string }

// overdueItems lists everything on the cleaning side that is past due.
func (s *store) overdueItems() []overdueItem {
	today := time.Now().Format(dayFmt)
	tt, _ := time.Parse(dayFmt, today)
	var out []overdueItem
	for _, r := range s.regLoad(regDefs["cleantasks"]) {
		if due, st := dueState(r, today); st == "Overdue" {
			out = append(out, overdueItem{"cleantasks|" + r["id"] + "|" + due, "Cleaning Schedule", r["id"], r["area"], r["task"], r["priority"], r["role"], due})
		}
	}
	for _, r := range s.regLoad(regDefs["mgrchecks"]) {
		if due, st := dueState(r, today); st == "Overdue" {
			out = append(out, overdueItem{"mgrchecks|" + r["id"] + "|" + due, "Manager Inspection", r["id"], r["area"], r["check"], r["priority"], r["owner"], due})
		}
	}
	for _, r := range s.regLoad(regDefs["deepclean"]) {
		if due, st := deepState(r, tt); st == "Overdue" {
			out = append(out, overdueItem{"deepclean|" + r["id"] + "|" + due, "Weekly Deep Clean", r["id"], r["area"], r["task"], "P2 - Urgent", r["role"], due})
		}
	}
	for _, v := range s.pmList() {
		if v.State == "Overdue" {
			area := firstNonEmpty(v.Area, v.Group)
			out = append(out, overdueItem{"pm|" + v.ID + "|" + v.NextDue, v.Group + " job " + v.Code, v.Code, area, v.Name, "P3 - Routine", v.Performer, v.NextDue})
		}
	}
	return out
}

// syncMissed keeps the Missed & Overdue log fed from the schedules: each overdue
// item gets one open entry, and the entry is closed once the item is done.
// Callers hold s.mu.
func (s *store) syncMissed() {
	mc := regDefs["missed"]
	rows := s.regLoad(mc)
	items := s.overdueItems()
	live := map[string]bool{}
	for _, it := range items {
		live[it.key] = true
	}
	have := map[string]bool{}
	changed := false
	today := time.Now().Format(dayFmt)
	for _, r := range rows {
		k := r["key"]
		if k == "" {
			continue
		}
		have[k] = true
		if !live[k] && (r["status"] == "Open" || r["status"] == "In Progress") {
			r["status"], r["corrected"] = "Corrected", today
			changed = true
		}
	}
	for _, it := range items {
		if have[it.key] {
			continue
		}
		n := rec{"id": mc.nextID(rows), "created": time.Now().Format("2006-01-02 15:04"), "key": it.key, "date": today, "taskId": it.id, "area": it.area, "task": it.task,
			"reason": "Overdue: was due " + it.due + ".", "priority": it.priority, "assignedTo": it.who, "target": today, "status": "Open", "source": it.src}
		if n["priority"] == "" {
			n["priority"] = "P3 - Routine"
		}
		if n["area"] == "" {
			n["area"] = it.src
		}
		rows = append(rows, n)
		changed = true
	}
	if changed {
		_ = s.regSave(mc, rows)
	}
}

// logPMCleaning records a finished Cleaning-group PM job in the Completion Log.
func (s *store) logPMCleaning(t *PMTask, by, date, note string) {
	_, _ = s.regAdd(regDefs["cleanlog"], rec{"date": date, "taskId": t.Code, "area": firstNonEmpty(t.Area, "Cleaning"), "task": t.Name, "completedBy": by, "issue": "No", "verifyStatus": "Not required", "notes": note, "source": "PM job"})
}

func firstNonEmpty(v ...string) string {
	for _, x := range v {
		if strings.TrimSpace(x) != "" {
			return x
		}
	}
	return ""
}

func (s *store) cleanSyncRoutes(mux *http.ServeMux) {
	// A weekly deep-clean job is done: log it and move its date on.
	mux.HandleFunc("POST /api/clean/deepdone", func(w http.ResponseWriter, r *http.Request) {
		var in2 struct{ ID, By, Notes string }
		if jsonDecode(r, &in2) != nil || strings.TrimSpace(in2.By) == "" {
			fail(w, 400, "Enter your name or initials.")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		dc := regDefs["deepclean"]
		rows := s.regLoad(dc)
		for _, x := range rows {
			if x["id"] != in2.ID {
				continue
			}
			now := time.Now()
			if _, err := s.regAdd(regDefs["cleanlog"], rec{"date": now.Format(dayFmt), "taskId": x["id"], "area": x["area"], "task": x["task"], "completedBy": strings.TrimSpace(in2.By), "time": now.Format("3:04 PM"), "issue": "No", "verifyStatus": "Not required", "notes": in2.Notes, "source": "Weekly deep clean"}); err != nil {
				fail(w, 500, err.Error())
				return
			}
			x["lastDone"] = now.Format(dayFmt)
			if err := s.regSave(dc, rows); err != nil {
				fail(w, 500, err.Error())
				return
			}
			s.syncMissed()
			writeJSON(w, 200, map[string]any{"def": dc, "rows": s.decorate(dc, s.regSorted(dc))})
			return
		}
		fail(w, 404, "That job was not found.")
	})

	// Creates equipment records from the asset IDs in the maintenance workbook
	// and links the PM jobs to them.
	mux.HandleFunc("POST /api/maint/import-equipment", func(w http.ResponseWriter, r *http.Request) {
		raw, err := starterFiles.ReadFile("starter/equipment_import.json")
		if err != nil {
			fail(w, 500, "The equipment list is missing from this build.")
			return
		}
		var src []struct{ AssetCode, Name, Type, Area string }
		if json.Unmarshal(raw, &src) != nil {
			fail(w, 500, "The equipment list could not be read.")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		var eq []*Equipment
		s.readMaint("equipment.json", &eq)
		byCode := map[string]*Equipment{}
		for _, e := range eq {
			if e.AssetCode != "" {
				byCode[e.AssetCode] = e
			}
		}
		added := 0
		for _, c := range src {
			if byCode[c.AssetCode] != nil {
				continue
			}
			e := &Equipment{ID: newID(), Name: c.Name, Area: c.Area, Type: c.Type, AssetCode: c.AssetCode, Status: "Operational"}
			eq = append(eq, e)
			byCode[c.AssetCode] = e
			added++
		}
		var tasks []*PMTask
		s.readMaint("pm.json", &tasks)
		linked := 0
		for _, t := range tasks {
			if e := byCode[t.AssetCode]; t.EquipmentID == "" && t.AssetCode != "" && e != nil {
				t.EquipmentID = e.ID
				linked++
			}
		}
		sort.SliceStable(eq, func(i, j int) bool { return eq[i].AssetCode < eq[j].AssetCode })
		if err := s.writeMaint("equipment.json", eq); err != nil {
			s.maintFail(w, err)
			return
		}
		if linked > 0 {
			if err := s.writeMaint("pm.json", tasks); err != nil {
				s.maintFail(w, err)
				return
			}
		}
		writeJSON(w, 200, map[string]int{"added": added, "linked": linked})
	})
}
