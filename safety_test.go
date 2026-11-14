package main

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestSafetyProgram(t *testing.T) {
	e := newEnv(t)
	// starters
	for name, min := range map[string]int{"binder": 12, "econtacts": 10, "eqplan": 10, "eqvendors": 10, "walkchecks": 25, "newhireitems": 24} {
		if n := len(e.s.regLoad(regDefs[name])); n < min {
			t.Errorf("%s starter has %d, want %d", name, n, min)
		}
	}
	// the one-time additions arrived exactly once
	e.get2("/api/guides")
	e.s.mu.Lock()
	e.s.seedOnce("guides-229", "guides", "guides_add.json")
	e.s.mu.Unlock()
	found := 0
	for _, g := range e.s.regLoad(regDefs["guides"]) {
		if g["id"] == "GDE-102" {
			found++
		}
	}
	if found != 1 {
		t.Fatalf("LOTO guide count %d", found)
	}
	// LOTO log and removal
	if c, _ := e.post("/api/reg/lotolog", `{"date":"2026-11-12","equipment":"Slicer","reason":"Blade change","authorized":"Brian"}`); c != 200 {
		t.Fatal("lotolog refused")
	}
	_, o := e.get2("/api/reg/lotolog")
	if o["rows"].([]any)[0].(map[string]any)["state"] != "Locked Out" {
		t.Fatal("not shown as locked out")
	}
	// new hire: progress survives an edit that omits it
	e.post("/api/reg/newhire", `{"employee":"Sam","role":"Cook","status":"In Progress","progress":"NHI-01|2026-11-12"}`)
	e.post("/api/reg/newhire", `{"id":"NH-0001","employee":"Sam","role":"Cook","status":"In Progress","trainer":"Ana"}`)
	if r := e.s.regLoad(regDefs["newhire"])[0]; r["progress"] == "" || r["trainer"] != "Ana" {
		t.Fatalf("hidden progress lost: %v", r)
	}
	_, o = e.get2("/api/reg/newhire")
	if st := o["rows"].([]any)[0].(map[string]any)["state"].(string); !strings.HasPrefix(st, "1 of ") {
		t.Fatalf("state %q", st)
	}
	// walk
	c, o := e.post("/api/walk", `{"service":"Dinner","by":"Ana","overall":"No","items":[{"n":3,"area":"Hot Line","check":"x","status":"Needs Attention","finding":"Pilot out","priority":"P2 - Urgent"},{"n":4,"area":"Prep","check":"y","status":"Ready"}]}`)
	if c != 200 || len(o["need"].([]any)) != 1 {
		t.Fatalf("walk: %d %v", c, o)
	}
	if r := e.s.regLoad(regDefs["walks"])[0]; !strings.Contains(r["issues"], "Pilot out") {
		t.Fatal("walk issues missing")
	}
	if c, _ := e.post("/api/walk", `{"service":"Dinner","by":" ","items":[]}`); c == 200 {
		t.Fatal("unsigned walk accepted")
	}
	// reports
	for _, id := range []string{"binder-all", "reg:incidents", "reg:walks", "rec:lotolog:LOTO-0001", "lototag:LP-001"} {
		if rep, ok := e.s.buildReport(id, ""); !ok || len(rep.Sections) == 0 {
			t.Errorf("report %s missing", id)
		}
	}
	// materials
	dir := e.s.materialsDir()
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "Deck.pptx"), []byte("x"), 0o644)
	_, o = e.get2("/api/materials")
	if len(o["files"].([]any)) != 1 {
		t.Fatal("materials not listed")
	}
	if c, _ := e.post("/api/materials/open", `{"name":"../x"}`); c == 200 {
		t.Fatal("path escape accepted")
	}
	if c, _ := e.post("/api/materials/open", `{"name":"Deck.pptx"}`); c != 200 {
		t.Fatal("open refused")
	}
}

func TestLotoFollowsOutOfService(t *testing.T) {
	e := newEnv(t)
	e.post("/api/reg/lotoprocs", `{"equipment":"Slicer","assetId":"K-9","energy":"Electrical | 120V | panel\nMechanical | blade | guard"}`)
	e.s.mu.Lock()
	eq := &Equipment{ID: "e1", Name: "Slicer", Area: "Prep", AssetCode: "K-9"}
	e.s.holdStart(eq, "Blade guard cracked", "Brian", "")
	e.s.mu.Unlock()
	rows := e.s.regLoad(regDefs["lotolog"])
	if len(rows) != 1 || rows[0]["equipment"] != "Slicer" || rows[0]["energy"] != "Electrical, Mechanical" || rows[0]["authorized"] != "Brian" || rows[0]["procId"] != "LP-001" {
		t.Fatalf("log not filled in: %v", rows)
	}
	if rep, ok := e.s.buildReport("lototag:"+rows[0]["id"], ""); !ok || !strings.Contains(rep.html(true), "Blade guard cracked") || !strings.Contains(rep.html(true), "Electrical, Mechanical") {
		t.Fatal("tag not pre-filled")
	}
	oos := e.s.oosList()
	oos[0].ClearedBy, oos[0].ClearNote = "Scott", "Guard replaced"
	e.s.mu.Lock()
	e.s.lotoEnd(oos[0])
	e.s.mu.Unlock()
	if r := e.s.regLoad(regDefs["lotolog"])[0]; r["removed"] == "" || !strings.Contains(r["notes"], "Guard replaced") {
		t.Fatalf("lock not marked removed: %v", r)
	}
}

