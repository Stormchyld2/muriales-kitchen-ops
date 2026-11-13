package main

// Training & safety program: Lockout/Tagout, safety meetings, incidents,
// new-hire training, the Daily Readiness Walk, the Emergency Binder and the
// Training Materials folder. Everything is kept in plain files in
// Maintenance Data, the same as the other lists.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var (
	trainCats   = []string{"Food Safety", "Personal Hygiene", "Allergen Awareness", "Chemical Safety", "Equipment Safety", "Knife Safety", "Slip / Fall Prevention", "Fire / Emergency", "Stewarding / Dish", "Receiving / Storage", "Temperature Control", "Cleaning / Sanitation", "Maintenance / Lockout", "Manager / Supervisor", "Other"}
	incTypes    = []string{"Injury", "Near Miss", "Burn", "Cut / Laceration", "Chemical Exposure", "Property Damage", "Food Safety", "Equipment", "Slip / Trip / Fall", "Other"}
	walkService = []string{"Breakfast/Lunch", "Dinner", "Both", "Prep"}
	hiddenF     = func(k string) fieldDef { return fieldDef{Key: k, Label: k, Type: "hidden"} }
	nhRoles     = []string{"Cook", "Prep", "Dish / Stewarding", "Salad", "Receiving", "Kitchen Operations Manager"}
)

