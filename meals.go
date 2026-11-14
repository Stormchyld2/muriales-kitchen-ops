package main

// Employee meals: a four-week rotating menu with a recipe for every meal, an
// appendix of the sauces and sides those recipes use, a meal log, and a
// dashboard showing which week the kitchen is on. Costs are optional and typed
// in by hand. Inventory, purchasing and menu costing stay in R365.

import (
	"encoding/json"
	"fmt"
	"html"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	mealDays  = []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}
	mealSlots = []string{"Breakfast and Lunch", "Dinner"}
)

func init() {
	regDefs["mealrecipes"] = &collDef{Name: "mealrecipes", Title: "Employee Meal Recipes", NewLabel: "Add Meal Recipe", Prefix: "EMR-", Digits: 3, Deletable: true, Starter: "", Report: "meals-costs",
		Intro:    "One recipe for each meal in the four-week rotation. Ingredients go one per line as: amount | ingredient | cost (the cost is optional). Fill in costs to see a cost per person.",
		ListCols: []string{"id", "week", "day", "meal", "name", "serves", "perServing"},
		Fields: []fieldDef{txt("name", "Meal", true), sel("week", "Rotation week", []string{"1", "2", "3", "4"}, false), sel("day", "Day", mealDays, false), sel("meal", "Meal", []string{"Breakfast and Lunch", "Dinner", "Breakfast", "Lunch"}, false), txt("serves", "People it feeds", true), txt("time", "Target time", false),
			docF("ingredients", "Ingredients (amount | ingredient | cost)", false), docF("method", "Method", false), lng("notes", "Production notes", false), txt("allergens", "Allergens", false), yn("active", "Active?")}}
	regDefs["mealappendix"] = &collDef{Name: "mealappendix", Title: "Meal Recipe Appendix", NewLabel: "Add Appendix Recipe", Prefix: "EMA-", Digits: 3, Deletable: true, Report: "reg:mealappendix",
		Intro:    "The sauces, gravies and sides that the meal recipes call for.",
		ListCols: []string{"id", "name", "kind", "yield", "time"},
		Fields:   []fieldDef{txt("name", "Recipe", true), txt("kind", "Kind", false), txt("yield", "Yield", false), txt("time", "Target time", false), docF("ingredients", "Ingredients (amount | ingredient | cost)", false), docF("method", "Method", false), lng("notes", "Production notes", false), yn("active", "Active?")}}
	regDefs["mealplan"] = &collDef{Name: "mealplan", Title: "Employee Meal Log", NewLabel: "Log a Meal", Prefix: "MP-", Digits: 4, Desc: true, Deletable: true, Report: "meals-costs",
		Intro:    "What staff were actually served. Pick the recipe and the cost is worked out when costs are entered. Print gives the cost report.",
		ListCols: []string{"id", "date", "service", "mealName", "servings", "assignedTo", "prepStart", "status", "cost"},
		Fields:   []fieldDef{dat("date", "Date", true), sel("service", "Meal", []string{"Breakfast", "Lunch", "Dinner"}, true), {Key: "recipeId", Label: "Recipe (optional)", Type: "ref", Src: "mealrecipes"}, txt("mealName", "Main dish", true), txt("sides", "Sides", false), txt("servings", "Expected people", false), txt("assignedTo", "Assigned to (first name)", false), txt("prepStart", "Prep / start time", false), sel("status", "Status", []string{"Planned", "Prepped", "Served", "Skipped"}, false), yn("overstock", "Using overstock or leftovers?"), txt("by", "Made by", false), lng("notes", "Notes", false)}}
}

// ---- costs ----

func mealNum(v string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(v, "$", ""), ",", "")), 64)
	return f
}

func money(f float64) string {
	if f == 0 {
		return ""
	}
	return fmt.Sprintf("$%.2f", f)
}

func (s *store) decorateMeals(c *collDef, rows []rec) {
	switch c.Name {
	case "mealrecipes":
		for _, r := range rows {
			r["cost"], r["perServing"] = money(s.mealCostOf(r["ingredients"])), money(s.mealPer(r))
		}
	case "mealplan":
		per, name := map[string]float64{}, map[string]string{}
		for _, r := range s.regLoad(regDefs["mealrecipes"]) {
			per[r["id"]], name[r["id"]] = s.mealPer(r), r["name"]
		}
		for _, r := range rows {
			r["cost"] = money(per[r["recipeId"]] * mealNum(r["servings"]))
			r["recipeName"] = name[r["recipeId"]]
		}
	}
}

