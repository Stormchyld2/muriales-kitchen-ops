package main

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMaintenanceFlow(t *testing.T) {
	e := newEnv(t)
	if c, _ := e.post("/api/maint/equipment", `{"name":" "}`); c != 400 {
		t.Error("a nameless equipment record should be refused")
	}
	_, out := e.post("/api/maint/equipment", `{"name":"Walk-in Cooler 1","area":"Drystock"}`)
	eq := out["equipment"].([]any)[0].(map[string]any)
	id := eq["id"].(string)
	if c, _ := e.post("/api/maint/equipment", `{"name":"walk-in cooler 1"}`); c != 400 {
		t.Error("a duplicate name should be refused")
	}
	if c, _ := e.post("/api/maint/issues", `{"equipmentId":"`+id+`","title":"Warm","reportedBy":""}`); c != 400 {
		t.Error("an issue needs the reporter's name")
	}
	_, out = e.post("/api/maint/issues", `{"equipmentId":"`+id+`","title":"Warm","reportedBy":"Tony","outOfService":true,"priority":"Urgent"}`)
	is := out["issues"].([]any)[0].(map[string]any)
	if out["equipment"].([]any)[0].(map[string]any)["status"] != "Out of Service" {
		t.Error("equipment should be out of service")
	}
	put := func(body string) map[string]any {
		req := httptest.NewRequest("PUT", "/api/maint/issues/"+is["id"].(string), strings.NewReader(body))
		_, o := e.do(req)
		return o
	}
	out = put(`{"status":"Repaired","by":"Tech"}`)
	if out["equipment"].([]any)[0].(map[string]any)["status"] != "Out of Service" {
		t.Error("a finished repair must not return equipment to service by itself")
	}
	out = put(`{"status":"Verified","by":"Manager"}`)
	if out["equipment"].([]any)[0].(map[string]any)["status"] != "Operational" {
		t.Error("manager verification should return the equipment to Operational")
	}
	if b, err := os.ReadFile(filepath.Join(e.s.maintDir(), "Maintenance Log.md")); err != nil || !strings.Contains(string(b), "verified by Manager") {
		t.Errorf("readable log missing or incomplete: %v", err)
	}
	req := httptest.NewRequest("DELETE", "/api/maint/equipment/"+id, nil)
	if c, _ := e.do(req); c != 400 {
		t.Error("equipment with history should not be deletable")
	}
	if c, _ := e.post("/api/maint/settings", `{"portalUrl":"http://insecure.example"}`); c != 400 {
		t.Error("only https portal addresses are allowed")
	}
	_, out = e.post("/api/maint/settings", `{"portalUrl":"acme.getmaintainx.com"}`)
	if out["portalUrl"] != "https://acme.getmaintainx.com" {
		t.Errorf("portal url: %v", out["portalUrl"])
	}
}

func TestPreventiveMaintenance(t *testing.T) {
	e := newEnv(t)
	_, out := e.post("/api/maint/equipment", `{"name":"Fryer 1"}`)
	id := out["equipment"].([]any)[0].(map[string]any)["id"].(string)
	if c, _ := e.post("/api/maint/pm", `{"equipmentId":"`+id+`","name":"Boil out","every":0,"unit":"days","firstDue":"2026-01-01"}`); c != 400 {
		t.Error("a zero interval should be refused")
	}
	yesterday := time.Now().AddDate(0, 0, -1).Format(dayFmt)
	_, out = e.post("/api/maint/pm", `{"equipmentId":"`+id+`","name":"Boil out","every":1,"unit":"months","firstDue":"`+yesterday+`"}`)
	pm := out["pm"].([]any)[0].(map[string]any)
	if pm["state"] != "Overdue" {
		t.Errorf("a task due yesterday should be overdue: %v", pm)
	}
	if c, _ := e.post("/api/maint/pm/"+pm["id"].(string)+"/done", `{"by":""}`); c != 400 {
		t.Error("marking done needs a name")
	}
	today := time.Now().Format(dayFmt)
	_, out = e.post("/api/maint/pm/"+pm["id"].(string)+"/done", `{"by":"Manager","date":"`+today+`"}`)
	pm = out["pm"].([]any)[0].(map[string]any)
	want := time.Now().AddDate(0, 1, 0).Format(dayFmt)
	if pm["nextDue"] != want || pm["state"] != "OK" {
		t.Errorf("next due should be %s and OK: %v", want, pm)
	}
	if c, _ := e.do(httptest.NewRequest("DELETE", "/api/maint/equipment/"+id, nil)); c != 400 {
		t.Error("equipment with PM jobs should not be deletable")
	}
}