func init() {
	regDefs["lotoprocs"] = &collDef{Name: "lotoprocs", Title: "LOTO Procedures", NewLabel: "Add LOTO Procedure", Prefix: "LP-", Digits: 3, Deletable: true, Report: "reg:lotoprocs",
		Intro:    "One written lockout/tagout procedure for each piece of equipment with more than one energy source, stored energy or complicated isolation. Print gives the full procedure; Tag prints a lock tag for it.",
		ListCols: []string{"id", "equipment", "assetId", "area", "role", "qualElec", "lastReview", "nextInspection"},
		Fields: []fieldDef{txt("equipment", "Equipment / system", true), txt("assetId", "Asset ID", false), txt("area", "Area", false), txt("model", "Manufacturer / model", false), txt("role", "Primary authorized role", false), yn("qualElec", "Qualified electrical person required?"),
			lng("shutdown", "Normal shutdown method", false), docF("energy", "Energy sources and isolation points (one per line: type | size or condition | source | isolation device and location | lockable? | lock or device | stored energy and how it is controlled | how verified)", false),
			docF("steps", "Machine-specific steps (one per line: action | device or location | lock or tag point | stored energy action | try-start check | test needed | how it is restored)", false),
			txt("photo", "Photo / diagram reference", false), txt("maintainx", "MaintainX asset reference", false), dat("lastReview", "Last procedure review", false), dat("nextInspection", "Next annual inspection", false), txt("approvedBy", "Approved by", false), lng("notes", "Notes", false)}}
	regDefs["lotolog"] = &collDef{Name: "lotolog", Title: "LOTO Log", NewLabel: "Record Lockout", Prefix: "LOTO-", Digits: 4, Desc: true, Report: "reg:lotolog",
		Intro:    "Record each lockout. Use Lock Removed when the work is done and the locks are off. This log does not replace each employee's own lock and tag.",
		ListCols: []string{"id", "date", "equipment", "reason", "authorized", "zero", "removed", "state"},
		Fields: []fieldDef{dat("date", "Date applied", true), txt("time", "Time applied", false), txt("equipment", "Equipment / circuit", true), txt("assetId", "Asset / panel ID", false), txt("area", "Area", false), lng("reason", "Reason / work", true), txt("energy", "Energy types", false),
			txt("authorized", "Authorized employee", true), txt("qualified", "Electrical qualified person", false), txt("locks", "Locks applied", false), yn("zero", "Zero energy verified?"), txt("removed", "Date / time removed", false), lng("notes", "Notes", false), hiddenF("oosId"), hiddenF("procId")}}
	regDefs["lotoinsp"] = &collDef{Name: "lotoinsp", Title: "Annual LOTO Inspection", NewLabel: "Record Inspection", Prefix: "LOTO-PI-", Digits: 3, Desc: true, Report: "reg:lotoinsp",
		Intro:    "The yearly check of each lockout procedure and of the program. The person inspecting should not be the one who normally uses the procedure.",
		ListCols: []string{"id", "procedure", "date", "inspector", "accurate", "following", "nextReview"},
		Fields: []fieldDef{txt("procedure", "Procedure / equipment", true), dat("date", "Inspection date", true), txt("inspector", "Inspector", true), yn("notUser", "Inspector is not a user of the procedure?"), yn("accurate", "Procedure accurate?"), yn("following", "Employees following it?"),
			yn("reviewed", "Authorized employee review done?"), lng("deficiencies", "Deficiencies / changes", false), txt("caRef", "Corrective action ref", false), dat("nextReview", "Next review", false)}}
	regDefs["meetings"] = &collDef{Name: "meetings", Title: "Safety Meetings", NewLabel: "Record Meeting", Prefix: "MTG-", Digits: 4, Desc: true, Deletable: true, Report: "reg:meetings",
		Intro:    "Safety meetings, toolbox talks and pre-shift reviews. List who attended by first name.",
		ListCols: []string{"id", "date", "type", "topic", "ledBy", "attended", "followUp"},
		Fields: []fieldDef{dat("date", "Date", true), sel("type", "Meeting type", []string{"Safety Meeting", "Toolbox Talk", "Pre-Shift Review", "Food Safety", "Retraining", "Other"}, true), txt("topic", "Topic", true), txt("reason", "Reason / trigger", false), txt("ledBy", "Led by", true),
			txt("expected", "How many expected", false), txt("attended", "How many attended", false), lng("who", "Who attended", false), yn("followUp", "Follow-up needed?"), txt("caRef", "Corrective action ref", false), txt("signoffRef", "Sign-in sheet reference", false), dat("nextReview", "Next review / follow-up", false), lng("notes", "Notes", false)}}
	regDefs["incidents"] = &collDef{Name: "incidents", Title: "Incidents & Near Misses", NewLabel: "Record Incident / Near Miss", Prefix: "INC-", Digits: 4, Desc: true, Report: "reg:incidents",
		Intro:    "The kitchen's own record of injuries, close calls and safety events. Use Corrective Action when something has to change.",
		ListCols: []string{"id", "date", "person", "area", "type", "severity", "status"},
		Fields: []fieldDef{dat("date", "Date", true), txt("time", "Time", false), txt("person", "Employee / person", true), txt("area", "Area / location", false), sel("type", "Type", incTypes, true), sel("severity", "Severity", []string{"Minor", "Moderate", "Serious", "Critical"}, true),
			lng("what", "What happened", true), lng("immediate", "Immediate action taken", false), yn("medical", "Medical attention?"), txt("notified", "Manager notified", false), yn("caNeeded", "Corrective action needed?"), txt("caRef", "Corrective action ref", false), dat("followUp", "Follow-up date", false),
			sel("status", "Status", []string{"Open", "In Progress", "Waiting", "Completed", "Closed", "Cancelled"}, true), lng("notes", "Notes", false)}}
	regDefs["newhireitems"] = &collDef{Name: "newhireitems", Title: "New Hire Checklist Items", NewLabel: "Add Item", Prefix: "NHI-", Digits: 2, Deletable: true, Starter: "newhire_items.json", Report: "reg:newhireitems",
		Intro:    "The items on every new-hire training checklist. Change, add or remove items to suit the kitchen.",
		ListCols: []string{"id", "area", "task", "type", "required", "signoff"},
		Fields:   []fieldDef{txt("area", "Training area", true), lng("task", "Topic / task", true), txt("type", "Training type", false), yn("required", "Required?"), yn("signoff", "Sign-off required?"), txt("ref", "SOP reference", false), yn("active", "Active?")}}
	regDefs["newhire"] = &collDef{Name: "newhire", Title: "New Hire Training", NewLabel: "Start New Hire Checklist", Prefix: "NH-", Digits: 4, Desc: true, Report: "reg:newhire",
		Intro:    "One checklist for each new hire or transfer. Open it to tick off each item as it is taught.",
		ListCols: []string{"id", "employee", "role", "hireDate", "trainer", "state"},
		Fields: []fieldDef{txt("employee", "Employee", true), sel("role", "Position / role", nhRoles, true), dat("hireDate", "Hire / transfer date", false), txt("trainer", "Trainer", false), dat("target", "Target completion date", false), hiddenF("progress"),
			sel("status", "Status", []string{"In Progress", "Complete"}, true), yn("ready", "Ready for independent work?"), lng("strengths", "Strengths / good progress", false), lng("gaps", "Training gaps / concerns", false), lng("retrain", "Retraining plan", false), lng("comments", "Employee comments", false),
			txt("trainerSign", "Trainer initials", false), txt("managerSign", "Manager initials", false), dat("signDate", "Final sign-off date", false), txt("trackerRef", "Training record ID", false), txt("signoffRef", "Signed sheet reference", false)}}
	regDefs["walkchecks"] = &collDef{Name: "walkchecks", Title: "Readiness Walk Checks", NewLabel: "Add Check", Prefix: "WC-", Digits: 2, Deletable: true, Starter: "walk_checks.json", Report: "reg:walkchecks",
		Intro:    "The checks on the Daily Readiness Walk, in order. Change, add or remove checks to suit the kitchen.",
		ListCols: []string{"id", "area", "check", "active"},
		Fields:   []fieldDef{txt("area", "Area", true), lng("check", "Readiness check", true), yn("active", "Active?")}}
	regDefs["walks"] = &collDef{Name: "walks", Title: "Readiness Walk History", NewLabel: "Add Entry", Prefix: "RDY-", Digits: 4, Desc: true, Report: "reg:walks",
		Intro:    "One line for each Daily Readiness Walk that was finished.",
		ListCols: []string{"id", "date", "service", "by", "overall", "p1", "oos", "sanit", "supply"},
		Fields: []fieldDef{dat("date", "Date", true), sel("service", "Service", walkService, true), txt("time", "Walk time", false), txt("by", "Completed by", true), yn("overall", "Overall ready?"), yn("p1", "P1 issue found?"), yn("oos", "Equipment out of service?"), txt("oosAsset", "Equipment / asset", false), txt("wo", "Work order ref", false),
			yn("sanit", "Sanitation follow-up?"), yn("supply", "Supply follow-up?"), txt("release", "Service released by", false), txt("reviewedBy", "Manager review", false), docF("issues", "Items needing attention", false), lng("notes", "Notes", false)}}
	regDefs["binder"] = &collDef{Name: "binder", Title: "Emergency Binder Index", NewLabel: "Add Binder Tab", Prefix: "BND-", Digits: 2, Deletable: true, Starter: "binder_index.json", Report: "reg:binder",
		Intro:    "The front page of the kitchen emergency binder: what each tab holds, where it is kept and when it was last reviewed.",
		ListCols: []string{"id", "tab", "doc", "location", "owner", "lastReviewed", "current"},
		Fields:   []fieldDef{txt("section", "Section", true), txt("tab", "Binder tab", false), txt("doc", "Document / reference", true), lng("purpose", "Purpose", false), txt("location", "Location / system", false), txt("owner", "Owner", false), dat("lastReviewed", "Last reviewed", false), yn("current", "Current?")}}
	regDefs["econtacts"] = &collDef{Name: "econtacts", Title: "Emergency Contacts", NewLabel: "Add Contact", Prefix: "EC-", Digits: 3, Deletable: true, Starter: "econtacts.json", Report: "reg:econtacts",
		Intro:    "Phone numbers for emergencies and escalation. Fill in the numbers and mark the date each was last checked.",
		ListCols: []string{"id", "type", "company", "purpose", "phone", "afterHours", "lastVerified"},
		Fields:   []fieldDef{txt("type", "Contact type", true), txt("company", "Company / person", false), txt("purpose", "Purpose", false), txt("phone", "Primary phone", false), txt("afterHours", "After-hours phone", false), txt("account", "Account / reference", false), dat("lastVerified", "Last verified", false), lng("notes", "Notes", false)}}
	regDefs["eqplan"] = &collDef{Name: "eqplan", Title: "Critical Equipment Emergency Plan", NewLabel: "Add Equipment Plan", Prefix: "EQP-", Digits: 3, Deletable: true, Starter: "eqplan.json", Report: "reg:eqplan",
		Intro:    "What to do when a critical piece of equipment fails: protect people and food first, take unsafe equipment out of service, and keep running if it is safe.",
		ListCols: []string{"id", "type", "safety", "food", "notify", "stop"},
		Fields:   []fieldDef{txt("type", "Equipment type", true), lng("safety", "Immediate safety action", false), lng("food", "Food / product protection", false), lng("workaround", "Operational workaround", false), lng("notify", "Who to notify", false), txt("vendor", "Service / vendor", false), lng("stop", "When to stop service", false), lng("records", "Required records", false), lng("recovery", "Recovery check", false), lng("notes", "Notes", false)}}
	regDefs["eqvendors"] = &collDef{Name: "eqvendors", Title: "Equipment Service Contacts", NewLabel: "Add Vendor", Prefix: "EQV-", Digits: 3, Deletable: true, Starter: "eqvendors.json", Report: "reg:eqvendors",
		Intro:    "Who to call to fix each kind of equipment, day or night.",
		ListCols: []string{"id", "equipment", "vendor", "phone", "afterHours", "backup", "lastVerified"},
		Fields:   []fieldDef{txt("equipment", "Equipment / system", true), txt("vendor", "Primary vendor", false), txt("phone", "Primary phone", false), txt("afterHours", "After-hours", false), txt("account", "Account / contract ref", false), txt("backup", "Backup vendor", false), dat("lastVerified", "Last verified", false), lng("notes", "Notes", false)}}
	regDefs["binderlog"] = &collDef{Name: "binderlog", Title: "Binder Review Log", NewLabel: "Record Review", Prefix: "EB-", Digits: 4, Desc: true, Report: "reg:binderlog",
		Intro:    "Each time the emergency binder is checked. Review it at least every quarter.",
		ListCols: []string{"id", "date", "by", "tabs", "contacts", "shutoffs", "changes"},
		Fields:   []fieldDef{dat("date", "Review date", true), txt("by", "Reviewed by", true), yn("tabs", "Tabs complete?"), yn("contacts", "Contacts verified?"), yn("shutoffs", "Shutoff info verified?"), lng("changes", "Changes made", false), lng("notes", "Notes", false)}}
}

