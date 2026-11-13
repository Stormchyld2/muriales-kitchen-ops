package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// A registry collection is a simple list of records with a fixed set of
// fields. It backs the cleaning schedule, standards, logs and similar lists.
type fieldDef struct {
	Key   string   `json:"key"`
	Label string   `json:"label"`
	Type  string   `json:"type"` // text, long, select, date, yn
	Opts  []string `json:"opts,omitempty"`
	Req   bool     `json:"req,omitempty"`
	Src   string   `json:"src,omitempty"` // for a "ref" field: the list its choices come from
}

type collDef struct {
	Name      string     `json:"name"`
	Title     string     `json:"title"`
	NewLabel  string     `json:"newLabel"`
	Prefix    string     `json:"-"`
	Digits    int        `json:"-"`
	Fields    []fieldDef `json:"fields"`
	ListCols  []string   `json:"listCols"`
	Deletable bool       `json:"deletable"`
	Desc      bool       `json:"desc"` // newest first
	Report    string     `json:"report"`
	Starter   string     `json:"-"`
	Intro     string     `json:"intro,omitempty"`
	// After runs when a record is changed (old and new copies), with s.mu held.
	After func(s *store, old, nw rec) `json:"-"`
}

type rec map[string]string

var (
	freqOpts = []string{"Each Shift", "Daily", "Weekly", "Biweekly", "Monthly", "Quarterly", "Semi-Annual", "Annual", "As Needed"}
	prioOpts = []string{"P1 - Critical", "P2 - Urgent", "P3 - Routine"}
)

func txt(k, l string, req bool) fieldDef { return fieldDef{Key: k, Label: l, Type: "text", Req: req} }
func lng(k, l string, req bool) fieldDef { return fieldDef{Key: k, Label: l, Type: "long", Req: req} }
func dat(k, l string, req bool) fieldDef { return fieldDef{Key: k, Label: l, Type: "date", Req: req} }
func yn(k, l string) fieldDef            { return fieldDef{Key: k, Label: l, Type: "yn"} }
func sel(k, l string, o []string, req bool) fieldDef {
	return fieldDef{Key: k, Label: l, Type: "select", Opts: o, Req: req}
}

