package main

// Food costing: a price list typed in once, and recipes (employee meals and
// specials) costed from it line by line. Ingredient lines are written
// "amount | ingredient | cost"; a cost typed on the line wins, otherwise the
// price list is used. Weights convert to weights and volumes to volumes; a
// count ("12 eggs") needs a price per each. Inventory, ordering and the menu
// itself stay in R365.

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var priceUnits = []string{"lb", "oz", "g", "kg", "gal", "qt", "pt", "cup", "fl oz", "tbsp", "tsp", "ml", "l", "each", "dozen", "can", "bunch", "head", "loaf", "pan", "bag", "box"}

func init() {
	regDefs["prices"] = &collDef{Name: "prices", Title: "Ingredient Price List", NewLabel: "Add Ingredient Price", Prefix: "PRC-", Digits: 4, Deletable: true, Report: "reg:prices",
		Intro:    "What ingredients cost. Enter the price of a case or bag and how much is in it (for example $62.00 for 15 lb) and the cost per pound is worked out. Recipes find an ingredient by name, so name it the way recipes do (Bacon, Eggs, Shredded Mozzarella).",
		ListCols: []string{"id", "ingredient", "packPrice", "packSize", "unit", "perUnit", "vendor"},
		Fields:   []fieldDef{txt("ingredient", "Ingredient", true), txt("packPrice", "Price paid ($)", true), txt("packSize", "How much that buys (number)", false), sel("unit", "Unit", priceUnits, true), txt("yield", "Usable % after trim (blank is 100)", false), txt("vendor", "Vendor", false), dat("updated", "Price checked on", false), lng("notes", "Notes", false)}}
	regDefs["specials"] = &collDef{Name: "specials", Title: "Specials", NewLabel: "Add Special", Prefix: "SPC-", Digits: 4, Desc: true, Deletable: true, Report: "reg:specials",
		Intro:    "A special with its recipe. Ingredients go one per line as: amount | ingredient. The cost per portion and food cost % are worked out from the Price List. Use Cost a Special to try one before saving it.",
		ListCols: []string{"id", "name", "startDate", "portions", "menuPrice", "perPortion", "foodPct", "missing"},
		Fields:   []fieldDef{txt("name", "Special", true), dat("startDate", "Starts", false), dat("endDate", "Ends", false), txt("portions", "Portions it makes", true), txt("menuPrice", "Menu price ($)", false), docF("ingredients", "Ingredients (amount | ingredient)", true), lng("method", "How it is made", false), lng("notes", "Notes", false), sel("status", "Status", []string{"Planned", "Running", "Ended"}, false)}}
}

// ---- units ----

type unitDef struct {
	fam  string // w weight, v volume, c count
	base float64
}

var unitTable = map[string]unitDef{
	"g": {"w", 1}, "kg": {"w", 1000}, "oz": {"w", 28.3495}, "lb": {"w", 453.592},
	"ml": {"v", 1}, "l": {"v", 1000}, "tsp": {"v", 4.92892}, "tbsp": {"v", 14.7868}, "fl oz": {"v", 29.5735}, "cup": {"v", 236.588}, "pt": {"v", 473.176}, "qt": {"v", 946.353}, "gal": {"v", 3785.41},
	"each": {"c", 1}, "dozen": {"c", 12},
}

var unitAlias = map[string]string{
	"lbs": "lb", "pound": "lb", "pounds": "lb", "#": "lb", "ounce": "oz", "ounces": "oz", "gram": "g", "grams": "g", "kilo": "kg", "kilos": "kg", "kilogram": "kg",
	"teaspoon": "tsp", "teaspoons": "tsp", "tbs": "tbsp", "tablespoon": "tbsp", "tablespoons": "tbsp", "cups": "cup", "c": "cup", "pint": "pt", "pints": "pt", "quart": "qt", "quarts": "qt",
	"gallon": "gal", "gallons": "gal", "liter": "l", "liters": "l", "litre": "l", "milliliter": "ml", "floz": "fl oz", "fl": "fl oz", "fluid ounce": "fl oz", "fluid ounces": "fl oz",
	"ea": "each", "ct": "each", "count": "each", "whole": "each", "piece": "each", "pieces": "each", "slice": "each", "slices": "each", "clove": "each", "cloves": "each", "doz": "dozen",
	"cans": "can", "bunches": "bunch", "heads": "head", "loaves": "loaf", "pans": "pan", "bags": "bag", "boxes": "box",
}

func normUnit(u string) string {
	u = strings.ToLower(strings.Trim(strings.TrimSpace(u), ".,"))
	if a, ok := unitAlias[u]; ok {
		return a
	}
	return u
}

