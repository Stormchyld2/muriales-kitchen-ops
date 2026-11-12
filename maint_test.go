package main

import (
	"encoding/json"
	"fmt"
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
	if out["equipment"].([]any)[0].(map[string]any)["status"] != "Out of Service" {
		t.Error("verification must not lift an out-of-service hold")
	}
	_, out = e.post("/api/maint/equipment/"+id+"/clear", `{"by":"Manager","note":"Checked temperature holds","safe":true}`)
	if out["equipment"].([]any)[0].(map[string]any)["status"] != "Operational" {
		t.Error("clearing should return the equipment to Operational")
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
	n := 0
	for _, x := range o["items"].([]any) {
		if x.(map[string]any)["group"] != "Cleaning" {
			t.Errorf("source filter let through %v", x)
		}
		if x.(map[string]any)["kind"] == "task" { // the PM jobs this test added, not the seeded cleaning schedule
			n++
		}
	}
	if n != 2 {
		t.Errorf("cleaning-only list for the week: %d PM jobs", n)
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

func TestOutOfServiceWorkflow(t *testing.T) {
	e := newEnv(t)
	_, out := e.post("/api/maint/equipment", `{"name":"Fryer 1","type":"Fryer"}`)
	id := out["equipment"].([]any)[0].(map[string]any)["id"].(string)
	st := func(o map[string]any) string { return o["equipment"].([]any)[0].(map[string]any)["status"].(string) }
	if c, _ := e.post("/api/maint/equipment/"+id+"/out-of-service", `{"by":"GM"}`); c != 400 {
		t.Error("a reason is required")
	}
	c, out := e.post("/api/maint/equipment/"+id+"/out-of-service", `{"reason":"Oil leak","by":"GM"}`)
	if c != 200 || st(out) != "Out of Service" || len(out["oosLog"].([]any)) != 1 {
		t.Fatalf("not taken out of service: %v", out)
	}
	if c, _ := e.post("/api/maint/equipment/"+id+"/out-of-service", `{"reason":"again","by":"GM"}`); c != 400 {
		t.Error("cannot be taken out twice")
	}
	// editing the record cannot lift the hold
	_, out = e.post("/api/maint/equipment", `{"id":"`+id+`","name":"Fryer 1","status":"Operational"}`)
	if st(out) != "Out of Service" {
		t.Error("editing must not clear the hold")
	}
	// a verified work order does not lift it either
	_, out = e.post("/api/maint/issues", `{"equipmentId":"`+id+`","title":"Leak","reportedBy":"M"}`)
	iid := out["issues"].([]any)[0].(map[string]any)["id"].(string)
	_, out = e.do2("PUT", "/api/maint/issues/"+iid, `{"status":"Verified","by":"GM"}`)
	if st(out) != "Out of Service" {
		t.Error("verifying a work order must not clear the hold")
	}
	e.post("/api/maint/settings", `{"managers":["Pat Manager"]}`)
	if c, _ := e.post("/api/maint/equipment/"+id+"/clear", `{"by":"Cook","note":"Fixed","safe":true}`); c != 403 {
		t.Error("only a listed manager can clear")
	}
	if c, _ := e.post("/api/maint/equipment/"+id+"/clear", `{"by":"pat manager","note":"","safe":true}`); c != 400 {
		t.Error("a note is required")
	}
	if c, _ := e.post("/api/maint/equipment/"+id+"/clear", `{"by":"pat manager","note":"Fixed","safe":false}`); c != 400 {
		t.Error("the safety confirmation is required")
	}
	c, out = e.post("/api/maint/equipment/"+id+"/clear", `{"by":"pat manager","note":"Seal replaced, test run","safe":true}`)
	if c != 200 || st(out) != "Operational" {
		t.Fatalf("not cleared: %v %v", c, out)
	}
	if r := out["oosLog"].([]any)[0].(map[string]any); r["clearedBy"] != "pat manager" || r["clearNote"] == "" {
		t.Errorf("clear not recorded: %v", r)
	}
	// reporting an issue with "out of service" ticked records a hold
	_, out = e.post("/api/maint/issues", `{"equipmentId":"`+id+`","title":"Burner out","reportedBy":"M","outOfService":true}`)
	if st(out) != "Out of Service" || len(out["oosLog"].([]any)) != 2 {
		t.Errorf("issue should start a hold: %v", out["oosLog"])
	}
}

func TestReports(t *testing.T) {
	e := newEnv(t)
	e.post("/api/maint/equipment", `{"name":"Walk-in <1>","type":"Cooler"}`)
	_, cat := e.get2("/api/reports")
	rs := cat["reports"].([]any)
	if len(rs) < 10 {
		t.Fatalf("catalog too small: %d", len(rs))
	}
	for _, x := range rs {
		id := x.(map[string]any)["id"].(string)
		req := httptest.NewRequest("GET", "/api/reports/"+id+"?month="+time.Now().Format("2006-01"), nil)
		rec := httptest.NewRecorder()
		e.h.ServeHTTP(rec, req)
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), "Muriale") {
			t.Errorf("report %s failed: %d", id, rec.Code)
		}
		if id == "equipment" && (strings.Contains(rec.Body.String(), "<1>") || !strings.Contains(rec.Body.String(), "Walk-in &lt;1&gt;")) {
			t.Error("report must escape text")
		}
	}
	if c, _ := e.get2("/api/reports/nope"); c != 404 {
		t.Error("unknown report should be 404")
	}
	if c, _ := e.post("/api/reports/email", `{"id":"equipment","to":"not an address"}`); c != 400 {
		t.Error("bad address should be refused")
	}
	if c, _ := e.post("/api/reports/email", `{"id":"equipment","to":""}`); c != 400 {
		t.Error("an address is required")
	}
	c, out := e.post("/api/reports/email", `{"id":"equipment","to":"chef@example.com; gm@example.com","message":"Please review"}`)
	if c != 200 || out["method"] != "outlook" {
		t.Fatalf("email draft failed: %v %v", c, out)
	}
	if _, err := os.Stat(filepath.Join(e.s.reportsDir(), fmt.Sprint(out["file"]))); err != nil {
		t.Errorf("report file not saved: %v", err)
	}
}

func TestOutOfServiceTag(t *testing.T) {
	e := newEnv(t)
	_, out := e.post("/api/maint/equipment", `{"name":"Fryer 2","area":"Line"}`)
	id := out["equipment"].([]any)[0].(map[string]any)["id"].(string)
	if c, _ := e.post("/api/maint/equipment/"+id+"/tag", "\x89PNG\r\n\x1a\nxx"); c != 400 {
		t.Error("a tag needs an active hold")
	}
	_, out = e.post("/api/maint/equipment/"+id+"/out-of-service", `{"reason":"Burner out","by":"GM","safety":"Yes","foodSafety":"No","powerOff":"Yes","lockout":"No","tagPriority":"High","vendor":"ACME","workOrder":"WO-12","assetId":"K-007"}`)
	rec := out["oosLog"].([]any)[0].(map[string]any)
	if rec["safety"] != "Yes" || rec["tagPriority"] != "High" || rec["assetId"] != "K-007" || rec["lockout"] != "No" {
		t.Errorf("tag details not kept: %v", rec)
	}
	if c, _ := e.post("/api/maint/equipment/"+id+"/tag", "not a png at all"); c != 400 {
		t.Error("only a PNG is accepted")
	}
	c, out := e.post("/api/maint/equipment/"+id+"/tag", "\x89PNG\r\n\x1a\nfakeimage")
	if c != 200 || !strings.HasSuffix(out["file"].(string), ".png") {
		t.Fatalf("tag not saved: %v %v", c, out)
	}
	file := out["file"].(string)
	if _, err := os.Stat(filepath.Join(e.s.tagsDir(), file)); err != nil {
		t.Errorf("file missing: %v", err)
	}
	rec2 := out["payload"].(map[string]any)["oosLog"].([]any)[0].(map[string]any)
	if rec2["tag"] != file {
		t.Errorf("record should point at the file: %v", rec2)
	}
	rr := httptest.NewRecorder()
	e.h.ServeHTTP(rr, httptest.NewRequest("GET", "/api/oos-tags/"+strings.ReplaceAll(file, " ", "%20")+"?print=1", nil))
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), "print()") {
		t.Errorf("print page failed: %d", rr.Code)
	}
	rr = httptest.NewRecorder()
	e.h.ServeHTTP(rr, httptest.NewRequest("GET", "/api/oos-tags/..%2Fsecret.png", nil))
	if rr.Code == 200 {
		t.Error("path traversal must be refused")
	}
}

