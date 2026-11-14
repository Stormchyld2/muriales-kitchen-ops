package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Par sheets: what each station should have before service, the vendor order
// guide pars, the days orders go in, and the smallware breakage log.

var (
	brkTypes  = []string{"Broken", "Damaged", "Lost / Missing", "Discarded", "Worn Out", "Other"}
	brkCauses = []string{"Normal Wear", "Dropped / Impact", "Improper Storage", "Misuse", "Cleaning Damage", "Heat Damage", "Chemical Damage", "Unknown", "Other"}
	brkAreas  = []string{"Cook Line", "Prep Area", "Salad", "Dish / Stewarding", "Dry Storage", "Walk-In / Cooler", "Freezer", "Receiving", "Barn / Storage", "Bar / Service", "Office / Manager", "Other"}
	brkStatus = []string{"Open", "Pending Approval", "Ordered", "Backordered", "Received", "Closed", "Cancelled"}
	orderVend = []string{"Sysco", "PFG", "Pennmac", "Paragon", "Tomaro's", "D'Annunzio's", "Young and Stout", "Goot's", "Other"}
	weekdays  = []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}
)

func init() {
	regDefs["parsheets"] = &collDef{Name: "parsheets", Title: "Station Par Sheets", NewLabel: "Add Par Item", Prefix: "PS-", Digits: 4, Deletable: true, Starter: "parsheets.json", Report: "parsheets-all",
		Intro:    "What each station must have before service. Type the par the way the crew reads it (for example 5 pans weekday / 9-10 pans weekend). Use Daily Par Check to count against it, and print a station's sheet for the line.",
		ListCols: []string{"id", "station", "item", "par"},
		Fields:   []fieldDef{station("station"), txt("item", "Product", true), txt("par", "PAR quantity", false), lng("notes", "Notes", false)}}
	regDefs["parsheets"].Fields[0].Req = true
	regDefs["parnotes"] = &collDef{Name: "parnotes", Title: "Par Sheet Notes", NewLabel: "Add Note", Prefix: "PN-", Digits: 3, Deletable: true, Starter: "parnotes.json", Report: "parsheets-all",
		Intro:    "The notes and standards printed under a station's par sheet.",
		ListCols: []string{"id", "station", "note"},
		Fields:   []fieldDef{station("station"), lng("note", "Note", true)}}
	regDefs["parnotes"].Fields[0].Req = true
	regDefs["orderpars"] = &collDef{Name: "orderpars", Title: "Order Guide Pars", NewLabel: "Add Order Item", Prefix: "OP-", Digits: 4, Deletable: true, Starter: "orderpars.json", Report: "orderpars-all",
		Intro:    "The vendor order guide: where it is stored, pack size, and the Busy and Slow par. Type new pars straight into the list. This is a par reference; purchasing and costs stay in 365.",
		ListCols: []string{"id", "vendor", "location", "item", "pack", "busy", "slow"},
		Fields:   []fieldDef{sel("vendor", "Vendor", orderVend, true), fieldDef{Key: "location", Label: "Where it is kept (pick from the Sysco places or type a new one)", Type: "combo", Src: "orderpars"}, txt("item", "Product", true), txt("upc", "Vendor item number", false), txt("pack", "Pack / size", false), txt("busy", "Busy par", false), txt("slow", "Slow par", false), lng("notes", "Notes", false)}}
	regDefs["orderdays"] = &collDef{Name: "orderdays", Title: "Order Days", NewLabel: "Add Day", Prefix: "OD-", Digits: 3, Starter: "orderdays.json", Report: "orderpars-days", Deletable: true,
		Intro:    "The vendors whose order must be placed each day.",
		ListCols: []string{"id", "day", "vendors"},
		Fields:   []fieldDef{sel("day", "Day", weekdays, true), lng("vendors", "Vendors to order from", true), lng("notes", "Notes", false)}}
	regDefs["breakage"] = &collDef{Name: "breakage", Title: "Breakage & Replacement Log", NewLabel: "Log Breakage", Prefix: "BRK-", Digits: 5, Desc: true, Deletable: true, Report: "breakage-month",
		Intro:    "Broken, damaged, worn or missing smallwares and what it costs to replace them. Current quantities and pars are in Smallware Pars. Repeat shows Yes when the same item was logged twice in 90 days.",
		ListCols: []string{"id", "date", "area", "item", "issueType", "qty", "estCost", "status", "repeat"},
		Fields:   []fieldDef{dat("date", "Date", true), sel("area", "Area", brkAreas, true), txt("item", "Item", true), sel("issueType", "Issue", brkTypes, true), txt("qty", "Quantity affected", true), sel("cause", "Cause", brkCauses, false), txt("by", "Reported by", false), yn("replace", "Replacement needed?"), txt("qtyReplace", "Quantity to replace", false), txt("unitCost", "Unit cost ($)", false), txt("actualCost", "Actual replacement cost ($)", false), sel("priority", "Priority", prioOpts, false), sel("status", "Status", brkStatus, true), txt("orderRef", "Order reference", false), lng("notes", "Notes", false)}}
	regDefs["breakrev"] = &collDef{Name: "breakrev", Title: "Repeat Breakage Review", NewLabel: "Add Review", Prefix: "REV-", Digits: 4, Desc: true, Deletable: true, Report: "reg:breakrev",
		Intro:    "For items or areas that keep breaking or going missing: the pattern, the likely cause and what will change. This is for corrective action, not single events.",
		ListCols: []string{"id", "item", "area", "pattern", "occurrences", "estCost", "action", "owner", "status"},
		Fields:   []fieldDef{txt("item", "Item", true), sel("area", "Area", brkAreas, false), lng("pattern", "Issue pattern", false), txt("occurrences", "Occurrences", false), txt("estCost", "Estimated cost ($)", false), lng("cause", "Likely cause", false), lng("action", "Corrective action", false), txt("owner", "Owner", false), dat("targetDate", "Target date", false), sel("status", "Status", []string{"Open", "In Progress", "Done"}, true), lng("notes", "Notes", false)}}
}