func TestAssetCostsAndReplacementAdvice(t *testing.T) {
	e := newEnv(t)
	_, out := e.post("/api/maint/equipment", `{"name":"Ice Machine","make":"Hoshizaki","model":"KM-500","serial":"A123","purchaseDate":"2018-03-01","purchaseCost":4000,"replaceCost":5000,"expectedYears":10}`)
	id := out["equipment"].([]any)[0].(map[string]any)["id"].(string)
	adv := func(o map[string]any) map[string]any { return o["assets"].(map[string]any)[id].(map[string]any) }
	if c, _ := e.post("/api/maint/service", `{"equipmentId":"`+id+`","description":"Pump","date":"2026-01-01","cost":-5,"by":"M"}`); c != 400 {
		t.Error("a negative cost should be refused")
	}
	_, out = e.post("/api/maint/service", `{"equipmentId":"`+id+`","description":"New pump","date":"2026-02-01","cost":900,"type":"Repair","by":"Manager"}`)
	if a := adv(out); a["advice"] != "OK" || a["total"].(float64) != 900 {
		t.Errorf("900 of 5000 should be OK: %v", a)
	}
	_, out = e.post("/api/maint/service", `{"equipmentId":"`+id+`","description":"Compressor","date":"2026-05-01","cost":1700,"by":"Manager"}`)
	if a := adv(out); a["advice"] != "Watch" && a["advice"] != "Consider Replacing" {
		t.Errorf("2600 of 5000 should at least be Watch: %v", a)
	}
	_, out = e.post("/api/maint/service", `{"equipmentId":"`+id+`","description":"Board","date":"2026-06-01","cost":900,"by":"Manager"}`)
	if a := adv(out); a["advice"] != "Consider Replacing" || a["percent"].(float64) != 70 {
		t.Errorf("3500 of 5000 should suggest replacing at 70%%: %v", a)
	}
	_, out = e.post("/api/maint/settings", `{"replacePercent":100}`)
	if adv(out)["advice"] == "Consider Replacing" {
		t.Error("a 100% limit should not suggest replacing at 70%")
	}
	if c, _ := e.post("/api/maint/settings", `{"replacePercent":1}`); c != 400 {
		t.Error("a silly limit should be refused")
	}
	// a repair cost entered on an issue becomes a service record
	_, out = e.post("/api/maint/issues", `{"equipmentId":"`+id+`","title":"Leak","reportedBy":"M"}`)
	iid := out["issues"].([]any)[0].(map[string]any)["id"].(string)
	_, out = e.do2("PUT", "/api/maint/issues/"+iid, `{"status":"Repaired","by":"M","parts":100,"labor":150,"downtime":3}`)
	if adv(out)["total"].(float64) != 3750 {
		t.Errorf("issue cost should join the total: %v", adv(out))
	}
}

func (e *env) do2(method, path, body string) (int, map[string]any) {
	return e.do(httptest.NewRequest(method, path, strings.NewReader(body)))
}

func TestCalendarEvents(t *testing.T) {
	e := newEnv(t)
	today := time.Now()
	d := func(n int) string { return today.AddDate(0, 0, n).Format(dayFmt) }
	for _, body := range []string{
		`{"group":"Cleaning","name":"Mop floors","every":1,"unit":"weeks","firstDue":"` + d(1) + `"}`,
		`{"group":"Cleaning","name":"Deep clean hood","every":1,"unit":"years","firstDue":"` + d(10) + `"}`,
		`{"group":"Grounds / Facility","name":"Pest control","every":1,"unit":"months","firstDue":"` + d(-3) + `"}`,
	} {
		if c, o := e.post("/api/maint/pm", body); c != 200 {
			t.Fatalf("task refused: %v", o)
		}
	}
	if c, _ := e.post("/api/maint/pm", `{"group":"Equipment PM","name":"x","every":1,"unit":"weeks","firstDue":"`+d(1)+`"}`); c != 400 {
		t.Error("equipment PM still needs equipment")
	}
	if c, _ := e.post("/api/maint/pm", `{"group":"Nonsense","name":"x","every":1,"unit":"weeks","firstDue":"`+d(1)+`"}`); c != 400 {
		t.Error("an unknown group should be refused")
	}
	_, out := e.get2("/api/maint/calendar?from=" + d(-7) + "&to=" + d(28))
	var mop, hood, pest, overdue int
	for _, x := range out["events"].([]any) {
		ev := x.(map[string]any)
		switch ev["title"] {
		case "Mop floors":
			mop++
			if ev["bucket"] != "Weekly" {
				t.Errorf("bucket: %v", ev["bucket"])
			}
		case "Deep clean hood":
			hood++
			if ev["bucket"] != "Yearly" {
				t.Errorf("bucket: %v", ev["bucket"])
			}
		case "Pest control":
			pest++
			if ev["state"] == "Overdue" {
				overdue++
			}
		}
	}
	if mop != 4 || hood != 1 || overdue != 1 {
		t.Errorf("mop=%d hood=%d pest=%d overdue=%d", mop, hood, pest, overdue)
	}
	if c, _ := e.get2("/api/maint/calendar?from=2020-01-01&to=2026-01-01"); c != 400 {
		t.Error("a multi-year range should be refused")
	}
}