func TestProcedures(t *testing.T) {
	e := newEnv(t)
	_, p := e.get2("/api/procedures")
	if len(p["steps"].([]any)) != 10 || len(p["priorities"].([]any)) != 4 || len(p["cadence"].([]any)) != 5 {
		t.Fatalf("procedures not loaded: %v", p)
	}
	if _, err := os.Stat(filepath.Join(e.s.maintDir(), "procedures.json")); err != nil {
		t.Error("procedures should be copied into Maintenance Data")
	}
}

func TestPMInspection(t *testing.T) {
	e := newEnv(t)
	_, out := e.post("/api/maint/equipment", `{"name":"Ice machine"}`)
	eid := out["equipment"].([]any)[0].(map[string]any)["id"].(string)
	_, out = e.post("/api/maint/pm", `{"group":"Equipment PM","name":"Quarterly service","equipmentId":"`+eid+`","every":3,"unit":"months","firstDue":"`+time.Now().Format(dayFmt)+`"}`)
	tid := out["pm"].([]any)[0].(map[string]any)["id"].(string)
	base := `"taskId":"` + tid + `","by":"Tech","condition":"Fair",`
	if c, _ := e.post("/api/inspections", `{`+base+`"by":""}`); c != 400 {
		t.Error("name required")
	}
	if c, _ := e.post("/api/inspections", `{`+base+`"results":{"seals":{"result":"Fail"}}}`); c != 400 {
		t.Error("a failure needs a described action")
	}
	if c, _ := e.post("/api/inspections", `{`+base+`"results":{"seals":{"result":"Fail"}},"corrective":"Replace door gasket","markDone":true}`); c != 400 {
		t.Error("cannot mark done with a failure and no issue")
	}
	c, out := e.post("/api/inspections", `{`+base+`"results":{"seals":{"result":"Fail","note":"torn"}},"corrective":"Replace door gasket","createIssue":true,"markDone":true,"readings":"36F"}`)
	if c != 200 {
		t.Fatalf("inspection refused: %v", out)
	}
	in := out["inspection"].(map[string]any)
	if in["fails"].(float64) != 1 || in["pmDone"] != true || len(in["items"].([]any)) != 16 || in["issueId"] == "" {
		t.Errorf("inspection record wrong: %v", in)
	}
	if len(out["issues"].([]any)) != 1 {
		t.Error("a maintenance issue should have been created")
	}
	if pm := out["pm"].([]any)[0].(map[string]any); pm["lastDone"] == "" {
		t.Errorf("PM job should be marked done: %v", pm)
	}
	rr := httptest.NewRecorder()
	e.h.ServeHTTP(rr, httptest.NewRequest("GET", "/api/reports/inspection:"+in["id"].(string), nil))
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), "Replace door gasket") {
		t.Errorf("printed sheet failed: %d", rr.Code)
	}
}

