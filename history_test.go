package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// historyEnv is the usual test setup with the edit-history wrapper in front.
func historyEnv(t *testing.T) *env {
	e := newEnv(t)
	e.h = e.s.history(e.h)
	return e
}

func TestEditHistory(t *testing.T) {
	e := historyEnv(t)
	c, _ := e.post("/api/reg/employees", `{"name":"Ana","position":"Cook","department":"Prep","status":"Active"}`)
	if c != 200 {
		t.Fatalf("employee refused: %d", c)
	}
	if c, _ := e.post("/api/reg/employees", `{"name":"","position":"x"}`); c == 200 {
		t.Fatal("a nameless employee should be refused")
	}
	var id string
	for _, r := range e.s.regLoad(regDefs["employees"]) {
		id = r["id"]
	}
	e.post("/api/reg/employees", `{"id":"`+id+`","name":"Ana","position":"Sous Chef","department":"Prep","status":"Active"}`)
	e.post("/api/reg/cleantasks", `{"type":"Area","area":"Walk-in","task":"Wipe shelves","frequency":"Daily","priority":"P3 - Routine"}`)
	var tid string
	for _, r := range e.s.regLoad(regDefs["cleantasks"]) {
		tid = r["id"]
	}
	if c, _ := e.do2("DELETE", "/api/reg/employees/"+id, ""); c == 200 {
		t.Fatal("employees are kept, not deleted")
	}
	e.do2("DELETE", "/api/reg/cleantasks/"+tid, "")
	e.post("/api/maint/todos", `{"title":"Call hood cleaner","createdBy":"Brian"}`)

	rows, total := e.s.historyRows("", 50)
	if total != 5 {
		t.Fatalf("expected 5 history rows (the refused ones must not count), got %d: %+v", total, rows)
	}
	// newest first: to-do, delete, add task, edit, add
	if rows[0].Who != "Brian" || rows[0].Kind != "To-do item" || rows[0].Label != "Call hood cleaner" {
		t.Errorf("to-do row wrong: %+v", rows[0])
	}
	if rows[1].Action != "Deleted" || !strings.Contains(rows[1].Detail, "Wipe shelves") || rows[1].Label != "Wipe shelves" {
		t.Errorf("delete row should keep the record: %+v", rows[1])
	}
	if rows[3].Action != "Changed" || !strings.Contains(rows[3].Detail, "Cook → Sous Chef") {
		t.Errorf("edit row should say what changed: %+v", rows[2])
	}
	if rows[4].Action != "Added" || rows[4].Kind != "Employees" {
		t.Errorf("add row wrong: %+v", rows[4])
	}
	if _, n := e.s.historyRows("hood", 50); n != 1 {
		t.Errorf("search should find one row, got %d", n)
	}
	code, out := e.get2("/api/history?limit=2")
	if code != 200 || len(out["rows"].([]any)) != 2 || out["total"].(float64) != 5 {
		t.Errorf("history endpoint: %v", out)
	}
	if code, _ := e.get2("/api/history"); code != 200 {
		t.Error("history should be readable")
	}
}

func TestDailyBackup(t *testing.T) {
	e := newEnv(t)
	e.s.dailyBackup()
	name := filepath.Join(e.s.backupDir(), "Daily Backup "+time.Now().Format(dayFmt)+".zip")
	st, err := os.Stat(name)
	if err != nil {
		t.Fatalf("daily backup not made: %v", err)
	}
	first := st.ModTime()
	e.s.dailyBackup()
	st2, _ := os.Stat(name)
	if !st2.ModTime().Equal(first) {
		t.Error("a second call on the same day must not make another backup")
	}
	if backupKind(filepath.Base(name)) != "full" {
		t.Error("a daily backup should restore as a full backup")
	}
	if n, when := e.s.lastBackup(); n == "" || time.Since(when) > time.Minute {
		t.Errorf("last backup not seen: %q", n)
	}
	// old dailies are pruned to the last two weeks
	for i := 1; i <= keepDailyZips+3; i++ {
		p := filepath.Join(e.s.backupDir(), "Daily Backup 2020-01-"+twoDigits(i)+".zip")
		_ = os.WriteFile(p, []byte("x"), 0o644)
		old := time.Now().AddDate(0, 0, -100+i)
		_ = os.Chtimes(p, old, old)
	}
	_ = os.Remove(name)
	e.s.dailyBackup()
	n := 0
	entries, _ := os.ReadDir(e.s.backupDir())
	for _, f := range entries {
		if strings.HasPrefix(f.Name(), "Daily Backup ") {
			n++
		}
	}
	if n != keepDailyZips {
		t.Errorf("expected %d daily backups kept, found %d", keepDailyZips, n)
	}
}

