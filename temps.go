package main

// Temperature log. Readings are entered here (one round at a time) or brought
// in from a Google Form's response sheet, either from a CSV file or from the
// sheet's "publish to web" link. Each temperature point (a cooler, a freezer, a
// hot-hold well...) has a safe range. A reading outside it needs a corrective
// action and stays on the to-do list until one is written. Kept in
// Maintenance Data\temps.json and temp-points.json.

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Defaults from the FDA Food Code. Each point can use its own limits.
var tempKinds = map[string][2]*float64{
	"Cold Holding": {nil, fp(41)},
	"Freezer":      {nil, fp(0)},
	"Hot Holding":  {fp(135), nil},
	"Cooking":      {fp(165), nil},
	"Other":        {nil, nil},
}
var tempKindOrder = []string{"Cold Holding", "Freezer", "Hot Holding", "Cooking", "Other"}

func fp(f float64) *float64 { return &f }

type TempPoint struct {
	ID   string   `json:"id"`
	Name string   `json:"name"`
	Kind string   `json:"kind"`
	Area string   `json:"area,omitempty"`
	Min  *float64 `json:"min"` // lowest safe, nil for none
	Max  *float64 `json:"max"` // highest safe, nil for none
}

type TempReading struct {
	ID      string  `json:"id"`
	At      string  `json:"at"` // 2006-01-02 15:04
	PointID string  `json:"pointId"`
	Point   string  `json:"point"`
	Temp    float64 `json:"temp"`
	By      string  `json:"by"`
	Notes   string  `json:"notes"`
	Action  string  `json:"action"` // corrective action, needed when out of range
	Source  string  `json:"source"` // entered, import
	Key     string  `json:"key,omitempty"`
}

type tempView struct {
	TempReading
	Limit  string `json:"limit"`
	Status string `json:"status"` // OK, Too Warm, Too Cold, Not Checked
}

func (p *TempPoint) limits() (min, max *float64) {
	if p.Min != nil || p.Max != nil {
		return p.Min, p.Max
	}
	d := tempKinds[p.Kind]
	return d[0], d[1]
}

func (p *TempPoint) check(t float64) string {
	min, max := p.limits()
	switch {
	case max != nil && t > *max:
		return "Too Warm"
	case min != nil && t < *min:
		return "Too Cold"
	case min == nil && max == nil:
		return "Not Checked"
	}
	return "OK"
}

func (p *TempPoint) limitText() string {
	min, max := p.limits()
	f := func(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) + "°F" }
	switch {
	case min != nil && max != nil:
		return f(*min) + " to " + f(*max)
	case max != nil:
		return "at or below " + f(*max)
	case min != nil:
		return "at or above " + f(*min)
	}
	return ""
}

func (s *store) tempPoints() []*TempPoint {
	out := []*TempPoint{}
	s.readMaint("temp-points.json", &out)
	return out
}

func (s *store) tempViews(from, to string) []tempView {
	var all []*TempReading
	s.readMaint("temps.json", &all)
	pts := map[string]*TempPoint{}
	byName := map[string]*TempPoint{}
	for _, p := range s.tempPoints() {
		pts[p.ID] = p
		byName[strings.ToLower(p.Name)] = p
	}
	out := []tempView{}
	for _, r := range all {
		day := r.At
		if len(day) >= 10 {
			day = day[:10]
		}
		if (from != "" && day < from) || (to != "" && day > to) {
			continue
		}
		v := tempView{TempReading: *r, Status: "Not Checked"}
		p := pts[r.PointID]
		if p == nil {
			p = byName[strings.ToLower(r.Point)]
		}
		if p != nil {
			v.Status, v.Limit = p.check(r.Temp), p.limitText()
		}
		out = append(out, v)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At > out[j].At })
	return out
}

// tempsNeedingAction lists out-of-range readings from the last 14 days with no corrective action.
func (s *store) tempsNeedingAction() []tempView {
	from := time.Now().AddDate(0, 0, -14).Format(dayFmt)
	out := []tempView{}
	for _, v := range s.tempViews(from, "") {
		if (v.Status == "Too Warm" || v.Status == "Too Cold") && strings.TrimSpace(v.Action) == "" {
			out = append(out, v)
		}
	}
	return out
}

// ---- reading CSV exported from a Google Form's response sheet ----