func (s *store) decoratePars(c *collDef, rows []rec) {
	if c.Name != "breakage" {
		return
	}
	all := s.regLoad(c)
	for _, r := range rows {
		est := mealNum(r["qtyReplace"]) * mealNum(r["unitCost"])
		r["estCost"] = ""
		if est > 0 {
			r["estCost"] = money(est)
		}
		r["repeat"] = ""
		d, err := time.Parse("2006-01-02", r["date"])
		if err != nil || strings.TrimSpace(r["item"]) == "" {
			continue
		}
		n := 0
		for _, o := range all {
			od, err := time.Parse("2006-01-02", o["date"])
			if err == nil && strings.EqualFold(strings.TrimSpace(o["item"]), strings.TrimSpace(r["item"])) && od.After(d.AddDate(0, 0, -90)) && !od.After(d.AddDate(0, 0, 90)) {
				n++
			}
		}
		if n >= 2 {
			r["repeat"] = "Yes"
		}
	}
}

func brkOpen(r rec) bool {
	switch r["status"] {
	case "Received", "Closed", "Cancelled":
		return false
	}
	return true
}

func (s *store) parsAttention(today string) []todoItem {
	out := []todoItem{}
	n, hi := 0, 0
	for _, r := range s.regLoad(regDefs["breakage"]) {
		if r["replace"] == "Yes" && brkOpen(r) {
			n++
			if strings.HasPrefix(r["priority"], "P1") || strings.HasPrefix(r["priority"], "P2") {
				hi++
			}
		}
	}
	if n > 0 {
		st := "Soon"
		if hi > 0 {
			st = "Overdue"
		}
		out = append(out, todoItem{Group: "Breakage", Title: fmt.Sprintf("Smallware replacements open: %d", n), Detail: fmt.Sprintf("%d urgent", hi), Due: today, State: st, Kind: "breakage", ID: "breakage"})
	}
	return out
}

// ---- daily par check ----

type parCheck struct {
	By   string            `json:"by"`
	Time string            `json:"time"`
	On   map[string]string `json:"on"`
	Make map[string]string `json:"make"`
	Init map[string]string `json:"init"`
}

func (s *store) parChecks() map[string]parCheck {
	m := map[string]parCheck{}
	s.readMaint("par-checks.json", &m)
	return m
}

func (s *store) parItems(station string) ([]rec, []string) {
	items, notes := []rec{}, []string{}
	for _, r := range s.regSorted(regDefs["parsheets"]) {
		if r["station"] == station {
			items = append(items, r)
		}
	}
	for _, r := range s.regSorted(regDefs["parnotes"]) {
		if r["station"] == station {
			notes = append(notes, r["note"])
		}
	}
	return items, notes
}

