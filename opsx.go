package main

// Operations lists: vendors and contracts, daily forecast, prep pars, smallware
// pars, winter preparedness and the shed inventory.

import (
	"fmt"
	"strings"
	"time"
)

func init() {
	regDefs["vendors"] = &collDef{Name: "vendors", Title: "Vendors & Contracts", NewLabel: "Add Vendor", Prefix: "VND-", Digits: 3, Deletable: true, Report: "reg:vendors",
		Intro:    "Every vendor and recurring service in one place: who to call, what they do for us, when they come, when the contract ends, and how and when we order from them. Pest control, cleaning contracts, equipment service and food vendors all go here.",
		ListCols: []string{"id", "name", "category", "phone", "nextService", "contractEnd", "state"},
		Fields:   []fieldDef{txt("name", "Vendor", true), sel("category", "Type", []string{"Food", "Chemical & Cleaning Supplies", "Cleaning Service", "Pest Control", "Equipment Service", "Maintenance Supplies", "Linen", "Grounds / Snow", "Waste & Grease", "Other"}, true), txt("contact", "Contact person", false), txt("phone", "Phone", false), txt("email", "Email", false), txt("account", "Account number", false), lng("service", "What they do for us", false), txt("frequency", "How often (service)", false), dat("nextService", "Next service", false), dat("contractEnd", "Contract ends", false), txt("orderDay", "Order days / cutoff", false), lng("reorder", "Order profile / reorder standard", false), txt("owner", "Who is responsible", false), lng("notes", "Notes", false)}}
	regDefs["forecasts"] = &collDef{Name: "forecasts", Title: "Daily Forecast", NewLabel: "Add Forecast", Prefix: "FCT-", Digits: 5, Desc: true, Deletable: true, Report: "reg:forecasts",
		Intro:    "Plan the day before it happens: expected covers, what is on the books, the weather, the specials and what the prep needs to look like. Enter the actual covers the next day and the variance works itself out.",
		ListCols: []string{"id", "date", "covers", "actual", "variance", "specials", "events"},
		Fields:   []fieldDef{dat("date", "Date", true), txt("covers", "Expected covers", false), txt("actual", "Actual covers", false), lng("events", "Reservations / events", false), txt("weather", "Weather", false), lng("history", "Same day last year / last week", false), lng("specials", "Specials", false), lng("prep", "Prep and production notes", false), lng("variance_note", "Why it was off", false), txt("by", "Planned by", false)}}
	regDefs["preppars"] = &collDef{Name: "preppars", Title: "Prep Pars", NewLabel: "Add Prep Item", Prefix: "PP-", Digits: 4, Deletable: true, Report: "reg:preppars",
		Intro:    "What each station should have prepped. Weekday and Friday/Saturday pars; print it by station for the prep list.",
		ListCols: []string{"id", "station", "item", "unit", "parWeekday", "parWeekend"},
		Fields:   []fieldDef{txt("station", "Station", true), txt("item", "Prep item", true), txt("unit", "Unit (qt, pan, each...)", false), txt("parWeekday", "Par Sun-Thu", false), txt("parWeekend", "Par Fri-Sat", false), lng("notes", "Notes (shelf life, how to make)", false)}}
	regDefs["smallware"] = &collDef{Name: "smallware", Title: "Smallware & Service-Ware Pars", NewLabel: "Add Item", Prefix: "SW-", Digits: 3, Deletable: true, Starter: "smallware.json", Report: "reg:smallware",
		Intro:    "China, glass, silver and banquet ware against a par. Count it, type the number in On hand, and anything under par shows as Short.",
		ListCols: []string{"id", "item", "category", "par", "onHand", "state", "lastCounted"},
		Fields:   []fieldDef{txt("item", "Item", true), sel("category", "Type", []string{"China", "Glass", "Silver", "Banquet ware", "Other"}, true), txt("par", "Par", false), txt("onHand", "On hand", false), dat("lastCounted", "Last counted", false), txt("countedBy", "Counted by", false), lng("notes", "Notes", false)}}
	regDefs["winterprep"] = &collDef{Name: "winterprep", Title: "Winter Preparedness", NewLabel: "Add Item", Prefix: "WP-", Digits: 3, Deletable: true, Starter: "winterprep.json", Report: "reg:winterprep",
		Intro:    "Snow, ice and severe-weather readiness. Give each item an owner and a date.",
		ListCols: []string{"id", "item", "area", "owner", "due", "status"},
		Fields:   []fieldDef{txt("item", "Item", true), lng("details", "What it covers", false), txt("area", "Area", false), txt("owner", "Owner", false), dat("due", "Due", false), sel("status", "Status", []string{"Not Started", "In Progress", "Done"}, true), lng("notes", "Notes", false)}}
	regDefs["shed"] = &collDef{Name: "shed", Title: "Shed Inventory", NewLabel: "Add Shed Item", Prefix: "SH-", Digits: 3, Deletable: true, Report: "reg:shed",
		Intro:    "Everything in the shed: sorted, labeled and counted, with what needs to be repaired, replaced, evaluated or thrown out.",
		ListCols: []string{"id", "item", "qty", "condition", "labeled", "action"},
		Fields:   []fieldDef{txt("item", "Item", true), txt("qty", "Quantity", false), sel("condition", "Condition", []string{"Good", "Needs Repair", "Needs Replacement", "Evaluate", "Dispose"}, true), txt("location", "Where it is kept", false), yn("labeled", "Labeled?"), lng("action", "What to do", false), lng("notes", "Notes", false)}}
}

