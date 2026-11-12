package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

type ReplHist struct {
	Date       string  `json:"date"`
	WO         string  `json:"wo"`
	Failure    string  `json:"failure"`
	Downtime   float64 `json:"downtime"`
	RepairCost float64 `json:"repairCost"`
	PartsCost  float64 `json:"partsCost"`
	Vendor     string  `json:"vendor"`
	Repeat     string  `json:"repeat"`
	Resolved   string  `json:"resolved"`
	Notes      string  `json:"notes"`
}

// ReplRequest is an Equipment Replacement Justification Form.
type ReplRequest struct {
	ID             string     `json:"id"`
	Date           string     `json:"date"`
	EquipmentID    string     `json:"equipmentId"`
	AssetCode      string     `json:"assetCode"`
	Equipment      string     `json:"equipment"`
	Area           string     `json:"area"`
	MakeModel      string     `json:"makeModel"`
	Age            string     `json:"age"`
	OriginalCost   float64    `json:"originalCost"`
	ReplaceCost    float64    `json:"replaceCost"`
	Condition      string     `json:"condition"`
	Critical       string     `json:"critical"`
	MaintainXRef   string     `json:"maintainxRef"`
	Vendor         string     `json:"vendor"`
	RequestedBy    string     `json:"requestedBy"`
	History        []ReplHist `json:"history"`
	Reasons        string     `json:"reasons"`
	OpImpact       string     `json:"opImpact"`
	SafetyRisk     string     `json:"safetyRisk"`
	PartsConcern   string     `json:"partsConcern"`
	AnnualRepair   float64    `json:"annualRepair"`
	AnnualDowntime float64    `json:"annualDowntime"`
	Energy         string     `json:"energy"`
	Spec           string     `json:"spec"`
	Recommendation string     `json:"recommendation"`
	Priority       string     `json:"priority"`
	Decision       string     `json:"decision"`
	DecisionDate   string     `json:"decisionDate"`
	ApprovedBy     string     `json:"approvedBy"`
	BudgetRef      string     `json:"budgetRef"`
	TargetPurchase string     `json:"targetPurchase"`
	VendorQuote    string     `json:"vendorQuote"`
	DecisionNotes  string     `json:"decisionNotes"`
	Completed      string     `json:"completed"`
	NewAssetID     string     `json:"newAssetId"`
	CompletionDate string     `json:"completionDate"`
	OldDisposition string     `json:"oldDisposition"`
	Created        string     `json:"created"`
}

var replConditions = []string{"Good", "Fair", "Poor", "Unsafe / End of Life"}
var replRecs = []string{"Repair Again", "Replace", "Monitor"}
var replDecisions = []string{"Approved", "Denied", "Hold"}

func (s *store) replList() []*ReplRequest {
	out := []*ReplRequest{}
	s.readMaint("replacements.json", &out)
	return out
}

// replPrefill fills a new request from the equipment record and its repair history.
func (s *store) replPrefill(equipmentID string) *ReplRequest {
	eq, issues, _ := s.loadMaint()
	var e *Equipment
	for _, x := range eq {
		if x.ID == equipmentID {
			e = x
		}
	}
	if e == nil {
		return nil
	}
	year := e.PurchaseDate
	if year == "" {
		year = e.Installed
	}
	if len(year) >= 4 {
		year = year[:4]
	}
	p := &ReplRequest{EquipmentID: e.ID, Equipment: e.Name, Area: e.Area, MakeModel: strings.TrimSpace(e.Make + " " + e.Model), Age: year, OriginalCost: e.PurchaseCost, ReplaceCost: e.ReplaceCost, Vendor: e.Vendor, MaintainXRef: e.MaintainXID, History: []ReplHist{}}
	cut := time.Now().AddDate(-1, 0, 0).Format(dayFmt)
	var mine []*Issue
	for _, is := range issues {
		if is.EquipmentID == equipmentID {
			mine = append(mine, is)
		}
	}
	sort.Slice(mine, func(i, j int) bool { return mine[i].ReportedAt < mine[j].ReportedAt })
	seen := map[string]int{}
	for _, is := range mine {
		d := is.CompletedAt
		if d == "" {
			d = is.ReportedAt
		}
		if len(d) > 10 {
			d = d[:10]
		}
		wo := is.WorkOrder
		if wo == "" {
			wo = fmt.Sprintf("#%d", is.Number)
		}
		key := strings.ToLower(strings.TrimSpace(is.Title))
		seen[key]++
		rep := ""
		if seen[key] > 1 {
			rep = "Yes"
		}
		res := "No"
		if is.Status == "Repaired" || is.Status == "Verified" || is.Status == "Closed" {
			res = "Yes"
		}
		p.History = append(p.History, ReplHist{Date: d, WO: wo, Failure: is.Title, Downtime: is.Downtime, RepairCost: is.LaborCost, PartsCost: is.PartsCost, Vendor: is.AssignedTo, Repeat: rep, Resolved: res, Notes: is.Notes})
		if d >= cut {
			p.AnnualRepair += is.LaborCost + is.PartsCost
		}
	}
	return p
}

