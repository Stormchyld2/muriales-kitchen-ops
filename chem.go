package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

// ChemIncident is one entry in the Chemical Incident Log.
type ChemIncident struct {
	ID           string `json:"id"`
	At           string `json:"at"` // 2006-01-02 15:04
	Kind         string `json:"kind"`
	ProductID    string `json:"productId"`
	Product      string `json:"product"`
	Manufacturer string `json:"manufacturer"`
	SDSLocation  string `json:"sdsLocation"`
	Amount       string `json:"amount"`
	Area         string `json:"area"`
	Persons      string `json:"persons"`
	PPE          string `json:"ppe"`
	FirstAid     string `json:"firstAid"`
	Emergency    string `json:"emergency"` // Yes / No
	Medical      string `json:"medical"`
	MgmtNotified string `json:"mgmtNotified"`
	ReopenedBy   string `json:"reopenedBy"`
	ReopenedAt   string `json:"reopenedAt"`
	CARef        string `json:"caRef"`
	Training     string `json:"training"` // Yes / No
	Summary      string `json:"summary"`
	Notes        string `json:"notes"`
	By           string `json:"by"`
	Created      string `json:"created"`
}

var chemKinds = []string{"Eye Contact", "Skin Contact", "Inhalation", "Ingestion", "Spill Only"}

type chemStep struct{ N, Title, Text string }

var chemSteps = []chemStep{
	{"1", "STOP", "Stop work and keep others away from the spill/exposure area."},
	{"2", "IDENTIFY", "Identify the chemical/product if it can be done safely. Get the SDS."},
	{"3", "PROTECT", "Use required PPE. Do not touch, smell closely, or mix chemicals."},
	{"4", "EXPOSURE", "Eyes: flush immediately with clean water as directed by SDS. Skin: remove contaminated clothing and rinse as directed. Inhalation: move to fresh air if safe. Ingestion: follow SDS/Poison Control directions; do not improvise."},
	{"5", "SPILL", "Contain/clean only if trained, the spill is small and the SDS permits it. Never mix cleanup chemicals."},
	{"6", "ESCALATE", "For serious symptoms, large/uncontrolled spill, unknown chemical, gas/vapor hazard or fire risk: evacuate/secure area and call appropriate emergency help."},
	{"7", "DOCUMENT", "Notify management, preserve SDS/product information and complete the Chemical Incident Log / Corrective Action as required."},
}

var chemQuick = [][]string{
	{"Eye Contact", "Begin flushing with clean water immediately and follow SDS duration/instructions", "Do not delay flushing while looking for paperwork", "Yes", "Severe pain, vision problem, corrosive exposure, persistent symptoms", "Call emergency/medical help as appropriate; bring SDS"},
	{"Skin Contact", "Remove contaminated clothing/PPE and rinse skin per SDS", "Do not apply neutralizers unless SDS directs", "Yes", "Chemical burn, large exposure, persistent symptoms", "Get medical help as appropriate; secure contaminated clothing"},
	{"Inhalation", "Move person to fresh air if it is safe to do so", "Do not enter hazardous vapor area without proper protection", "Yes", "Breathing difficulty, dizziness, loss of consciousness, toxic gas", "Call emergency services as appropriate; ventilate/evacuate only if safe"},
	{"Ingestion", "Follow SDS/Poison Control instructions", "Do not induce vomiting unless specifically directed", "Yes", "Any significant ingestion or symptoms", "Contact Poison Control/emergency medical help as appropriate"},
	{"Spill Only", "Isolate area; consult SDS; use spill kit/PPE only if trained and appropriate", "Do not mix chemicals or wash unknown spill into drains", "Yes", "Large spill, fumes, unknown product, fire/reactivity risk", "Evacuate/secure and call vendor/emergency response as appropriate"},
}

const chemNote = "Immediate-reference sheet only. Always follow the product SDS and emergency instructions. For serious exposure, breathing difficulty, major burns, uncontrolled spill or uncertain hazard, contact emergency services as appropriate."