func TestEmployeeMealsAndAttention(t *testing.T) {
	e := newEnv(t)
	if c, _ := e.post("/api/reg/mealrecipes", `{"name":"Pasta & Sausage","serves":"10","ingredients":"2 lb | sausage | 9.50\n3 lb | rigatoni | 5.50","active":"Yes"}`); c != 200 {
		t.Fatal("recipe refused")
	}
	var id string
	_, o := e.get2("/api/reg/mealrecipes")
	for _, x := range o["rows"].([]any) {
		if r := x.(map[string]any); r["name"] == "Pasta & Sausage" {
			id = r["id"].(string)
			if r["cost"] != "$15.00" || r["perServing"] != "$1.50" {
				t.Fatalf("costing wrong: %v", r)
			}
		}
	}
	if id == "" {
		t.Fatal("new recipe missing")
	}
	e.post("/api/reg/mealplan", `{"date":"`+time.Now().Format(dayFmt)+`","service":"Lunch","recipeId":"`+id+`","mealName":"Pasta & Sausage","servings":"8"}`)
	_, o = e.get2("/api/reg/mealplan")
	if r := o["rows"].([]any)[0].(map[string]any); r["cost"] != "$12.00" {
		t.Fatalf("plan cost wrong: %v", r)
	}
	if rep, ok := e.s.buildReport("meals-costs", ""); !ok || !strings.Contains(rep.html(true), "$12.00") {
		t.Fatal("cost report")
	}
	// attention: a lock left on, and an unreviewed binder
	e.post("/api/reg/lotolog", `{"date":"2026-01-01","equipment":"Slicer","reason":"x","authorized":"Brian"}`)
	titles := ""
	for _, it := range e.s.safetyAttention(time.Now().Format(dayFmt)) {
		titles += it.Title + ";"
	}
	if !strings.Contains(titles, "Lockout still on: Slicer") || !strings.Contains(titles, "binder review") {
		t.Fatalf("attention items: %s", titles)
	}
}

func TestMealRotation(t *testing.T) {
	e := newEnv(t)
	// anchor week 1, so the rotation repeats every four weeks
	mon, _ := time.Parse(dayFmt, "2026-08-31")
	for i, want := range []int{1, 2, 3, 4, 1, 2} {
		if got := rotationWeek("2026-08-31", mon.AddDate(0, 0, 7*i)); got != want {
			t.Fatalf("week %d: got %d want %d", i, got, want)
		}
	}
	if got := rotationWeek("2026-08-31", mon.AddDate(0, 0, -7)); got != 4 {
		t.Fatalf("before anchor: %d", got)
	}
	_, cov := e.get2("/api/meals/coverage")
	ws := cov["weeks"].([]any)
	have := []int{}
	for _, w := range ws {
		have = append(have, 14-len(w.(map[string]any)["missing"].([]any)))
	}
	if have[0] != 14 || have[1] != 14 || have[2] != 14 || have[3] != 14 {
		t.Fatalf("coverage %v", have)
	}
	_, d := e.get2("/api/meals/dash")
	if d["have"].(float64) != 56 || len(d["days"].([]any)) != 7 {
		t.Fatalf("dash %v", d["have"])
	}
	_, app := e.get2("/api/reg/mealappendix")
	if len(app["rows"].([]any)) != 17 {
		t.Fatalf("appendix rows %d", len(app["rows"].([]any)))
	}
	if c, o := e.post("/api/meals/settings", `{"anchor":"nope","weekdayPeople":30,"weekendPeople":40,"minutes":30}`); c != 400 {
		t.Fatalf("bad anchor accepted %v", o)
	}
	if rep, ok := e.s.buildReport("mealweek:3", ""); !ok || !strings.Contains(rep.html(true), "Week 3") {
		t.Fatal("week report")
	}
	if rep, ok := e.s.buildReport("meal:EMR-001", ""); !ok || !strings.Contains(rep.html(true), "Ingredients") {
		t.Fatal("recipe report")
	}
}