type tempMap struct {
	Timestamp string   `json:"timestamp"`
	Time      string   `json:"time"` // optional separate time column
	Point     string   `json:"point"`
	Temp      []string `json:"temp"` // one column, or several when each unit has its own column
	By        string   `json:"by"`
	Notes     string   `json:"notes"`
	Action    string   `json:"action"`
}

func readCSV(text string) ([][]string, error) {
	text = strings.TrimPrefix(text, "\ufeff")
	r := csv.NewReader(strings.NewReader(text))
	r.LazyQuotes, r.FieldsPerRecord = true, -1
	return r.ReadAll()
}

func suggestTempMap(h []string) tempMap {
	isTemp := func(c string) bool {
		lc := strings.ToLower(c)
		return strings.Contains(lc, "temp") && !strings.Contains(lc, "corrective")
	}
	find := func(words ...string) string {
		for _, w := range words {
			for _, c := range h {
				if !isTemp(c) && strings.Contains(strings.ToLower(c), w) {
					return c
				}
			}
		}
		return ""
	}
	m := tempMap{Timestamp: find("timestamp", "completion"), By: find("initials", "your name", "employee", "checked by", "completed by", "name"), Notes: find("note", "comment", "remark"), Action: find("corrective", "action")}
	if m.Timestamp == "" {
		m.Timestamp = find("date", "time", "when")
	}
	m.Point = find("which", "unit", "equipment", "location", "station", "item", "cooler", "freezer", "area")
	// Microsoft Forms style: every column after the standard ones is a unit.
	meta := map[string]bool{"id": true, "start time": true, "completion time": true, "email": true, "name": true, "timestamp": true}
	for _, c := range []string{m.Timestamp, m.By, m.Notes, m.Action} {
		meta[strings.ToLower(strings.TrimSpace(c))] = true
	}
	var rest []string
	for _, c := range h {
		if k := strings.ToLower(strings.TrimSpace(c)); k != "" && !meta[k] {
			rest = append(rest, c)
		}
	}
	if len(rest) >= 3 {
		m.Temp, m.Point = rest, ""
	} else {
		for _, c := range h {
			if isTemp(c) {
				m.Temp = append(m.Temp, c)
			}
		}
	}
	if len(m.Temp) > 1 {
		m.Point = "" // each unit has its own column
	}
	return m
}

var numRe = regexp.MustCompile(`-?\d+(\.\d+)?`)

func parseTemp(v string) (float64, bool) {
	m := numRe.FindString(v)
	if m == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(m, 64)
	if err != nil {
		return 0, false
	}
	lv := strings.ToUpper(v)
	if strings.Contains(lv, "C") && !strings.Contains(lv, "F") {
		f = f*9/5 + 32
	}
	return f, true
}

var tsLayouts = []string{"1/2/2006 15:04:05", "1/2/2006 3:04:05 PM", "1/2/2006 15:04", "1/2/2006 3:04 PM", "2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02T15:04:05Z07:00", "2006-01-02T15:04:05", "1/2/2006", "2006-01-02"}

func parseWhen(date, clock string) (string, bool) {
	date, clock = strings.TrimSpace(date), strings.TrimSpace(clock)
	if f, err := strconv.ParseFloat(date, 64); err == nil && f > 20000 && f < 80000 && clock == "" {
		d := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC).Add(time.Duration(f*86400+0.5) * time.Second)
		return time.Date(d.Year(), d.Month(), d.Day(), d.Hour(), d.Minute(), 0, 0, time.Local).Format("2006-01-02 15:04"), true
	}
	if clock != "" {
		for _, l := range tsLayouts {
			if d, err := time.ParseInLocation(l, date, time.Local); err == nil {
				for _, tl := range []string{"15:04:05", "3:04:05 PM", "15:04", "3:04 PM"} {
					if c, err := time.Parse(tl, clock); err == nil {
						return time.Date(d.Year(), d.Month(), d.Day(), c.Hour(), c.Minute(), 0, 0, time.Local).Format("2006-01-02 15:04"), true
					}
				}
				return d.Format("2006-01-02 15:04"), true
			}
		}
		return "", false
	}
	for _, l := range tsLayouts {
		if d, err := time.ParseInLocation(l, date, time.Local); err == nil {
			return d.Format("2006-01-02 15:04"), true
		}
	}
	return "", false
}

