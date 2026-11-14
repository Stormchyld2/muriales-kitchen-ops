package main

import (
	"fmt"
	"html"
	"math"
	"net/http"
	"strings"
	"time"
)

// Recipe Book: every menu recipe with its procedure, standards, food safety
// controls, cost and cost-change history.

var rbCats = []string{"Appetizer", "Entree", "Side", "Sauce", "Soup", "Dessert", "Prep Component", "Staff Meal", "Other"}

func init() {
	regDefs["recipes"] = &collDef{Name: "recipes", Title: "Recipe Book", NewLabel: "Add Recipe", Prefix: "RC-", Digits: 4, Deletable: true, Report: "rbook-all", After: rbAfter,
		Intro:    "Every menu recipe in one place: ingredients, steps, service standards, food safety and approval. The cost per portion and food cost % come from the Price List; a recipe over 35% (or more than 3 points over its target) shows REVIEW. Any change to cost or price is written to Cost History.",
		ListCols: []string{"id", "name", "category", "station", "status", "perPortion", "menuPrice", "foodPct", "flag"},
		Fields: []fieldDef{txt("name", "Recipe name", true), sel("category", "Category", rbCats, true), sel("status", "Status", []string{"Active", "Inactive", "Test"}, true), txt("version", "Version", false), station("station"), txt("menuSection", "Menu section", false), lng("purpose", "Purpose / where it is used", false),
			txt("yieldQty", "Yield (number)", false), txt("yieldUnit", "Yield unit", false), txt("portionSize", "Portion size", false), txt("portions", "Portions per recipe", true), txt("menuPrice", "Current selling price ($)", false), txt("targetPct", "Target food cost % (blank uses the default)", false),
			docF("ingredients", "Ingredients (amount | ingredient)", true), lng("ingNotes", "Ingredient notes (prep, brand, substitutions, allergens)", false),
			docF("procedure", "Procedure (one step per line: step | equipment | time | temp | CCP)", false),
			txt("serviceTemp", "Service temperature", false), txt("holdLimit", "Holding limit", false), yn("reheat", "Reheat allowed?"), txt("vessel", "Plating vessel", false), txt("garnish", "Garnish / finish", false), lng("standards", "Appearance, texture and flavor standard", false), lng("commonErrors", "Common errors", false), lng("storage", "Storage and cooling", false),
			txt("allergens", "Contains major allergens", false), lng("crossContact", "Cross-contact risk", false), txt("cookTemp", "Cook temperature", false), txt("shelfLife", "Shelf life", false), lng("foodSafety", "Hot/cold holding, cooling, reheating, labeling", false),
			txt("preparedBy", "Prepared by", false), txt("approvedBy", "Approved by", false), dat("revised", "Date revised", false), lng("revReason", "Reason for revision", false), yn("trainReq", "Training required for this version?"), txt("changeReason", "Reason for a cost or price change (goes in Cost History)", false), lng("notes", "Notes", false),
			{Key: "lastCost", Label: "Last costed portion", Type: "hidden"}, {Key: "lastPrice", Label: "Last price", Type: "hidden"}}}
	regDefs["costhist"] = &collDef{Name: "costhist", Title: "Cost Change History", NewLabel: "Log a Cost Change", Prefix: "CHG-", Digits: 4, Desc: true, Deletable: true, Report: "reg:costhist",
		Intro:    "Every change to a recipe's portion cost or selling price. Entries are added by themselves when a recipe or the Price List changes; you can also log one by hand.",
		ListCols: []string{"id", "date", "recipe", "oldCost", "newCost", "oldPrice", "newPrice", "reason", "by"},
		Fields:   []fieldDef{dat("date", "Date", true), txt("recipe", "Recipe", true), txt("oldCost", "Old portion cost ($)", false), txt("newCost", "New portion cost ($)", false), txt("oldPrice", "Old selling price ($)", false), txt("newPrice", "New selling price ($)", false), txt("reason", "Reason", false), txt("by", "Changed by", false), lng("notes", "Notes", false)}}
	regDefs["prices"].After = func(s *store, old, nw rec) {
		for _, k := range []string{"packPrice", "packSize", "unit", "yield", "ingredient"} {
			if old[k] != nw[k] {
				s.rbRecost("Price list: "+nw["ingredient"], "")
				return
			}
		}
	}
}

func station(k string) fieldDef { return fieldDef{Key: k, Label: "Station", Type: "station"} }

func rbTarget(s *store, r rec) float64 {
	if t := mealNum(strings.TrimSuffix(strings.TrimSpace(r["targetPct"]), "%")); t > 0 && t <= 100 {
		return t
	}
	return s.costTarget()
}

func rbFlag(pct, target float64) string {
	if pct > 35 || (pct > 0 && pct > target+3) {
		return "REVIEW"
	}
	if pct > 0 {
		return "OK"
	}
	return ""
}