func TestRoleTrainingMatrix(t *testing.T) {
	e := newEnv(t)
	e.post("/api/reg/employees", `{"name":"Dana","position":"Dish Machine","department":"Dish","status":"Active"}`)
	e.post("/api/reg/employees", `{"name":"Pat","position":"Line Cook","department":"Prep","status":"Active"}`)
	e.s.mu.Lock()
	m := e.s.roleMatrix()
	e.s.mu.Unlock()
	roles := m["roles"].([]roleRowOut)
	if len(roles) < 5 || roles[0].Role != "All Staff" {
		t.Fatalf("roles: %v", len(roles))
	}
	var dish roleRowOut
	for _, r := range roles {
		if r.Role == "Dish" {
			dish = r
		}
	}
	if dish.People != 1 || !dish.Cells["TT-008"].Required || dish.Cells["TT-011"].Required {
		t.Fatalf("dish row wrong: %+v", dish.Cells["TT-008"])
	}
	// excuse Dish from Allergen Awareness and give Dish a 6 month Handwashing refresh
	e.post("/api/reg/trainreq", `{"role":"Dish","typeId":"TT-003","required":"No"}`)
	e.post("/api/reg/trainreq", `{"role":"Dish","typeId":"TT-004","required":"Yes","refresh":"6"}`)
	_, mx := e.get2("/api/training/matrix")
	for _, r := range mx["rows"].([]any) {
		row := r.(map[string]any)
		if row["name"] == "Dana" {
			cells := row["cells"].(map[string]any)
			if _, ok := cells["TT-003"]; ok {
				t.Fatal("excused training still on Dana's row")
			}
			if _, ok := cells["TT-008"]; !ok {
				t.Fatal("dish training missing for Dana")
			}
		}
		if row["name"] == "Pat" {
			if _, ok := row["cells"].(map[string]any)["TT-003"]; !ok {
				t.Fatal("Pat should still need allergen training")
			}
		}
	}
	code, _ := e.post("/api/reg/training", `{"employeeId":"EMP-001","typeId":"TT-004","completed":"2026-01-15"}`)
	if code != 200 {
		t.Fatal("record refused")
	}
	for _, r := range e.s.regLoad(regDefs["training"]) {
		if r["typeId"] == "TT-004" && r["expires"] != "2026-07-15" {
			t.Fatalf("refresh override ignored: %v", r["expires"])
		}
	}
	if rep, ok := e.s.buildReport("training-roles", ""); !ok || !strings.Contains(rep.html(true), "Dish") {
		t.Fatal("role report")
	}
}

func TestDashboardExtras(t *testing.T) {
	e := newEnv(t)
	now := time.Now().Format(dayFmt)
	e.post("/api/reg/incidents", `{"date":"`+now+`","person":"Sam","type":"Near Miss","severity":"Minor","what":"Wet floor","status":"Open"}`)
	e.post("/api/reg/mealplan", `{"date":"`+now+`","service":"Lunch","mealName":"Pasta","servings":"10"}`)
	_, d := e.get2("/api/dashboard")
	pr := d["progress"].(map[string]any)
	if len(pr["parts"].([]any)) != 4 || pr["total"].(float64) < 1 {
		t.Fatalf("progress %v", pr)
	}
	if _, ok := d["alerts"].([]any); !ok {
		t.Fatal("alerts missing")
	}
	got := map[string]float64{}
	for _, m := range d["metrics"].([]any) {
		mm := m.(map[string]any)
		got[mm["label"].(string)] = mm["now"].(float64)
	}
	if len(got) != 8 || got["Incidents & near misses"] != 1 || got["Employee meals logged"] != 1 {
		t.Fatalf("metrics %v", got)
	}
}

func TestPacketSummaryAndStations(t *testing.T) {
	e := newEnv(t)
	now := time.Now().Format(dayFmt)
	rep, ok := e.s.buildReport("packet:"+time.Now().AddDate(0, 0, -29).Format(dayFmt)+":"+now+":", "")
	if !ok {
		t.Fatal("packet not built")
	}
	h := rep.html(true)
	for _, w := range []string{"Part 1: Food Safety", "Part 4: Chemical Safety", "1.1  Temperature log", "Prepared by", "Emergency response binder"} {
		if !strings.Contains(h, w) {
			t.Fatalf("packet missing %q", w)
		}
	}
	if _, ok := e.s.buildReport("packet:2026-02-01:2026-01-01:", ""); ok {
		t.Fatal("reversed dates accepted")
	}
	if r, _ := e.s.buildReport("packet:"+now+":"+now+":temps,ca", ""); !strings.Contains(r.html(true), "Part 2: Corrective Actions &amp; Safety Events") || strings.Contains(r.html(true), "Equipment list") {
		t.Fatal("part selection ignored")
	}
	// summary list is validated and the copy folder gets the daily zip
	if c, _ := e.post("/api/summary/settings", `{"recipients":"nope"}`); c != 400 {
		t.Fatal("bad address accepted")
	}
	dst := filepath.Join(t.TempDir(), "OneDrive copy")
	if c, o := e.post("/api/summary/settings", `{"recipients":"a@b.com, c@d.org","copyFolder":`+strconv.Quote(dst)+`}`); c != 200 {
		t.Fatalf("settings: %v", o)
	}
	e.s.dailyBackup()
	ents, _ := os.ReadDir(dst)
	if len(ents) != 1 || !strings.HasPrefix(ents[0].Name(), "Daily Backup ") {
		t.Fatalf("copy folder: %v", ents)
	}
	if sum := e.s.dailySummaryReport().text(); !strings.Contains(sum, "Overdue:") {
		t.Fatal("summary text")
	}
	// stations: a Saute cook is covered by "Sauté" and Stewards by Dish
	cook := rec{"position": "Cook", "stations": "Sauce Jack, Sauté"}
	if !trainApplies("Saute", cook) || !trainApplies("sauce jack", cook) || trainApplies("Fryer", cook) {
		t.Fatal("station matching")
	}
	if !trainApplies("Dish", rec{"position": "Steward"}) || !trainApplies("Bread & Dessert", rec{"stations": "Bread and Desserts"}) {
		t.Fatal("dish/bread alias")
	}
}