func twoDigits(i int) string {
	if i < 10 {
		return "0" + itoa(i)
	}
	return itoa(i)
}

func TestDashboard(t *testing.T) {
	e := newEnv(t)
	code, out := e.get2("/api/dashboard")
	if code != 200 {
		t.Fatalf("dashboard: %d", code)
	}
	g := out["gauges"].([]any)
	keys := []string{}
	for _, x := range g {
		keys = append(keys, x.(map[string]any)["key"].(string))
	}
	if strings.Join(keys, ",") != "ready,temps,clean,pm,train" {
		t.Errorf("gauges: %v", keys)
	}
	if len(out["tiles"].([]any)) != 5 {
		t.Errorf("tiles: %v", out["tiles"])
	}
	// a late job shows up as overdue and in the attention list
	d := func(n int) string { return time.Now().AddDate(0, 0, n).Format(dayFmt) }
	e.post("/api/maint/pm", `{"group":"Cleaning","name":"Late job","every":1,"unit":"months","firstDue":"`+d(-5)+`"}`)
	_, out = e.get2("/api/dashboard")
	if out["overdue"].(float64) < 1 {
		t.Errorf("overdue not counted: %v", out["overdue"])
	}
	found := false
	for _, x := range out["attention"].([]any) {
		if x.(map[string]any)["title"] == "Late job" {
			found = true
		}
	}
	if !found {
		t.Error("the late job should be listed under needs attention")
	}
	for _, x := range out["gauges"].([]any) {
		m := x.(map[string]any)
		if m["key"] == "pm" && m["pct"].(float64) >= 100 {
			t.Errorf("maintenance gauge should drop below 100 with a late job: %v", m)
		}
	}
	// no employees yet: the training gauge says not set up
	for _, x := range out["gauges"].([]any) {
		m := x.(map[string]any)
		if m["key"] == "train" && m["pct"].(float64) != -1 {
			t.Errorf("training gauge should be 'not set up': %v", m)
		}
	}
}

func TestWeeklyReviewAndOwners(t *testing.T) {
	e := newEnv(t)
	e.post("/api/maint/notes", `{}`)
	e.s.mu.Lock()
	rep := e.s.weeklyReviewReport()
	e.s.mu.Unlock()
	heads := []string{}
	for _, s := range rep.Sections {
		heads = append(heads, s.Heading)
	}
	all := strings.Join(heads, "|")
	for _, want := range []string{"Inspection readiness", "Notes for the next manager", "Reviewed by"} {
		if !strings.Contains(all, want) {
			t.Errorf("weekly review is missing %q: %v", want, heads)
		}
	}
	if code, _ := e.get2("/api/reports/weekly-review"); code != 200 {
		t.Errorf("weekly review should be a report: %d", code)
	}
	// owners come from the employee list, not from a name written into the program
	e.post("/api/reg/employees", `{"name":"Brian Smith","position":"Lead Steward","department":"Dish","status":"Active"}`)
	e.post("/api/reg/employees", `{"name":"Gone Guy","position":"Cook","department":"Prep","status":"Inactive"}`)
	e.s.mu.Lock()
	owners := strings.Join(e.s.readyOwners(), ",")
	e.s.mu.Unlock()
	if !strings.Contains(owners, "Brian") || strings.Contains(owners, "Gone") || !strings.HasSuffix(owners, "Other") {
		t.Errorf("owners: %s", owners)
	}
}
