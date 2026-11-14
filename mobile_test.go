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