func TestFoodCosting(t *testing.T) {
	e := newEnv(t)
	// $62 for a 15 lb case of bacon, $3.60 for a 5 lb bag of flour... via the paste box
	_, o := e.post("/api/costing/prices/import", `{"text":"Bacon | 60.00 | 15 | lb | Sysco\nEggs | 4.80 | 12 | each\nMozzarella, Shredded | 36 | 12 | lb\nOlive Oil | 24 | 1 | gal\nBad Line | x | 1 | lb"}`)
	if o["added"].(float64) != 4 || len(o["bad"].([]any)) != 1 {
		t.Fatalf("import: %v", o)
	}
	_, o = e.post("/api/costing/prices/import", `{"text":"Bacon | 75.00 | 15 | lb"}`)
	if o["updated"].(float64) != 1 || o["added"].(float64) != 0 {
		t.Fatalf("update: %v", o)
	}
	// 2 lb bacon @5.00 = 10, 8 oz mozzarella @3.00/lb = 1.50, 6 eggs @0.40 = 2.40, 2 tbsp olive oil, 1 cup flour (no price), salt as needed
	_, c := e.post("/api/costing/calc", `{"ingredients":"2 lb | Bacon strips\n8 oz | Shredded Mozzarella\n6 | Large Eggs\n2 tbsp | Olive Oil\n1 cup | All Purpose Flour\nAs needed | Salt","portions":"4","price":"14"}`)
	total := c["total"].(float64)
	oil := 24.0 / 3785.41 * 14.7868 * 2
	want := 10 + 1.5 + 2.4 + oil
	if total < want-0.01 || total > want+0.01 {
		t.Fatalf("total %v want %v: %v", total, want, c["lines"])
	}
	if c["missing"].(float64) != 1 || c["perPortion"].(float64) < total/4-0.001 {
		t.Fatalf("missing/perPortion wrong: %v", c)
	}
	if pct := c["pct"].(float64); pct < 25 || pct > 26 {
		t.Fatalf("food cost pct %v", pct)
	}
	// a typed cost wins over the price list; a weight against a count price is refused
	_, c = e.post("/api/costing/calc", `{"ingredients":"2 lb | Bacon | 3.00\n1 lb | Eggs","portions":"1","price":"10"}`)
	lines := c["lines"].([]any)
	if lines[0].(map[string]any)["cost"].(float64) != 3 || lines[1].(map[string]any)["priced"].(bool) {
		t.Fatalf("override/mismatch: %v", lines)
	}
	// specials list shows the computed columns, and employee meals now cost from the price list
	e.post("/api/reg/specials", `{"name":"BLT Pizza","portions":"4","menuPrice":"14","ingredients":"2 lb | Bacon\n6 | Eggs"}`)
	_, l := e.get2("/api/reg/specials")
	r := l["rows"].([]any)[0].(map[string]any)
	if r["perPortion"] != "$3.10" || r["foodPct"] == "" {
		t.Fatalf("special row: %v", r)
	}
	e.post("/api/reg/mealrecipes", `{"name":"Costed Meal","week":"1","day":"Monday","meal":"Dinner","serves":"10","ingredients":"4 lb | Bacon"}`)
	_, l = e.get2("/api/reg/mealrecipes")
	for _, x := range l["rows"].([]any) {
		if m := x.(map[string]any); m["name"] == "Costed Meal" && m["perServing"] != "$2.00" {
			t.Fatalf("meal cost: %v", m)
		}
	}
	if rep, ok := e.s.buildReport("special:"+r["id"].(string), ""); !ok || !strings.Contains(rep.html(true), "Bacon") {
		t.Fatal("special report")
	}
	if q, u, ok := parseQty("1 1/2 cups"); !ok || q != 1.5 || u != "cup" {
		t.Fatalf("parseQty %v %v", q, u)
	}
}