func guessKind(name string) string {
	n := strings.ToLower(name)
	switch {
	case strings.Contains(n, "freez"):
		return "Freezer"
	case strings.Contains(n, "cooler"), strings.Contains(n, "walk-in"), strings.Contains(n, "walk in"), strings.Contains(n, "reach"), strings.Contains(n, "fridge"), strings.Contains(n, "refrig"), strings.Contains(n, "cold"), strings.Contains(n, "prep table"), strings.Contains(n, "salad"):
		return "Cold Holding"
	case strings.Contains(n, "hot"), strings.Contains(n, "steam"), strings.Contains(n, "warm"), strings.Contains(n, "holding"):
		return "Hot Holding"
	}
	return "Other"
}

func tempKey(at, point string, t float64, by string) string {
	h := fnv.New64a()
	fmt.Fprintf(h, "%s|%s|%.2f|%s", at, strings.ToLower(point), t, strings.ToLower(by))
	return strconv.FormatUint(h.Sum64(), 36)
}

type importResult struct {
	Added, Skipped, Unreadable, NewPoints, OutOfRange int
}

// importTemps adds readings from CSV rows. The caller holds s.mu.
func (s *store) importTemps(rows [][]string, m tempMap) (importResult, error) {
	var res importResult
	if len(rows) < 2 {
		return res, fmt.Errorf("The file has no responses in it.")
	}
	idx := map[string]int{}
	for i, c := range rows[0] {
		idx[strings.TrimSpace(c)] = i
	}
	col := func(name string) int {
		if name == "" {
			return -1
		}
		if i, ok := idx[strings.TrimSpace(name)]; ok {
			return i
		}
		return -2
	}
	get := func(row []string, i int) string {
		if i >= 0 && i < len(row) {
			return strings.TrimSpace(row[i])
		}
		return ""
	}
	ts, tm, pc, bc, nc, ac := col(m.Timestamp), col(m.Time), col(m.Point), col(m.By), col(m.Notes), col(m.Action)
	for _, c := range []int{ts, tm, pc, bc, nc, ac} {
		if c == -2 {
			return res, fmt.Errorf("A column in the setup was not found in this file. The column names may have changed.")
		}
	}
	if ts < 0 || len(m.Temp) == 0 {
		return res, fmt.Errorf("Choose the timestamp column and at least one temperature column.")
	}
	var tcols []int
	for _, t := range m.Temp {
		if i := col(t); i >= 0 {
			tcols = append(tcols, i)
		} else {
			return res, fmt.Errorf("The temperature column “%s” was not found in this file.", t)
		}
	}
	if len(tcols) == 1 && pc < 0 {
		return res, fmt.Errorf("Choose the column that says which unit was checked.")
	}
	var existing []*TempReading
	s.readMaint("temps.json", &existing)
	known := map[string]bool{}
	for _, r := range existing {
		if r.Key != "" {
			known[r.Key] = true
		}
	}
	clean := func(h string) string {
		n := regexp.MustCompile(`(?i)\b(temp(erature)?s?|reading|°?f)\b`).ReplaceAllString(h, "")
		return strings.TrimSpace(strings.Trim(strings.TrimSpace(n), "-:()?"))
	}
	pts := s.tempPoints()
	pointFor := func(name string) *TempPoint {
		for _, p := range pts {
			if strings.EqualFold(p.Name, name) || strings.EqualFold(clean(p.Name), clean(name)) {
				return p
			}
		}
		p := &TempPoint{ID: newID(), Name: name, Kind: guessKind(name)}
		pts = append(pts, p)
		res.NewPoints++
		return p
	}
	for _, row := range rows[1:] {
		at, ok := parseWhen(get(row, ts), get(row, tm))
		if !ok {
			res.Unreadable++
			continue
		}
		for _, tc := range tcols {
			raw := get(row, tc)
			if raw == "" {
				continue
			}
			t, ok := parseTemp(raw)
			if !ok {
				if !strings.ContainsAny(raw, "0123456789") {
					continue // "Not in use", "N/A" and the like
				}
				res.Unreadable++
				continue
			}
			name := get(row, pc)
			if len(tcols) > 1 || pc < 0 {
				name = strings.TrimSpace(rows[0][tc])
			}
			if name == "" {
				res.Unreadable++
				continue
			}
			p := pointFor(name)
			key := tempKey(at, p.Name, t, get(row, bc))
			if known[key] {
				res.Skipped++
				continue
			}
			known[key] = true
			r := &TempReading{ID: newID(), At: at, PointID: p.ID, Point: p.Name, Temp: t, By: get(row, bc), Notes: get(row, nc), Action: get(row, ac), Source: "import", Key: key}
			if st := p.check(t); st == "Too Warm" || st == "Too Cold" {
				res.OutOfRange++
			}
			existing = append(existing, r)
			res.Added++
		}
	}
	if res.Added > 0 || res.NewPoints > 0 {
		if err := s.writeMaint("temp-points.json", pts); err != nil {
			return res, err
		}
		if err := s.writeMaint("temps.json", existing); err != nil {
			return res, err
		}
	}
	return res, nil
}