var regDefs = map[string]*collDef{
	"cleantasks": {Name: "cleantasks", Title: "Cleaning Schedule", NewLabel: "Add Cleaning Task", Prefix: "CLN-", Digits: 4, Deletable: true, Report: "clean-schedule", Starter: "clean_tasks.json",
		Intro:    "The master list of what gets cleaned, how often, by whom and to which standard. Use Done on a task to record it; the Today's Checklist screen works from this list.",
		ListCols: []string{"id", "area", "task", "frequency", "role", "priority", "due", "state"},
		Fields: []fieldDef{sel("type", "Task type", []string{"Area", "Equipment", "Periodic"}, true), txt("area", "Area / equipment", true), lng("task", "Cleaning task", true), sel("frequency", "Frequency", freqOpts, true), txt("cadence", "Cadence in days (leave blank to use the frequency)", false),
			txt("timing", "Shift / timing", false), txt("role", "Assigned role", false), txt("backup", "Backup role", false), txt("chem", "Chemical / tool ref", false), txt("standard", "Standard ID", false), sel("priority", "Priority", prioOpts, true),
			yn("verify", "Verification required?"), yn("foodContact", "Food-contact surface?"), yn("deep", "Deep clean?"), yn("active", "Active?"), dat("lastDone", "Last completed", false), lng("notes", "Notes", false)}},
	"standards": {Name: "standards", Title: "Cleaning Standards", NewLabel: "Add Standard", Prefix: "STD-", Digits: 3, Deletable: true, Report: "clean-standards", Starter: "clean_standards.json",
		Intro:    "Short, practical standards so the crew knows what clean means for each job.",
		ListCols: []string{"id", "title", "appliesTo", "status", "revision"},
		Fields: []fieldDef{txt("title", "Standard / task", true), txt("appliesTo", "Applies to", false), lng("prep", "Preparation", false), lng("steps", "Cleaning steps", false), lng("sanitize", "Sanitize / rinse step", false), txt("tools", "Tools / chemical ref", false), txt("ppe", "PPE", false),
			lng("verification", "Verification standard", false), lng("safety", "Safety notes", false), dat("revision", "Revision date", false), sel("status", "Status", []string{"Active", "Draft", "Retired"}, true), lng("notes", "Notes", false)}},
	"cleanref": {Name: "cleanref", Title: "Chemical & Tool Reference", NewLabel: "Add Chemical or Tool", Prefix: "CT-", Digits: 3, Deletable: true, Report: "clean-chem",
		Intro:    "What chemical or tool is approved for each use, with dilution, PPE, food-contact handling and storage. The product's safety data sheet is in the SDS Manager.",
		ListCols: []string{"id", "name", "type", "use", "dilution", "foodContactApproved", "ppe", "storage"},
		Fields: []fieldDef{txt("name", "Chemical / tool", true), sel("type", "Type", []string{"Chemical", "Tool", "Equipment", "PPE"}, true), lng("use", "Primary use", false), lng("areas", "Approved areas / surfaces", false), lng("doNotUse", "Do not use on", false), txt("dilution", "Dilution / concentration", false),
			txt("contact", "Contact time", false), yn("rinse", "Rinse required?"), yn("foodContactApproved", "Food-contact approved?"), txt("ppe", "PPE required", false), txt("storage", "Storage location", false), yn("sdsOnFile", "SDS on file?"), dat("sdsReview", "SDS review date", false), txt("vendor", "Vendor / product ref", false), lng("notes", "Notes", false)}},
	"mgrchecks": {Name: "mgrchecks", Title: "Manager Inspection Schedule", NewLabel: "Add Inspection", Prefix: "KOM-", Digits: 3, Deletable: true, Report: "mgr-schedule", Starter: "mgr_checks.json",
		Intro:    "The recurring checks the Kitchen Operations Manager walks. Use Done to record one in the Manager Inspection Log.",
		ListCols: []string{"id", "frequency", "check", "area", "owner", "priority", "due", "state"},
		Fields: []fieldDef{sel("frequency", "Frequency", freqOpts, true), txt("check", "Inspection / check", true), txt("area", "Area", false), txt("timing", "Preferred timing", false), txt("owner", "Owner", false), lng("whatCheck", "What I check", false), txt("record", "Record / log", false),
			sel("priority", "Priority", prioOpts, true), yn("active", "Active?"), dat("lastDone", "Last completed", false), lng("notes", "Notes", false)}},
	"cleanlog": {Name: "cleanlog", Title: "Cleaning Completion Log", NewLabel: "Add Entry", Prefix: "LOG-", Digits: 5, Desc: true, Report: "clean-log",
		Intro:    "The permanent record of cleaning completed. Entries are added when a task is marked Done. Open an entry to verify it.",
		ListCols: []string{"id", "date", "shift", "area", "task", "completedBy", "source", "verifyStatus", "issue"},
		Fields: []fieldDef{dat("date", "Date", true), txt("shift", "Shift", false), txt("taskId", "Task ID", false), txt("area", "Area / equipment", true), lng("task", "Cleaning task", true), txt("completedBy", "Completed by", true), txt("time", "Completion time", false),
			txt("verifiedBy", "Verified by", false), txt("verifyTime", "Verification time", false), yn("issue", "Issue found?"), sel("verifyStatus", "Verification status", []string{"Pending", "Verified", "Failed", "Not required"}, false), txt("caRef", "Corrective action ref", false), txt("source", "Where it came from", false), lng("notes", "Notes", false)}},
	"missed": {Name: "missed", Title: "Missed & Overdue Log", NewLabel: "Report Cleaning Issue", Prefix: "MISS-", Digits: 4, Desc: true, Report: "clean-missed",
		Intro:    "Anything missed, overdue, rejected or not done to standard, and how it was put right.",
		ListCols: []string{"id", "date", "area", "task", "reason", "source", "priority", "assignedTo", "target", "status"},
		Fields: []fieldDef{dat("date", "Date identified", true), txt("taskId", "Task ID", false), txt("area", "Area / equipment", true), lng("task", "Cleaning task", true), lng("reason", "Reason missed / failed", true), sel("priority", "Priority", prioOpts, true), txt("assignedTo", "Assigned to", false),
			dat("target", "Target correction date", false), sel("status", "Status", []string{"Open", "In Progress", "Corrected", "Verified"}, true), dat("corrected", "Corrected date", false), txt("verifiedBy", "Verified by", false), yn("repeat", "Repeat issue?"), txt("source", "Where it came from", false), lng("action", "Corrective action", false), lng("notes", "Notes", false)}},
	"deepclean": {Name: "deepclean", Title: "Weekly Deep Clean", NewLabel: "Add Deep-Clean Job", Prefix: "DEEP-", Digits: 3, Deletable: true, Report: "clean-weekly", Starter: "clean_deep.json",
		Intro:    "The deep-clean job for each day of the week. Done logs it in the Completion Log; a job not done by the end of its day shows up in Missed & Overdue.",
		ListCols: []string{"id", "day", "area", "task", "standard", "role", "due", "state"},
		Fields:   []fieldDef{sel("day", "Day", []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}, true), txt("area", "Area / equipment", true), lng("task", "Cleaning task", true), txt("standard", "Standard ID", false), txt("role", "Assigned role", false), txt("chem", "Chemical / tool ref", false), yn("active", "Active?"), dat("lastDone", "Last completed", false), lng("notes", "Notes", false)}},
	"cleanareas": {Name: "cleanareas", Title: "Area Cleaning Matrix", NewLabel: "Add Area Task", Prefix: "AREA-", Digits: 3, Deletable: true, Report: "clean-areas", Starter: "clean_areas.json",
		Intro:    "How each part of the kitchen is cleaned: the feature, task, how often, who, with what, and what gets inspected.",
		ListCols: []string{"id", "area", "feature", "task", "frequency", "timing", "role", "chem", "standard", "inspectionPoint"},
		Fields: []fieldDef{txt("area", "Area", true), txt("feature", "Feature / surface", false), lng("task", "Cleaning task", true), txt("frequency", "Frequency", false), txt("timing", "Timing", false), txt("role", "Assigned role", false), txt("chem", "Chemical / tool ref", false), txt("standard", "Standard ID", false),
			yn("foodContact", "Food-contact surface?"), txt("deep", "Deep clean?", false), txt("verify", "Verification", false), txt("inspectionPoint", "Inspection point", false), yn("active", "Active?"), lng("notes", "Notes", false)}},
	"chemquick": {Name: "chemquick", Title: "Exposure Quick Reference", NewLabel: "Add Reference Row", Prefix: "QR-", Digits: 3, Deletable: true, Report: "chem-response", Starter: "chem_quick.json",
		Intro:    "What to do for each kind of chemical exposure, spill or mix-up. Edit any row, or add your own, to match your products and procedures.",
		ListCols: []string{"id", "category", "situation", "immediate", "doNot", "escalate", "manager"},
		Fields: []fieldDef{sel("category", "Category", []string{"Eye Contact", "Skin Contact", "Inhalation / Fumes", "Ingestion", "Spill / Leak", "Mixing / Wrong Product", "Equipment / Gas", "Food & Food Contact", "Other"}, true), txt("situation", "Situation", true), lng("immediate", "Immediate action", true), lng("doNot", "Do not", false), txt("ppe", "PPE", false),
			yn("sds", "SDS needed?"), lng("escalate", "Call emergency help when", false), lng("manager", "Manager action", false), yn("active", "Active?"), lng("notes", "Notes", false)}},
	"mgrlog": {Name: "mgrlog", Title: "Manager Inspection Log", NewLabel: "Add Entry", Prefix: "INSP-", Digits: 4, Desc: true, Report: "mgr-log",
		Intro:    "Your record of receiving, storage, cleaning, temperature and other manager checks. Entries are added when an inspection is marked Done.",
		ListCols: []string{"id", "date", "check", "area", "result", "by", "caNeeded", "followUp"},
		Fields: []fieldDef{dat("date", "Date", true), txt("inspectionId", "Inspection ID", false), txt("area", "Area", false), txt("check", "Inspection / check", true), sel("result", "Result", []string{"Pass", "Needs Attention", "N/A"}, true), txt("by", "Checked by", true),
			yn("caNeeded", "Corrective action needed?"), txt("followUp", "Follow-up ref (CA number)", false), txt("verified", "Verified / closed by", false), lng("notes", "Notes", false)}},
}