func TestAutoSendSummary(t *testing.T) {
	e := newEnv(t)
	if c, _ := e.post("/api/summary/settings", `{"recipients":"","autoSend":"16:00"}`); c != 400 {
		t.Fatal("auto send without recipients accepted")
	}
	if c, _ := e.post("/api/summary/settings", `{"recipients":"a@b.com","autoSend":"25:99"}`); c != 400 {
		t.Fatal("bad time accepted")
	}
	if c, _ := e.post("/api/summary/settings", `{"recipients":"a@b.com","autoSend":"16:00"}`); c != 200 {
		t.Fatal("settings refused")
	}
	sent := 0
	var fail error
	send := func(to []string, subject, body string) error {
		if fail != nil {
			return fail
		}
		sent++
		return nil
	}
	day := time.Now().Truncate(24 * time.Hour)
	at := func(h, m int) time.Time { return time.Date(day.Year(), day.Month(), day.Day(), h, m, 0, 0, time.Local) }
	e.s.autoSendTick(at(15, 59), send)
	if sent != 0 {
		t.Fatal("sent before the set time")
	}
	fail = errors.New("outlook is closed")
	e.s.autoSendTick(at(16, 0), send)
	e.s.autoSendTick(at(16, 10), send) // inside the 30 minute wait: no second try
	if st := e.s.summarySettings(); st.LastError == "" || st.LastSent != "" {
		t.Fatalf("failure not recorded: %+v", st)
	}
	fail = nil
	e.s.autoSendTick(at(16, 31), send)
	e.s.autoSendTick(at(17, 5), send) // already sent today
	if sent != 1 {
		t.Fatalf("sent %d times", sent)
	}
	if st := e.s.summarySettings(); st.LastError != "" || st.LastSent == "" {
		t.Fatalf("success not recorded: %+v", st)
	}
}

func TestDailyWorkflowAnd90Day(t *testing.T) {
	e := newEnv(t)
	_, o := e.get2("/api/dw/today")
	blocks := o["blocks"].([]any)
	if len(blocks) != 9 || o["total"].(float64) != 44 || o["done"].(float64) != 0 {
		t.Fatalf("workflow: %v blocks, %v items", len(blocks), o["total"])
	}
	first := blocks[0].(map[string]any)["items"].([]any)[0].(map[string]any)["id"].(string)
	if c, _ := e.post("/api/dw/tick", `{"itemId":"`+first+`","by":"  sam ","on":true}`); c != 200 {
		t.Fatal("tick refused")
	}
	e.post("/api/dw/tick", `{"itemId":"`+first+`","by":"sam","on":true}`) // ticking twice stays one record
	_, o = e.get2("/api/dw/today")
	if o["done"].(float64) != 1 {
		t.Fatalf("done after tick: %v", o["done"])
	}
	_, l := e.get2("/api/reg/dwlog")
	if len(l["rows"].([]any)) != 1 {
		t.Fatal("duplicate log rows")
	}
	e.post("/api/dw/tick", `{"itemId":"`+first+`","on":false}`)
	_, o = e.get2("/api/dw/today")
	if o["done"].(float64) != 0 {
		t.Fatal("untick failed")
	}
	if c, _ := e.post("/api/dw/tick", `{"itemId":"DW-999","on":true}`); c != 404 {
		t.Fatal("unknown item ticked")
	}
	// handoff item due yesterday shows as attention
	y := time.Now().AddDate(0, 0, -1).Format(dayFmt)
	e.post("/api/reg/dwhandoff", `{"id":"","issue":"Walk-in door seal","priority":"High","owner":"Sam","due":"`+y+`","status":"Open"}`)
	if a := e.s.dwAttention(time.Now().Format(dayFmt)); len(a) != 1 || a[0].State != "Overdue" {
		t.Fatalf("handoff attention: %v", a)
	}
	// 90-day goals
	_, g := e.get2("/api/g90")
	if g["total"].(float64) != 9 || len(g["objectives"].([]any)) != 6 || g["day"].(float64) != 0 {
		t.Fatalf("g90: %v", g)
	}
	if c, _ := e.post("/api/g90/start", `{"start":"nope"}`); c != 400 {
		t.Fatal("bad start accepted")
	}
	st := time.Now().AddDate(0, 0, -9).Format(dayFmt)
	e.post("/api/g90/start", `{"start":"`+st+`"}`)
	_, g = e.get2("/api/g90")
	if g["day"].(float64) != 10 || g["week"].(float64) != 2 || g["phase"].(float64) != 1 {
		t.Fatalf("g90 day/week/phase: %v %v %v", g["day"], g["week"], g["phase"])
	}
	e.post("/api/reg/goals90", `{"id":"G90-001","title":"Three suitable mop stations","status":"Blocked","target":"`+y+`"}`)
	if a := e.s.g90Attention(time.Now().Format(dayFmt)); len(a) != 1 {
		t.Fatalf("g90 attention: %v", a)
	}
	if rep, ok := e.s.g90Report(), true; !ok || len(rep.Sections) != 3 {
		t.Fatal("g90 report")
	}
}