func (s *store) decorateRecipes(rows []rec) {
	for _, r := range rows {
		c := s.costRecipe(r["ingredients"], r["portions"], r["menuPrice"])
		r["perPortion"], r["foodPct"], r["flag"], r["missing"], r["suggest"] = "", "", "", "", ""
		if c.PerPortion > 0 {
			r["perPortion"] = money(c.PerPortion)
			t := rbTarget(s, r)
			r["suggest"] = money(c.PerPortion / (t / 100))
			if c.Pct > 0 {
				r["foodPct"] = fmt.Sprintf("%.1f%%", c.Pct)
				r["flag"] = rbFlag(c.Pct, t)
			}
		}
		if c.Missing > 0 {
			r["missing"] = fmt.Sprintf("%d unpriced", c.Missing)
		}
	}
}

// rbRecost compares each recipe's current cost and price with what was last
// recorded and writes a history entry for every change. A recipe with no
// record yet just gets its starting point saved. Caller holds s.mu.
func (s *store) rbRecost(reason, onlyID string) int {
	c, h := regDefs["recipes"], regDefs["costhist"]
	rows := s.regLoad(c)
	hist := s.regLoad(h)
	n, today := 0, time.Now().Format("2006-01-02")
	dirty := false
	for _, r := range rows {
		if onlyID != "" && r["id"] != onlyID {
			continue
		}
		cr := s.costRecipe(r["ingredients"], r["portions"], r["menuPrice"])
		cost := fmt.Sprintf("%.2f", cr.PerPortion)
		price := fmt.Sprintf("%.2f", cr.Price)
		if r["lastCost"] == "" && r["lastPrice"] == "" {
			r["lastCost"], r["lastPrice"], dirty = cost, price, true
			continue
		}
		oc, op := mealNum(r["lastCost"]), mealNum(r["lastPrice"])
		if math.Abs(oc-cr.PerPortion) < 0.005 && math.Abs(op-cr.Price) < 0.005 {
			continue
		}
		why := reason
		if w := strings.TrimSpace(r["changeReason"]); w != "" {
			why = w
		}
		e := rec{"date": today, "recipe": r["name"], "oldCost": r["lastCost"], "newCost": cost, "oldPrice": r["lastPrice"], "newPrice": price, "reason": why, "by": "", "created": time.Now().Format("2006-01-02 15:04")}
		e["id"] = h.nextID(hist)
		hist = append(hist, e)
		r["lastCost"], r["lastPrice"], r["changeReason"], dirty = cost, price, "", true
		n++
	}
	if dirty {
		_ = s.regSave(c, rows)
	}
	if n > 0 {
		_ = s.regSave(h, hist)
	}
	return n
}

func rbAfter(s *store, old, nw rec) { s.rbRecost("Recipe edited", nw["id"]) }

func (s *store) recipeBookRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/rbook/recost", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		writeJSON(w, 200, map[string]any{"changed": s.rbRecost("Re-costed from the Price List", "")})
	})
	mux.HandleFunc("GET /api/rbook/summary", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		writeJSON(w, 200, s.rbSummary())
	})
}

func (s *store) rbSummary() map[string]any {
	rows := s.regLoad(regDefs["recipes"])
	s.decorateRecipes(rows)
	act, rev, review, sum, n := 0, 0, 0, 0.0, 0
	val := 0.0
	for _, r := range rows {
		if r["status"] == "Inactive" {
			continue
		}
		act++
		if r["flag"] == "REVIEW" {
			rev++
		}
		if r["missing"] != "" || r["foodPct"] == "" {
			review++
		}
		if p := mealNum(strings.TrimSuffix(r["foodPct"], "%")); p > 0 {
			sum += p
			n++
		}
		val += mealNum(r["menuPrice"])
	}
	avg := 0.0
	if n > 0 {
		avg = sum / float64(n)
	}
	return map[string]any{"active": act, "avgPct": avg, "over": rev, "needsInfo": review, "menuValue": val}
}

func rbLines(s string) [][]string {
	out := [][]string{}
	for i, ln := range strings.Split(s, "\n") {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		p := strings.Split(ln, "|")
		for len(p) < 5 {
			p = append(p, "")
		}
		out = append(out, []string{fmt.Sprint(i + 1), strings.TrimSpace(p[0]), strings.TrimSpace(p[1]), strings.TrimSpace(p[2]), strings.TrimSpace(p[3]), strings.TrimSpace(p[4])})
	}
	return out
}