func (s *store) regFile(c *collDef) string { return "reg-" + c.Name + ".json" }

func (s *store) regLoad(c *collDef) []rec {
	out := []rec{}
	if _, err := os.Stat(filepath.Join(s.maintDir(), s.regFile(c))); err != nil && c.Starter != "" {
		if b, err := starterFiles.ReadFile("starter/" + c.Starter); err == nil {
			_ = json.Unmarshal(b, &out)
			_ = s.writeMaint(s.regFile(c), out)
		}
		return out
	}
	s.readMaint(s.regFile(c), &out)
	return out
}

func (s *store) regSave(c *collDef, rows []rec) error { return s.writeMaint(s.regFile(c), rows) }

func (c *collDef) validate(r rec) string {
	for _, f := range c.Fields {
		v := strings.TrimSpace(r[f.Key])
		r[f.Key] = v
		switch {
		case f.Req && v == "":
			return f.Label + " is needed."
		case v == "":
		case f.Type == "date":
			if _, err := time.Parse(dayFmt, v); err != nil {
				return f.Label + " is not a valid date."
			}
		case f.Type == "yn" && v != "Yes" && v != "No":
			return f.Label + ": choose Yes or No."
		case f.Type == "ref":
		case f.Type == "select" && !oneOf(f.Opts, v):
			return f.Label + ": choose one of the listed options."
		}
		if f.Type == "doc" {
			if len(v) > 300000 {
				return f.Label + " is too long."
			}
			continue
		}
		if len(v) > 4000 {
			return f.Label + " is too long."
		}
	}
	return ""
}