func unitInfo(u string) unitDef {
	if d, ok := unitTable[u]; ok {
		return d
	}
	return unitDef{"c", 1}
}

var fracs = strings.NewReplacer("½", " 1/2", "¼", " 1/4", "¾", " 3/4", "⅓", " 1/3", "⅔", " 2/3", "⅛", " 1/8")
var qtyRe = regexp.MustCompile(`^(\d+(?:\.\d+)?(?:\s+\d+/\d+)?|\d+/\d+)(?:\s*[-–]\s*[\d./ ]+)?\s*(.*)$`)

// parseQty reads "2 lb", "1 1/2 cups", "12" and gives the number and unit.
func parseQty(a string) (float64, string, bool) {
	a = strings.TrimSpace(fracs.Replace(a))
	m := qtyRe.FindStringSubmatch(a)
	if m == nil {
		return 0, "", false
	}
	q := 0.0
	for _, part := range strings.Fields(m[1]) {
		if strings.Contains(part, "/") {
			n := strings.SplitN(part, "/", 2)
			x, _ := strconv.ParseFloat(n[0], 64)
			y, _ := strconv.ParseFloat(n[1], 64)
			if y != 0 {
				q += x / y
			}
		} else {
			x, _ := strconv.ParseFloat(part, 64)
			q += x
		}
	}
	return q, normUnit(m[2]), true
}

// ---- price book ----

type priceEntry struct {
	Name    string
	Display string
	Unit    string
	PerUnit float64 // after trim loss
}

type priceBook struct{ Entries []priceEntry }

func nameKey(v string) string {
	var out []string
	for _, w := range strings.Fields(strings.ToLower(regexp.MustCompile(`[^a-z0-9 ]+`).ReplaceAllString(strings.ToLower(v), " "))) {
		switch {
		case strings.HasSuffix(w, "oes") && len(w) > 4:
			w = w[:len(w)-2]
		case strings.HasSuffix(w, "s") && !strings.HasSuffix(w, "ss") && len(w) > 3:
			w = w[:len(w)-1]
		}
		out = append(out, w)
	}
	return strings.Join(out, " ")
}

func perUnitOf(r rec) float64 {
	price, size := mealNum(r["packPrice"]), mealNum(r["packSize"])
	if size <= 0 {
		size = 1
	}
	p := price / size
	if y := mealNum(r["yield"]); y > 0 && y < 100 {
		p = p / (y / 100)
	}
	return p
}

func (s *store) priceBook() *priceBook {
	pb := &priceBook{}
	for _, r := range s.regSorted(regDefs["prices"]) {
		if k := nameKey(r["ingredient"]); k != "" && mealNum(r["packPrice"]) > 0 {
			pb.Entries = append(pb.Entries, priceEntry{k, r["ingredient"], normUnit(r["unit"]), perUnitOf(r)})
		}
	}
	return pb
}

// find picks the price entry whose words are all in the ingredient text, in any
// order, preferring the one with the most words ("Shredded Mozzarella" finds
// "Mozzarella, Shredded").
func (pb *priceBook) find(ingredient string) *priceEntry {
	have := map[string]bool{}
	for _, w := range strings.Fields(nameKey(ingredient)) {
		have[w] = true
	}
	var best *priceEntry
	bestN := 0
	for i := range pb.Entries {
		e := &pb.Entries[i]
		ws := strings.Fields(e.Name)
		ok := len(ws) > 0
		for _, w := range ws {
			if !have[w] {
				ok = false
			}
		}
		if ok && (best == nil || len(ws) > bestN || (len(ws) == bestN && len(e.Name) > len(best.Name))) {
			best, bestN = e, len(ws)
		}
	}
	return best
}

// ---- costing a recipe ----

type costLine struct {
	Amount     string  `json:"amount"`
	Ingredient string  `json:"ingredient"`
	Cost       float64 `json:"cost"`
	Note       string  `json:"note"`
	Priced     bool    `json:"priced"`
	Skipped    bool    `json:"skipped"` // as needed, not costed on purpose
}

