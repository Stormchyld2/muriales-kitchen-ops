package main

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