func (s *store) recipeReport(id string) (*report, bool) {
	if id == "rbook-cost" {
		rows := s.regSorted(regDefs["recipes"])
		s.decorateRecipes(rows)
		out := [][]string{}
		for _, r := range rows {
			out = append(out, []string{r["id"], r["name"], r["category"], r["perPortion"], money(mealNum(r["menuPrice"])), r["foodPct"], r["suggest"], r["flag"]})
		}
		return &report{Title: "Recipe Cost Summary", Subtitle: time.Now().Format("January 2, 2006"), Sections: []reportSection{{Cols: []string{"ID", "Recipe", "Category", "Portion cost", "Price", "Food cost", "Price at target", "Flag"}, Rows: out}}}, true
	}
	if id == "rbook-all" {
		rows := s.regSorted(regDefs["recipes"])
		secs := []reportSection{}
		for _, r := range rows {
			if rep, ok := s.recipeReport("rbook:" + r["id"]); ok {
				secs = append(secs, reportSection{Heading: r["name"], Group: true})
				secs = append(secs, rep.Sections...)
			}
		}
		if len(secs) == 0 {
			secs = []reportSection{{Note: "No recipes have been added yet."}}
		}
		return &report{Title: "Recipe Book", Subtitle: time.Now().Format("January 2, 2006"), Sections: secs}, true
	}
	if !strings.HasPrefix(id, "rbook:") {
		return nil, false
	}
	rid := strings.TrimPrefix(id, "rbook:")
	rows := s.regLoad(regDefs["recipes"])
	s.decorateRecipes(rows)
	for _, r := range rows {
		if r["id"] != rid {
			continue
		}
		kv := func(pairs ...string) [][]string {
			o := [][]string{}
			for i := 0; i+1 < len(pairs); i += 2 {
				if strings.TrimSpace(pairs[i+1]) != "" {
					o = append(o, []string{pairs[i], pairs[i+1]})
				}
			}
			return o
		}
		yn2 := func(v string) string {
			if v == "Yes" || v == "yes" || v == "true" {
				return "Yes"
			}
			if v == "No" || v == "no" || v == "false" {
				return "No"
			}
			return v
		}
		info := kv("Recipe", r["name"]+" ("+r["id"]+")", "Category", r["category"], "Status", r["status"], "Version", r["version"], "Station", r["station"], "Menu section", r["menuSection"], "Yield", strings.TrimSpace(r["yieldQty"]+" "+r["yieldUnit"]), "Portion size", r["portionSize"], "Portions", r["portions"], "Purpose", r["purpose"])
		cr := s.costRecipe(r["ingredients"], r["portions"], r["menuPrice"])
		ing := [][]string{}
		for _, l := range cr.Lines {
			ing = append(ing, []string{l.Amount, l.Ingredient, money(l.Cost), l.Note})
		}
		secs := []reportSection{{Heading: "Recipe Information", Cols: []string{"", ""}, Rows: info}, {Heading: "Ingredients", Cols: []string{"Amount", "Ingredient", "Cost", "How priced"}, Rows: ing}}
		if r["ingNotes"] != "" {
			secs = append(secs, reportSection{Text: r["ingNotes"], HTML: "<p style=\"white-space:pre-wrap\">" + html.EscapeString(r["ingNotes"]) + "</p>"})
		}
		if st := rbLines(r["procedure"]); len(st) > 0 {
			rowsP := [][]string{}
			for i, p := range st {
				rowsP = append(rowsP, []string{fmt.Sprint(i + 1), p[1], p[2], p[3], p[4], p[5]})
			}
			secs = append(secs, reportSection{Heading: "Procedure", Cols: []string{"Step", "What to do", "Equipment", "Time", "Temp", "CCP"}, Rows: rowsP})
		}
		if v := kv("Service temperature", r["serviceTemp"], "Holding limit", r["holdLimit"], "Reheat allowed", yn2(r["reheat"]), "Plating vessel", r["vessel"], "Garnish / finish", r["garnish"], "Standard", r["standards"], "Common errors", r["commonErrors"], "Storage and cooling", r["storage"]); len(v) > 0 {
			secs = append(secs, reportSection{Heading: "Service and Quality Standards", Cols: []string{"", ""}, Rows: v})
		}
		if v := kv("Major allergens", r["allergens"], "Cross-contact risk", r["crossContact"], "Cook temperature", r["cookTemp"], "Shelf life", r["shelfLife"], "Holding, cooling, reheating, labeling", r["foodSafety"]); len(v) > 0 {
			secs = append(secs, reportSection{Heading: "Food Safety and Allergen Controls", Cols: []string{"", ""}, Rows: v})
		}
		costNote := fmt.Sprintf("Cost %s a portion", money(cr.PerPortion))
		if cr.Price > 0 {
			costNote += fmt.Sprintf(", price %s, food cost %.1f%% (target %.0f%%) %s", money(cr.Price), cr.Pct, rbTarget(s, r), r["flag"])
		}
		if cr.Missing > 0 {
			costNote += fmt.Sprintf(". %d ingredient(s) not priced.", cr.Missing)
		}
		secs = append(secs, reportSection{Heading: "Cost", Note: costNote})
		if v := kv("Prepared by", r["preparedBy"], "Approved by", r["approvedBy"], "Date revised", r["revised"], "Reason for revision", r["revReason"], "Training required", yn2(r["trainReq"]), "Notes", r["notes"]); len(v) > 0 {
			secs = append(secs, reportSection{Heading: "Approval and Revision", Cols: []string{"", ""}, Rows: v})
		}
		return &report{Title: "Recipe: " + r["name"], Subtitle: r["category"] + " " + r["version"], Sections: secs}, true
	}
	return nil, false
}