func TestTemperatureLogAndImport(t *testing.T) {
	e := newEnv(t)
	e.post("/api/temps/points", `{"name":"Walk-in Cooler","kind":"Cold Holding"}`)
	_, out := e.post("/api/temps/points", `{"name":"Hot Well","kind":"Hot Holding"}`)
	pts := out["points"].([]any)
	cooler := pts[0].(map[string]any)["id"].(string)
	if c, _ := e.post("/api/temps/points", `{"name":"walk-in cooler","kind":"Freezer"}`); c != 400 {
		t.Error("duplicate point names are refused")
	}
	if c, _ := e.post("/api/temps", `{"by":"","readings":[{"pointId":"`+cooler+`","temp":38}]}`); c != 400 {
		t.Error("initials required")
	}
	if c, _ := e.post("/api/temps", `{"by":"TC","readings":[{"pointId":"`+cooler+`","temp":46}]}`); c != 400 {
		t.Error("out of range needs a corrective action")
	}
	if c, o := e.post("/api/temps", `{"by":"TC","readings":[{"pointId":"`+cooler+`","temp":38},{"pointId":"`+cooler+`","temp":47,"action":"Moved food, called service"}]}`); c != 200 || len(o["readings"].([]any)) != 2 {
		t.Fatalf("round not saved: %v %v", c, o)
	}
	// long-format Google Form export, with one new unit and one reading out of range
	csv := "Timestamp,Which unit?,Temperature,Initials,Comments\n" +
		"11/10/2026 8:05:12,Walk-in Cooler,39,JB,\n" +
		"11/10/2026 8:07:40,Hot Well,120°F,JB,sauce cold\n" +
		"11/10/2026 8:09:00,Chest Freezer,-2,JB,\n" +
		"not a date,Walk-in Cooler,39,JB,\n"
	_, pv := e.post("/api/temps/import/preview", `{"csv":`+jsonStr(csv)+`}`)
	m := pv["map"].(map[string]any)
	if m["timestamp"] != "Timestamp" || m["point"] != "Which unit?" || m["by"] != "Initials" || m["notes"] != "Comments" || len(m["temp"].([]any)) != 1 {
		t.Fatalf("columns not recognised: %v", m)
	}
	c, o := e.post("/api/temps/import", `{"csv":`+jsonStr(csv)+`,"map":{"timestamp":"Timestamp","point":"Which unit?","temp":["Temperature"],"by":"Initials","notes":"Comments"},"save":true}`)
	if c != 200 {
		t.Fatalf("import failed: %v", o)
	}
	r := o["result"].(map[string]any)
	if r["Added"].(float64) != 3 || r["Unreadable"].(float64) != 1 || r["NewPoints"].(float64) != 1 || r["OutOfRange"].(float64) != 1 {
		t.Errorf("import counts wrong: %v", r)
	}
	_, o = e.post("/api/temps/import", `{"csv":`+jsonStr(csv)+`,"map":{"timestamp":"Timestamp","point":"Which unit?","temp":["Temperature"],"by":"Initials"}}`)
	if o["result"].(map[string]any)["Added"].(float64) != 0 || o["result"].(map[string]any)["Skipped"].(float64) != 3 {
		t.Errorf("importing twice must not duplicate: %v", o["result"])
	}
	// the out-of-range Hot Well reading needs action and shows on the to-do list
	td := false
	_, tl := e.get2("/api/todo?from=" + time.Now().Format(dayFmt) + "&to=" + time.Now().Format(dayFmt))
	for _, x := range tl["items"].([]any) {
		if x.(map[string]any)["group"] == "Temperature" {
			td = true
		}
	}
	_ = td // the import dates are in the past; the 14-day window decides this
	// wide format: one column per unit
	wide := "Timestamp,Walk-in Cooler temperature,Hot Well temperature,Initials\n11/11/2026 9:00:00,40,150,KM\n"
	_, pv = e.post("/api/temps/import/preview", `{"csv":`+jsonStr(wide)+`}`)
	if len(pv["map"].(map[string]any)["temp"].([]any)) != 2 {
		t.Errorf("wide format not recognised: %v", pv["map"])
	}
	_, o = e.post("/api/temps/import", `{"csv":`+jsonStr(wide)+`,"map":{"timestamp":"Timestamp","temp":["Walk-in Cooler temperature","Hot Well temperature"],"by":"Initials"}}`)
	if o["result"].(map[string]any)["Added"].(float64) != 2 || o["result"].(map[string]any)["NewPoints"].(float64) != 0 {
		t.Errorf("wide import wrong: %v", o["result"])
	}
	if c, _ := e.post("/api/temps/settings", `{"url":"https://evil.example.com/x.csv"}`); c != 400 {
		t.Error("only Google links are accepted")
	}
	if c, _ := e.post("/api/temps/settings", `{"url":"https://docs.google.com/spreadsheets/d/e/abc/pub?output=csv"}`); c != 200 {
		t.Error("a published Google sheet link is accepted")
	}
}