func (s *store) chemIncidents() []*ChemIncident {
	out := []*ChemIncident{}
	s.readMaint("chemical-incidents.json", &out)
	return out
}

func (s *store) chemReport(id string) (*report, bool) {
	today := time.Now().Format("January 2, 2006")
	switch id {
	case "chem-response":
		var steps, quick [][]string
		for _, st := range chemSteps {
			steps = append(steps, []string{st.N, st.Title, st.Text})
		}
		for _, q := range chemQuick {
			quick = append(quick, q)
		}
		return &report{Title: "Chemical Spill / Exposure Response Sheet", Subtitle: today, Sections: []reportSection{
			{Note: chemNote},
			{Heading: "Immediate Response", Cols: []string{"#", "Step", "What to do"}, Rows: steps},
			{Heading: "Exposure Quick Reference", Cols: []string{"Exposure", "Immediate Action", "Do Not", "SDS Needed?", "Emergency Escalation Trigger", "Manager Action"}, Rows: quick},
		}}, true
	case "chem-blank":
		labels := []string{"Product / Chemical", "Manufacturer", "SDS Location", "Amount / Concentration", "Area", "Exposure / Spill Time", "Person(s) Involved", "PPE Used", "First Aid Given", "Emergency Services Called? (Yes / No)", "Poison Control / Medical Contact", "Management Notified? (Yes / No)", "Area Reopened By", "Time Reopened", "Corrective Action Ref", "Training Follow-Up? (Yes / No)", "Incident Summary"}
		var rows [][]string
		for _, l := range labels {
			h := ""
			if l == "Incident Summary" {
				h = "\n\n\n"
			}
			rows = append(rows, []string{l, h})
		}
		return &report{Title: "Chemical Incident Form (blank)", Subtitle: "Fill in by hand, then enter it in the Incident Log", Sections: []reportSection{{Note: chemNote, Cols: []string{"Required information to capture", "Entry"}, Rows: rows}}}, true
	case "chem-log":
		list := s.chemIncidents()
		sort.SliceStable(list, func(i, j int) bool { return list[i].ID > list[j].ID })
		rows := [][]string{}
		for _, c := range list {
			rows = append(rows, []string{c.ID, c.At, c.Product, c.Kind, c.Persons, c.Emergency, c.CARef, strings.TrimSpace(c.ReopenedBy + " " + c.ReopenedAt), c.Training, c.Notes})
		}
		return &report{Title: "Chemical Incident Log", Subtitle: today, Sections: []reportSection{{Cols: []string{"ID", "Date / Time", "Product", "Spill / Exposure", "Person(s)", "Emergency Help?", "Corrective Action Ref", "Area Reopened", "Training Follow-Up?", "Notes"}, Rows: rows}}}, true
	}
	if strings.HasPrefix(id, "chem:") {
		for _, c := range s.chemIncidents() {
			if c.ID != strings.TrimPrefix(id, "chem:") {
				continue
			}
			f := func(l, v string) []string { return []string{l, v} }
			rows := [][]string{f("Incident ID", c.ID), f("Date / Time", c.At), f("Spill / Exposure", c.Kind), f("Product / Chemical", c.Product), f("Manufacturer", c.Manufacturer), f("SDS Location", c.SDSLocation), f("Amount / Concentration", c.Amount), f("Area", c.Area),
				f("Person(s) Involved", c.Persons), f("PPE Used", c.PPE), f("First Aid Given", c.FirstAid), f("Emergency Services Called?", c.Emergency), f("Poison Control / Medical Contact", c.Medical), f("Management Notified?", c.MgmtNotified),
				f("Area Reopened By", strings.TrimSpace(c.ReopenedBy+" "+c.ReopenedAt)), f("Corrective Action Ref", c.CARef), f("Training Follow-Up?", c.Training), f("Incident Summary", c.Summary), f("Notes", c.Notes), f("Recorded By", c.By)}
			return &report{Title: "Chemical Incident " + c.ID, Subtitle: c.At, Sections: []reportSection{{Cols: []string{"Item", "Entry"}, Rows: rows}}}, true
		}
	}
	return nil, false
}