func (s *store) fetchSheet(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || (u.Host != "docs.google.com" && !strings.HasSuffix(u.Host, ".googleusercontent.com")) {
		return "", fmt.Errorf("Use the link from Google Sheets: File, Share, Publish to web, choose the responses sheet and CSV.")
	}
	c := http.Client{Timeout: 25 * time.Second}
	resp, err := c.Get(u.String())
	if err != nil {
		return "", fmt.Errorf("The sheet could not be reached. Check the internet connection.")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Google did not give the sheet (error %d). Check that it is published to the web as CSV.", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(strings.TrimSpace(string(b[:min(len(b), 200)])), "<") {
		return "", fmt.Errorf("That link gave a web page, not CSV data. Publish the sheet as CSV.")
	}
	return string(b), nil
}

func (s *store) tempRoutes(mux *http.ServeMux) {
	settings := func() (st maintSettings) { _, _, st = s.loadMaint(); return }
	payload := func(from, to string) map[string]any {
		st := settings()
		kinds := []map[string]any{}
		for _, k := range tempKindOrder {
			d := tempKinds[k]
			kinds = append(kinds, map[string]any{"kind": k, "min": d[0], "max": d[1]})
		}
		return map[string]any{"points": s.tempPoints(), "readings": s.tempViews(from, to), "kinds": kinds,
			"url": st.TempsURL, "map": st.TempMap, "syncedAt": st.TempsSyncedAt, "needAction": len(s.tempsNeedingAction())}
	}
	mux.HandleFunc("GET /api/temps", func(w http.ResponseWriter, r *http.Request) {
		from := r.URL.Query().Get("from")
		if from == "" {
			from = time.Now().AddDate(0, 0, -30).Format(dayFmt)
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		writeJSON(w, 200, payload(from, r.URL.Query().Get("to")))
	})
	mux.HandleFunc("POST /api/temps/points", func(w http.ResponseWriter, r *http.Request) {
		var in2 TempPoint
		if json.NewDecoder(r.Body).Decode(&in2) != nil || strings.TrimSpace(in2.Name) == "" {
			fail(w, 400, "Enter a name for the temperature point.")
			return
		}
		if _, ok := tempKinds[in2.Kind]; !ok {
			in2.Kind = "Other"
		}
		in2.Name = strings.TrimSpace(in2.Name)
		s.mu.Lock()
		defer s.mu.Unlock()
		pts := s.tempPoints()
		found := false
		for i, p := range pts {
			if p.ID == in2.ID && in2.ID != "" {
				pts[i], found = &in2, true
			} else if strings.EqualFold(p.Name, in2.Name) {
				fail(w, 400, "There is already a point named “"+p.Name+"”.")
				return
			}
		}
		if !found {
			in2.ID = newID()
			pts = append(pts, &in2)
		}
		if err := s.writeMaint("temp-points.json", pts); err != nil {
			s.maintFail(w, err)
			return
		}
		writeJSON(w, 200, payload("", ""))
	})
	mux.HandleFunc("DELETE /api/temps/points/{id}", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		out := []*TempPoint{}
		for _, p := range s.tempPoints() {
			if p.ID != r.PathValue("id") {
				out = append(out, p)
			}
		}
		if err := s.writeMaint("temp-points.json", out); err != nil {
			s.maintFail(w, err)
			return
		}
		writeJSON(w, 200, payload("", ""))
	})
	// A round of readings.
	mux.HandleFunc("POST /api/temps", func(w http.ResponseWriter, r *http.Request) {
		var in2 struct {
			By       string `json:"by"`
			Readings []struct {
				PointID string  `json:"pointId"`
				Temp    float64 `json:"temp"`
				Notes   string  `json:"notes"`
				Action  string  `json:"action"`
			} `json:"readings"`
		}
		if json.NewDecoder(r.Body).Decode(&in2) != nil || strings.TrimSpace(in2.By) == "" {
			fail(w, 400, "Enter your name or initials.")
			return
		}
		if len(in2.Readings) == 0 {
			fail(w, 400, "Enter at least one temperature.")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		pts := map[string]*TempPoint{}
		for _, p := range s.tempPoints() {
			pts[p.ID] = p
		}
		var all []*TempReading
		s.readMaint("temps.json", &all)
		now := time.Now().Format("2006-01-02 15:04")
		for _, x := range in2.Readings {
			p := pts[x.PointID]
			if p == nil {
				fail(w, 400, "A temperature point was not found.")
				return
			}
			if x.Temp < -80 || x.Temp > 500 {
				fail(w, 400, "“"+p.Name+"”: that temperature does not look right.")
				return
			}
			if st := p.check(x.Temp); (st == "Too Warm" || st == "Too Cold") && strings.TrimSpace(x.Action) == "" {
				fail(w, 400, "“"+p.Name+"” is out of range ("+strconv.FormatFloat(x.Temp, 'f', -1, 64)+"°F, safe: "+p.limitText()+"). Write the corrective action.")
				return
			}
			all = append(all, &TempReading{ID: newID(), At: now, PointID: p.ID, Point: p.Name, Temp: x.Temp, By: strings.TrimSpace(in2.By), Notes: strings.TrimSpace(x.Notes), Action: strings.TrimSpace(x.Action), Source: "entered"})
		}
		if err := s.writeMaint("temps.json", all); err != nil {
			s.maintFail(w, err)
			return
		}
		writeJSON(w, 200, payload(time.Now().AddDate(0, 0, -30).Format(dayFmt), ""))
	})
	mux.HandleFunc("PUT /api/temps/{id}/action", func(w http.ResponseWriter, r *http.Request) {
		var in2 struct{ Action, By string }
		if json.NewDecoder(r.Body).Decode(&in2) != nil || strings.TrimSpace(in2.Action) == "" || strings.TrimSpace(in2.By) == "" {
			fail(w, 400, "Write the corrective action and your name.")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		var all []*TempReading
		s.readMaint("temps.json", &all)
		for _, x := range all {
			if x.ID == r.PathValue("id") {
				x.Action = strings.TrimSpace(in2.Action) + " (" + strings.TrimSpace(in2.By) + ", " + time.Now().Format("1/2/2006") + ")"
				if err := s.writeMaint("temps.json", all); err != nil {
					s.maintFail(w, err)
					return
				}
				writeJSON(w, 200, payload(time.Now().AddDate(0, 0, -30).Format(dayFmt), ""))
				return
			}
		}
		fail(w, 404, "That reading was not found.")
	})
	mux.HandleFunc("POST /api/temps/xlsx", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(io.LimitReader(r.Body, 40<<20))
		text, err := xlsxToCSV(b)
		if err != nil {
			fail(w, 400, err.Error())
			return
		}
		writeJSON(w, 200, map[string]string{"csv": text})
	})
	// Adds Muriale's own temperature points and their limits (existing names are kept).
	mux.HandleFunc("POST /api/temps/points/starter", func(w http.ResponseWriter, r *http.Request) {
		b, _ := starterFiles.ReadFile("starter/temp_points.json")
		var seed []*TempPoint
		if json.Unmarshal(b, &seed) != nil {
			fail(w, 500, "The starter list could not be read.")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		pts := s.tempPoints()
		have := map[string]bool{}
		for _, p := range pts {
			have[strings.ToLower(p.Name)] = true
		}
		for _, p := range seed {
			if !have[strings.ToLower(p.Name)] {
				p.ID = newID()
				pts = append(pts, p)
			}
		}
		if err := s.writeMaint("temp-points.json", pts); err != nil {
			fail(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, payload(time.Now().AddDate(0, 0, -30).Format(dayFmt), ""))
	})
	mux.HandleFunc("POST /api/temps/import/preview", func(w http.ResponseWriter, r *http.Request) {
		var in2 struct{ CSV string }
		if json.NewDecoder(io.LimitReader(r.Body, 25<<20)).Decode(&in2) != nil {
			fail(w, 400, "Could not read that file.")
			return
		}
		rows, err := readCSV(in2.CSV)
		if err != nil || len(rows) < 1 {
			fail(w, 400, "That does not look like a CSV file.")
			return
		}
		sample := rows[1:]
		if len(sample) > 3 {
			sample = sample[:3]
		}
		writeJSON(w, 200, map[string]any{"headers": rows[0], "sample": sample, "count": len(rows) - 1, "map": suggestTempMap(rows[0])})
	})
	mux.HandleFunc("POST /api/temps/import", func(w http.ResponseWriter, r *http.Request) {
		var in2 struct {
			CSV  string  `json:"csv"`
			Map  tempMap `json:"map"`
			Save bool    `json:"save"`
		}
		if json.NewDecoder(io.LimitReader(r.Body, 25<<20)).Decode(&in2) != nil {
			fail(w, 400, "Could not read that file.")
			return
		}
		rows, err := readCSV(in2.CSV)
		if err != nil {
			fail(w, 400, "That does not look like a CSV file.")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		res, err := s.importTemps(rows, in2.Map)
		if err != nil {
			fail(w, 400, err.Error())
			return
		}
		if in2.Save {
			_, _, st := s.loadMaint()
			st.TempMap = &in2.Map
			_ = s.writeMaint("settings.json", st)
		}
		writeJSON(w, 200, map[string]any{"result": res, "data": payload(time.Now().AddDate(0, 0, -30).Format(dayFmt), "")})
	})
	mux.HandleFunc("POST /api/temps/settings", func(w http.ResponseWriter, r *http.Request) {
		var in2 struct {
			URL string   `json:"url"`
			Map *tempMap `json:"map"`
		}
		if json.NewDecoder(r.Body).Decode(&in2) != nil {
			fail(w, 400, "Could not read that.")
			return
		}
		u := strings.TrimSpace(in2.URL)
		if u != "" {
			if p, err := url.Parse(u); err != nil || p.Scheme != "https" || (p.Host != "docs.google.com" && !strings.HasSuffix(p.Host, ".googleusercontent.com")) {
				fail(w, 400, "Use the link from Google Sheets: File, Share, Publish to web, choose the responses sheet and CSV.")
				return
			}
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		_, _, st := s.loadMaint()
		st.TempsURL = u
		if in2.Map != nil {
			st.TempMap = in2.Map
		}
		if err := s.writeMaint("settings.json", st); err != nil {
			s.maintFail(w, err)
			return
		}
		writeJSON(w, 200, payload(time.Now().AddDate(0, 0, -30).Format(dayFmt), ""))
	})
	// Reads the published sheet's columns so the mapping can be checked.
	mux.HandleFunc("POST /api/temps/sheet-preview", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		_, _, st := s.loadMaint()
		s.mu.Unlock()
		if st.TempsURL == "" {
			fail(w, 400, "Add the published sheet link first.")
			return
		}
		text, err := s.fetchSheet(st.TempsURL)
		if err != nil {
			fail(w, 502, err.Error())
			return
		}
		rows, err := readCSV(text)
		if err != nil || len(rows) < 1 {
			fail(w, 502, "The sheet could not be read as CSV.")
			return
		}
		sample := rows[1:]
		if len(sample) > 3 {
			sample = sample[:3]
		}
		m := suggestTempMap(rows[0])
		if st.TempMap != nil {
			m = *st.TempMap
		}
		writeJSON(w, 200, map[string]any{"headers": rows[0], "sample": sample, "count": len(rows) - 1, "map": m})
	})
	// Fetches the published sheet and adds any new responses.
	mux.HandleFunc("POST /api/temps/sync", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		_, _, st := s.loadMaint()
		s.mu.Unlock()
		if st.TempsURL == "" {
			fail(w, 400, "Add the published sheet link first.")
			return
		}
		text, err := s.fetchSheet(st.TempsURL)
		if err != nil {
			fail(w, 502, err.Error())
			return
		}
		rows, err := readCSV(text)
		if err != nil || len(rows) < 1 {
			fail(w, 502, "The sheet could not be read as CSV.")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		m := suggestTempMap(rows[0])
		if st.TempMap != nil {
			m = *st.TempMap
		}
		res, err := s.importTemps(rows, m)
		if err != nil {
			fail(w, 400, err.Error()+" Open Temp Setup and check the columns.")
			return
		}
		_, _, st = s.loadMaint()
		st.TempsSyncedAt = time.Now().Format("2006-01-02 15:04")
		_ = s.writeMaint("settings.json", st)
		writeJSON(w, 200, map[string]any{"result": res, "data": payload(time.Now().AddDate(0, 0, -30).Format(dayFmt), "")})
	})
}