// ---- rotation ----

type mealSettings struct {
	Anchor        string  `json:"anchor"` // a Monday that starts week 1
	WeekdayPeople int     `json:"weekdayPeople"`
	WeekendPeople int     `json:"weekendPeople"`
	BLTarget      float64 `json:"blTarget"`
	DinnerTarget  float64 `json:"dinnerTarget"`
	Minutes       int     `json:"minutes"`
}

func (s *store) mealSettings() mealSettings {
	m := mealSettings{Anchor: "2026-08-31", WeekdayPeople: 30, WeekendPeople: 45, BLTarget: 3, DinnerTarget: 5, Minutes: 30}
	s.readMaint("meal-settings.json", &m)
	if _, err := time.Parse(dayFmt, m.Anchor); err != nil {
		m.Anchor = "2026-08-31"
	}
	return m
}

func mondayOf(t time.Time) time.Time {
	t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
	return t.AddDate(0, 0, -((int(t.Weekday()) + 6) % 7))
}

// rotationWeek says which of the four weeks the Monday starting this week is.
func rotationWeek(anchor string, monday time.Time) int {
	a, _ := time.Parse(dayFmt, anchor)
	am := mondayOf(a)
	days := int(math.Round(monday.Sub(am).Hours() / 24))
	w := (days / 7) % 4
	if w < 0 {
		w += 4
	}
	return w + 1
}

type mealCell struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Serves    string  `json:"serves"`
	Time      string  `json:"time"`
	PerPerson string  `json:"perPerson"`
	PerNum    float64 `json:"perNum"`
}

type mealDay struct {
	Day    string    `json:"day"`
	Date   string    `json:"date,omitempty"`
	Today  bool      `json:"today"`
	People int       `json:"people"`
	BL     *mealCell `json:"bl"`
	Dinner *mealCell `json:"dinner"`
}

func (s *store) mealWeek(n int, monday time.Time) []mealDay {
	st := s.mealSettings()
	cell := map[string]*mealCell{}
	for _, r := range s.regSorted(regDefs["mealrecipes"]) {
		if r["active"] == "No" || r["week"] != strconv.Itoa(n) || r["day"] == "" {
			continue
		}
		slot := "bl"
		if strings.EqualFold(r["meal"], "Dinner") {
			slot = "dinner"
		}
		p := s.mealPer(r)
		cell[r["day"]+"|"+slot] = &mealCell{ID: r["id"], Name: r["name"], Serves: r["serves"], Time: r["time"], PerPerson: money(p), PerNum: p}
	}
	today := time.Now().Format(dayFmt)
	out := []mealDay{}
	for i, d := range mealDays {
		md := mealDay{Day: d, People: st.WeekdayPeople, BL: cell[d+"|bl"], Dinner: cell[d+"|dinner"]}
		if i >= 5 {
			md.People = st.WeekendPeople
		}
		if !monday.IsZero() {
			dt := monday.AddDate(0, 0, i).Format(dayFmt)
			md.Date, md.Today = dt, dt == today
		}
		out = append(out, md)
	}
	return out
}

func (s *store) mealDash() map[string]any {
	st := s.mealSettings()
	mon := mondayOf(time.Now())
	wk := rotationWeek(st.Anchor, mon)
	next := wk%4 + 1
	days := s.mealWeek(wk, mon)
	var todayDay *mealDay
	for i := range days {
		if days[i].Today {
			todayDay = &days[i]
		}
	}
	// recipes entered, out of the 56 meals in the rotation
	have, missing := 0, []string{}
	for w := 1; w <= 4; w++ {
		c := 0
		for _, d := range s.mealWeek(w, time.Time{}) {
			if d.BL != nil {
				c++
			}
			if d.Dinner != nil {
				c++
			}
		}
		have += c
		if c < 14 {
			missing = append(missing, fmt.Sprintf("Week %d has %d of 14", w, c))
		}
	}
	// meals logged this week
	logged := 0
	for _, r := range s.regLoad(regDefs["mealplan"]) {
		if r["date"] >= mon.Format(dayFmt) && r["date"] <= mon.AddDate(0, 0, 6).Format(dayFmt) {
			logged++
		}
	}
	return map[string]any{"week": wk, "next": next, "monday": mon.Format(dayFmt), "sunday": mon.AddDate(0, 0, 6).Format(dayFmt), "days": days, "nextDays": s.mealWeek(next, mon.AddDate(0, 0, 7)),
		"today": todayDay, "settings": st, "have": have, "total": 56, "missing": missing, "logged": logged}
}

