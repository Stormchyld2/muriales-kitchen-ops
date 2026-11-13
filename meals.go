package main

// Employee meals: the recipes the kitchen uses for staff meals, a plan of what
// is served each day, and a simple cost per serving. Prices are typed in by
// hand. Purchasing, inventory and recipe costing for the menu stay in R365.

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

func init() {
	regDefs["mealrecipes"] = &collDef{Name: "mealrecipes", Title: "Employee Meal Recipes", NewLabel: "Add Meal Recipe", Prefix: "EMR-", Digits: 3, Deletable: true, Report: "meals-costs",
		Intro:    "Recipes for staff meals. List one ingredient per line as: amount | item | cost of that amount, for example 2 lb | ground beef | 9.50. Cost per serving is worked out for you.",
		ListCols: []string{"id", "name", "category", "serves", "cost", "perServing", "active"},
		Fields: []fieldDef{txt("name", "Meal", true), sel("category", "Category", []string{"Pasta", "Protein", "Sandwich", "Soup", "Salad", "Rice / Grain", "Breakfast", "Other"}, false), txt("serves", "Servings it makes", true),
			docF("ingredients", "Ingredients (amount | item | cost)", false), docF("method", "Method", false), txt("allergens", "Allergens", false), yn("active", "Active?"), lng("notes", "Notes", false)}}
	regDefs["mealplan"] = &collDef{Name: "mealplan", Title: "Employee Meal Plan", NewLabel: "Add Meal", Prefix: "MP-", Digits: 4, Desc: true, Deletable: true, Report: "meals-costs",
		Intro:    "What staff were or will be served each day. Pick a recipe so the cost is worked out, or just type what was served. Print gives the cost report.",
		ListCols: []string{"id", "date", "service", "mealName", "servings", "cost", "by"},
		Fields:   []fieldDef{dat("date", "Date", true), sel("service", "Meal", []string{"Breakfast", "Lunch", "Dinner"}, true), {Key: "recipeId", Label: "Recipe (optional)", Type: "ref", Src: "mealrecipes"}, txt("mealName", "What was served", true), txt("servings", "How many servings", false), txt("by", "Planned / made by", false), lng("notes", "Notes", false)}}
}

func mealNum(v string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(v, "$", ""), ",", "")), 64)
	return f
}

// mealCost adds up the last column of each ingredient line.
func mealCost(ingredients string) float64 {
	t := 0.0
	for _, l := range strings.Split(ingredients, "\n") {
		if i := strings.LastIndex(l, "|"); i >= 0 {
			t += mealNum(l[i+1:])
		}
	}
	return t
}

func money(f float64) string {
	if f == 0 {
		return ""
	}
	return fmt.Sprintf("$%.2f", f)
}

func mealPer(r rec) float64 {
	if n := mealNum(r["serves"]); n > 0 {
		return mealCost(r["ingredients"]) / n
	}
	return 0
}

func (s *store) decorateMeals(c *collDef, rows []rec) {
	switch c.Name {
	case "mealrecipes":
		for _, r := range rows {
			r["cost"], r["perServing"] = money(mealCost(r["ingredients"])), money(mealPer(r))
		}
	case "mealplan":
		per, name := map[string]float64{}, map[string]string{}
		for _, r := range s.regLoad(regDefs["mealrecipes"]) {
			per[r["id"]], name[r["id"]] = mealPer(r), r["name"]
		}
		for _, r := range rows {
			r["cost"] = money(per[r["recipeId"]] * mealNum(r["servings"]))
			r["recipeName"] = name[r["recipeId"]]
		}
	}
}

func (s *store) mealsReport() *report {
	rep := &report{Title: "Employee Meal Costs", Subtitle: time.Now().Format("January 2, 2006")}
	rec1 := regDefs["mealrecipes"]
	recipes := [][]string{}
	per := map[string]float64{}
	for _, r := range s.regSorted(rec1) {
		per[r["id"]] = mealPer(r)
		recipes = append(recipes, []string{r["name"], r["serves"], money(mealCost(r["ingredients"])), money(mealPer(r))})
	}
	rep.Sections = append(rep.Sections, reportSection{Heading: "Meal recipes", Cols: []string{"Meal", "Servings", "Total cost", "Cost per serving"}, Rows: recipes})
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
	note := "Last 30 days. Total cost " + money(total)
	if servings > 0 && total > 0 {
		note += fmt.Sprintf(", about $%.2f a serving.", total/servings)
	}
	rep.Sections = append(rep.Sections, reportSection{Heading: "Meals served", Note: note, Cols: []string{"Date", "Meal", "What was served", "Servings", "Cost"}, Rows: rows})
	return rep
}
