package main

// Equipment and maintenance. Everything is kept as plain files in the
// "Maintenance Data" folder beside the "SDS Data" folder. It is shared and locked
// together with the library, and every backup also saves a copy of it:
//
//	equipment.json   the equipment list
//	issues.json      reported problems with their history
//	settings.json    the MaintainX web address
//	Maintenance Log.md   a readable copy of the open and recent issues
//
// Nothing here talks to MaintainX. Work orders are entered in the MaintainX
// web portal, and the work order number can be written on the issue.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Equipment struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Area        string `json:"area"`   // storage/work area it sits in
	Type        string `json:"type"`   // walk-in, fryer, mixer ...
	Make        string `json:"make"`   //
	Model       string `json:"model"`  //
	Serial      string `json:"serial"` //
	Installed   string `json:"installed"`
	MaintainXID string `json:"maintainxId"` // optional: the asset name or number in MaintainX
	Notes       string `json:"notes"`
	Status      string `json:"status"` // Operational, Needs Attention, Out of Service

	// Asset record
	Vendor        string  `json:"vendor"`
	PurchaseDate  string  `json:"purchaseDate"`
	PurchaseCost  float64 `json:"purchaseCost"`
	WarrantyEnd   string  `json:"warrantyEnd"`
	ReplaceCost   float64 `json:"replaceCost"`   // what a new unit would cost today
	ExpectedYears int     `json:"expectedYears"` // normal useful life
}

type Entry struct {
	At   string `json:"at"`
	By   string `json:"by"`
	Text string `json:"text"`
}

