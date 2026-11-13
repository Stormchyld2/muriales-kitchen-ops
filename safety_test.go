package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