func (s *store) mealCoverage() []map[string]any {
	out := []map[string]any{}
	for w := 1; w <= 4; w++ {
		days := s.mealWeek(w, time.Time{})
		missing := []string{}
		for _, d := range days {
			if d.BL == nil {
				missing = append(missing, d.Day+" breakfast/lunch")
			}
			if d.Dinner == nil {
				missing = append(missing, d.Day+" dinner")
			}
		}
		out = append(out, map[string]any{"week": w, "days": days, "missing": missing})
	}
	return out
}

// ---- recipe pages ----

func (s *store) mealMarkdown(r rec, withCost bool) string {
	var b strings.Builder
	meta := []string{}
	for _, p := range [][2]string{{"Week", r["week"]}, {"Day", r["day"]}, {"Meal", r["meal"]}, {"Kind", r["kind"]}, {"Feeds", firstNonEmpty(r["serves"], r["yield"])}, {"Target time", r["time"]}} {
		if p[1] != "" {
			meta = append(meta, "**"+p[0]+":** "+p[1])
		}
	}
	b.WriteString(strings.Join(meta, "  \n") + "\n\n## Ingredients\n\n")
	lines, total, missing := s.priceBook().cost(r["ingredients"])
	cost := withCost && total > 0
	if cost {
		b.WriteString("| Amount | Ingredient | Cost |\n|---|---|---|\n")
	} else {
		b.WriteString("| Amount | Ingredient |\n|---|---|\n")
	}
	for _, l := range lines {
		row := "| " + l.Amount + " | " + l.Ingredient
		if cost {
			c := money(l.Cost)
			if !l.Priced && !l.Skipped {
				c = "no price"
			}
			row += " | " + c
		}
		b.WriteString(row + " |\n")
	}
	if cost {
		per := 0.0
		if n := mealNum(firstNonEmpty(r["serves"], r["yield"])); n > 0 {
			per = total / n
		}
		b.WriteString(fmt.Sprintf("\n**Total %s, about $%.2f a person.**", money(total), per))
		if missing > 0 {
			b.WriteString(fmt.Sprintf(" (%d ingredient(s) have no price yet, so this is understated.)", missing))
		}
		b.WriteString("\n")
	}
	if strings.TrimSpace(r["method"]) != "" {
		b.WriteString("\n## Method\n\n" + r["method"] + "\n")
	}
	if strings.TrimSpace(r["notes"]) != "" {
		b.WriteString("\n> " + strings.ReplaceAll(strings.TrimSpace(r["notes"]), "\n", " ") + "\n")
	}
	return b.String()
}

// mealKey makes a recipe name comparable: "&" and "mac" spelled out, case folded.
func mealKey(v string) string {
	v = strings.ToLower(strings.ReplaceAll(v, "&", "and"))
	v = strings.ReplaceAll(v, "mac and cheese", "macaroni and cheese")
	return strings.ReplaceAll(v, "mac & cheese", "macaroni and cheese")
}

// mealAppendixLinks finds the appendix recipes a recipe's ingredient lines or
// method refer to, matching the full name or its core ("Brown Gravy for
// Schnitzel" -> "brown gravy", "Chicken Pot Pie Filling" -> "chicken pot pie").
func (s *store) mealAppendixLinks(r rec) []map[string]string {
	out := []map[string]string{}
	text := mealKey(r["name"] + " " + r["ingredients"] + " " + r["method"])
	for _, a := range s.regSorted(regDefs["mealappendix"]) {
		full := mealKey(strings.TrimSpace(a["name"]))
		if full == "" || strings.EqualFold(a["name"], r["name"]) {
			continue
		}
		core := full
		if i := strings.Index(core, " for "); i > 0 {
			core = core[:i]
		}
		if i := strings.Index(core, " over "); i > 0 {
			core = core[:i]
		}
		for _, suf := range []string{" filling", " sauce"} {
			if t := strings.TrimSuffix(core, suf); t != core && strings.Count(t, " ") >= 1 {
				core = t
			}
		}
		if strings.Contains(text, full) || strings.Contains(text, core) {
			out = append(out, map[string]string{"id": a["id"], "name": a["name"]})
		}
	}
	return out
}

