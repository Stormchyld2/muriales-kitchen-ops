package main

// Asset records and maintenance costs. Every piece of maintenance that cost
// money is a service record in service.json. From those and the purchase
// details the program suggests when replacing a unit may make more sense than
// repairing it again. The suggestion is a rule of thumb, not a verdict: it
// compares what has been spent on upkeep with the cost of a new unit.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

type ServiceRecord struct {
	ID          string  `json:"id"`
	EquipmentID string  `json:"equipmentId"`
	Date        string  `json:"date"`
	Type        string  `json:"type"` // Repair, Preventive, Inspection, Other
	Description string  `json:"description"`
	Cost        float64 `json:"cost"`
	Vendor      string  `json:"vendor"` // who did the work
	By          string  `json:"by"`     // who recorded it
	IssueID     string  `json:"issueId,omitempty"`
	PMID        string  `json:"pmId,omitempty"`
}

type assetSummary struct {
	Total    float64 `json:"total"`    // all upkeep cost
	Last12   float64 `json:"last12"`   // last 12 months
	Repairs  int     `json:"repairs"`  // number of repair records
	Last     string  `json:"last"`     // date of the latest record
	AgeYears float64 `json:"ageYears"` // from purchase or install date, 0 if unknown
	Percent  int     `json:"percent"`  // total as % of the cost of a new unit, -1 if unknown
	Advice   string  `json:"advice"`   // OK, Watch, Consider Replacing, or "" when there is not enough to say
	Reason   string  `json:"reason"`
	Warranty string  `json:"warranty"` // "", "In warranty", "Warranty ended"
}

var serviceTypes = []string{"Repair", "Preventive", "Inspection", "Other"}

func (s *store) serviceList() []ServiceRecord {
	out := []ServiceRecord{}
	s.readMaint("service.json", &out)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Date > out[j].Date })
	return out
}

// addService appends a record. The caller holds s.mu and saves nothing else.
func (s *store) addService(r ServiceRecord) {
	var all []ServiceRecord
	s.readMaint("service.json", &all)
	r.ID = newID()
	all = append(all, r)
	_ = s.writeMaint("service.json", all)
}

func (s *store) assetSummaries(eq []*Equipment, pct int) map[string]assetSummary {
	svc := s.serviceList()
	today := time.Now()
	cutoff := today.AddDate(-1, 0, 0).Format(dayFmt)
	out := map[string]assetSummary{}
	for _, e := range eq {
		a := assetSummary{Percent: -1}
		for _, r := range svc {
			if r.EquipmentID != e.ID {
				continue
			}
			a.Total += r.Cost
			if r.Date >= cutoff {
				a.Last12 += r.Cost
			}
			if r.Type == "Repair" {
				a.Repairs++
			}
			if r.Date > a.Last {
				a.Last = r.Date
			}
		}
		start := e.PurchaseDate
		if start == "" {
			start = e.Installed
		}
		if d, err := time.Parse(dayFmt, start); err == nil {
			a.AgeYears = float64(int(today.Sub(d).Hours()/24/36.525)) / 10
		}
		if w, err := time.Parse(dayFmt, e.WarrantyEnd); err == nil {
			if w.Before(today) {
				a.Warranty = "Warranty ended"
			} else {
				a.Warranty = "In warranty"
			}
		}
		base := e.ReplaceCost
		if base <= 0 {
			base = e.PurchaseCost
		}
		pastLife := e.ExpectedYears > 0 && a.AgeYears >= float64(e.ExpectedYears)
		if base > 0 {
			a.Percent = int(a.Total / base * 100)
			switch {
			case a.Percent >= pct:
				a.Advice = "Consider Replacing"
				a.Reason = fmt.Sprintf("Upkeep has cost %d%% of a new unit (your limit is %d%%).", a.Percent, pct)
			case pastLife && a.Percent >= pct/2:
				a.Advice = "Consider Replacing"
				a.Reason = fmt.Sprintf("Past its expected life of %d years, and upkeep has cost %d%% of a new unit.", e.ExpectedYears, a.Percent)
			case a.Percent >= pct/2 || pastLife:
				a.Advice = "Watch"
				if pastLife {
					a.Reason = fmt.Sprintf("Past its expected life of %d years; upkeep so far is %d%% of a new unit.", e.ExpectedYears, a.Percent)
				} else {
					a.Reason = fmt.Sprintf("Upkeep has reached %d%% of a new unit.", a.Percent)
				}
			default:
				a.Advice = "OK"
				a.Reason = fmt.Sprintf("Upkeep so far is %d%% of a new unit.", a.Percent)
			}
		} else if pastLife {
			a.Advice, a.Reason = "Watch", fmt.Sprintf("Past its expected life of %d years. Enter the cost of a new unit to compare repair costs.", e.ExpectedYears)
		}
		out[e.ID] = a
	}
	return out
}

func (s *store) serviceRoutes(mux *http.ServeMux, reply func(http.ResponseWriter)) {
	mux.HandleFunc("POST /api/maint/service", func(w http.ResponseWriter, r *http.Request) {
		var in2 ServiceRecord
		if json.NewDecoder(r.Body).Decode(&in2) != nil || in2.EquipmentID == "" || strings.TrimSpace(in2.Description) == "" || strings.TrimSpace(in2.By) == "" {
			fail(w, 400, "Choose the equipment, say what was done, and enter your name.")
			return
		}
		d, err := time.Parse(dayFmt, in2.Date)
		if err != nil || d.After(time.Now().AddDate(0, 0, 1)) {
			fail(w, 400, "Enter the date the work was done (today or earlier).")
			return
		}
		if in2.Cost < 0 || in2.Cost > 10000000 {
			fail(w, 400, "Enter the cost as a dollar amount, or 0 if there was no charge.")
			return
		}
		if !oneOf(serviceTypes, in2.Type) {
			in2.Type = "Repair"
		}
		in2.Description, in2.By, in2.Vendor = strings.TrimSpace(in2.Description), strings.TrimSpace(in2.By), strings.TrimSpace(in2.Vendor)
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.isReadOnly() {
			s.maintFail(w, errReadOnly)
			return
		}
		in2.IssueID, in2.PMID = "", ""
		s.addService(in2)
		reply(w)
	})
	mux.HandleFunc("DELETE /api/maint/service/{id}", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		var all, keep []ServiceRecord
		s.readMaint("service.json", &all)
		keep = []ServiceRecord{}
		for _, x := range all {
			if x.ID != r.PathValue("id") {
				keep = append(keep, x)
			}
		}
		if err := s.writeMaint("service.json", keep); err != nil {
			s.maintFail(w, err)
			return
		}
		reply(w)
	})
}