func TestOperationsLists(t *testing.T) {
	e := newEnv(t)
	y := time.Now().AddDate(0, 0, -1).Format(dayFmt)
	e.post("/api/reg/vendors", `{"id":"","name":"Pest Co","category":"Pest Control","contractEnd":"`+y+`"}`)
	e.post("/api/reg/vendors", `{"id":"","name":"Sysco","category":"Food","nextService":"2999-01-01"}`)
	_, o := e.get2("/api/reg/vendors")
	rows := o["rows"].([]any)
	got := map[string]string{}
	for _, r := range rows {
		m := r.(map[string]any)
		got[m["name"].(string)], _ = m["state"].(string)
	}
	if got["Pest Co"] != "Expired" || got["Sysco"] != "" {
		t.Fatalf("vendor states: %v", got)
	}
	e.post("/api/reg/forecasts", `{"id":"","date":"`+y+`","covers":"100","actual":"85"}`)
	_, o = e.get2("/api/reg/forecasts")
	if v := o["rows"].([]any)[0].(map[string]any)["variance"]; v != "-15 (-15%)" {
		t.Fatalf("variance %v", v)
	}
	_, o = e.get2("/api/reg/smallware")
	if len(o["rows"].([]any)) != 14 || o["rows"].([]any)[0].(map[string]any)["state"] != "Set par" {
		t.Fatal("smallware seed")
	}
	e.post("/api/reg/smallware", `{"id":"SW-001","item":"Dinner plates","category":"China","par":"200","onHand":"150"}`)
	_, o = e.get2("/api/reg/smallware")
	if o["rows"].([]any)[0].(map[string]any)["state"] != "Short" {
		t.Fatal("short not flagged")
	}
	_, o = e.get2("/api/reg/winterprep")
	if len(o["rows"].([]any)) != 12 {
		t.Fatal("winter seed")
	}
	a := e.s.opsAttention(time.Now().Format(dayFmt))
	if len(a) != 2 {
		t.Fatalf("attention %v", a)
	}
}

func TestStationBook(t *testing.T) {
	e := newEnv(t)
	rep, ok := e.s.guideReport("stationbook:Salad")
	if !ok || rep.Title != "Salad Station Book" {
		t.Fatal("no station book")
	}
	groups, html := 0, 0
	for _, sec := range rep.Sections {
		if sec.Group {
			groups++
		}
		if sec.HTML != "" {
			html++
		}
	}
	if groups < 8 || html < 7 {
		t.Fatalf("salad book: %d banners, %d guides", groups, html)
	}
	if rep, _ := e.s.guideReport("stationbook:Nowhere"); len(rep.Sections) != 1 {
		t.Fatal("empty station should say so")
	}
}

func TestSelfInspection(t *testing.T) {
	e := newEnv(t)
	_, o := e.get2("/api/selfinsp")
	areas := o["areas"].([]any)
	if len(areas) != 20 {
		t.Fatalf("areas %d", len(areas))
	}
	var lines []string
	for i, a := range areas {
		id := a.(map[string]any)["id"].(string)
		r := "C"
		switch i {
		case 0:
			r = "CR"
		case 1:
			r = "M"
		case 2:
			r = "COS"
		case 3:
			r = "NA"
		}
		lines = append(lines, id+"|"+r+"|note")
	}
	if c, _ := e.post("/api/selfinsp", `{"date":"2026-11-02","by":"","results":"x"}`); c != 400 {
		t.Fatal("missing inspector accepted")
	}
	if c, _ := e.post("/api/selfinsp", `{"date":"2026-11-02","by":"Sam","results":"SIA-001|Z"}`); c != 400 {
		t.Fatal("bad rating accepted")
	}
	body := `{"date":"2026-11-02","by":"Sam","makeCA":true,"results":"` + strings.Join(lines, `\n`) + `"}`
	c, o := e.post("/api/selfinsp", body)
	if c != 200 || o["made"].(float64) != 2 {
		t.Fatalf("save %d %v", c, o)
	}
	_, l := e.get2("/api/selfinsp")
	row := l["rows"].([]any)[0].(map[string]any)
	// 19 rated areas: deductions 5+3+1 = 9 of 95 -> 100-9*100/95 = 91
	if row["score"] != "91" || row["band"] != "Red" || row["critical"] != "1" {
		t.Fatalf("score row: %v", row)
	}
	e.post("/api/selfinsp", `{"id":"`+row["id"].(string)+`","date":"2026-11-02","by":"Sam","critResolved":"Yes","results":"`+strings.Join(lines, `\n`)+`"}`)
	_, l = e.get2("/api/selfinsp")
	if l["rows"].([]any)[0].(map[string]any)["band"] != "Yellow" {
		t.Fatalf("band after resolving critical: %v", l["rows"])
	}
	// next week: same major again flags a repeat and stays yellow even at a high score
	e.post("/api/selfinsp", `{"date":"2026-11-09","by":"Sam","results":"SIA-002|M|again\nSIA-003|C|"}`)
	_, m := e.get2("/api/selfinsp/month?month=2026-11")
	if m["count"].(float64) != 2 || m["criticals"].(float64) != 1 {
		t.Fatalf("month: %v", m)
	}
	if rep, ok := e.s.selfReport("selfinsp-month"); !ok || len(rep.Sections) == 0 {
		t.Fatal("monthly report")
	}
	if a := e.s.selfAttention(time.Date(2026, 11, 12, 9, 0, 0, 0, time.Local)); len(a) != 0 {
		t.Fatal("alert too early")
	}
	if a := e.s.selfAttention(time.Date(2026, 11, 20, 9, 0, 0, 0, time.Local)); len(a) != 1 {
		t.Fatalf("overdue self-inspection alert: %v", a)
	}
	if rep, ok := e.s.selfReport("selfinsp-blank"); !ok || len(rep.Sections[0].Rows) != 20 {
		t.Fatal("blank form")
	}
	if len(e.s.caForms()) != 2 {
		t.Fatalf("CA forms %d", len(e.s.caForms()))
	}
}

