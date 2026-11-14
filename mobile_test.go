package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestPhoneGate(t *testing.T) {
	hit := ""
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hit = r.Method + " " + r.URL.Path; w.WriteHeader(200) })
	g := phoneGate(inner)
	call := func(addr, method, path, body string) int {
		hit = ""
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.RemoteAddr = addr
		w := httptest.NewRecorder()
		g.ServeHTTP(w, req)
		return w.Code
	}
	lan := "192.168.1.50:5000"
	if c := call("8.8.8.8:5000", "GET", "/m/", ""); c != 403 {
		t.Errorf("public address = %d, want 403", c)
	}
	if c := call(lan, "GET", "/m/", ""); c != 200 {
		t.Errorf("phone page = %d, want 200", c)
	}
	for _, p := range []string{"/api/library", "/api/backups", "/api/restore", "/api/reg/employees", "/api/phone", "/api/temps/points", "/api/products"} {
		if c := call(lan, "GET", p, ""); c != 403 {
			t.Errorf("GET %s = %d, want 403", p, c)
		}
		if c := call(lan, "POST", p, "{}"); c != 403 {
			t.Errorf("POST %s = %d, want 403", p, c)
		}
	}
	if c := call(lan, "DELETE", "/api/reg/breakage/BRK-00001", ""); c != 403 {
		t.Errorf("breakage delete = %d, want 403", c)
	}
	if c := call(lan, "PUT", "/api/temps/abc", "{}"); c != 403 {
		t.Errorf("temp edit = %d, want 403", c)
	}
	if c := call(lan, "POST", "/api/reg/breakage", `{"id":"BRK-00001","item":"x"}`); c != 403 {
		t.Errorf("breakage edit = %d, want 403", c)
	}
	if c := call(lan, "POST", "/api/reg/breakage", `{"item":"x"}`); c != 200 || hit != "POST /api/reg/breakage" {
		t.Errorf("breakage add = %d %q", c, hit)
	}
	for _, p := range []string{"/api/temps", "/api/parcheck", "/api/dw/today"} {
		if c := call(lan, "GET", p, ""); c != 200 || hit != "GET "+p {
			t.Errorf("GET %s = %d %q", p, c, hit)
		}
	}
	for _, p := range []string{"/api/clean/verify", "/api/maint/pm", "/api/maint/issues/x", "/api/clean/missed", "/api/maint/todos/x", "/api/maint/findings/x", "/api/maint/findings"} {
		if c := call(lan, "POST", p, "{}"); c != 403 {
			t.Errorf("POST %s = %d, want 403", p, c)
		}
	}
	for _, p := range []string{"/api/clean/done", "/api/clean/deepdone", "/api/mgr/done", "/api/maint/pm/abc/done", "/api/maint/todos", "/api/maint/todos/abc/done", "/api/walk"} {
		if c := call(lan, "POST", p, "{}"); c != 200 {
			t.Errorf("POST %s = %d, want 200", p, c)
		}
	}
	for _, p := range []string{"/api/clean/today", "/api/todo", "/api/phone/options", "/api/reg/walkchecks"} {
		if c := call(lan, "GET", p, ""); c != 200 {
			t.Errorf("GET %s = %d, want 200", p, c)
		}
	}
	if c := call(lan, "POST", "/api/notes", `{"by":"a","text":"b"}`); c != 200 {
		t.Errorf("note = %d", c)
	}
}

func TestPhoneSetting(t *testing.T) {
	t.Setenv("SDSBOOK_DATA", t.TempDir())
	e := newEnv(t)
	phone.inner = http.NotFoundHandler()
	defer e.s.phoneApply(false)
	if e.s.phoneEnabled() {
		t.Fatal("phone page should start off")
	}
	if c, m := e.post("/api/phone", `{"on":true}`); c != 200 || m["on"] != true {
		t.Fatalf("turn on = %d %v", c, m)
	}
	if !e.s.phoneEnabled() {
		t.Error("setting did not persist")
	}
	if c, m := e.post("/api/phone", `{"on":false}`); c != 200 || m["on"] != false || m["running"] != false {
		t.Fatalf("turn off = %d %v", c, m)
	}
}