// seedOnce adds starter records that a new version brought, one time only,
// so records the kitchen later deletes do not come back.
func (s *store) seedOnce(tag, coll, file string) {
	flag := filepath.Join(s.maintDir(), "seed-"+tag+".flag")
	if _, err := os.Stat(flag); err == nil {
		return
	}
	c := regDefs[coll]
	b, err := starterFiles.ReadFile("starter/" + file)
	if err != nil {
		return
	}
	var add []rec
	if json.Unmarshal(b, &add) != nil {
		return
	}
	rows := s.regLoad(c)
	have := map[string]bool{}
	for _, r := range rows {
		have[r["id"]] = true
	}
	for _, r := range add {
		if !have[r["id"]] {
			rows = append(rows, r)
		}
	}
	if s.regSave(c, rows) == nil {
		_ = os.MkdirAll(s.maintDir(), 0o755)
		_ = os.WriteFile(flag, []byte(time.Now().Format(time.RFC3339)), 0o644)
	}
}

func (s *store) materialsDir() string {
	return filepath.Join(filepath.Dir(s.dir), "Training Materials")
}

func (s *store) safetyRoutes(mux *http.ServeMux) {
	s.mu.Lock()
	s.seedOnce("guides-229", "guides", "guides_add.json")
	s.seedOnce("types-229", "trainingtypes", "training_types_add.json")
	s.mu.Unlock()

	mux.HandleFunc("POST /api/walk", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Service, By, Notes, Overall, P1, Oos, OosAsset, Wo, Sanit, Supply, Release, Reviewed string
			Items                                                                                []struct {
				N                                                        int
				Area, Check, Status, Finding, Corrected, Priority, Owner string
			}
		}
		if jsonDecode(r, &in) != nil || strings.TrimSpace(in.By) == "" {
			fail(w, 400, "Type your first name so the walk can be signed.")
			return
		}
		var iss []string
		need := []string{}
		for _, it := range in.Items {
			if it.Status != "Needs Attention" && it.Status != "Corrected" {
				continue
			}
			line := fmt.Sprintf("#%d %s - %s: %s", it.N, it.Area, it.Status, strings.TrimSpace(it.Finding))
			if it.Priority != "" || it.Owner != "" {
				line += " [" + strings.TrimSpace(it.Priority+" "+it.Owner) + "]"
			}
			iss = append(iss, line)
			if it.Status == "Needs Attention" && it.Corrected != "Yes" {
				need = append(need, it.Area+": "+strings.TrimSpace(it.Finding))
			}
		}
		now := time.Now()
		s.mu.Lock()
		defer s.mu.Unlock()
		rowRec := rec{"date": now.Format(dayFmt), "service": in.Service, "time": now.Format("3:04 PM"), "by": strings.TrimSpace(in.By), "overall": in.Overall, "p1": in.P1, "oos": in.Oos, "oosAsset": in.OosAsset, "wo": in.Wo,
			"sanit": in.Sanit, "supply": in.Supply, "release": in.Release, "reviewedBy": in.Reviewed, "issues": strings.Join(iss, "\n"), "notes": in.Notes}
		if msg := regDefs["walks"].validate(rowRec); msg != "" {
			fail(w, 400, msg)
			return
		}
		n, err := s.regAdd(regDefs["walks"], rowRec)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"id": n["id"], "need": need})
	})

	mux.HandleFunc("GET /api/materials", func(w http.ResponseWriter, r *http.Request) {
		dir := s.materialsDir()
		_ = os.MkdirAll(dir, 0o755)
		ents, _ := os.ReadDir(dir)
		out := []map[string]any{}
		for _, e := range ents {
			if e.IsDir() || strings.HasPrefix(e.Name(), ".") || strings.HasPrefix(e.Name(), "~$") {
				continue
			}
			if fi, err := e.Info(); err == nil {
				out = append(out, map[string]any{"name": e.Name(), "size": fi.Size(), "modified": fi.ModTime().Format("2006-01-02")})
			}
		}
		sort.Slice(out, func(i, j int) bool {
			return strings.ToLower(out[i]["name"].(string)) < strings.ToLower(out[j]["name"].(string))
		})
		writeJSON(w, 200, map[string]any{"files": out, "folder": dir})
	})
	mux.HandleFunc("POST /api/materials/open", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Name string }
		if jsonDecode(r, &in) != nil || in.Name == "" || filepath.Base(in.Name) != in.Name {
			fail(w, 400, "Could not read that.")
			return
		}
		p := filepath.Join(s.materialsDir(), in.Name)
		if _, err := os.Stat(p); err != nil {
			fail(w, 404, "That file is no longer in the Training Materials folder.")
			return
		}
		openFile(p)
		writeJSON(w, 200, map[string]any{"ok": true})
	})
	mux.HandleFunc("POST /api/materials/folder", func(w http.ResponseWriter, r *http.Request) {
		_ = os.MkdirAll(s.materialsDir(), 0o755)
		openFolder(s.materialsDir())
		writeJSON(w, 200, map[string]any{"ok": true})
	})
}