type Issue struct {
	ID          string  `json:"id"`
	Number      int     `json:"number"`
	EquipmentID string  `json:"equipmentId"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Priority    string  `json:"priority"` // Low, Medium, High, Urgent
	Status      string  `json:"status"`   // Open, Work Order Submitted, In Progress, Repaired, Verified, Closed
	ReportedBy  string  `json:"reportedBy"`
	ReportedAt  string  `json:"reportedAt"`
	WorkOrder   string  `json:"workOrder"` // number or link from MaintainX
	VerifiedBy  string  `json:"verifiedBy"`
	VerifiedAt  string  `json:"verifiedAt"`
	History     []Entry `json:"history"`
}

type maintSettings struct {
	PortalURL      string `json:"portalUrl"`
	ReplacePercent int    `json:"replacePercent"` // repair spend, as a % of a new unit, at which to suggest replacing
}

const defaultPortal = "https://app.getmaintainx.com"

var (
	equipStatuses = []string{"Operational", "Needs Attention", "Out of Service"}
	priorities    = []string{"P1 - Critical", "P2 - Urgent", "P3 - Routine", "P4 - Improvement"}
	// priorityUse is the guidance shown beside each priority.
	priorityUse = map[string]string{
		"P1 - Critical":    "Immediate safety, food-safety, refrigeration, fire, or critical operational issue. Address immediately and escalate.",
		"P2 - Urgent":      "Major operational impact but the restaurant can continue operating. Address as soon as practical.",
		"P3 - Routine":     "Repair is needed but there is limited operational impact. Schedule and track to completion.",
		"P4 - Improvement": "Cosmetic, convenience, or planned improvement. Complete as time and budget allow.",
	}
	issueStatuses = []string{"Open", "Work Order Submitted", "In Progress", "Repaired", "Verified", "Closed"}
)

// The maintenance files sit beside the "SDS Data" folder, so SDS Data holds
// only the library and its PDFs.
// fixPriority maps the earlier names (Urgent, High, Medium, Low) to the P1 to P4 scale.
func fixPriority(p string) string {
	switch p {
	case "Urgent":
		return priorities[0]
	case "High":
		return priorities[1]
	case "Medium":
		return priorities[2]
	case "Low":
		return priorities[3]
	}
	return p
}

func (s *store) maintDir() string { return filepath.Join(filepath.Dir(s.dir), "Maintenance Data") }

func (s *store) readMaint(name string, v any) {
	if b, err := os.ReadFile(filepath.Join(s.maintDir(), name)); err == nil {
		if json.Unmarshal(b, v) != nil {
			// a damaged file: try the backup copy rather than showing nothing
			if b, err := os.ReadFile(filepath.Join(s.maintDir(), name+".bak")); err == nil {
				_ = json.Unmarshal(b, v)
			}
		}
	}
}

func (s *store) writeMaint(name string, v any) error {
	if s.isReadOnly() {
		return errReadOnly
	}
	if err := os.MkdirAll(s.maintDir(), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	p := filepath.Join(s.maintDir(), name)
	if err := os.WriteFile(p+".tmp", b, 0o644); err != nil {
		return err
	}
	if old, err := os.ReadFile(p); err == nil {
		_ = os.WriteFile(p+".bak", old, 0o644)
	}
	return os.Rename(p+".tmp", p)
}

func (s *store) loadMaint() (eq []*Equipment, is []*Issue, st maintSettings) {
	eq, is = []*Equipment{}, []*Issue{}
	s.readMaint("equipment.json", &eq)
	s.readMaint("issues.json", &is)
	for _, p := range is {
		p.Priority = fixPriority(p.Priority)
	}
	s.readMaint("settings.json", &st)
	if st.PortalURL == "" {
		st.PortalURL = defaultPortal
	}
	if st.ReplacePercent < 1 {
		st.ReplacePercent = 50
	}
	return
}

func oneOf(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func stamp() string { return time.Now().Format("2006-01-02T15:04:05") }

// writeLog keeps a readable copy of the issues next to the data.
func (s *store) writeLog(eq []*Equipment, is []*Issue) {
	name := map[string]string{}
	for _, e := range eq {
		name[e.ID] = e.Name
	}
	var b strings.Builder
	b.WriteString("# Maintenance Log\n\nUpdated " + time.Now().Format("January 2, 2006 3:04 PM") + ". This file is rewritten by the program; edit issues in the program.\n")
	section := func(title string, keep func(*Issue) bool) {
		b.WriteString("\n## " + title + "\n\n")
		n := 0
		for i := len(is) - 1; i >= 0; i-- {
			p := is[i]
			if !keep(p) {
				continue
			}
			n++
			fmt.Fprintf(&b, "### #%d %s (%s)\n\n- Equipment: %s\n- Priority: %s\n- Status: %s\n- Reported: %s by %s\n", p.Number, p.Title, p.Priority, name[p.EquipmentID], p.Priority, p.Status, p.ReportedAt[:10], p.ReportedBy)
			if p.WorkOrder != "" {
				b.WriteString("- Work order: " + p.WorkOrder + "\n")
			}
			if p.VerifiedBy != "" {
				fmt.Fprintf(&b, "- Returned to service: verified by %s on %s\n", p.VerifiedBy, p.VerifiedAt[:10])
			}
			if p.Description != "" {
				b.WriteString("\n" + p.Description + "\n")
			}
			b.WriteString("\n")
		}
		if n == 0 {
			b.WriteString("None.\n")
		}
	}
	open := func(p *Issue) bool { return p.Status != "Verified" && p.Status != "Closed" }
	section("Open issues", open)
	section("Completed", func(p *Issue) bool { return !open(p) })
	_ = os.WriteFile(filepath.Join(s.maintDir(), "Maintenance Log.md"), []byte(b.String()), 0o644)
}

func (s *store) maintRoutes(mux *http.ServeMux) {
	reply := func(w http.ResponseWriter) {
		eq, is, st := s.loadMaint()
		sort.Slice(eq, func(i, j int) bool { return strings.ToLower(eq[i].Name) < strings.ToLower(eq[j].Name) })
		writeJSON(w, 200, map[string]any{"equipment": eq, "issues": is, "portalUrl": st.PortalURL,
			"priorityUse": priorityUse, "pm": s.pmList(), "service": s.serviceList(), "assets": s.assetSummaries(eq, st.ReplacePercent), "replacePercent": st.ReplacePercent, "statuses": equipStatuses, "priorities": priorities, "issueStatuses": issueStatuses})
	}
	save := func(w http.ResponseWriter, eq []*Equipment, is []*Issue) bool {
		if err := s.writeMaint("equipment.json", eq); err != nil {
			s.maintFail(w, err)
			return false
		}
		if err := s.writeMaint("issues.json", is); err != nil {
			s.maintFail(w, err)
			return false
		}
		s.writeLog(eq, is)
		return true
	}

	s.pmRoutes(mux, reply)
	s.serviceRoutes(mux, reply)
	s.calendarRoutes(mux)
	s.importRoutes(mux)
	s.todoRoutes(mux)
	mux.HandleFunc("GET /api/maint", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		reply(w)
	})

	mux.HandleFunc("POST /api/maint/equipment", func(w http.ResponseWriter, r *http.Request) {
		var in2 Equipment
		if json.NewDecoder(r.Body).Decode(&in2) != nil || strings.TrimSpace(in2.Name) == "" {
			fail(w, 400, "Enter a name for the equipment.")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		eq, is, _ := s.loadMaint()
		in2.Name = strings.TrimSpace(in2.Name)
		if !oneOf(equipStatuses, in2.Status) {
			in2.Status = "Operational"
		}
		found := false
		for i, e := range eq {
			if e.ID == in2.ID && in2.ID != "" {
				eq[i], found = &in2, true
			}
		}
		if !found {
			for _, e := range eq {
				if strings.EqualFold(e.Name, in2.Name) {
					fail(w, 400, "There is already equipment named “"+e.Name+"”.")
					return
				}
			}
			in2.ID = newID()
			eq = append(eq, &in2)
		}
		if save(w, eq, is) {
			reply(w)
		}
	})

	mux.HandleFunc("DELETE /api/maint/equipment/{id}", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		eq, is, _ := s.loadMaint()
		id := r.PathValue("id")
		for _, r := range s.serviceList() {
			if r.EquipmentID == id {
				fail(w, 400, "This equipment has service records. Keep it, or set its status instead of deleting it.")
				return
			}
		}
		for _, t := range s.pmList() {
			if t.EquipmentID == id {
				fail(w, 400, "This equipment has preventive maintenance jobs. Delete those first.")
				return
			}
		}
		for _, p := range is {
			if p.EquipmentID == id {
				fail(w, 400, "This equipment has maintenance history. Keep it, or set its status instead of deleting it.")
				return
			}
		}
		out := eq[:0]
		for _, e := range eq {
			if e.ID != id {
				out = append(out, e)
			}
		}
		if save(w, out, is) {
			reply(w)
		}
	})

	mux.HandleFunc("POST /api/maint/issues", func(w http.ResponseWriter, r *http.Request) {
		var in2 struct {
			Issue
			OutOfService bool `json:"outOfService"`
		}
		if json.NewDecoder(r.Body).Decode(&in2) != nil || strings.TrimSpace(in2.Title) == "" || strings.TrimSpace(in2.ReportedBy) == "" {
			fail(w, 400, "Enter what is wrong and your name.")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		eq, is, _ := s.loadMaint()
		var e *Equipment
		for _, x := range eq {
			if x.ID == in2.EquipmentID {
				e = x
			}
		}
		if e == nil {
			fail(w, 400, "Choose the equipment.")
			return
		}
		p := in2.Issue
		p.ID, p.Status, p.ReportedAt = newID(), "Open", stamp()
		p.Title, p.ReportedBy = strings.TrimSpace(p.Title), strings.TrimSpace(p.ReportedBy)
		if p.Priority = fixPriority(p.Priority); !oneOf(priorities, p.Priority) {
			p.Priority = priorities[2]
		}
		for _, x := range is {
			if x.Number >= p.Number {
				p.Number = x.Number + 1
			}
		}
		if p.Number == 0 {
			p.Number = 1
		}
		p.History = []Entry{{p.ReportedAt, p.ReportedBy, "Reported the issue."}}
		is = append(is, &p)
		if in2.OutOfService {
			e.Status = "Out of Service"
		} else if e.Status == "Operational" {
			e.Status = "Needs Attention"
		}
		if save(w, eq, is) {
			reply(w)
		}
	})

	// Update an issue: status, work order number, a note, or manager sign-off.
	mux.HandleFunc("PUT /api/maint/issues/{id}", func(w http.ResponseWriter, r *http.Request) {
		var in2 struct {
			Status    *string `json:"status"`
			WorkOrder *string `json:"workOrder"`
			Note      string  `json:"note"`
			By        string  `json:"by"`
			Cost      float64 `json:"cost"`
		}
		if json.NewDecoder(r.Body).Decode(&in2) != nil || strings.TrimSpace(in2.By) == "" {
			fail(w, 400, "Enter your name.")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		eq, is, _ := s.loadMaint()
		var p *Issue
		for _, x := range is {
			if x.ID == r.PathValue("id") {
				p = x
			}
		}
		if p == nil {
			fail(w, 404, "That issue was not found.")
			return
		}
		by, t := strings.TrimSpace(in2.By), stamp()
		add := func(text string) { p.History = append(p.History, Entry{t, by, text}) }
		if in2.WorkOrder != nil && strings.TrimSpace(*in2.WorkOrder) != p.WorkOrder {
			p.WorkOrder = strings.TrimSpace(*in2.WorkOrder)
			add("Work order set to " + p.WorkOrder + ".")
		}
		if in2.Status != nil && *in2.Status != p.Status {
			if !oneOf(issueStatuses, *in2.Status) {
				fail(w, 400, "Unknown status.")
				return
			}
			p.Status = *in2.Status
			add("Status changed to " + p.Status + ".")
			if p.Status == "Verified" {
				p.VerifiedBy, p.VerifiedAt = by, t
				add("Verified that the equipment is operational and ready for food production.")
			} else {
				p.VerifiedBy, p.VerifiedAt = "", ""
			}
		}
		if n := strings.TrimSpace(in2.Note); n != "" {
			add(n)
		}
		if in2.Cost > 0 {
			s.addService(ServiceRecord{EquipmentID: p.EquipmentID, IssueID: p.ID, Date: stamp()[:10], Type: "Repair",
				Description: "#" + fmt.Sprint(p.Number) + " " + p.Title, Cost: in2.Cost, By: by})
			add(fmt.Sprintf("Recorded a repair cost of $%.2f.", in2.Cost))
		}
		// Equipment goes back to Operational only when a manager has verified
		// the last open issue on it.
		for _, e := range eq {
			if e.ID != p.EquipmentID {
				continue
			}
			open := false
			for _, x := range is {
				if x.EquipmentID == e.ID && x.Status != "Verified" && x.Status != "Closed" {
					open = true
				}
			}
			if !open && p.Status == "Verified" {
				e.Status = "Operational"
			}
			if open && e.Status == "Operational" {
				e.Status = "Needs Attention"
			}
		}
		if save(w, eq, is) {
			reply(w)
		}
	})

	mux.HandleFunc("POST /api/maint/settings", func(w http.ResponseWriter, r *http.Request) {
		var in2 struct {
			PortalURL      *string `json:"portalUrl"`
			ReplacePercent *int    `json:"replacePercent"`
		}
		if json.NewDecoder(r.Body).Decode(&in2) != nil {
			fail(w, 400, "Could not read that.")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		_, _, st := s.loadMaint()
		if in2.PortalURL != nil {
			u := strings.TrimSpace(*in2.PortalURL)
			if u != "" {
				if !strings.Contains(u, "://") {
					u = "https://" + u
				}
				if p, err := url.Parse(u); err != nil || p.Scheme != "https" || p.Host == "" {
					fail(w, 400, "Enter the MaintainX web address starting with https://")
					return
				}
			}
			st.PortalURL = u
		}
		if in2.ReplacePercent != nil {
			if *in2.ReplacePercent < 5 || *in2.ReplacePercent > 200 {
				fail(w, 400, "Enter a percentage between 5 and 200.")
				return
			}
			st.ReplacePercent = *in2.ReplacePercent
		}
		if err := s.writeMaint("settings.json", st); err != nil {
			s.maintFail(w, err)
			return
		}
		reply(w)
	})

	// Opens the saved MaintainX address in the browser. The address is read from
	// the settings file, never from the request, so the page cannot ask the
	// program to open anything else.
	mux.HandleFunc("POST /api/maint/open-portal", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		_, _, st := s.loadMaint()
		s.mu.Unlock()
		openURL(st.PortalURL)
		writeJSON(w, 200, map[string]string{"opened": st.PortalURL})
	})
}

func (s *store) maintFail(w http.ResponseWriter, err error) {
	if err == errReadOnly {
		fail(w, 423, "This copy is view-only because someone else is editing.")
		return
	}
	fail(w, 500, "Could not save: "+err.Error())
}