func (s *store) decorateOps(c *collDef, rows []rec) bool {
	today := time.Now().Format(dayFmt)
	switch c.Name {
	case "vendors":
		soon := time.Now().AddDate(0, 0, 60).Format(dayFmt)
		for _, r := range rows {
			r["state"] = ""
			switch {
			case r["contractEnd"] != "" && r["contractEnd"] < today:
				r["state"] = "Expired"
			case r["contractEnd"] != "" && r["contractEnd"] <= soon:
				r["state"] = "Ending Soon"
			case r["nextService"] != "" && r["nextService"] < today:
				r["state"] = "Overdue"
			}
		}
	case "forecasts":
		for _, r := range rows {
			r["variance"] = ""
			if e, a := mealNum(r["covers"]), mealNum(r["actual"]); e > 0 && strings.TrimSpace(r["actual"]) != "" {
				r["variance"] = fmt.Sprintf("%+.0f (%+.0f%%)", a-e, (a-e)/e*100)
			}
		}
	case "smallware":
		for _, r := range rows {
			par, have := mealNum(r["par"]), mealNum(r["onHand"])
			switch {
			case par <= 0:
				r["state"] = "Set par"
			case strings.TrimSpace(r["onHand"]) == "":
				r["state"] = "Not counted"
			case have < par:
				r["state"] = "Short"
			default:
				r["state"] = "OK"
			}
		}
	default:
		return false
	}
	return true
}

func (s *store) opsAttention(today string) []todoItem {
	out := []todoItem{}
	for _, v := range s.regLoad(regDefs["vendors"]) {
		if v["contractEnd"] != "" && v["contractEnd"] < today {
			out = append(out, todoItem{Group: "Vendor", Title: "Contract ended: " + v["name"], Detail: v["category"], Due: v["contractEnd"], State: "Overdue", Kind: "vendor", ID: v["id"]})
		} else if v["nextService"] != "" && v["nextService"] < today {
			out = append(out, todoItem{Group: "Vendor", Title: "Service overdue: " + v["name"], Detail: v["category"], Due: v["nextService"], State: "Overdue", Kind: "vendor", ID: v["id"]})
		}
	}
	short := 0
	for _, r := range s.regLoad(regDefs["smallware"]) {
		if p := mealNum(r["par"]); p > 0 && strings.TrimSpace(r["onHand"]) != "" && mealNum(r["onHand"]) < p {
			short++
		}
	}
	if short > 0 {
		out = append(out, todoItem{Group: "Smallware", Title: fmt.Sprintf("Smallware below par: %d item%s", short, map[bool]string{true: "", false: "s"}[short == 1]), Due: today, State: "Overdue", Kind: "smallware", ID: "smallware"})
	}
	for _, r := range s.regLoad(regDefs["winterprep"]) {
		if r["status"] != "Done" && r["due"] != "" && r["due"] < today {
			out = append(out, todoItem{Group: "Winter Prep", Title: "Winter prep: " + clipText(r["item"], 60), Detail: r["owner"], Due: r["due"], State: "Overdue", Kind: "winter", ID: r["id"]})
		}
	}
	return out
}