func (s *store) replReport(id string) (*report, bool) {
	today := time.Now().Format("January 2, 2006")
	money := func(v float64) string {
		if v == 0 {
			return ""
		}
		return fmt.Sprintf("$%.2f", v)
	}
	if id == "replace-log" {
		list := s.replList()
		sort.SliceStable(list, func(i, j int) bool { return list[i].ID > list[j].ID })
		rows := [][]string{}
		for _, r := range list {
			rows = append(rows, []string{r.ID, r.Date, r.AssetCode, r.Equipment, r.Reasons, r.Recommendation, r.Decision, money(r.ReplaceCost), r.CompletionDate, r.DecisionNotes})
		}
		return &report{Title: "Equipment Replacement History", Subtitle: today, Sections: []reportSection{{Cols: []string{"Request ID", "Date", "Asset ID", "Equipment", "Reason", "Recommendation", "Decision", "Estimated Cost", "Completed", "Notes"}, Rows: rows}}}, true
	}
	if !strings.HasPrefix(id, "replace:") {
		return nil, false
	}
	for _, r := range s.replList() {
		if r.ID != strings.TrimPrefix(id, "replace:") {
			continue
		}
		f := func(l, v string) []string { return []string{l, v} }
		hist := [][]string{}
		for _, h := range r.History {
			hist = append(hist, []string{h.Date, h.WO, h.Failure, fmt.Sprintf("%g", h.Downtime), money(h.RepairCost), money(h.PartsCost), h.Vendor, h.Repeat, h.Resolved, h.Notes})
		}
		return &report{Title: "Equipment Replacement Justification " + r.ID, Subtitle: r.Equipment + " · requested " + r.Date + " by " + r.RequestedBy, Sections: []reportSection{
			{Heading: "Asset Information", Cols: []string{"Item", "Entry"}, Rows: [][]string{f("Asset ID", r.AssetCode), f("Equipment", r.Equipment), f("Area", r.Area), f("Manufacturer / Model", r.MakeModel), f("Age / Install Year", r.Age), f("Original Cost", money(r.OriginalCost)), f("Estimated Replacement Cost", money(r.ReplaceCost)), f("Current Condition", r.Condition), f("Critical to Operation?", r.Critical), f("MaintainX / PM Ref", r.MaintainXRef), f("Primary Vendor", r.Vendor)}},
			{Heading: "Repair / Failure History", Cols: []string{"Date", "WO / Service Ref", "Failure / Repair", "Downtime Hrs", "Repair Cost", "Parts Cost", "Vendor", "Repeat?", "Resolved?", "Notes"}, Rows: hist},
			{Heading: "Replacement Case", Cols: []string{"Item", "Entry"}, Rows: [][]string{f("Reason(s) for Replacement", r.Reasons), f("Operational Impact", r.OpImpact), f("Food Safety / Safety Risk", r.SafetyRisk), f("Availability / Parts Concern", r.PartsConcern), f("Estimated Annual Repair Cost", money(r.AnnualRepair)), f("Estimated Annual Downtime Cost", money(r.AnnualDowntime)), f("Energy / Efficiency Concern?", r.Energy), f("Recommended Replacement / Specification", r.Spec), f("Recommendation", r.Recommendation), f("Priority", r.Priority)}},
			{Heading: "Approval / Decision", Cols: []string{"Item", "Entry"}, Rows: [][]string{f("Management Decision", r.Decision), f("Decision Date", r.DecisionDate), f("Approved By", r.ApprovedBy), f("Budget / Capital Ref", r.BudgetRef), f("Target Purchase Date", r.TargetPurchase), f("Vendor / Quote", r.VendorQuote), f("Final Decision Notes", r.DecisionNotes), f("Replacement Completed?", r.Completed), f("New Asset ID", r.NewAssetID), f("Completion Date", r.CompletionDate), f("Old Asset Disposition", r.OldDisposition)}},
		}}, true
	}
	return nil, false
}