func (pb *priceBook) cost(ingredients string) (lines []costLine, total float64, missing int) {
	for _, raw := range strings.Split(ingredients, "\n") {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		p := strings.Split(raw, "|")
		for len(p) < 3 {
			p = append(p, "")
		}
		l := costLine{Amount: strings.TrimSpace(p[0]), Ingredient: strings.TrimSpace(p[1])}
		if l.Ingredient == "" { // a bare line with no bar: treat the whole thing as an ingredient
			l.Ingredient, l.Amount = l.Amount, ""
		}
		switch {
		case strings.TrimSpace(p[2]) != "":
			l.Cost, l.Priced, l.Note = mealNum(p[2]), true, "typed in"
		case strings.HasPrefix(strings.ToLower(l.Amount), "as needed") || strings.HasPrefix(strings.ToLower(l.Amount), "to taste"):
			l.Skipped, l.Note = true, "as needed, not costed"
		default:
			qty, unit, ok := parseQty(l.Amount)
			e := pb.find(l.Ingredient)
			switch {
			case e == nil:
				l.Note = "not on the price list"
			case !ok:
				l.Note = "no amount to cost"
			default:
				a, b := unitInfo(unit), unitInfo(e.Unit)
				switch {
				case a.fam == "c" && b.fam == "c" && (unit == "" || unit == e.Unit || unit == "each" || e.Unit == "each" || e.Unit == "dozen" || unit == "dozen"):
					n := qty * a.base
					if unit == "" {
						n = qty
					}
					l.Cost, l.Priced = n/b.base*e.PerUnit, true
					l.Note = fmt.Sprintf("%s at $%.2f per %s", e.Display, e.PerUnit, e.Unit)
				case a.fam == b.fam && a.fam != "c":
					l.Cost, l.Priced = qty*a.base/b.base*e.PerUnit, true
					l.Note = fmt.Sprintf("%s at $%.2f per %s", e.Display, e.PerUnit, e.Unit)
				default:
					l.Note = "price is per " + e.Unit + ", the recipe uses " + firstNonEmpty(unit, "a count") + " (cannot convert)"
				}
			}
		}
		if !l.Priced && !l.Skipped {
			missing++
		}
		total += l.Cost
		lines = append(lines, l)
	}
	return
}

// mealCostOf and mealPer price an employee meal recipe, typed costs first.
func (s *store) mealCostOf(ingredients string) float64 {
	_, t, _ := s.priceBook().cost(ingredients)
	return t
}

func (s *store) mealPer(r rec) float64 {
	if n := mealNum(r["serves"]); n > 0 {
		return s.mealCostOf(r["ingredients"]) / n
	}
	return 0
}

// ---- specials ----

type costResult struct {
	Lines      []costLine `json:"lines"`
	Total      float64    `json:"total"`
	Portions   float64    `json:"portions"`
	PerPortion float64    `json:"perPortion"`
	Price      float64    `json:"price"`
	Pct        float64    `json:"pct"`
	Target     float64    `json:"target"`
	SuggestAt  float64    `json:"suggest"` // menu price that would hit the target
	Missing    int        `json:"missing"`
}

type costSettings struct {
	TargetPct float64 `json:"targetPct"`
}

func (s *store) costTarget() float64 {
	c := costSettings{TargetPct: 30}
	s.readMaint("costing-settings.json", &c)
	if c.TargetPct <= 0 || c.TargetPct > 100 {
		c.TargetPct = 30
	}
	return c.TargetPct
}

func (s *store) costRecipe(ingredients, portions, price string) costResult {
	lines, total, missing := s.priceBook().cost(ingredients)
	r := costResult{Lines: lines, Total: total, Portions: mealNum(portions), Price: mealNum(price), Target: s.costTarget(), Missing: missing}
	if r.Portions > 0 {
		r.PerPortion = total / r.Portions
	}
	if r.Price > 0 && r.PerPortion > 0 {
		r.Pct = r.PerPortion / r.Price * 100
	}
	if r.PerPortion > 0 {
		r.SuggestAt = r.PerPortion / (r.Target / 100)
	}
	return r
}

func (s *store) decorateCosting(c *collDef, rows []rec) {
	switch c.Name {
	case "prices":
		for _, r := range rows {
			r["perUnit"] = ""
			if p := perUnitOf(r); p > 0 && mealNum(r["packPrice"]) > 0 {
				r["perUnit"] = fmt.Sprintf("$%.2f per %s", p, r["unit"])
			}
		}
	case "specials":
		for _, r := range rows {
			cr := s.costRecipe(r["ingredients"], r["portions"], r["menuPrice"])
			r["perPortion"], r["foodPct"], r["missing"] = "", "", ""
			if cr.PerPortion > 0 {
				r["perPortion"] = money(cr.PerPortion)
			}
			if cr.Pct > 0 {
				r["foodPct"] = fmt.Sprintf("%.0f%%", cr.Pct)
			}
			if cr.Missing > 0 {
				r["missing"] = fmt.Sprintf("%d unpriced", cr.Missing)
			}
		}
	}
}

