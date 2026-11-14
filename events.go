package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Event costing: price a catering event from its menu and other costs, then
// compare it with what really happened.

var (
	evStatus  = []string{"Inquiry", "Quoted", "Confirmed", "Completed", "Cancelled"}
	evService = []string{"Buffet", "Plated", "Drop-Off", "Pickup", "Other"}
	evCostTyp = []string{"Labor", "Disposables", "Rental", "Delivery", "Decor/Misc", "Other"}
)

type evSettings struct {
	Good  float64 `json:"good"`
	Watch float64 `json:"watch"`
}

func (s *store) evSet() evSettings {
	c := evSettings{Good: 40, Watch: 25}
	s.readMaint("event-settings.json", &c)
	if c.Good <= 0 || c.Watch <= 0 || c.Watch > c.Good {
		c = evSettings{Good: 40, Watch: 25}
	}
	return c
}

func (c evSettings) band(margin float64, hasRevenue bool) string {
	switch {
	case !hasRevenue:
		return ""
	case margin >= c.Good:
		return "GOOD"
	case margin >= c.Watch:
		return "WATCH"
	}
	return "LOW"
}

func init() {
	regDefs["events"] = &collDef{Name: "events", Title: "Events", NewLabel: "Add Event", Prefix: "EVT-", Digits: 4, Desc: true, Deletable: true, Report: "events-summary",
		Intro:    "Catering and special events. Add the menu items and other costs, enter the quoted price, and the projected cost, profit and margin are worked out (GOOD, WATCH or LOW). After the event, record what really happened under Event Results.",
		ListCols: []string{"id", "name", "date", "serviceType", "guests", "status", "totalCost", "quotedPrice", "margin", "band"},
		Fields:   []fieldDef{txt("name", "Event / client", true), dat("date", "Event date", true), sel("serviceType", "Service type", evService, true), txt("guests", "Guest count", true), txt("quotedPrice", "Quoted price ($)", false), sel("status", "Status", evStatus, true), txt("approvedBy", "Approved by", false), lng("notes", "Notes", false)}}
	regDefs["eventmenu"] = &collDef{Name: "eventmenu", Title: "Event Menus", NewLabel: "Add Menu Item", Prefix: "EM-", Digits: 4, Deletable: true, Report: "reg:eventmenu",
		Intro:    "What an event serves. Pick a recipe from the Recipe Book to use its cost per portion, or type a cost per serving. The waste / buffer % adds extra servings.",
		ListCols: []string{"id", "eventName", "item", "unitCost", "servings", "waste", "itemCost", "approved"},
		Fields:   []fieldDef{{Key: "event", Label: "Event", Type: "ref", Src: "events", Req: true}, txt("item", "Menu item", true), {Key: "recipeId", Label: "Recipe (optional)", Type: "ref", Src: "recipes"}, txt("cost", "Cost per serving ($, blank uses the recipe)", false), txt("servings", "Servings planned", true), txt("waste", "Waste / buffer %", false), yn("approved", "Approved?"), lng("notes", "Notes", false)}}
	regDefs["eventcost"] = &collDef{Name: "eventcost", Title: "Event Labor & Other Costs", NewLabel: "Add Cost", Prefix: "EC-", Digits: 4, Deletable: true, Report: "reg:eventcost",
		Intro:    "Labor, disposables, rentals, delivery and anything else an event costs. Projected is quantity times rate; enter the actual after the event.",
		ListCols: []string{"id", "eventName", "costType", "description", "qty", "rate", "projected", "actual", "closed"},
		Fields:   []fieldDef{{Key: "event", Label: "Event", Type: "ref", Src: "events", Req: true}, sel("costType", "Cost type", evCostTyp, true), txt("description", "Description", true), txt("qty", "Quantity or hours", true), txt("rate", "Rate ($)", true), txt("actual", "Actual cost ($)", false), txt("vendor", "Vendor / employee", false), txt("invoice", "Invoice / reference", false), yn("closed", "Paid / closed?"), lng("notes", "Notes", false)}}
	regDefs["eventres"] = &collDef{Name: "eventres", Title: "Actual Event Results", NewLabel: "Record Results", Prefix: "RES-", Digits: 4, Desc: true, Deletable: true, Report: "reg:eventres",
		Intro:    "What really happened: guests, revenue and costs, compared with the projection, with a lesson for next time.",
		ListCols: []string{"id", "eventName", "guestsActual", "revenue", "totalActual", "profit", "margin", "variance"},
		Fields:   []fieldDef{{Key: "event", Label: "Event", Type: "ref", Src: "events", Req: true}, txt("guestsActual", "Guests (actual)", false), txt("revenue", "Revenue (actual $)", true), txt("foodActual", "Food cost (actual $)", true), txt("otherActual", "Labor / other cost (actual $)", false), lng("review", "Review / lessons", false), txt("reviewedBy", "Reviewed by", false)}}
}