func (s *store) replTodo() []todoItem {
	out := []todoItem{}
	for _, r := range s.replList() {
		if r.Decision == "" || r.Decision == "Hold" {
			out = append(out, todoItem{Group: "Equipment", Title: r.ID + " Replacement decision: " + r.Equipment, Detail: strings.Join(nonEmpty(r.Recommendation, r.Priority, "requested by "+r.RequestedBy), " · "), Due: r.Date, State: "Open", Kind: "repl", ID: r.ID})
		}
	}
	return out
}

func (s *store) replRoutes(mux *http.ServeMux) {
	payload := func() map[string]any {
		list := s.replList()
		sort.SliceStable(list, func(i, j int) bool { return list[i].ID > list[j].ID })
		eq, _, _ := s.loadMaint()
		type e struct{ ID, Name string }
		es := []e{}
		for _, x := range eq {
			es = append(es, e{x.ID, x.Name})
		}
		return map[string]any{"requests": list, "equipment": es, "conditions": replConditions, "recs": replRecs, "decisions": replDecisions, "priorities": caPriorities}
	}
	mux.HandleFunc("GET /api/replacements", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		writeJSON(w, 200, payload())
	})
	mux.HandleFunc("GET /api/replacements/prefill", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		p := s.replPrefill(r.URL.Query().Get("equipmentId"))
		s.mu.Unlock()
		if p == nil {
			fail(w, 404, "That equipment was not found.")
			return
		}
		writeJSON(w, 200, p)
	})
	mux.HandleFunc("POST /api/replacements", func(w http.ResponseWriter, r *http.Request) {
		var in2 ReplRequest
		if jsonDecode(r, &in2) != nil {
			fail(w, 400, "Could not read that request.")
			return
		}
		in2.Equipment, in2.RequestedBy = strings.TrimSpace(in2.Equipment), strings.TrimSpace(in2.RequestedBy)
		chk := func(v string, list []string) bool { return v == "" || oneOf(list, v) }
		switch {
		case in2.Equipment == "":
			fail(w, 400, "Choose or enter the equipment.")
			return
		case in2.RequestedBy == "":
			fail(w, 400, "Enter who is requesting the replacement.")
			return
		case func() bool { _, err := time.Parse(dayFmt, in2.Date); return err != nil }():
			fail(w, 400, "Choose the request date.")
			return
		case !oneOf(replConditions, in2.Condition):
			fail(w, 400, "Choose the current condition.")
			return
		case !oneOf(replRecs, in2.Recommendation):
			fail(w, 400, "Choose a recommendation: repair again, replace or monitor.")
			return
		case strings.TrimSpace(in2.Reasons) == "":
			fail(w, 400, "Write the reason for replacement.")
			return
		case !chk(in2.Decision, replDecisions) || !chk(in2.Priority, caPriorities):
			fail(w, 400, "Choose one of the listed options.")
			return
		case in2.Decision != "" && (strings.TrimSpace(in2.ApprovedBy) == "" || in2.DecisionDate == ""):
			fail(w, 400, "A decision needs the decision date and who made it.")
			return
		case in2.Completed == "Yes" && in2.Decision != "Approved":
			fail(w, 400, "A replacement can only be completed after it is approved.")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		list := s.replList()
		found := false
		for i, x := range list {
			if in2.ID != "" && x.ID == in2.ID {
				in2.Created = x.Created
				list[i], found = &in2, true
			}
		}
		if !found {
			max := 0
			for _, x := range list {
				var n int
				if _, err := fmt.Sscanf(x.ID, "ER-%d", &n); err == nil && n > max {
					max = n
				}
			}
			in2.ID = fmt.Sprintf("ER-%04d", max+1)
			in2.Created = time.Now().Format("2006-01-02 15:04")
			list = append(list, &in2)
		}
		if err := s.writeMaint("replacements.json", list); err != nil {
			fail(w, 500, "The request could not be saved: "+err.Error())
			return
		}
		p := payload()
		p["saved"] = in2.ID
		writeJSON(w, 200, p)
	})
}
