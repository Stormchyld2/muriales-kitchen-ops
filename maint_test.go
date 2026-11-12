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
	_, out = e.do2("PUT", "/api/maint/issues/"+iid, `{"status":"Repaired","by":"M","cost":250}`)
	if adv(out)["total"].(float64) != 3750 {
		t.Errorf("issue cost should join the total: %v", adv(out))
	}
}

func (e *env) do2(method, path, body string) (int, map[string]any) {
	return e.do(httptest.NewRequest(method, path, strings.NewReader(body)))
}