func (s *store) mealsReport() *report {
	rep := &report{Title: "Employee Meal Costs", Subtitle: time.Now().Format("January 2, 2006")}
	recipes := [][]string{}
	per := map[string]float64{}
	for _, r := range s.regSorted(regDefs["mealrecipes"]) {
		per[r["id"]] = s.mealPer(r)
		recipes = append(recipes, []string{"Week " + r["week"] + " " + r["day"], r["meal"], r["name"], r["serves"], money(s.mealCostOf(r["ingredients"])), money(s.mealPer(r))})
	}
	rep.Sections = append(rep.Sections, reportSection{Heading: "Meal recipes", Note: "Costs show only where ingredient costs were entered.", Cols: []string{"When", "Meal", "Recipe", "Feeds", "Total cost", "Per person"}, Rows: recipes})
	from := time.Now().AddDate(0, 0, -29).Format(dayFmt)
	plan := s.regLoad(regDefs["mealplan"])
	sort.SliceStable(plan, func(i, j int) bool { return plan[i]["date"] > plan[j]["date"] })
	rows, total, servings := [][]string{}, 0.0, 0.0
	for _, r := range plan {
		if r["date"] < from {
			continue
		}
		c := per[r["recipeId"]] * mealNum(r["servings"])
		total += c
		servings += mealNum(r["servings"])
		rows = append(rows, []string{r["date"], r["service"], r["mealName"], r["servings"], money(c)})
	}
	note := "Last 30 days."
	if total > 0 {
		note += " Total cost " + money(total)
		if servings > 0 {
			note += fmt.Sprintf(", about $%.2f a person.", total/servings)
		}
	}
	rep.Sections = append(rep.Sections, reportSection{Heading: "Meals served", Note: note, Cols: []string{"Date", "Meal", "What was served", "People", "Cost"}, Rows: rows})
	return rep
}

func (s *store) mealReports(id string) (*report, bool) {
	if id == "meal-binder" {
		return s.mealBinderReport(), true
	}
	if n, ok := strings.CutPrefix(id, "mealweek:"); ok {
		wk, _ := strconv.Atoi(n)
		if wk < 1 || wk > 4 {
			return nil, false
		}
		st := s.mealSettings()
		rows := [][]string{}
		for _, d := range s.mealWeek(wk, time.Time{}) {
			cell := func(c *mealCell) string {
				if c == nil {
					return "(no recipe yet)"
				}
				return c.Name
			}
			rows = append(rows, []string{d.Day, fmt.Sprint(d.People), cell(d.BL), cell(d.Dinner)})
		}
		note := fmt.Sprintf("Each meal is planned to take about %d minutes. Breakfast and lunch cost target $%.2f a person, dinner $%.2f.", st.Minutes, st.BLTarget, st.DinnerTarget)
		return &report{Title: "Employee Meals - Week " + n, Subtitle: time.Now().Format("January 2, 2006"), Sections: []reportSection{{Note: note, Cols: []string{"Day", "People", "Breakfast and Lunch", "Dinner"}, Rows: rows}}}, true
	}
	for _, p := range []struct{ pre, coll string }{{"meal:", "mealrecipes"}, {"mealapp:", "mealappendix"}} {
		if rid, ok := strings.CutPrefix(id, p.pre); ok {
			for _, r := range s.regLoad(regDefs[p.coll]) {
				if r["id"] == rid {
					return &report{Title: r["name"], Subtitle: "Employee meal recipe", Sections: []reportSection{{HTML: mdHTML(s.mealMarkdown(r, true)), Text: mdPlain(s.mealMarkdown(r, true))}}}, true
				}
			}
		}
	}
	return nil, false
}