func (s *store) chemRoutes(mux *http.ServeMux) {
	payload := func() map[string]any {
		list := s.chemIncidents()
		sort.SliceStable(list, func(i, j int) bool { return list[i].ID > list[j].ID })
		type prod struct {
			ID, Name, Manufacturer, Location, Eyes, Skin, Inhalation, Swallowed, PPE, Signal string
		}
		prods := []prod{}
		for _, p := range s.lib.Products {
			prods = append(prods, prod{p.ID, p.Name, p.Manufacturer, p.Location, p.FirstAidEyes, p.FirstAidSkin, p.FirstAidInhalation, p.FirstAidSwallowed, p.PPE, p.SignalWord})
		}
		sort.Slice(prods, func(i, j int) bool { return strings.ToLower(prods[i].Name) < strings.ToLower(prods[j].Name) })
		return map[string]any{"incidents": list, "kinds": chemKinds, "products": prods, "steps": chemSteps, "quick": chemQuick, "note": chemNote}
	}
	mux.HandleFunc("GET /api/chem", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		writeJSON(w, 200, payload())
	})
	mux.HandleFunc("POST /api/chem", func(w http.ResponseWriter, r *http.Request) {
		var in2 ChemIncident
		if jsonDecode(r, &in2) != nil {
			fail(w, 400, "Could not read that form.")
			return
		}
		for _, v := range []*string{&in2.Product, &in2.Area, &in2.Summary, &in2.By, &in2.Persons} {
			*v = strings.TrimSpace(*v)
		}
		if _, err := time.Parse("2006-01-02T15:04", in2.At); err == nil {
			in2.At = strings.Replace(in2.At, "T", " ", 1)
		}
		switch {
		case func() bool { _, err := time.Parse("2006-01-02 15:04", in2.At); return err != nil }():
			fail(w, 400, "Enter the date and time of the incident.")
			return
		case !oneOf(chemKinds, in2.Kind):
			fail(w, 400, "Choose what happened (the type of exposure or spill).")
			return
		case in2.Product == "":
			fail(w, 400, "Enter the product or chemical.")
			return
		case in2.Area == "":
			fail(w, 400, "Enter the area where it happened.")
			return
		case in2.Kind != "Spill Only" && in2.Persons == "":
			fail(w, 400, "Enter who was exposed.")
			return
		case in2.Emergency != "Yes" && in2.Emergency != "No":
			fail(w, 400, "Say whether emergency services were called.")
			return
		case in2.MgmtNotified != "Yes" && in2.MgmtNotified != "No":
			fail(w, 400, "Say whether management was notified.")
			return
		case in2.Summary == "":
			fail(w, 400, "Write a short summary of what happened.")
			return
		case in2.By == "":
			fail(w, 400, "Enter your name.")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		list := s.chemIncidents()
		found := false
		for i, c := range list {
			if in2.ID != "" && c.ID == in2.ID {
				in2.Created = c.Created
				list[i], found = &in2, true
			}
		}
		if !found {
			max := 0
			for _, c := range list {
				var n int
				if _, err := fmt.Sscanf(c.ID, "CHEM-%d", &n); err == nil && n > max {
					max = n
				}
			}
			in2.ID = fmt.Sprintf("CHEM-%04d", max+1)
			in2.Created = time.Now().Format("2006-01-02 15:04")
			list = append(list, &in2)
		}
		if err := s.writeMaint("chemical-incidents.json", list); err != nil {
			fail(w, 500, "The incident could not be saved: "+err.Error())
			return
		}
		p := payload()
		p["saved"] = in2
		writeJSON(w, 200, p)
	})
}