func (s *store) parStations() []string {
	seen, out := map[string]bool{}, []string{}
	for _, r := range s.regSorted(regDefs["parsheets"]) {
		if !seen[r["station"]] {
			seen[r["station"]] = true
			out = append(out, r["station"])
		}
	}
	return out
}

func (s *store) parsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/parcheck", func(w http.ResponseWriter, r *http.Request) {
		st := r.URL.Query().Get("station")
		s.mu.Lock()
		defer s.mu.Unlock()
		if st == "" {
			writeJSON(w, 200, map[string]any{"stations": s.parStations()})
			return
		}
		items, notes := s.parItems(st)
		day := time.Now().Format(dayFmt)
		c := s.parChecks()[day+"|"+st]
		writeJSON(w, 200, map[string]any{"station": st, "date": day, "items": items, "notes": notes, "check": c, "stations": s.parStations()})
	})
	mux.HandleFunc("POST /api/parcheck", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Station, By string
			Rows        []struct{ ID, On, Make, Init string }
		}
		if jsonDecode(r, &in) != nil || strings.TrimSpace(in.By) == "" || in.Station == "" {
			fail(w, 400, "Type your first name so the par check can be signed.")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		c := parCheck{By: strings.TrimSpace(in.By), Time: time.Now().Format("3:04 PM"), On: map[string]string{}, Make: map[string]string{}, Init: map[string]string{}}
		for _, x := range in.Rows {
			if strings.TrimSpace(x.On) != "" {
				c.On[x.ID] = strings.TrimSpace(x.On)
			}
			if strings.TrimSpace(x.Make) != "" {
				c.Make[x.ID] = strings.TrimSpace(x.Make)
			}
			if strings.TrimSpace(x.Init) != "" {
				c.Init[x.ID] = strings.TrimSpace(x.Init)
			}
		}
		m := s.parChecks()
		m[time.Now().Format(dayFmt)+"|"+in.Station] = c
		// keep a year of checks
		cut := time.Now().AddDate(-1, 0, 0).Format(dayFmt)
		for k := range m {
			if len(k) >= 10 && k[:10] < cut {
				delete(m, k)
			}
		}
		if err := s.writeMaint("par-checks.json", m); err != nil {
			fail(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true, "time": c.Time})
	})
}

// ---- reports ----

