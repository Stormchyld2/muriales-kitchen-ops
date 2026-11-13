package main

import (
	"strings"
	"testing"
)

func TestMarkdown(t *testing.T) {
	h := mdHTML("## Head\n\n**Bold** and <b>x</b>\n\n- a\n  - b\n- c\n\n1. one\n2. two\n\n| A | B |\n|---|---|\n| 1 | 2 |\n")
	for _, w := range []string{"<h3>Head</h3>", "<b>Bold</b>", "&lt;b&gt;x", "<ul><li>a<ul><li>b</li></ul></li><li>c</li></ul>", "<ol><li>one</li><li>two</li></ol>", "<th>A</th>", "<td>2</td>"} {
		if !strings.Contains(h, w) {
			t.Errorf("missing %q in %s", w, h)
		}
	}
}

func TestGuidesAndChecklists(t *testing.T) {
	e := newEnv(t)
	c, o := e.get2("/api/guides")
	if c != 200 || len(o["guides"].([]any)) < 60 {
		t.Fatalf("starter guides missing: %d", c)
	}
	_, o = e.get2("/api/guides?q=meatball")
	if len(o["guides"].([]any)) == 0 {
		t.Fatal("search found nothing")
	}
	if c, o = e.get2("/api/guides/GDE-001"); c != 200 || !strings.Contains(o["html"].(string), "<h3>") {
		t.Fatal("guide did not render")
	}
	if rep, ok := e.s.buildReport("guide:GDE-001", ""); !ok || !strings.Contains(rep.html(true), "<h3>") {
		t.Fatal("guide report")
	}
	if rep, ok := e.s.buildReport("checklist:CHK-001", ""); !ok || !strings.Contains(rep.html(true), "☐") {
		t.Fatal("checklist report")
	}
	// a long guide edit must be accepted
	long := strings.Repeat("line of text\\n", 2000)
	if c, _ := e.post("/api/reg/guides", `{"id":"GDE-001","station":"Takeout","kind":"Overview","title":"T","body":"`+long+`"}`); c != 200 {
		t.Fatalf("long guide refused: %d", c)
	}
	// finish a checklist
	if c, _ = e.post("/api/stchecklog/finish", `{"id":"CHK-001","by":"","done":[1]}`); c == 200 {
		t.Fatal("unsigned checklist accepted")
	}
	c, o = e.post("/api/stchecklog/finish", `{"id":"CHK-001","by":"Ana","done":[1,2]}`)
	if c != 200 || o["done"].(float64) != 2 || o["missed"].(float64) < 1 {
		t.Fatalf("finish: %d %v", c, o)
	}
	_, o = e.get2("/api/stchecklists")
	found := false
	for _, l := range o["lists"].([]any) {
		if m := l.(map[string]any); m["id"] == "CHK-001" && m["finished"] != nil {
			found = true
		}
	}
	if !found {
		t.Fatal("finished mark missing")
	}
	// renaming a station renames its guides
	var sid string
	for _, r := range e.s.regLoad(regDefs["stations"]) {
		if r["name"] == "Takeout" {
			sid = r["id"]
		}
	}
	if c, _ := e.post("/api/reg/stations", `{"id":"`+sid+`","name":"To-Go","active":"Yes"}`); c != 200 {
		t.Fatal("rename refused")
	}
	for _, r := range e.s.regLoad(regDefs["guides"]) {
		if r["station"] == "Takeout" {
			t.Fatal("guide kept the old station name")
		}
	}
}