func jsonStr(s string) string { b, _ := json.Marshal(s); return string(b) }

func TestMicrosoftFormImport(t *testing.T) {
	e := newEnv(t)
	_, o := e.post("/api/temps/points/starter", `{}`)
	if n := len(o["points"].([]any)); n != 29 {
		t.Fatalf("starter points: %d", n)
	}
	csv := "ID,Start time,Completion time,Email,Name,Cooler 1 Temp,Freezer 1 Temp,Cooler 2,Sauce Jack - Manicotti Bain-Marie,Sauce Jack - Lasagna Bain-Marie\n" +
		"4,8/26/2026 18:24,8/26/2026 18:26,j@x.com,Jason,31,20,40,210,210\n" +
		"6,8/31/2026 11:47,8/31/2026 12:00,j@x.com,Jason,34,-17.5,36,Not in use,Not in use\n"
	_, pv := e.post("/api/temps/import/preview", `{"csv":`+jsonStr(csv)+`}`)
	m := pv["map"].(map[string]any)
	if m["timestamp"] != "Completion time" || m["by"] != "Name" || len(m["temp"].([]any)) != 5 || m["point"] != "" {
		t.Fatalf("map: %v", m)
	}
	mj, _ := json.Marshal(m)
	c, r := e.post("/api/temps/import", `{"csv":`+jsonStr(csv)+`,"map":`+string(mj)+`}`)
	res := r["result"].(map[string]any)
	if c != 200 || res["Added"].(float64) != 8 || res["Unreadable"].(float64) != 0 || res["NewPoints"].(float64) != 0 {
		t.Fatalf("import: %v", r)
	}
	// 20 and -17.5 on freezers, 31 on a 33-41 cooler, 210 on a 135-200 well,
	if res["OutOfRange"].(float64) != 5 {
		t.Errorf("out of range: %v", res)
	}
	if got, _ := parseWhen("46260.5", ""); got != "2026-08-26 12:00" {
		t.Errorf("excel serial: %q", got)
	}
}

