package main

// Required training by role: which position needs which training, how often it
// is refreshed and whether a sign-off is needed. The employee matrix and the
// expiry dates follow these rows.

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

const allStaff = "All Staff"

func init() {
	regDefs["trainreq"] = &collDef{Name: "trainreq", Title: "Role Training Requirements", NewLabel: "Add Requirement", Prefix: "REQ-", Digits: 4, Deletable: true, Report: "training-roles", Starter: "",
		Intro:    "What each position must be trained on. Role is a position or department word (Dish, Prep, Manager) or All Staff. Set Required to No to excuse a role from a training that applies to everyone else. A refresh interval here replaces the one on the training type for that role.",
		ListCols: []string{"id", "role", "typeName", "required", "refresh", "signoff", "reviewed"},
		Fields:   []fieldDef{txt("role", "Position / role", true), {Key: "typeId", Label: "Training", Type: "ref", Src: "trainingtypes", Req: true}, yn("required", "Required?"), txt("initial", "Initial training (when, who)", false), txt("refresh", "Refresh every (months, blank to use the training's)", false), txt("trainer", "Trainer / qualified by", false), yn("signoff", "Sign-off required?"), txt("sop", "SOP / policy reference", false), dat("reviewed", "Last reviewed", false), lng("notes", "Notes", false)}}
}

func (s *store) decorateTrainReq(rows []rec) {
	typ := map[string]string{}
	for _, t := range s.regLoad(regDefs["trainingtypes"]) {
		typ[t["id"]] = t["name"]
	}
	for _, r := range rows {
		r["typeName"] = firstNonEmpty(typ[r["typeId"]], "(removed)")
	}
}

// roleRule says what the requirement rows decide for one employee and training:
// yes, no, or neither (0), plus any refresh override in months.
func (s *store) roleRule(e rec, typeID string) (yes, no bool, refresh int) {
	for _, q := range s.regLoad(regDefs["trainreq"]) {
		if q["typeId"] != typeID || !trainApplies(q["role"], e) {
			continue
		}
		if q["required"] == "No" {
			if !strings.EqualFold(strings.TrimSpace(q["role"]), allStaff) {
				no = true // a specific position excused from it
			}
			continue
		}
		yes = true
		if m, err := strconv.Atoi(strings.TrimSpace(q["refresh"])); err == nil && m > 0 {
			refresh = m
		}
	}
	return
}

// trainNeeded combines the training's own "applies to" with the role rows.
func (s *store) trainNeeded(t, e rec) bool {
	yes, no, _ := s.roleRule(e, t["id"])
	if no {
		return false
	}
	if yes {
		return true
	}
	return trainApplies(t["appliesTo"], e)
}

type roleCell struct {
	Required bool   `json:"required"`
	Signoff  bool   `json:"signoff"`
	Refresh  string `json:"refresh,omitempty"`
	People   int    `json:"people"`
	Current  int    `json:"current"`
	ReqID    string `json:"reqId,omitempty"`
}

type roleRowOut struct {
	Role   string              `json:"role"`
	People int                 `json:"people"`
	Cells  map[string]roleCell `json:"cells"`
}

// roleMatrix lays the roles against the trainings, with how many of the people
// in each role are current. Callers hold s.mu.
func (s *store) roleMatrix() map[string]any {
	today, _ := time.Parse(dayFmt, time.Now().Format(dayFmt))
	var types []rec
	out := []map[string]string{}
	for _, t := range s.regSorted(regDefs["trainingtypes"]) {
		if t["active"] != "No" {
			types = append(types, t)
			out = append(out, map[string]string{"id": t["id"], "name": t["name"]})
		}
	}
	reqs := s.regSorted(regDefs["trainreq"])
	seen, roles := map[string]bool{}, []string{}
	for _, q := range reqs {
		k := strings.TrimSpace(q["role"])
		if k != "" && !seen[strings.ToLower(k)] {
			seen[strings.ToLower(k)] = true
			roles = append(roles, k)
		}
	}
	sort.SliceStable(roles, func(i, j int) bool {
		if (roles[i] == allStaff) != (roles[j] == allStaff) {
			return roles[i] == allStaff
		}
		return strings.ToLower(roles[i]) < strings.ToLower(roles[j])
	})
	latest := map[string]rec{}
	for _, r := range s.regLoad(regDefs["training"]) {
		k := r["employeeId"] + "|" + r["typeId"]
		if old, ok := latest[k]; !ok || r["completed"] > old["completed"] || (r["completed"] == old["completed"] && r["id"] > old["id"]) {
			latest[k] = r
		}
	}
	var emps []rec
	for _, e := range s.regLoad(regDefs["employees"]) {
		if e["status"] != "Inactive" {
			emps = append(emps, e)
		}
	}
	rows := []roleRowOut{}
	for _, role := range roles {
		ro := roleRowOut{Role: role, Cells: map[string]roleCell{}}
		var mine []rec
		for _, e := range emps {
			if trainApplies(role, e) {
				mine = append(mine, e)
			}
		}
		ro.People = len(mine)
		for _, t := range types {
			c := roleCell{}
			for _, q := range reqs {
				if q["typeId"] == t["id"] && strings.EqualFold(strings.TrimSpace(q["role"]), role) {
					c.ReqID = q["id"]
					c.Required = q["required"] != "No"
					c.Signoff = q["signoff"] == "Yes"
					c.Refresh = strings.TrimSpace(q["refresh"])
				}
			}
			if c.Required {
				for _, e := range mine {
					if !s.trainNeeded(t, e) {
						continue
					}
					c.People++
					if r, ok := latest[e["id"]+"|"+t["id"]]; ok && trainState(r["expires"], today) != "Expired" {
						c.Current++
					}
				}
			}
			ro.Cells[t["id"]] = c
		}
		rows = append(rows, ro)
	}
	return map[string]any{"types": out, "roles": rows}
}

func (s *store) trainRoleRoutes(mux *http.ServeMux) {
	s.mu.Lock()
	s.seedOnce("trainreq-231", "trainreq", "training_req.json")
	s.mu.Unlock()
	mux.HandleFunc("GET /api/training/rolematrix", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		writeJSON(w, 200, s.roleMatrix())
	})
}

func (s *store) roleReport() *report {
	m := s.roleMatrix()
	types := m["types"].([]map[string]string)
	cols := []string{"Position / role", "People"}
	for _, t := range types {
		cols = append(cols, t["name"])
	}
	rows := [][]string{}
	for _, ro := range m["roles"].([]roleRowOut) {
		row := []string{ro.Role, strconv.Itoa(ro.People)}
		for _, t := range types {
			c := ro.Cells[t["id"]]
			v := ""
			if c.Required {
				v = "Required"
				if c.Signoff {
					v += " + sign-off"
				}
				if c.Refresh != "" {
					v += ", every " + c.Refresh + " mo"
				}
				if c.People > 0 {
					v += " (" + strconv.Itoa(c.Current) + "/" + strconv.Itoa(c.People) + " current)"
				}
			}
			row = append(row, v)
		}
		rows = append(rows, row)
	}
	return &report{Title: "Required Training by Position", Subtitle: time.Now().Format("January 2, 2006"), Sections: []reportSection{{Note: "Blank means not required for that position. All Staff requirements apply to everyone.", Cols: cols, Rows: rows}}}
}