func TestFixTempLimits(t *testing.T) {
	e := newEnv(t)
	lo, hi, hot, cap := -10.0, 0.0, 200.0, 33.0
	h135, h41 := 135.0, 41.0
	pts := []*TempPoint{
		{ID: "1", Name: "Back Line Single Door Cooler 2", Kind: "Freezer", Min: &lo, Max: &hi},
		{ID: "2", Name: "Cooler 1", Kind: "Cold Holding", Min: &cap, Max: &h41},
		{ID: "3", Name: "Freezer 1", Kind: "Freezer", Min: &lo, Max: &hi},
		{ID: "4", Name: "Steam", Kind: "Hot Holding", Min: &h135, Max: &hot},
		{ID: "5", Name: "Custom", Kind: "Cold Holding", Min: &lo, Max: &h41},
	}
	if err := e.s.writeMaint("temp-points.json", pts); err != nil {
		t.Fatal(err)
	}
	e.s.fixTempLimits()
	got := e.s.tempPoints()
	if got[0].Kind != "Cold Holding" || got[0].Min != nil || *got[0].Max != 41 {
		t.Errorf("cooler 2 not fixed: %+v", got[0])
	}
	if got[1].Min != nil || got[2].Min != nil || got[3].Max != nil || *got[3].Min != 135 {
		t.Error("starter floors and ceiling not cleared")
	}
	if got[4].Min == nil || *got[4].Min != -10 {
		t.Error("a point the user changed must be left alone")
	}
	if got[0].check(41) != "OK" || got[0].check(45) != "Too Warm" || got[2].check(-5) != "OK" {
		t.Error("limits are wrong after the fix")
	}
}

func TestTempBatch(t *testing.T) {
	e := newEnv(t)
	_, o := e.post("/api/temps/points/starter", `{}`)
	pts := o["points"].([]any)
	a, b := pts[0].(map[string]any)["id"].(string), pts[2].(map[string]any)["id"].(string)
	_, o = e.post("/api/temps", `{"by":"Dana","readings":[{"pointId":"`+a+`","temp":38},{"pointId":"`+b+`","temp":39}]}`)
	rs := o["readings"].([]any)
	if len(rs) != 2 {
		t.Fatalf("readings: %d", len(rs))
	}
	b0, b1 := rs[0].(map[string]any)["batch"], rs[1].(map[string]any)["batch"]
	if b0 == nil || b0 != b1 {
		t.Errorf("readings saved together should share a batch: %v %v", b0, b1)
	}
}

func TestTempDelete(t *testing.T) {
	e := newEnv(t)
	_, o := e.post("/api/temps/points/starter", `{}`)
	a := o["points"].([]any)[0].(map[string]any)["id"].(string)
	rs := []*TempReading{
		{ID: "r1", At: "2026-11-10 08:00", PointID: a, Point: "Cooler 1 Temp", Temp: 38, By: "A", Key: "k1", Source: "import"},
		{ID: "r2", At: "2026-11-10 09:00", PointID: a, Point: "Cooler 1 Temp", Temp: 39, By: "A"},
		{ID: "r3", At: "2026-11-11 09:00", PointID: a, Point: "Cooler 1 Temp", Temp: 40, By: "A"},
	}
	if err := e.s.writeMaint("temps.json", rs); err != nil {
		t.Fatal(err)
	}
	del := func(path string) int {
		req := httptest.NewRequest("DELETE", path, nil)
		w := httptest.NewRecorder()
		e.h.ServeHTTP(w, req)
		return w.Code
	}
	if c := del("/api/temps/r1"); c != 200 {
		t.Fatalf("delete one = %d", c)
	}
	var gone []string
	e.s.readMaint("temps-deleted.json", &gone)
	if len(gone) != 1 || gone[0] != "k1" {
		t.Errorf("a form reading must be remembered as deleted: %v", gone)
	}
	if c := del("/api/temps/day/2026-11-10"); c != 200 {
		t.Fatalf("delete day = %d", c)
	}
	var left []*TempReading
	e.s.readMaint("temps.json", &left)
	if len(left) != 1 || left[0].ID != "r3" {
		t.Errorf("only the other day should remain: %v", left)
	}
	if c := del("/api/temps/day/2026-11-10"); c != 404 {
		t.Errorf("empty day = %d, want 404", c)
	}
	if c := del("/api/temps/day/nope"); c != 400 {
		t.Errorf("bad day = %d, want 400", c)
	}
}