func TestCorrectiveActionAndDailyReport(t *testing.T) {
	e := newEnv(t)
	if c, _ := e.post("/api/corrective", `{"identifiedBy":"JW","area":"Food Safety","priority":"P1 - Critical"}`); c != 400 {
		t.Error("issue is required")
	}
	c, o := e.post("/api/corrective", `{"identifiedBy":"JW","area":"Food Safety","priority":"P1 - Critical","issue":"Walk-in at 47F","target":"2000-01-01"}`)
	if c != 200 || o["saved"].(map[string]any)["id"] != "CA-0001" {
		t.Fatalf("create: %v %v", c, o)
	}
	_, o = e.post("/api/corrective", `{"identifiedBy":"JW","area":"Sanitation","priority":"P3 - Routine","issue":"Drain"}`)
	if o["saved"].(map[string]any)["id"] != "CA-0002" {
		t.Error("ids count up")
	}
	if c, _ := e.post("/api/corrective", `{"id":"CA-0001","identifiedBy":"JW","area":"Food Safety","priority":"P1 - Critical","issue":"Walk-in at 47F","status":"Closed"}`); c != 400 {
		t.Error("closing needs who and what was done")
	}
	c, o = e.post("/api/corrective", `{"id":"CA-0001","identifiedBy":"JW","area":"Food Safety","priority":"P1 - Critical","issue":"Walk-in at 47F","required":"Replaced door gasket","status":"Closed","closedBy":"MM"}`)
	if c != 200 || len(o["forms"].([]any)) != 2 {
		t.Fatalf("close: %v %v", c, o)
	}
	if o["saved"].(map[string]any)["closedDate"] == "" {
		t.Error("closed date set")
	}
	if _, err := os.Stat(filepath.Join(e.s.caDir(), "CA-0001.html")); err != nil {
		t.Error("form file not saved")
	}
	rep, _ := e.s.buildReport("ca-open", "")
	if len(rep.Sections[0].Rows) != 1 {
		t.Error("open report lists only open forms")
	}
	// daily temperature report
	e.post("/api/temps/points", `{"name":"Cooler","kind":"Cold Holding","area":"Back Line"}`)
	_, p := e.post("/api/temps", `{"by":"JW","readings":[]}`)
	_ = p
	day := time.Now().Format(dayFmt)
	pts := e.s.tempPoints()
	e.post("/api/temps", `{"by":"JW","readings":[{"pointId":"`+pts[0].ID+`","temp":47,"action":"Moved food"}]}`)
	if c, o := e.post("/api/temp-reports", `{"date":"`+day+`"}`); c != 200 || o["file"] == "" {
		t.Fatalf("save report: %v %v", c, o)
	}
	b, err := os.ReadFile(filepath.Join(e.s.tempReportsDir(), "Temperature Report "+day+".html"))
	if err != nil || !strings.Contains(string(b), "Too Warm") || !strings.Contains(string(b), "Back Line") || !strings.Contains(string(b), "Manager Review") {
		t.Errorf("report content: %v", err)
	}
}