type evCalc struct{ food, other, total, rev, profit, margin float64 }

func (s *store) evMenuCost(m rec) (unit, item float64) {
	unit = mealNum(m["cost"])
	if strings.TrimSpace(m["cost"]) == "" && m["recipeId"] != "" {
		for _, r := range s.regLoad(regDefs["recipes"]) {
			if r["id"] == m["recipeId"] {
				unit = s.costRecipe(r["ingredients"], r["portions"], r["menuPrice"]).PerPortion
			}
		}
	}
	item = unit * mealNum(m["servings"]) * (1 + mealNum(m["waste"])/100)
	return
}

func (s *store) evCalcs() map[string]evCalc {
	out := map[string]evCalc{}
	for _, m := range s.regLoad(regDefs["eventmenu"]) {
		_, it := s.evMenuCost(m)
		c := out[m["event"]]
		c.food += it
		out[m["event"]] = c
	}
	for _, x := range s.regLoad(regDefs["eventcost"]) {
		c := out[x["event"]]
		c.other += mealNum(x["qty"]) * mealNum(x["rate"])
		out[x["event"]] = c
	}
	set := s.evSet()
	_ = set
	for _, e := range s.regLoad(regDefs["events"]) {
		c := out[e["id"]]
		c.total = c.food + c.other
		c.rev = mealNum(e["quotedPrice"])
		c.profit = c.rev - c.total
		if c.rev > 0 {
			c.margin = c.profit / c.rev * 100
		}
		out[e["id"]] = c
	}
	return out
}

func (s *store) decorateEvents(c *collDef, rows []rec) {
	names := map[string]string{}
	for _, e := range s.regLoad(regDefs["events"]) {
		names[e["id"]] = e["name"]
	}
	set := s.evSet()
	switch c.Name {
	case "events":
		calc := s.evCalcs()
		for _, r := range rows {
			x := calc[r["id"]]
			r["foodCost"], r["otherCost"], r["totalCost"], r["profit"], r["margin"], r["band"] = money(x.food), money(x.other), money(x.total), money(x.profit), "", ""
			if x.rev > 0 {
				r["margin"] = fmt.Sprintf("%.1f%%", x.margin)
				r["band"] = set.band(x.margin, true)
			}
		}
	case "eventmenu":
		for _, r := range rows {
			u, it := s.evMenuCost(r)
			r["eventName"], r["unitCost"], r["itemCost"] = names[r["event"]], money(u), money(it)
			if r["waste"] != "" {
				r["waste"] = strings.TrimSuffix(r["waste"], "%") + "%"
			}
		}
	case "eventcost":
		for _, r := range rows {
			r["eventName"], r["projected"] = names[r["event"]], money(mealNum(r["qty"])*mealNum(r["rate"]))
			if r["actual"] != "" {
				r["actual"] = money(mealNum(r["actual"]))
			}
		}
	case "eventres":
		calc := s.evCalcs()
		for _, r := range rows {
			tot := mealNum(r["foodActual"]) + mealNum(r["otherActual"])
			rev := mealNum(r["revenue"])
			prof := rev - tot
			r["eventName"], r["totalActual"], r["profit"], r["margin"], r["variance"] = names[r["event"]], money(tot), money(prof), "", ""
			r["revenue"] = money(rev)
			if rev > 0 {
				r["margin"] = fmt.Sprintf("%.1f%%", prof/rev*100)
			}
			if p, ok := calc[r["event"]]; ok && p.rev > 0 {
				d := prof - p.profit
				r["variance"] = money(d)
				if d < 0 {
					r["variance"] = "-" + money(-d)
				}
			}
		}
	}
}

func (s *store) evSummary() map[string]any {
	calc := s.evCalcs()
	set := s.evSet()
	open, conf := 0, 0
	rev, cost := 0.0, 0.0
	for _, e := range s.regLoad(regDefs["events"]) {
		switch e["status"] {
		case "Inquiry", "Quoted":
			open++
		case "Confirmed":
			conf++
		default:
			continue
		}
		if e["status"] == "Quoted" || e["status"] == "Confirmed" {
			rev += calc[e["id"]].rev
			cost += calc[e["id"]].total
		}
	}
	m := 0.0
	if rev > 0 {
		m = (rev - cost) / rev * 100
	}
	return map[string]any{"open": open, "confirmed": conf, "revenue": rev, "cost": cost, "profit": rev - cost, "margin": m, "good": set.Good, "watch": set.Watch}
}