func TestPhoneStamp(t *testing.T) {
	phone.stamp = 0
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	g := phoneGate(inner)
	req := httptest.NewRequest("POST", "/api/notes", strings.NewReader(`{}`))
	req.RemoteAddr = "192.168.1.9:1"
	g.ServeHTTP(httptest.NewRecorder(), req)
	if phone.stamp != 1 {
		t.Errorf("a phone save should move the stamp, got %d", phone.stamp)
	}
	req = httptest.NewRequest("GET", "/api/temps", nil)
	req.RemoteAddr = "192.168.1.9:1"
	g.ServeHTTP(httptest.NewRecorder(), req)
	if phone.stamp != 1 {
		t.Error("reading must not move the stamp")
	}
}

func TestWalkMakesInspectionItems(t *testing.T) {
	e := newEnv(t)
	body := `{"service":"Dinner","by":"Dana","overall":"No","inspection":true,"items":[
	 {"n":1,"area":"Freezers","status":"Needs Attention","finding":"Door will not seal","priority":"P2 - Urgent","owner":"Brian","corrected":"No"},
	 {"n":2,"area":"Hot Line","status":"Corrected","finding":"Knob replaced","corrected":"Yes"},
	 {"n":3,"area":"Hand Sinks","status":"Ready"},
	 {"n":101,"area":"Dining Room","status":"Needs Attention","finding":"Light out","priority":"P3 - Routine","corrected":"No"}]}`
	c, o := e.post("/api/walk", body)
	if c != 200 || o["findings"].(float64) != 2 {
		t.Fatalf("walk: %d %v", c, o)
	}
	fs := e.s.findingList()
	if len(fs) != 2 || fs[0].Category != "Equipment" || fs[0].AssignedTo != "Brian" || fs[0].Priority != "P2 - Urgent" || fs[1].Number != 2 {
		t.Errorf("findings wrong: %+v %+v", fs[0], fs[1])
	}
	// without the flag the computer's walk adds nothing to Inspection Items
	if _, o := e.post("/api/walk", strings.Replace(body, `"inspection":true`, `"inspection":false`, 1)); o["findings"].(float64) != 0 {
		t.Error("the computer walk must not add inspection items by itself")
	}
}

func TestWalkKeepsItemsAndDeletes(t *testing.T) {
	e := newEnv(t)
	if c, o := e.post("/api/walk", `{"service":"Dinner","by":"Sam","overall":"No","items":[{"n":1,"area":"Floors","check":"x","status":"Ready"},{"n":2,"area":"Hood","check":"y","status":"Needs Attention","finding":"greasy"}]}`); c != 200 {
		t.Fatalf("walk: %d %v", c, o)
	}
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/reg/walks", nil))
	var got struct{ Rows []map[string]any }
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	rows := got.Rows
	if len(rows) != 1 || !strings.Contains(rows[0]["itemsJson"].(string), "Hood") {
		t.Fatalf("itemsJson not stored: %v", rows)
	}
	rec = httptest.NewRecorder()
	e.h.ServeHTTP(rec, httptest.NewRequest("DELETE", "/api/reg/walks/"+rows[0]["id"].(string), nil))
	if rec.Code != 200 {
		t.Fatalf("delete status %d", rec.Code)
	}
}

func TestPhoneQueuedSaveNoDoubleAndRealTime(t *testing.T) {
	e := newEnv(t)
	phone.dir, phone.seen, phone.seenLog = e.s.dir, nil, nil
	g := phoneGate(e.h)
	send := func(id string, at int64) (int, string) {
		req := httptest.NewRequest("POST", "/api/notes", strings.NewReader(`{"by":"Sam","text":"queued note"}`))
		req.RemoteAddr = "192.168.1.50:5000"
		req.Header.Set("X-Queue-Id", id)
		req.Header.Set("X-Queued-At", strconv.FormatInt(at, 10))
		w := httptest.NewRecorder()
		g.ServeHTTP(w, req)
		return w.Code, w.Body.String()
	}
	at := time.Now().Add(-3 * time.Hour).UnixMilli()
	if c, _ := send("abc123", at); c != 200 {
		t.Fatalf("first send = %d", c)
	}
	if c, b := send("abc123", at); c != 200 || !strings.Contains(b, "duplicate") {
		t.Fatalf("retry = %d %s", c, b)
	}
	e.s.mu.Lock()
	n := len(e.s.lib.Notes)
	var got string
	if n > 0 {
		got = e.s.lib.Notes[0].At
	}
	e.s.mu.Unlock()
	if n != 1 {
		t.Fatalf("notes = %d, want 1 (retry must not double)", n)
	}
	want := time.UnixMilli(at).Format("2006-01-02T15")
	if !strings.HasPrefix(got, want) {
		t.Errorf("note time %s, want it stamped with the real entry hour %s", got, want)
	}
	if clockAt.Load() != nil {
		t.Error("clock override must be cleared after the save")
	}
}