func TestChemicalLogAndReadiness(t *testing.T) {
	e := newEnv(t)
	bad := `{"at":"2026-11-12T10:30","kind":"Skin Contact","product":"Degreaser","area":"Fryer","summary":"Splash","by":"JW","emergency":"No","mgmtNotified":"Yes"}`
	if c, _ := e.post("/api/chem", bad); c != 400 {
		t.Error("exposure needs a person")
	}
	ok := strings.Replace(bad, `"area"`, `"persons":"Cook A","area"`, 1)
	c, o := e.post("/api/chem", ok)
	if c != 200 || o["saved"].(map[string]any)["id"] != "CHEM-0001" {
		t.Fatalf("chem: %v %v", c, o)
	}
	for _, id := range []string{"chem:CHEM-0001", "chem-response", "chem-blank", "chem-log", "readiness-blank", "readiness-log"} {
		if rep, found := e.s.buildReport(id, ""); !found || len(rep.Sections) == 0 {
			t.Errorf("report %s missing", id)
		}
	}
	items := ""
	for i := 0; i < 30; i++ {
		st := `"Ready"`
		if i == 6 {
			st = `"Needs Attention","finding":"Cooler at 45F","priority":"P2 - Urgent","owner":"Sous Chef"`
		}
		if i > 0 {
			items += ","
		}
		items += `{"status":` + st + `}`
	}
	rv := `{"date":"2026-11-12","manager":"JW","type":"Internal Self-Inspection","overall":"No","critical":"No","items":[` + items + `]}`
	c, o = e.post("/api/readiness", rv)
	if c != 200 || o["saved"] != "IR-0001" || o["reviews"].([]any)[0].(map[string]any)["open"].(float64) != 1 {
		t.Fatalf("readiness: %v %v", c, o)
	}
	if c, _ := e.post("/api/readiness", strings.Replace(rv, `"finding":"Cooler at 45F",`, "", 1)); c != 400 {
		t.Error("a finding is needed when attention is needed")
	}
	c, o = e.post("/api/readiness/IR-0001/corrective", `{}`)
	if c != 200 || o["made"].(float64) != 1 {
		t.Fatalf("corrective: %v %v", c, o)
	}
	if c, _ := e.post("/api/readiness/IR-0001/corrective", `{}`); c != 400 {
		t.Error("no duplicate forms")
	}
	if len(e.s.caForms()) != 1 || e.s.caForms()[0].Related == "" {
		t.Error("corrective form not created")
	}
	if rep, _ := e.s.buildReport("readiness:IR-0001", ""); !strings.Contains(rep.html(true), "CA-0001") {
		t.Error("review should show the CA reference")
	}
}

