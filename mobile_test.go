package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