func TestMealUpdate238(t *testing.T) {
	e := newEnv(t)
	_, o := e.get2("/api/reg/mealrecipes")
	names := map[string]bool{}
	for _, r := range o["rows"].([]any) {
		names[r.(map[string]any)["name"].(string)] = true
	}
	if !names["Grilled Cheese with Fries"] || names["Grilled Cheese with Cream of Tomato Soup"] || len(o["rows"].([]any)) != 56 {
		t.Fatalf("recipes after update: %d", len(o["rows"].([]any)))
	}
	_, a := e.get2("/api/reg/mealappendix")
	if len(a["rows"].([]any)) != 17 {
		t.Fatalf("appendix %d", len(a["rows"].([]any)))
	}
}

func TestRecipeBook(t *testing.T) {
	e := newEnv(t)
	e.post("/api/costing/prices/import", `{"text":"Bacon | 60.00 | 15 | lb"}`)
	code, o := e.post("/api/reg/recipes", `{"name":"BLT","category":"Entree","status":"Active","portions":"10","menuPrice":"5","ingredients":"5 lb | Bacon","procedure":"Cook bacon | oven | 20 min | 350F | Yes"}`)
	if code != 200 {
		t.Fatalf("add recipe %d %v", code, o)
	}
	rows := o["rows"].([]any)
	r := rows[0].(map[string]any)
	if r["perPortion"] != "$2.00" || r["flag"] != "REVIEW" {
		t.Fatalf("cost/flag wrong: %v %v", r["perPortion"], r["flag"])
	}
	id := r["id"].(string)
	// first save sets the baseline, so no history yet
	e.post("/api/rbook/recost", `{}`)
	_, h := e.get2("/api/reg/costhist")
	if len(h["rows"].([]any)) != 0 {
		t.Fatalf("baseline should not log: %v", h["rows"])
	}
	// a price change on the list is logged
	_, p := e.get2("/api/reg/prices")
	pid := p["rows"].([]any)[0].(map[string]any)["id"].(string)
	e.post("/api/reg/prices", `{"id":"`+pid+`","ingredient":"Bacon","packPrice":"75.00","packSize":"15","unit":"lb"}`)
	_, h = e.get2("/api/reg/costhist")
	hr := h["rows"].([]any)
	if len(hr) != 1 || hr[0].(map[string]any)["newCost"] != "2.50" {
		t.Fatalf("history after price change: %v", hr)
	}
	// editing the selling price is logged with the typed reason
	e.post("/api/reg/recipes", `{"id":"`+id+`","name":"BLT","category":"Entree","status":"Active","portions":"10","menuPrice":"6","ingredients":"5 lb | Bacon","procedure":"Cook bacon | oven | 20 min | 350F | Yes","changeReason":"Menu reprice"}`)
	_, h = e.get2("/api/reg/costhist")
	hr = h["rows"].([]any)
	if len(hr) != 2 || hr[0].(map[string]any)["reason"] != "Menu reprice" {
		t.Fatalf("history after price edit: %v", hr)
	}
	rr, _ := e.get("/api/reports/rbook:" + id + "?print=1")
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), "Cook bacon") {
		t.Fatalf("recipe sheet: %d", rr.Code)
	}
}