func TestWalkPhotos(t *testing.T) {
	e := newEnv(t)
	g := phoneGate(e.h)
	png := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="
	req := httptest.NewRequest("POST", "/api/photo", strings.NewReader(`{"data":"`+png+`"}`))
	req.RemoteAddr = "192.168.1.50:5000"
	w := httptest.NewRecorder()
	g.ServeHTTP(w, req)
	var r struct{ Name string }
	_ = json.Unmarshal(w.Body.Bytes(), &r)
	if w.Code != 200 || r.Name == "" {
		t.Fatalf("photo upload %d %s", w.Code, w.Body.String())
	}
	req = httptest.NewRequest("POST", "/api/photo", strings.NewReader(`{"data":"data:text/html;base64,AAAA"}`))
	req.RemoteAddr = "192.168.1.50:5000"
	w = httptest.NewRecorder()
	g.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("non-picture accepted: %d", w.Code)
	}
	body := `{"service":"Dinner","by":"Dana","overall":"No","inspection":true,"items":[{"n":1,"area":"Freezers","status":"Needs Attention","finding":"Door loose","priority":"P2 - Urgent","corrected":"No","photos":["` + r.Name + `","../../evil.jpg","nope.jpg"]}]}`
	if c, o := e.post("/api/walk", body); c != 200 || o["findings"].(float64) != 1 {
		t.Fatalf("walk %d %v", c, o)
	}
	fs := e.s.findingList()
	if len(fs) != 1 || len(fs[0].Photos) != 1 || fs[0].Photos[0] != r.Name {
		t.Errorf("photos on finding: %+v", fs[0].Photos)
	}
	if c, _ := e.get("/api/photo/" + r.Name); c.Code != 200 {
		t.Errorf("view photo = %d", c.Code)
	}
	for _, p := range []string{"/api/phone/attention", "/api/phone/sds", "/api/stchecklists", "/api/guides", "/api/phone/notes"} {
		req := httptest.NewRequest("GET", p, nil)
		req.RemoteAddr = "192.168.1.50:5000"
		w := httptest.NewRecorder()
		g.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Errorf("phone GET %s = %d", p, w.Code)
		}
	}
}

func TestUndoGuideDrafts(t *testing.T) {
	e := newEnv(t)
	b, _ := starterFiles.ReadFile("starter/guides_undo.json")
	var drafts map[string]string
	_ = json.Unmarshal(b, &drafts)
	e.s.mu.Lock()
	rows := e.s.regLoad(regDefs["guides"])
	for _, r := range rows {
		if r["id"] == "GDE-053" {
			r["body"] = drafts["GDE-053"]
		}
		if r["id"] == "GDE-052" {
			r["body"] = drafts["GDE-052"] + "\nMy own change"
		}
	}
	_ = e.s.regSave(regDefs["guides"], rows)
	_ = os.Remove(filepath.Join(e.s.maintDir(), "seed-guides-unfill.flag"))
	e.s.undoGuideDrafts()
	got := map[string]string{}
	for _, r := range e.s.regLoad(regDefs["guides"]) {
		got[r["id"]] = r["body"]
	}
	e.s.mu.Unlock()
	if !strings.HasPrefix(got["GDE-053"], "*This document was only a title page") {
		t.Error("untouched draft was not removed")
	}
	if !strings.Contains(got["GDE-052"], "My own change") {
		t.Error("an edited guide must not be changed")
	}
}