// ---- reports ----

// recReport is a plain two-column sheet of one record.
func (s *store) recReport(c *collDef, r rec) *report {
	rows := [][]string{{"ID", r["id"]}}
	for _, f := range c.Fields {
		if f.Type != "hidden" && r[f.Key] != "" {
			rows = append(rows, []string{f.Label, r[f.Key]})
		}
	}
	return &report{Title: c.Title + " " + r["id"], Subtitle: time.Now().Format("January 2, 2006"), Sections: []reportSection{{Cols: []string{"Field", "Entry"}, Rows: rows}}}
}

func (s *store) safetyReport(id string) (*report, bool) {
	if id == "binder-all" {
		rep := &report{Title: "Emergency Response Binder", Subtitle: time.Now().Format("January 2, 2006")}
		for _, n := range []string{"binder", "econtacts", "eqplan", "eqvendors"} {
			c := regDefs[n]
			sub := s.regReport(c, c.Title, "", nil)
			sub.Sections[0].Heading = c.Title
			rep.Sections = append(rep.Sections, sub.Sections...)
		}
		return rep, true
	}
	if rest, ok := strings.CutPrefix(id, "rec:"); ok {
		name, rid, _ := strings.Cut(rest, ":")
		if c := regDefs[name]; c != nil {
			for _, r := range s.regLoad(c) {
				if r["id"] == rid {
					return s.recReport(c, r), true
				}
			}
		}
		return nil, false
	}
	if rid, ok := strings.CutPrefix(id, "lototag:"); ok {
		p, lg := rec{}, rec{}
		if strings.HasPrefix(rid, "LOTO-") {
			for _, r := range s.regLoad(regDefs["lotolog"]) {
				if r["id"] == rid {
					lg = r
				}
			}
			rid = lg["procId"]
		}
		for _, r := range s.regLoad(regDefs["lotoprocs"]) {
			if r["id"] == rid {
				p = r
			}
		}
		pick := func(a, b string) string { return firstNonEmpty(a, b) }
		applied := strings.TrimSpace(lg["date"] + " " + lg["time"])
		line := func(l, v string) []string { return []string{l, v} }
		rows := [][]string{line("Equipment / circuit", pick(lg["equipment"], p["equipment"])), line("Asset / panel / circuit ID", pick(lg["assetId"], p["assetId"])), line("Location", pick(lg["area"], p["area"])), line("Date / time applied", applied), line("Energy type(s)", lg["energy"]),
			line("Reason for lockout", lg["reason"]), line("Lock owner / authorized employee", lg["authorized"]), line("Department / phone", ""), line("Lock number", lg["locks"]), line("Electrical work / exposed parts?  Yes / No", ""),
			line("Zero energy verified?  Yes / No", lg["zero"]), line("Try-start completed?  Yes / No", ""), line("Lock / tag removed by", ""), line("Date / time removed", lg["removed"])}
		return &report{Title: "DANGER - DO NOT OPERATE", Subtitle: "LOCKED OUT FOR SERVICE / MAINTENANCE", Sections: []reportSection{{Cols: []string{"", ""}, Rows: rows,
			Note: "Print, cut and attach with the employee's lock. This tag is a warning device and does not replace a required lock.\nDO NOT REMOVE THIS LOCK OR TAG EXCEPT BY THE AUTHORIZED EMPLOYEE OR UNDER THE EMPLOYER'S FORMAL LOCK-REMOVAL PROCEDURE."}}}, true
	}
	if name, ok := strings.CutPrefix(id, "reg:"); ok {
		if c := regDefs[name]; c != nil {
			return s.regReport(c, c.Title, "", nil), true
		}
	}
	return nil, false
}