func (c *collDef) nextID(rows []rec) string {
	max := 0
	for _, r := range rows {
		if strings.HasPrefix(r["id"], c.Prefix) {
			if n, err := strconv.Atoi(strings.TrimPrefix(r["id"], c.Prefix)); err == nil && n > max {
				max = n
			}
		}
	}
	return fmt.Sprintf("%s%0*d", c.Prefix, c.Digits, max+1)
}

// regAdd appends a record with a new ID. The caller holds s.mu.
func (s *store) regAdd(c *collDef, r rec) (rec, error) {
	rows := s.regLoad(c)
	n := rec{}
	for _, f := range c.Fields {
		n[f.Key] = strings.TrimSpace(r[f.Key])
	}
	n["id"] = c.nextID(rows)
	n["created"] = time.Now().Format("2006-01-02 15:04")
	return n, s.regSave(c, append(rows, n))
}

func (s *store) regSorted(c *collDef) []rec {
	rows := s.regLoad(c)
	sort.SliceStable(rows, func(i, j int) bool {
		if c.Desc {
			return rows[i]["id"] > rows[j]["id"]
		}
		return rows[i]["id"] < rows[j]["id"]
	})
	return rows
}

func (s *store) regReport(c *collDef, title, note string, filter func(rec) bool) *report {
	cols, keys := []string{}, []string{}
	for _, f := range c.Fields {
		if f.Type == "long" && len(c.Fields) > 9 && c.Name != "standards" {
			continue
		}
		cols, keys = append(cols, f.Label), append(keys, f.Key)
	}
	cols, keys = append([]string{"ID"}, cols...), append([]string{"id"}, keys...)
	rows := [][]string{}
	for _, r := range s.regSorted(c) {
		if filter != nil && !filter(r) {
			continue
		}
		row := []string{}
		for _, k := range keys {
			row = append(row, r[k])
		}
		rows = append(rows, row)
	}
	return &report{Title: title, Subtitle: time.Now().Format("January 2, 2006"), Sections: []reportSection{{Note: note, Cols: cols, Rows: rows}}}
}

func (s *store) registryRoutes(mux *http.ServeMux) {
	get := func(w http.ResponseWriter, r *http.Request) *collDef {
		c := regDefs[r.PathValue("c")]
		if c == nil {
			fail(w, 404, "Not found.")
		}
		return c
	}
	mux.HandleFunc("GET /api/reg/{c}", func(w http.ResponseWriter, r *http.Request) {
		c := get(w, r)
		if c == nil {
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		if c.Name == "missed" {
			s.syncMissed()
		}
		writeJSON(w, 200, map[string]any{"def": c, "rows": s.decorate(c, s.regSorted(c))})
	})
	mux.HandleFunc("POST /api/reg/{c}", func(w http.ResponseWriter, r *http.Request) {
		c := get(w, r)
		if c == nil {
			return
		}
		var in2 rec
		if jsonDecode(r, &in2) != nil {
			fail(w, 400, "Could not read that.")
			return
		}
		if msg := c.validate(in2); msg != "" {
			fail(w, 400, msg)
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		if c.Name == "training" {
			s.trainingExpiry(in2)
		}
		rows := s.regLoad(c)
		if id := in2["id"]; id != "" {
			found := false
			for i, x := range rows {
				if x["id"] == id {
					n := rec{}
					for k, v := range x {
						n[k] = v
					}
					for _, f := range c.Fields {
						n[f.Key] = in2[f.Key]
					}
					if c.After != nil {
						defer c.After(s, x, n)
					}
					rows[i], found = n, true
				}
			}
			if !found {
				fail(w, 404, "That record was not found.")
				return
			}
			if err := s.regSave(c, rows); err != nil {
				fail(w, 500, err.Error())
				return
			}
		} else if _, err := s.regAdd(c, in2); err != nil {
			fail(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"def": c, "rows": s.decorate(c, s.regSorted(c))})
	})
	mux.HandleFunc("DELETE /api/reg/{c}/{id}", func(w http.ResponseWriter, r *http.Request) {
		c := get(w, r)
		if c == nil {
			return
		}
		if !c.Deletable {
			fail(w, 400, "Records in this list are kept.")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		rows, keep := s.regLoad(c), []rec{}
		for _, x := range rows {
			if x["id"] != r.PathValue("id") {
				keep = append(keep, x)
			}
		}
		if err := s.regSave(c, keep); err != nil {
			fail(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"def": c, "rows": s.regSorted(c)})
	})
}