func TestEventCosting(t *testing.T) {
	e := newEnv(t)
	e.post("/api/costing/prices/import", `{"text":"Bacon | 60.00 | 15 | lb"}`)
	_, o := e.post("/api/reg/recipes", `{"name":"BLT","category":"Entree","status":"Active","portions":"10","menuPrice":"5","ingredients":"5 lb | Bacon"}`)
	rid := o["rows"].([]any)[0].(map[string]any)["id"].(string)
	_, o = e.post("/api/reg/events", `{"name":"Smith Wedding","date":"2026-12-05","serviceType":"Buffet","guests":"100","quotedPrice":"1000","status":"Confirmed"}`)
	eid := o["rows"].([]any)[0].(map[string]any)["id"].(string)
	// 100 servings at $2.00 (from the recipe) with 10% buffer = $220
	e.post("/api/reg/eventmenu", `{"event":"`+eid+`","item":"BLT trays","recipeId":"`+rid+`","servings":"100","waste":"10"}`)
	e.post("/api/reg/eventcost", `{"event":"`+eid+`","costType":"Labor","description":"Servers","qty":"10","rate":"20"}`)
	_, o = e.get2("/api/reg/events")
	r := o["rows"].([]any)[0].(map[string]any)
	if r["foodCost"] != "$220.00" || r["otherCost"] != "$200.00" || r["totalCost"] != "$420.00" || r["margin"] != "58.0%" || r["band"] != "GOOD" {
		t.Fatalf("event calc wrong: %v", r)
	}
	_, s := e.get2("/api/events/summary")
	if s["confirmed"].(float64) != 1 || s["revenue"].(float64) != 1000 {
		t.Fatalf("summary: %v", s)
	}
	if c, _ := e.post("/api/events/settings", `{"good":20,"watch":30}`); c != 400 {
		t.Fatalf("bad thresholds accepted: %d", c)
	}
	e.post("/api/events/settings", `{"good":70,"watch":50}`)
	_, o = e.get2("/api/reg/events")
	if o["rows"].([]any)[0].(map[string]any)["band"] != "WATCH" {
		t.Fatalf("band after threshold change: %v", o["rows"])
	}
	e.post("/api/reg/eventres", `{"event":"`+eid+`","revenue":"1100","foodActual":"250","otherActual":"200","review":"Ran long"}`)
	_, o = e.get2("/api/reg/eventres")
	rr := o["rows"].([]any)[0].(map[string]any)
	if rr["profit"] != "$650.00" || rr["variance"] != "$70.00" {
		t.Fatalf("result calc: %v", rr)
	}
	rep, _ := e.get("/api/reports/event:" + eid + "?print=1")
	if rep.Code != 200 || !strings.Contains(rep.Body.String(), "BLT trays") || !strings.Contains(rep.Body.String(), "Ran long") {
		t.Fatalf("cost sheet: %d", rep.Code)
	}
}

func TestReferenceLibrary(t *testing.T) {
	e := newEnv(t)
	_, o := e.get2("/api/reference")
	files := o["files"].([]any)
	if len(files) != 2 {
		t.Fatalf("expected the 2 bundled documents, got %v", files)
	}
	if c, _ := e.post("/api/reference/open", `{"name":"../x.pdf"}`); c != 400 {
		t.Fatalf("path escape accepted: %d", c)
	}
	if c, _ := e.post("/api/reference/open", `{"name":"missing.pdf"}`); c != 404 {
		t.Fatalf("missing file: %d", c)
	}
	n := files[0].(map[string]any)["name"].(string)
	if c, _ := e.post("/api/reference/open", `{"name":"`+n+`"}`); c != 200 {
		t.Fatalf("open: %d", c)
	}
	_, g := e.get2("/api/guides/GDE-112")
	if g["title"] == nil && g["html"] == nil {
		t.Fatalf("64-17 summary guide missing: %v", g)
	}
}

func TestMealBinderAndPlanFields(t *testing.T) {
	e := newEnv(t)
	rr, _ := e.get("/api/reports/meal-binder?print=1")
	b := rr.Body.String()
	if rr.Code != 200 || !strings.Contains(b, "EMPLOYEE MEAL SYSTEM &amp; RECIPE BINDER") || !strings.Contains(b, "binder-cover-logo.png") || !strings.Contains(b, "Appendix Recipes") || !strings.Contains(b, "Production Guides") {
		t.Fatalf("meal binder: %d", rr.Code)
	}
	if strings.Index(b, "EMPLOYEE MEAL SYSTEM &amp; RECIPE BINDER") > strings.Index(b, "Monthly Menus") {
		t.Fatal("cover is not first")
	}
	if lb, err := uiFiles.ReadFile("ui/binder-cover-logo.png"); err != nil || len(lb) < 1000 {
		t.Fatalf("cover logo not embedded: %v", err)
	}
	_, o := e.post("/api/reg/mealplan", `{"date":"2026-11-16","service":"Dinner","mealName":"Chicken Parm","sides":"Pasta","servings":"12","assignedTo":"Sam","prepStart":"3:00 PM","status":"Planned","overstock":"Yes"}`)
	r := o["rows"].([]any)[0].(map[string]any)
	if r["assignedTo"] != "Sam" || r["status"] != "Planned" || r["sides"] != "Pasta" || r["overstock"] != "Yes" {
		t.Fatalf("plan fields not kept: %v", r)
	}
}