// ---- lockout log filled in automatically when equipment is taken out of service ----

func (s *store) lotoProcFor(e *Equipment) rec {
	for _, p := range s.regLoad(regDefs["lotoprocs"]) {
		if (e.AssetCode != "" && strings.EqualFold(p["assetId"], e.AssetCode)) || strings.EqualFold(strings.TrimSpace(p["equipment"]), strings.TrimSpace(e.Name)) {
			return p
		}
	}
	return nil
}

func energyTypes(energy string) string {
	seen, out := map[string]bool{}, []string{}
	for _, l := range strings.Split(energy, "\n") {
		t := strings.TrimSpace(strings.SplitN(l, "|", 2)[0])
		if t != "" && !seen[strings.ToLower(t)] {
			seen[strings.ToLower(t)] = true
			out = append(out, t)
		}
	}
	return strings.Join(out, ", ")
}

// lotoStart adds a LOTO Log line for a new out-of-service hold. The caller holds s.mu.
func (s *store) lotoStart(e *Equipment, o *OOSRecord) {
	proc, now := s.lotoProcFor(e), time.Now()
	r := rec{"date": now.Format(dayFmt), "time": now.Format("3:04 PM"), "equipment": e.Name, "assetId": e.AssetCode, "area": e.Area, "reason": o.Reason, "authorized": o.RemovedBy, "oosId": o.ID,
		"notes": "Added automatically when the equipment was taken out of service."}
	if proc != nil {
		r["energy"], r["procId"] = energyTypes(proc["energy"]), proc["id"]
	}
	_, _ = s.regAdd(regDefs["lotolog"], r)
}