func (s *store) eventsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/events/summary", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		writeJSON(w, 200, s.evSummary())
	})
	mux.HandleFunc("POST /api/events/settings", func(w http.ResponseWriter, r *http.Request) {
		var in evSettings
		if jsonDecode(r, &in) != nil || in.Good <= 0 || in.Watch <= 0 || in.Watch > in.Good || in.Good > 100 {
			fail(w, 400, "Enter two percents: the GOOD margin, and a lower one for WATCH.")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		if err := s.writeMaint("event-settings.json", in); err != nil {
			fail(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, s.evSummary())
	})
}

func (s *store) eventReport(id string) (*report, bool) {
	if id == "events-summary" {
		rows := s.regSorted(regDefs["events"])
		s.decorateEvents(regDefs["events"], rows)
		out := [][]string{}
		for _, r := range rows {
			out = append(out, []string{r["id"], r["name"], r["date"], r["serviceType"], r["guests"], r["status"], r["totalCost"], money(mealNum(r["quotedPrice"])), r["margin"], r["band"]})
		}
		m := s.evSummary()
		note := fmt.Sprintf("%v open, %v confirmed. Quoted and confirmed events: revenue %s, cost %s, gross margin %.1f%%.", m["open"], m["confirmed"], money(m["revenue"].(float64)), money(m["cost"].(float64)), m["margin"])
		return &report{Title: "Event Costing Summary", Subtitle: time.Now().Format("January 2, 2006"), Sections: []reportSection{{Note: note, Cols: []string{"ID", "Event", "Date", "Service", "Guests", "Status", "Cost", "Quote", "Margin", "Band"}, Rows: out}}}, true
	}
	if !strings.HasPrefix(id, "event:") {
		return nil, false
	}
	eid := strings.TrimPrefix(id, "event:")
	ev := regDefs["events"]
	rows := s.regLoad(ev)
	s.decorateEvents(ev, rows)
	for _, e := range rows {
		if e["id"] != eid {
			continue
		}
		menu, other := s.regLoad(regDefs["eventmenu"]), s.regLoad(regDefs["eventcost"])
		s.decorateEvents(regDefs["eventmenu"], menu)
		s.decorateEvents(regDefs["eventcost"], other)
		mr, or := [][]string{}, [][]string{}
		for _, m := range menu {
			if m["event"] == eid {
				mr = append(mr, []string{m["item"], m["unitCost"], m["servings"], m["waste"], m["itemCost"], m["approved"]})
			}
		}
		for _, x := range other {
			if x["event"] == eid {
				or = append(or, []string{x["costType"], x["description"], x["qty"], money(mealNum(x["rate"])), x["projected"], x["actual"]})
			}
		}
		g := mealNum(e["guests"])
		x := s.evCalcs()[eid]
		per := ""
		if g > 0 {
			per = fmt.Sprintf(" That is %s a guest in cost and %s a guest quoted.", money(x.total/g), money(x.rev/g))
		}
		sum := fmt.Sprintf("%s, %s guests, %s. Food %s + other %s = total cost %s. Quote %s, profit %s", e["serviceType"], e["guests"], e["status"], e["foodCost"], e["otherCost"], e["totalCost"], money(x.rev), e["profit"])
		if e["margin"] != "" {
			sum += ", margin " + e["margin"] + " (" + e["band"] + ")"
		}
		secs := []reportSection{{Note: sum + "." + per}}
		secs = append(secs, reportSection{Heading: "Menu", Cols: []string{"Item", "Cost each", "Servings", "Buffer", "Cost", "Approved"}, Rows: mr})
		secs = append(secs, reportSection{Heading: "Labor and other costs", Cols: []string{"Type", "Description", "Qty", "Rate", "Projected", "Actual"}, Rows: or})
		for _, r := range s.regLoad(regDefs["eventres"]) {
			if r["event"] == eid {
				all := []rec{r}
				s.decorateEvents(regDefs["eventres"], all)
				secs = append(secs, reportSection{Heading: "Actual results", Note: fmt.Sprintf("Guests %s. Revenue %s, cost %s, profit %s, margin %s, vs projected %s. %s", r["guestsActual"], r["revenue"], r["totalActual"], r["profit"], r["margin"], r["variance"], r["review"])})
			}
		}
		return &report{Title: "Event Cost Sheet: " + e["name"], Subtitle: e["date"], Sections: secs}, true
	}
	return nil, false
}