func (e *env) get2(path string) (int, map[string]any) {
	return e.do(httptest.NewRequest("GET", path, nil))
}

func TestMasterTodo(t *testing.T) {
	e := newEnv(t)
	d := func(n int) string { return time.Now().AddDate(0, 0, n).Format(dayFmt) }
	e.post("/api/maint/pm", `{"group":"Cleaning","name":"Late job","every":1,"unit":"months","firstDue":"`+d(-5)+`"}`)
	e.post("/api/maint/pm", `{"group":"Cleaning","name":"Today job","every":1,"unit":"months","firstDue":"`+d(0)+`"}`)
	e.post("/api/maint/pm", `{"group":"Cleaning","name":"Next week job","every":1,"unit":"months","firstDue":"`+d(8)+`"}`)
	e.post("/api/maint/todos", `{"title":"Call the hood cleaner","createdBy":"M","due":"`+d(1)+`"}`)
	_, eqo := e.post("/api/maint/equipment", `{"name":"Walk-in"}`)
	eid := eqo["equipment"].([]any)[0].(map[string]any)["id"].(string)
	e.post("/api/maint/issues", `{"equipmentId":"`+eid+`","title":"Door seal","reportedBy":"M","priority":"High"}`)
	_, o := e.get2("/api/todo?from=" + d(0) + "&to=" + d(0))
	titles := []string{}
	for _, x := range o["items"].([]any) {
		titles = append(titles, x.(map[string]any)["title"].(string))
	}
	has := func(s string) bool {
		for _, x := range titles {
			if strings.Contains(x, s) {
				return true
			}
		}
		return false
	}
	if !has("Late job") || !has("Today job") || has("Next week job") || has("Call the hood") || !has("Door seal") {
		t.Errorf("today's list: %v", titles)
	}
	_, o = e.get2("/api/todo?from=" + d(0) + "&to=" + d(7) + "&sources=Cleaning")
	if n := len(o["items"].([]any)); n != 2 {
		t.Errorf("cleaning-only list for the week: %d items", n)
	}
	_, o = e.get2("/api/todo?from=" + d(0) + "&to=" + d(30))
	titles = titles[:0]
	for _, x := range o["items"].([]any) {
		titles = append(titles, x.(map[string]any)["title"].(string))
	}
	if !has("Next week job") || !has("Call the hood") {
		t.Errorf("month list: %v", titles)
	}
	if first := o["items"].([]any)[0].(map[string]any)["state"]; first != "Overdue" {
		t.Errorf("overdue items should come first, got %v", first)
	}
}

func TestScheduleImportAndAsNeeded(t *testing.T) {
	e := newEnv(t)
	_, o := e.post("/api/maint/import-schedule", `{"pastDates":"today"}`)
	if o["added"].(float64) < 80 {
		t.Fatalf("expected the workbook jobs to load: %v", o)
	}
	_, o = e.post("/api/maint/import-schedule", `{"pastDates":"today"}`)
	if o["added"].(float64) != 0 || o["skipped"].(float64) < 80 {
		t.Errorf("a second import must not add duplicates: %v", o)
	}
	_, out := e.get2("/api/maint")
	today := time.Now().Format(dayFmt)
	asNeeded, past := 0, 0
	for _, x := range out["pm"].([]any) {
		v := x.(map[string]any)
		if v["unit"] == "asneeded" {
			asNeeded++
			if v["state"] != "As Needed" || v["nextDue"] != "" {
				t.Errorf("as-needed jobs are not scheduled: %v", v)
			}
		} else if v["lastDone"] == "" && v["firstDue"].(string) < today {
			past++
		}
	}
	if asNeeded < 2 || past != 0 {
		t.Errorf("asNeeded=%d pastDue=%d", asNeeded, past)
	}
	_, cal := e.get2("/api/maint/calendar?from=" + today + "&to=" + today)
	if n := len(cal["events"].([]any)); n == 0 || n > 45 {
		t.Errorf("today should have the daily jobs but not everything: %d events", n)
	}
	if c, _ := e.post("/api/maint/pm", `{"group":"Cleaning","name":"Mop buckets","unit":"asneeded"}`); c != 200 {
		t.Error("an as-needed job needs no interval or date")
	}
}