func (s *store) lotoUpdate(oosID string, fn func(r rec)) {
	c := regDefs["lotolog"]
	rows := s.regLoad(c)
	for _, r := range rows {
		if r["oosId"] == oosID && oosID != "" && r["removed"] == "" {
			fn(r)
			_ = s.regSave(c, rows)
			return
		}
	}
}

// lotoRefine copies the details typed on the out-of-service tag form onto the log line.
func (s *store) lotoRefine(o *OOSRecord) {
	s.lotoUpdate(o.ID, func(r rec) {
		r["area"] = firstNonEmpty(o.Location, r["area"])
		r["assetId"] = firstNonEmpty(o.AssetID, r["assetId"])
		if o.Lockout == "Yes" {
			r["locks"] = firstNonEmpty(r["locks"], "1")
		}
		extra := []string{}
		if o.WorkOrder != "" {
			extra = append(extra, "Work order "+o.WorkOrder)
		}
		if o.Vendor != "" {
			extra = append(extra, "Vendor "+o.Vendor)
		}
		if o.PowerOff == "Yes" {
			extra = append(extra, "Power / gas disconnected")
		}
		if len(extra) > 0 {
			r["notes"] += " " + strings.Join(extra, ". ") + "."
		}
	})
}

// lotoEnd marks the lock removed when the equipment is cleared for service.
func (s *store) lotoEnd(o *OOSRecord) {
	s.lotoUpdate(o.ID, func(r rec) {
		r["removed"] = time.Now().Format(dayFmt + " 3:04 PM")
		r["notes"] += " Cleared for service by " + o.ClearedBy + ": " + o.ClearNote
	})
}