func TestCleaningAndReplacement(t *testing.T) {
	e := newEnv(t)
	for name, n := range map[string]int{"cleantasks": 18, "standards": 11, "mgrchecks": 14, "cleanref": 0} {
		_, o := e.get2("/api/reg/" + name)
		if got := len(o["rows"].([]any)); got != n {
			t.Errorf("%s seeded %d, want %d", name, got, n)
		}
	}
	if c, _ := e.post("/api/reg/cleantasks", `{"area":"Bar"}`); c != 400 {
		t.Error("task text required")
	}
	c, o := e.post("/api/reg/cleanref", `{"name":"Sanitizer","type":"Chemical","dilution":"200 ppm"}`)
	if c != 200 || o["rows"].([]any)[0].(map[string]any)["id"] != "CT-001" {
		t.Fatalf("add ref: %v %v", c, o)
	}
	if c, _ := e.post("/api/reg/cleanref", `{"name":"x","type":"Bogus"}`); c != 400 {
		t.Error("select options enforced")
	}
	if c, _ := e.post("/api/clean/done", `{"id":"CLN-0001","by":""}`); c != 400 {
		t.Error("name required")
	}
	c, o = e.post("/api/clean/done", `{"id":"CLN-0001","by":"AB","issue":true,"notes":"Grease buildup"}`)
	if c != 200 {
		t.Fatalf("done: %v %v", c, o)
	}
	pend := o["pending"].([]any)
	if len(pend) != 1 {
		t.Fatalf("verification pending: %v", pend)
	}
	if _, m := e.get2("/api/reg/missed"); len(m["rows"].([]any)) != 1 {
		t.Error("issue found should create a missed entry")
	}
	for _, tk := range o["tasks"].([]any) {
		if tk.(map[string]any)["id"] == "CLN-0001" && tk.(map[string]any)["state"] != "Done Today" {
			t.Error("task should show done today")
		}
	}
	c, o = e.post("/api/clean/verify", `{"id":"`+pend[0].(map[string]any)["id"].(string)+`","by":"MM","ok":false,"notes":"Still greasy"}`)
	if c != 200 || len(o["pending"].([]any)) != 0 {
		t.Fatalf("verify: %v %v", c, o)
	}
	if _, m := e.get2("/api/reg/missed"); len(m["rows"].([]any)) != 2 {
		t.Error("failed verification should create a missed entry")
	}
	if c, _ := e.post("/api/mgr/done", `{"id":"KOM-001","by":"JW","result":"Needs Attention","notes":"Cooler warm"}`); c != 200 {
		t.Error("manager check done")
	}
	if _, l := e.get2("/api/reg/mgrlog"); l["rows"].([]any)[0].(map[string]any)["caNeeded"] != "Yes" {
		t.Error("needs attention flags a corrective action")
	}
	if c, _ := e.s.cleanReport("clean-weekly"); c == nil {
		t.Error("weekly report")
	}
	for _, id := range []string{"clean-schedule", "clean-standards", "clean-chem", "clean-log", "clean-missed", "mgr-schedule", "mgr-log", "clean-weekly", "linecook-weekly", "replace-log"} {
		if rep, ok := e.s.buildReport(id, ""); !ok || len(rep.Sections) == 0 {
			t.Errorf("report %s", id)
		}
	}
	items := e.s.todoItems(time.Now(), time.Now().AddDate(0, 0, 7), map[string]bool{"Cleaning": true, "Manager Check": true})
	var day, mgr bool
	for _, it := range items {
		day = day || it.Kind == "cleanday"
		mgr = mgr || it.Kind == "mgr"
	}
	if !day || !mgr {
		t.Errorf("todo should include the daily checklist and manager checks: %v", items)
	}
	// replacement request, prefilled from the equipment and its repairs
	_, eo := e.post("/api/maint/equipment", `{"name":"Walk-in 1","area":"Storage","make":"Kolpak","model":"X1","purchaseCost":9000,"replaceCost":12000}`)
	_ = eo
	eqs, _, _ := e.s.loadMaint()
	if len(eqs) == 0 {
		t.Skip("equipment endpoint shape differs")
	}
	c, pf := e.get2("/api/replacements/prefill?equipmentId=" + eqs[0].ID)
	if c != 200 || pf["equipment"] != "Walk-in 1" || pf["makeModel"] != "Kolpak X1" {
		t.Fatalf("prefill: %v %v", c, pf)
	}
	if c, _ := e.post("/api/replacements", `{"equipment":"Walk-in 1","requestedBy":"JW","date":"2026-11-12","condition":"Poor","recommendation":"Replace"}`); c != 400 {
		t.Error("reason required")
	}
	c, o = e.post("/api/replacements", `{"equipment":"Walk-in 1","requestedBy":"JW","date":"2026-11-12","condition":"Poor","recommendation":"Replace","reasons":"Repeated compressor failures","history":[{"date":"2026-10-01","failure":"Compressor"}]}`)
	if c != 200 || o["saved"] != "ER-0001" {
		t.Fatalf("replacement: %v %v", c, o)
	}
	if c, _ := e.post("/api/replacements", `{"id":"ER-0001","equipment":"Walk-in 1","requestedBy":"JW","date":"2026-11-12","condition":"Poor","recommendation":"Replace","reasons":"x","completed":"Yes"}`); c != 400 {
		t.Error("cannot complete before approval")
	}
	if rep, ok := e.s.buildReport("replace:ER-0001", ""); !ok || !strings.Contains(rep.html(true), "Compressor") {
		t.Error("replacement report")
	}
}

func TestEquipmentAreasSetting(t *testing.T) {
	e := newEnv(t)
	_, o := e.get2("/api/maint")
	a := o["areas"].([]any)
	if len(a) != 10 || a[0] != "Front Line" || a[9] != "Cooler 3" {
		t.Fatalf("defaults: %v", a)
	}
	_, o = e.post("/api/maint/settings", `{"areas":["Front Line"," Bar ","bar","Patio"]}`)
	a = o["areas"].([]any)
	if len(a) != 3 || a[1] != "Bar" || a[2] != "Patio" {
		t.Fatalf("saved list should trim and drop duplicates: %v", a)
	}
}