func (s *store) mealRoutes(mux *http.ServeMux) {
	s.mu.Lock()
	s.seedOnce("meals-231a", "mealrecipes", "meal_recipes.json")
	s.seedOnce("meals-231b", "mealappendix", "meal_appendix.json")
	s.seedOnce("meals-231c", "mealrecipes", "meal_recipes_w2.json")
	s.mealUpdate238()
	s.mu.Unlock()
	mux.HandleFunc("GET /api/meals/dash", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		writeJSON(w, 200, s.mealDash())
	})
	mux.HandleFunc("GET /api/meals/coverage", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		writeJSON(w, 200, map[string]any{"weeks": s.mealCoverage(), "settings": s.mealSettings(), "current": rotationWeek(s.mealSettings().Anchor, mondayOf(time.Now()))})
	})
	mux.HandleFunc("GET /api/meals/settings", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		writeJSON(w, 200, s.mealSettings())
	})
	mux.HandleFunc("POST /api/meals/settings", func(w http.ResponseWriter, r *http.Request) {
		var in mealSettings
		if jsonDecode(r, &in) != nil {
			fail(w, 400, "Could not read that.")
			return
		}
		if _, err := time.Parse(dayFmt, in.Anchor); err != nil {
			fail(w, 400, "The start date is not a valid date.")
			return
		}
		if in.WeekdayPeople < 1 || in.WeekendPeople < 1 || in.Minutes < 1 || in.BLTarget < 0 || in.DinnerTarget < 0 {
			fail(w, 400, "Check the numbers: head counts and minutes need to be 1 or more.")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		if err := s.writeMaint("meal-settings.json", in); err != nil {
			fail(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, s.mealSettings())
	})
	page := func(coll string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			s.mu.Lock()
			defer s.mu.Unlock()
			for _, x := range s.regLoad(regDefs[coll]) {
				if x["id"] == r.PathValue("id") {
					writeJSON(w, 200, map[string]any{"rec": x, "html": mdHTML(s.mealMarkdown(x, true)), "appendix": s.mealAppendixLinks(x), "title": html.EscapeString(x["name"])})
					return
				}
			}
			fail(w, 404, "That recipe was not found.")
		}
	}
	mux.HandleFunc("GET /api/meals/recipe/{id}", page("mealrecipes"))
	mux.HandleFunc("GET /api/meals/appendix/{id}", page("mealappendix"))
}

// mealUpdate238 brings in the revised Week 1 and Week 4 recipes and five new
// appendix recipes. It runs once; it replaces only the five named slots.
func (s *store) mealUpdate238() {
	flag := filepath.Join(s.maintDir(), "seed-meals-238.flag")
	if _, err := os.Stat(flag); err == nil {
		return
	}
	b, err := starterFiles.ReadFile("starter/meal_updates.json")
	if err != nil {
		return
	}
	var u struct {
		Replace  []rec `json:"replace"`
		Appendix []rec `json:"appendix"`
	}
	if json.Unmarshal(b, &u) != nil {
		return
	}
	rc := regDefs["mealrecipes"]
	rows := s.regLoad(rc)
	for _, n := range u.Replace {
		for _, r := range rows {
			if r["week"] == n["week"] && r["day"] == n["day"] && r["meal"] == n["meal"] {
				for _, k := range []string{"name", "serves", "time", "ingredients", "method", "notes"} {
					r[k] = n[k]
				}
			}
		}
	}
	ok := s.regSave(rc, rows) == nil
	ac := regDefs["mealappendix"]
	arows := s.regLoad(ac)
	have := map[string]bool{}
	for _, r := range arows {
		have[strings.ToLower(r["name"])] = true
	}
	for _, a := range u.Appendix {
		if !have[strings.ToLower(a["name"])] {
			next := 0
			for _, r := range arows {
				var n int
				if _, err := fmt.Sscanf(r["id"], "EMA-%d", &n); err == nil && n > next {
					next = n
				}
			}
			a["id"] = fmt.Sprintf("EMA-%03d", next+1)
			arows = append(arows, a)
		}
	}
	if ok && s.regSave(ac, arows) == nil {
		_ = os.MkdirAll(s.maintDir(), 0o755)
		_ = os.WriteFile(flag, []byte("done"), 0o644)
	}
}