func (s *store) parsReport(id, month string) (*report, bool) {
	sheet := func(st string, filled bool) []reportSection {
		items, notes := s.parItems(st)
		var c parCheck
		if filled {
			c = s.parChecks()[time.Now().Format(dayFmt)+"|"+st]
		}
		rows := [][]string{}
		for _, r := range items {
			rows = append(rows, []string{r["item"], r["par"], c.On[r["id"]], c.Make[r["id"]], c.Init[r["id"]]})
		}
		secs := []reportSection{}
		note := ""
		if filled && c.By != "" {
			note = fmt.Sprintf("Checked by %s at %s.", c.By, c.Time)
		}
		secs = append(secs, reportSection{Heading: st + " Station Par Sheet", Note: note, Cols: []string{"Product", "PAR quantity", "On hand", "Make", "Initial"}, Rows: rows})
		if len(notes) > 0 {
			secs = append(secs, reportSection{Heading: "Notes", Text: strings.Join(notes, "\n"), HTML: "<ul><li>" + strings.Join(escAll(notes), "</li><li>") + "</li></ul>"})
		}
		return secs
	}
	switch {
	case id == "parsheets-all":
		secs := []reportSection{}
		for _, st := range s.parStations() {
			secs = append(secs, reportSection{Heading: st, Group: true})
			secs = append(secs, sheet(st, false)...)
		}
		if len(secs) == 0 {
			secs = []reportSection{{Note: "No par sheets have been added yet."}}
		}
		return &report{Title: "Station Par Sheets", Subtitle: time.Now().Format("January 2, 2006"), Sections: secs}, true
	case strings.HasPrefix(id, "parsheet:"), strings.HasPrefix(id, "parcheck:"):
		st := id[strings.Index(id, ":")+1:]
		return &report{Title: st + " Station Par Sheet", Subtitle: time.Now().Format("January 2, 2006"), Sections: sheet(st, strings.HasPrefix(id, "parcheck:"))}, true
	case id == "orderpars-days":
		rows := [][]string{}
		for _, r := range s.regLoad(regDefs["orderdays"]) {
			rows = append(rows, []string{r["day"], r["vendors"]})
		}
		sort.SliceStable(rows, func(i, j int) bool { return dayIndex(rows[i][0]) < dayIndex(rows[j][0]) })
		return &report{Title: "Ordering Schedule for Vendors", Subtitle: "The days the order must be placed", Sections: []reportSection{{Cols: []string{"Day", "Vendors"}, Rows: rows}}}, true
	case id == "orderpars-all" || strings.HasPrefix(id, "orderpars:"):
		only := strings.TrimPrefix(id, "orderpars:")
		type line struct{ loc, item, upc, pack, busy, slow string }
		by := map[string][]line{}
		order := []string{}
		for _, r := range s.regSorted(regDefs["orderpars"]) {
			if id != "orderpars-all" && r["vendor"] != only {
				continue
			}
			if _, ok := by[r["vendor"]]; !ok {
				order = append(order, r["vendor"])
			}
			by[r["vendor"]] = append(by[r["vendor"]], line{r["location"], r["item"], r["upc"], r["pack"], r["busy"], r["slow"]})
		}
		secs := []reportSection{}
		for _, v := range order {
			if id == "orderpars-all" {
				secs = append(secs, reportSection{Heading: v, Group: true})
			}
			locs, groups := []string{}, map[string][][]string{}
			for _, l := range by[v] {
				k := l.loc
				if k == "" {
					k = "No location"
				}
				if _, ok := groups[k]; !ok {
					locs = append(locs, k)
				}
				groups[k] = append(groups[k], []string{l.item, l.upc, l.pack, l.busy, l.slow, "", ""})
			}
			for _, k := range locs {
				secs = append(secs, reportSection{Heading: k, Cols: []string{"Product", "Item #", "Pack", "Busy par", "Slow par", "On hand", "Order"}, Rows: groups[k]})
			}
		}
		if len(secs) == 0 {
			secs = []reportSection{{Note: "No order guide items yet for this vendor."}}
		}
		t := "Order Guide Pars"
		sub := time.Now().Format("January 2, 2006")
		if id != "orderpars-all" {
			t = only + " Order Guide"
			sub = "Count against busy or slow par. " + sub
		}
		return &report{Title: t, Subtitle: sub, Sections: secs}, true
	case id == "breakage-month":
		if _, err := time.Parse("2006-01", month); err != nil {
			month = time.Now().Format("2006-01")
		}
		rows := s.regLoad(regDefs["breakage"])
		s.decoratePars(regDefs["breakage"], rows)
		issues, qty, broke, lost, repeat, open := 0, 0.0, 0, 0, 0, 0
		cost := 0.0
		causes := map[string]int{}
		list := [][]string{}
		end := month + "-31"
		for _, r := range rows {
			if r["date"] >= month+"-01" && r["date"] <= end {
				issues++
				qty += mealNum(r["qty"])
				switch r["issueType"] {
				case "Broken", "Damaged":
					broke++
				case "Lost / Missing":
					lost++
				}
				if r["repeat"] == "Yes" {
					repeat++
				}
				c := mealNum(r["actualCost"])
				if c == 0 {
					c = mealNum(r["qtyReplace"]) * mealNum(r["unitCost"])
				}
				cost += c
				if r["cause"] != "" {
					causes[r["cause"]]++
				}
				list = append(list, []string{r["id"], r["date"], r["area"], r["item"], r["issueType"], r["qty"], r["cause"], r["status"]})
			}
			if r["date"] <= end && brkOpen(r) {
				open++
			}
		}
		top, tn := "", 0
		for k, v := range causes {
			if v > tn || (v == tn && k < top) {
				top, tn = k, v
			}
		}
		if top == "" {
			top = "none logged"
		}
		note := fmt.Sprintf("%d issues reported, %.0f items affected (%d broken or damaged, %d lost or missing). Replacement cost %s. %d still open. %d repeat issues. Top cause: %s.", issues, qty, broke, lost, money(cost), open, repeat, top)
		t, _ := time.Parse("2006-01", month)
		return &report{Title: "Monthly Breakage & Replacement Summary", Subtitle: t.Format("January 2006"), Sections: []reportSection{{Note: note, Cols: []string{"ID", "Date", "Area", "Item", "Issue", "Qty", "Cause", "Status"}, Rows: list}}}, true
	}
	return nil, false
}

func escAll(in []string) []string {
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(v)
	}
	return out
}

func dayIndex(d string) int {
	for i, x := range weekdays {
		if x == d {
			return i
		}
	}
	return 9
}