// safetyAttention lists the safety items that need someone today: locks still
// on, overdue procedure inspections, an overdue binder review and open incidents.
func (s *store) safetyAttention(today string) []todoItem {
	out := []todoItem{}
	for _, r := range s.regLoad(regDefs["lotolog"]) {
		if r["removed"] == "" {
			st := "Due Today"
			if r["date"] < today {
				st = "Overdue"
			}
			out = append(out, todoItem{Group: "LOTO", Title: "Lockout still on: " + r["equipment"], Detail: "since " + r["date"] + " " + r["time"], Due: r["date"], State: st, Kind: "loto", ID: r["id"]})
		}
	}
	for _, r := range s.regLoad(regDefs["lotoinsp"]) {
		if r["nextReview"] != "" && r["nextReview"] < today {
			out = append(out, todoItem{Group: "LOTO", Title: "Annual LOTO inspection overdue: " + r["procedure"], Due: r["nextReview"], State: "Overdue", Kind: "loto", ID: r["id"]})
		}
	}
	last := ""
	for _, r := range s.regLoad(regDefs["binderlog"]) {
		if r["date"] > last {
			last = r["date"]
		}
	}
	if len(s.regLoad(regDefs["binder"])) > 0 {
		cut := time.Now().AddDate(0, 0, -92).Format(dayFmt)
		if last == "" || last < cut {
			d := "never reviewed"
			if last != "" {
				d = "last reviewed " + last
			}
			out = append(out, todoItem{Group: "Binder", Title: "Emergency binder review is due", Detail: d, Due: last, State: "Overdue", Kind: "binder", ID: "binder"})
		}
	}
	for _, r := range s.regLoad(regDefs["incidents"]) {
		if (r["status"] == "Open" || r["status"] == "In Progress") && r["followUp"] != "" && r["followUp"] <= today {
			st := "Due Today"
			if r["followUp"] < today {
				st = "Overdue"
			}
			out = append(out, todoItem{Group: "Incident", Title: "Incident follow-up: " + r["type"] + " " + r["id"], Detail: r["person"], Due: r["followUp"], State: st, Kind: "incident", ID: r["id"]})
		}
	}
	return out
}