// importPrices reads pasted lines: Ingredient | price | size | unit | vendor.
func (s *store) importPrices(text string) (added, updated int, bad []string) {
	c := regDefs["prices"]
	rows := s.regLoad(c)
	have := map[string]rec{}
	for _, r := range rows {
		have[nameKey(r["ingredient"])] = r
	}
	var fresh []rec
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		p := strings.Split(line, "|")
		for len(p) < 5 {
			p = append(p, "")
		}
		for i := range p {
			p[i] = strings.TrimSpace(p[i])
		}
		unit := normUnit(p[3])
		okUnit := false
		for _, u := range priceUnits {
			if u == unit {
				okUnit = true
			}
		}
		price := mealNum(p[1])
		if p[0] == "" || price <= 0 || !okUnit {
			bad = append(bad, line)
			continue
		}
		size := p[2]
		if mealNum(size) <= 0 {
			size = "1"
		}
		in := rec{"ingredient": p[0], "packPrice": strconv.FormatFloat(price, 'f', 2, 64), "packSize": size, "unit": unit, "vendor": p[4], "updated": todayStr()}
		if old, ok := have[nameKey(p[0])]; ok {
			for k, v := range in {
				if v != "" {
					old[k] = v
				}
			}
			updated++
			continue
		}
		fresh = append(fresh, in)
		have[nameKey(p[0])] = in
	}
	if updated > 0 {
		_ = s.regSave(c, rows)
	}
	for _, in := range fresh {
		if _, err := s.regAdd(c, in); err == nil {
			added++
		}
	}
	return
}

func todayStr() string { return time.Now().Format(dayFmt) }

func (s *store) costingRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/costing/settings", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		writeJSON(w, 200, map[string]any{"targetPct": s.costTarget(), "units": priceUnits})
	})
	mux.HandleFunc("POST /api/costing/settings", func(w http.ResponseWriter, r *http.Request) {
		var in costSettings
		if jsonDecode(r, &in) != nil || in.TargetPct <= 0 || in.TargetPct > 100 {
			fail(w, 400, "Enter a food cost percent between 1 and 100.")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		if err := s.writeMaint("costing-settings.json", in); err != nil {
			fail(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"targetPct": s.costTarget()})
	})
	mux.HandleFunc("POST /api/costing/calc", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Ingredients, Portions, Price string }
		if jsonDecode(r, &in) != nil {
			fail(w, 400, "Could not read that.")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		writeJSON(w, 200, s.costRecipe(in.Ingredients, in.Portions, in.Price))
	})
	mux.HandleFunc("POST /api/costing/prices/import", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Text, By string }
		if jsonDecode(r, &in) != nil {
			fail(w, 400, "Could not read that.")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		a, u, bad := s.importPrices(in.Text)
		s.rbRecost("Price list import", "")
		writeJSON(w, 200, map[string]any{"added": a, "updated": u, "bad": bad})
	})
	mux.HandleFunc("GET /api/costing/special/{id}", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, x := range s.regLoad(regDefs["specials"]) {
			if x["id"] == r.PathValue("id") {
				writeJSON(w, 200, map[string]any{"rec": x, "cost": s.costRecipe(x["ingredients"], x["portions"], x["menuPrice"])})
				return
			}
		}
		fail(w, 404, "That special was not found.")
	})
}

func (s *store) specialReport(id string) (*report, bool) {
	for _, x := range s.regLoad(regDefs["specials"]) {
		if x["id"] != id {
			continue
		}
		c := s.costRecipe(x["ingredients"], x["portions"], x["menuPrice"])
		rows := [][]string{}
		for _, l := range c.Lines {
			rows = append(rows, []string{l.Amount, l.Ingredient, money(l.Cost), l.Note})
		}
		sum := fmt.Sprintf("Makes %s portions. Total cost %s, %s a portion.", x["portions"], money(c.Total), money(c.PerPortion))
		if c.Price > 0 {
			sum += fmt.Sprintf(" Menu price %s, food cost %.0f%% (target %.0f%%).", money(c.Price), c.Pct, c.Target)
		}
		if c.SuggestAt > 0 {
			sum += fmt.Sprintf(" To hit the target, price it at about $%.2f.", c.SuggestAt)
		}
		if c.Missing > 0 {
			sum += fmt.Sprintf(" %d ingredient(s) have no price yet, so the cost is understated.", c.Missing)
		}
		secs := []reportSection{{Note: sum, Cols: []string{"Amount", "Ingredient", "Cost", "How priced"}, Rows: rows}}
		if strings.TrimSpace(x["method"]) != "" {
			secs = append(secs, reportSection{Heading: "How it is made", Text: x["method"]})
		}
		return &report{Title: "Special: " + x["name"], Subtitle: strings.TrimSpace(x["startDate"] + " " + x["endDate"]), Sections: secs}, true
	}
	return nil, false
}