// mealBinderReport is the whole Employee Meal binder: cover, the four weekly
// menus, every recipe, the appendix recipes and the production notes.
func (s *store) mealBinderReport() *report {
	st := s.mealSettings()
	cover := `<div style="text-align:center;padding:40px 0 20px"><img src="/binder-cover-logo.png" alt="" style="width:360px;max-width:80%"><h1 style="font-size:34px;letter-spacing:.04em;margin:34px 0 10px">EMPLOYEE MEAL SYSTEM &amp; RECIPE BINDER</h1>` +
		`<p style="font-size:17px;letter-spacing:.08em;margin:4px 0">MONTHLY MENUS &bull; WEEKLY RECIPES</p><p style="font-size:17px;letter-spacing:.08em;margin:4px 0">APPENDIX RECIPES &bull; PRODUCTION GUIDES</p>` +
		`<p style="font-size:17px;letter-spacing:.08em;margin:4px 0 40px">EMPLOYEE MEAL SYSTEM INSTRUCTIONS</p><p style="font-size:20px;font-weight:700;margin:0">Muriale's Italian Kitchen</p><p style="font-size:16px;margin:4px 0">Kitchen Operations</p></div>`
	secs := []reportSection{{HTML: cover, Text: "Employee Meal System & Recipe Binder - Muriale's Italian Kitchen, Kitchen Operations"}}
	secs = append(secs, reportSection{Heading: "Employee Meal System Instructions", Group: true})
	secs = append(secs, reportSection{Note: fmt.Sprintf("Staff meals run on a 4-week rotation. Each meal is planned to take about %d minutes. Breakfast and lunch cost target $%.2f a person; dinner $%.2f. The cook plans from the weekly menu, uses the recipe in this binder, and logs what was served in the Employee Meal Log (main dish, sides, how many people, who made it, whether overstock or leftovers were used). Appendix recipes are the sauces, sides and bases the weekly recipes call for.", st.Minutes, st.BLTarget, st.DinnerTarget)})
	secs = append(secs, reportSection{Heading: "Monthly Menus", Group: true})
	for wk := 1; wk <= 4; wk++ {
		rows := [][]string{}
		for _, d := range s.mealWeek(wk, time.Time{}) {
			bl, dn := "(no recipe yet)", "(no recipe yet)"
			if d.BL != nil {
				bl = d.BL.Name
			}
			if d.Dinner != nil {
				dn = d.Dinner.Name
			}
			rows = append(rows, []string{d.Day, fmt.Sprint(d.People), bl, dn})
		}
		secs = append(secs, reportSection{Heading: fmt.Sprintf("Week %d", wk), Cols: []string{"Day", "People", "Breakfast and Lunch", "Dinner"}, Rows: rows})
	}
	prod := [][]string{}
	for _, part := range []struct{ title, coll string }{{"Weekly Recipes", "mealrecipes"}, {"Appendix Recipes", "mealappendix"}} {
		secs = append(secs, reportSection{Heading: part.title, Group: true})
		n := 0
		for _, r := range s.regSorted(regDefs[part.coll]) {
			if r["active"] == "No" {
				continue
			}
			n++
			md := s.mealMarkdown(r, true)
			secs = append(secs, reportSection{Heading: r["name"], HTML: mdHTML(md), Text: mdPlain(md)})
			if strings.TrimSpace(r["notes"]) != "" {
				prod = append(prod, []string{r["name"], r["yield"], r["time"], r["notes"]})
			}
		}
		if n == 0 {
			secs = append(secs, reportSection{Note: "No recipes have been added yet."})
		}
	}
	secs = append(secs, reportSection{Heading: "Production Guides", Group: true})
	if len(prod) == 0 {
		secs = append(secs, reportSection{Note: "No production notes have been written yet. Add them on each recipe under Production notes."})
	} else {
		secs = append(secs, reportSection{Cols: []string{"Recipe", "Yield", "Target time", "Production notes"}, Rows: prod})
	}
	return &report{Title: "Employee Meal System & Recipe Binder", Subtitle: time.Now().Format("January 2, 2006"), Sections: secs}
}