func TestMonthlySummary(t *testing.T) {
	e := newEnv(t)
	_, out := e.post("/api/maint/equipment", `{"name":"Walk-in cooler","type":"Refrigeration","by":"M"}`)
	id := out["equipment"].([]any)[0].(map[string]any)["id"].(string)
	month := time.Now().Format("2006-01")
	for _, title := range []string{"Leak", "Door seal"} {
		_, out = e.post("/api/maint/issues", `{"equipmentId":"`+id+`","title":"`+title+`","priority":"P2 - Urgent","reportedBy":"M"}`)
	}
	iid := out["issues"].([]any)[0].(map[string]any)["id"].(string)
	e.do2("PUT", "/api/maint/issues/"+iid, `{"status":"Repaired","by":"M","parts":100,"labor":150,"downtime":3}`)
	_, s := e.get2("/api/maint/summary?month=" + month)
	if s["opened"].(float64) != 2 || s["completed"].(float64) != 1 || s["openNow"].(float64) != 2 || s["p1p2Open"].(float64) != 2 {
		t.Errorf("counts wrong: %v", s)
	}
	if s["repairCost"].(float64) != 250 || s["downtime"].(float64) != 3 {
		t.Errorf("repair cost/downtime wrong: %v", s)
	}
	if len(s["repeat"].([]any)) != 1 || len(s["major"].([]any)) != 1 {
		t.Errorf("repeat/major wrong: %v", s)
	}
	if c, _ := e.post("/api/maint/summary-notes", `{"month":"`+month+`","highlights":"x"}`); c != 400 {
		t.Error("notes need a name")
	}
	_, s = e.post("/api/maint/summary-notes", `{"month":"`+month+`","highlights":"Good month","concerns":"Cooler","actions":"Quote","by":"GM"}`)
	if s["notes"].(map[string]any)["highlights"] != "Good month" {
		t.Errorf("notes not kept: %v", s["notes"])
	}
}

func TestInspectionItems(t *testing.T) {
	e := newEnv(t)
	if c, _ := e.post("/api/maint/findings", `{"what":"Fix cove base","foundBy":""}`); c != 400 {
		t.Error("needs a name")
	}
	_, out := e.post("/api/maint/findings", `{"what":"Fix cove base","area":"Kitchen","category":"Facility / Grounds","priority":"P3 - Routine","foundBy":"GM","due":"`+time.Now().Format(dayFmt)+`"}`)
	fs := out["findings"].([]any)
	if len(fs) != 1 || fs[0].(map[string]any)["number"].(float64) != 1 {
		t.Fatalf("item not added: %v", out)
	}
	id := fs[0].(map[string]any)["id"].(string)
	_, td := e.get2("/api/todo?from=" + time.Now().Format(dayFmt) + "&to=" + time.Now().Format(dayFmt))
	found := false
	for _, x := range td["items"].([]any) {
		if x.(map[string]any)["group"] == "Inspection" {
			found = true
		}
	}
	if !found {
		t.Errorf("open item should be on the to-do list: %v", td)
	}
	e.do2("PUT", "/api/maint/findings/"+id, `{"status":"Done","by":"GM","note":"Fixed"}`)
	_, out = e.get2("/api/maint/findings")
	if out["findings"].([]any)[0].(map[string]any)["doneAt"] == "" {
		t.Error("done date not set")
	}
	_, td = e.get2("/api/todo?from=" + time.Now().Format(dayFmt) + "&to=" + time.Now().Format(dayFmt))
	for _, x := range td["items"].([]any) {
		if x.(map[string]any)["group"] == "Inspection" {
			t.Error("done item should leave the to-do list")
		}
	}
}