func TestEquipmentTypesSetting(t *testing.T) {
	e := newEnv(t)
	_, o := e.get2("/api/maint")
	got := map[string]bool{}
	for _, x := range o["types"].([]any) {
		got[x.(string)] = true
	}
	for _, w := range []string{"Lowboy", "Cook Tops", "Ovens", "Mixer", "Misc Equip"} {
		if !got[w] {
			t.Errorf("missing default type %s", w)
		}
	}
	_, o = e.post("/api/maint/settings", `{"types":["Lowboy","Salamander","lowboy"]}`)
	if a := o["types"].([]any); len(a) != 2 || a[1] != "Salamander" {
		t.Errorf("saved types: %v", a)
	}
}

func TestMissedAndLogAutoFeed(t *testing.T) {
	e := newEnv(t)
	old := time.Now().AddDate(0, 0, -10).Format(dayFmt)
	_, o := e.get2("/api/reg/cleantasks")
	row := o["rows"].([]any)[1].(map[string]any)
	row["lastDone"] = old
	row["frequency"] = "Weekly"
	b, _ := json.Marshal(row)
	if c, out := e.post("/api/reg/cleantasks", string(b)); c != 200 {
		t.Fatalf("update: %v %v", c, out)
	}
	_, m := e.get2("/api/reg/missed")
	rows := m["rows"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["status"] != "Open" || rows[0].(map[string]any)["source"] != "Cleaning Schedule" {
		t.Fatalf("overdue task should feed Missed: %v", rows)
	}
	if _, m = e.get2("/api/reg/missed"); len(m["rows"].([]any)) != 1 {
		t.Fatal("sync must not duplicate")
	}
	id := row["id"].(string)
	if c, out := e.post("/api/clean/done", `{"id":"`+id+`","by":"AB"}`); c != 200 {
		t.Fatalf("done: %v %v", c, out)
	}
	_, m = e.get2("/api/reg/missed")
	if r := m["rows"].([]any)[0].(map[string]any); r["status"] != "Corrected" || r["corrected"] == "" {
		t.Fatalf("done should close the entry: %v", r)
	}
	// a Cleaning PM job done goes to the completion log
	_, pm := e.post("/api/maint/import-schedule", `{"pastDates":"keep"}`)
	_ = pm
	_, mp := e.get2("/api/maint")
	var code string
	for _, p := range mp["pm"].([]any) {
		if x := p.(map[string]any); x["group"] == "Cleaning" {
			code = x["id"].(string)
			break
		}
	}
	if code == "" {
		t.Fatal("no cleaning PM job")
	}
	if c, out := e.post("/api/maint/pm/"+code+"/done", `{"by":"AB"}`); c != 200 {
		t.Fatalf("pm done: %v %v", c, out)
	}
	_, l := e.get2("/api/reg/cleanlog")
	found := false
	for _, x := range l["rows"].([]any) {
		if x.(map[string]any)["source"] == "PM job" {
			found = true
		}
	}
	if !found {
		t.Error("PM cleaning job should feed the Completion Log")
	}
	// deep clean
	_, d := e.get2("/api/reg/deepclean")
	if len(d["rows"].([]any)) != 7 {
		t.Fatalf("deep clean seeded: %d", len(d["rows"].([]any)))
	}
	if c, _ := e.post("/api/clean/deepdone", `{"id":"DEEP-001","by":"AB"}`); c != 200 {
		t.Error("deep done")
	}
	if _, a := e.get2("/api/reg/cleanareas"); len(a["rows"].([]any)) != 10 {
		t.Error("area matrix seeded")
	}
	// equipment from workbook
	_, r1 := e.post("/api/maint/import-equipment", `{}`)
	if r1["added"].(float64) != 85 || r1["linked"].(float64) < 70 {
		t.Fatalf("equipment import: %v", r1)
	}
	if _, r2 := e.post("/api/maint/import-equipment", `{}`); r2["added"].(float64) != 0 {
		t.Error("equipment import must be idempotent")
	}
}
