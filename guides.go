package main

// Station Guides: the written how-to for every station (overview, opening and
// closing steps, service notes, par sheets, recipes) plus the daily Opening
// and Closing checklists. Guides are kept as plain markdown text in
// Maintenance Data/reg-guides.json so they can be edited on screen.

import (
	"fmt"
	"html"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
)

var guideKinds = []string{"Overview", "Opening SOP", "Opening Checklist", "Service 1 - Lunch", "Service 2 - Dinner", "Service SOP", "Closing SOP", "Closing Checklist", "Par Sheet", "Recipes", "Production Guide", "Role Cards", "Emergency Plan", "Training Program", "Other"}

func docF(k, l string, req bool) fieldDef { return fieldDef{Key: k, Label: l, Type: "doc", Req: req} }

func init() {
	regDefs["stations"] = &collDef{Name: "stations", Title: "Stations", NewLabel: "Add Station", Prefix: "STA-", Digits: 3, Starter: "stations.json", Deletable: true,
		Intro:    "The stations in the kitchen. Renaming a station renames it on its guides and checklists too.",
		ListCols: []string{"id", "name", "active"},
		Fields:   []fieldDef{txt("name", "Station", true), yn("active", "Active?")},
		After: func(s *store, old, nw rec) {
			if old["name"] == nw["name"] || old["name"] == "" {
				return
			}
			for _, n := range []string{"guides", "stchecklists", "stchecklog"} {
				c := regDefs[n]
				rows, ch := s.regLoad(c), false
				for _, r := range rows {
					if r["station"] == old["name"] {
						r["station"], ch = nw["name"], true
					}
				}
				if ch {
					_ = s.regSave(c, rows)
				}
			}
		}}
	regDefs["guides"] = &collDef{Name: "guides", Title: "Station Guides", NewLabel: "Add Guide", Prefix: "GDE-", Digits: 3, Deletable: true, Starter: "guides.json",
		Intro:    "Written guides for each station. Use ## for headings, - for bullets, ** around bold words and | for tables.",
		ListCols: []string{"id", "station", "kind", "title", "updated"},
		Fields: []fieldDef{{Key: "station", Label: "Station", Type: "station", Req: true}, sel("kind", "Kind", guideKinds, true), txt("title", "Title", true),
			docF("body", "Guide text", true), txt("source", "Where it came from", false), dat("updated", "Last updated", false)}}
	regDefs["stchecklists"] = &collDef{Name: "stchecklists", Title: "Station Checklists", NewLabel: "Add Checklist", Prefix: "CHK-", Digits: 3, Deletable: true, Starter: "station_checklists.json",
		Intro:    "One checklist per station and kind. Each line is a tick box. A line starting with # is a section heading and a line starting with > is a note.",
		ListCols: []string{"id", "station", "kind", "title", "active"},
		Fields: []fieldDef{{Key: "station", Label: "Station", Type: "station", Req: true}, sel("kind", "Kind", []string{"Opening", "Closing"}, true), txt("title", "Title", true),
			docF("items", "Checklist lines", true), yn("active", "Active?")}}
	regDefs["stchecklog"] = &collDef{Name: "stchecklog", Title: "Station Checklist Log", NewLabel: "Add Entry", Prefix: "SCL-", Digits: 5, Desc: true, Report: "stchecklog",
		Intro:    "Each time a station checklist was finished, who did it and what was missed.",
		ListCols: []string{"id", "date", "station", "kind", "by", "done", "missed"},
		Fields: []fieldDef{dat("date", "Date", true), txt("station", "Station", true), sel("kind", "Kind", []string{"Opening", "Closing"}, true), txt("by", "Done by", true), txt("time", "Time", false),
			txt("done", "Done", false), lng("missed", "Not done", false), lng("notes", "Notes", false)}}
}

// ---- checklist lines ----

type chkLine struct {
	Kind string `json:"kind"` // head, note, item
	Text string `json:"text"`
	N    int    `json:"n"` // item number, 1-based, for items only
}

func parseChecklist(items string) []chkLine {
	out, n := []chkLine{}, 0
	for _, ln := range strings.Split(strings.ReplaceAll(items, "\r", ""), "\n") {
		t := strings.TrimSpace(ln)
		switch {
		case t == "":
		case strings.HasPrefix(t, "#"):
			out = append(out, chkLine{Kind: "head", Text: strings.TrimSpace(strings.TrimLeft(t, "#"))})
		case strings.HasPrefix(t, ">"):
			out = append(out, chkLine{Kind: "note", Text: strings.TrimSpace(strings.TrimLeft(t, ">"))})
		default:
			n++
			out = append(out, chkLine{Kind: "item", Text: t, N: n})
		}
	}
	return out
}

// ---- markdown (the small part of it the guides use) ----

var (
	reBold   = regexp.MustCompile(`\*\*(.+?)\*\*`)
	reItalic = regexp.MustCompile(`(^|[^*\w])\*([^*\s][^*]*?)\*`)
	reCode   = regexp.MustCompile("`([^`]+)`")
	reOL     = regexp.MustCompile(`^(\d+)[.)]\s+(.*)$`)
	reUL     = regexp.MustCompile(`^[-*+]\s+(.*)$`)
	reHead   = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
)

func mdInline(s string) string {
	s = html.EscapeString(s)
	s = reCode.ReplaceAllString(s, "<code>$1</code>")
	s = reBold.ReplaceAllString(s, "<b>$1</b>")
	s = reItalic.ReplaceAllString(s, "$1<i>$2</i>")
	return s
}

func mdCells(l string) []string {
	l = strings.TrimSpace(l)
	l = strings.TrimPrefix(strings.TrimSuffix(l, "|"), "|")
	out := []string{}
	for _, c := range strings.Split(l, "|") {
		out = append(out, strings.TrimSpace(c))
	}
	return out
}

func isSepRow(l string) bool {
	t := strings.TrimSpace(l)
	if !strings.HasPrefix(t, "|") || !strings.Contains(t, "-") {
		return false
	}
	return strings.Trim(t, "|-: ") == ""
}

type listLevel struct {
	indent int
	tag    string
}

// mdHTML turns guide markdown into safe HTML. Everything is escaped first.
func mdHTML(src string) string {
	lines := strings.Split(strings.ReplaceAll(src, "\r", ""), "\n")
	var b strings.Builder
	var stack []listLevel
	para := []string{}
	closeLists := func(to int) {
		for len(stack) > to {
			b.WriteString("</li></" + stack[len(stack)-1].tag + ">")
			stack = stack[:len(stack)-1]
		}
	}
	flush := func() {
		if len(para) > 0 {
			b.WriteString("<p>" + strings.Join(para, "<br>") + "</p>")
			para = para[:0]
		}
	}
	for i := 0; i < len(lines); i++ {
		raw := strings.TrimRight(lines[i], " \t")
		t := strings.TrimSpace(raw)
		indent := len(raw) - len(strings.TrimLeft(raw, " \t"))
		if t == "" {
			flush()
			closeLists(0)
			continue
		}
		if m := reHead.FindStringSubmatch(t); m != nil {
			flush()
			closeLists(0)
			lv := len(m[1]) + 1
			if lv > 4 {
				lv = 4
			}
			b.WriteString(fmt.Sprintf("<h%d>%s</h%d>", lv, mdInline(m[2]), lv))
			continue
		}
		if t == "---" || t == "***" {
			flush()
			closeLists(0)
			b.WriteString("<hr>")
			continue
		}
		if strings.HasPrefix(t, "|") && i+1 < len(lines) && isSepRow(lines[i+1]) {
			flush()
			closeLists(0)
			b.WriteString("<table><thead><tr>")
			for _, c := range mdCells(t) {
				b.WriteString("<th>" + mdInline(c) + "</th>")
			}
			b.WriteString("</tr></thead><tbody>")
			for i += 2; i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), "|"); i++ {
				b.WriteString("<tr>")
				for _, c := range mdCells(lines[i]) {
					b.WriteString("<td>" + mdInline(c) + "</td>")
				}
				b.WriteString("</tr>")
			}
			i--
			b.WriteString("</tbody></table>")
			continue
		}
		if strings.HasPrefix(t, ">") {
			flush()
			closeLists(0)
			b.WriteString("<blockquote>" + mdInline(strings.TrimSpace(strings.TrimPrefix(t, ">"))) + "</blockquote>")
			continue
		}
		tag, text := "", ""
		if m := reUL.FindStringSubmatch(t); m != nil {
			tag, text = "ul", m[1]
		} else if m := reOL.FindStringSubmatch(t); m != nil {
			tag, text = "ol", m[2]
		}
		if tag != "" {
			flush()
			switch {
			case len(stack) == 0 || indent > stack[len(stack)-1].indent:
				stack = append(stack, listLevel{indent, tag})
				b.WriteString("<" + tag + "><li>")
			default:
				for len(stack) > 1 && indent < stack[len(stack)-1].indent {
					b.WriteString("</li></" + stack[len(stack)-1].tag + ">")
					stack = stack[:len(stack)-1]
				}
				b.WriteString("</li><li>")
			}
			b.WriteString(mdInline(text))
			continue
		}
		if len(stack) > 0 && indent > 0 { // wrapped text inside a list item
			b.WriteString("<br>" + mdInline(t))
			continue
		}
		closeLists(0)
		para = append(para, mdInline(t))
	}
	flush()
	closeLists(0)
	return b.String()
}

// mdPlain strips the markdown marks for the plain-text email.
func mdPlain(src string) string {
	r := strings.NewReplacer("**", "", "`", "")
	var out []string
	for _, l := range strings.Split(strings.ReplaceAll(src, "\r", ""), "\n") {
		if isSepRow(l) {
			continue
		}
		if m := reHead.FindStringSubmatch(strings.TrimSpace(l)); m != nil {
			l = strings.ToUpper(m[2])
		}
		out = append(out, r.Replace(l))
	}
	return strings.Join(out, "\n")
}

// ---- reports ----

func (s *store) guideReport(id string) (*report, bool) {
	today := time.Now().Format("January 2, 2006")
	if id == "stchecklog" {
		return s.regReport(regDefs["stchecklog"], "Station Checklist Log", "", nil), true
	}
	if g, ok := strings.CutPrefix(id, "guide:"); ok {
		for _, r := range s.regLoad(regDefs["guides"]) {
			if r["id"] == g {
				return &report{Title: r["title"], Subtitle: r["station"] + " - " + r["kind"], Sections: []reportSection{{HTML: mdHTML(r["body"]), Text: mdPlain(r["body"])}}}, true
			}
		}
		return nil, false
	}
	if st, ok := strings.CutPrefix(id, "stationbook:"); ok {
		return s.stationBook(st), true
	}
	cid, _ := strings.CutPrefix(id, "checklist:")
	for _, r := range s.regLoad(regDefs["stchecklists"]) {
		if r["id"] != cid {
			continue
		}
		rep := &report{Title: r["title"], Subtitle: "Date: ____________   Done by: ____________________   " + today}
		sec := reportSection{Cols: []string{"", "Task"}}
		flush := func() {
			if len(sec.Rows) > 0 || sec.Heading != "" || sec.Note != "" {
				rep.Sections = append(rep.Sections, sec)
			}
		}
		for _, l := range parseChecklist(r["items"]) {
			switch l.Kind {
			case "head":
				flush()
				sec = reportSection{Heading: l.Text, Cols: []string{"", "Task"}}
			case "note":
				if sec.Note != "" {
					sec.Note += "\n"
				}
				sec.Note += l.Text
			default:
				sec.Rows = append(sec.Rows, []string{"☐", l.Text})
			}
		}
		flush()
		rep.Sections = append(rep.Sections, reportSection{Note: "Not done / notes:\n\n\n"})
		return rep, true
	}
	return nil, false
}

// ---- routes ----

func (s *store) guideRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/guides", func(w http.ResponseWriter, r *http.Request) {
		q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
		s.mu.Lock()
		rows := s.regLoad(regDefs["guides"])
		s.mu.Unlock()
		out := []map[string]string{}
		for _, g := range rows {
			snip := ""
			if q != "" {
				hay := strings.ToLower(g["title"] + "\n" + g["station"] + "\n" + g["kind"] + "\n" + g["body"])
				at := strings.Index(hay, q)
				if at < 0 {
					continue
				}
				if b := strings.ToLower(g["body"]); strings.Contains(b, q) {
					i := strings.Index(b, q)
					a, e := i-50, i+len(q)+80
					if a < 0 {
						a = 0
					}
					if e > len(g["body"]) {
						e = len(g["body"])
					}
					for a > 0 && g["body"][a]&0xC0 == 0x80 {
						a--
					}
					for e < len(g["body"]) && g["body"][e]&0xC0 == 0x80 {
						e++
					}
					snip = strings.Join(strings.Fields(g["body"][a:e]), " ")
				}
			}
			out = append(out, map[string]string{"id": g["id"], "station": g["station"], "kind": g["kind"], "title": g["title"], "updated": g["updated"], "snip": snip})
		}
		sort.SliceStable(out, func(i, j int) bool { return out[i]["id"] < out[j]["id"] })
		writeJSON(w, 200, map[string]any{"guides": out})
	})
	mux.HandleFunc("GET /api/guides/{id}", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		rows := s.regLoad(regDefs["guides"])
		s.mu.Unlock()
		for _, g := range rows {
			if g["id"] == r.PathValue("id") {
				writeJSON(w, 200, map[string]any{"guide": g, "html": mdHTML(g["body"])})
				return
			}
		}
		fail(w, 404, "That guide was not found.")
	})
	// Finish a checklist: one log line with what was ticked and what was not.
	mux.HandleFunc("POST /api/stchecklog/finish", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			ID    string `json:"id"`
			By    string `json:"by"`
			Notes string `json:"notes"`
			Done  []int  `json:"done"`
		}
		if jsonDecode(r, &in) != nil || strings.TrimSpace(in.By) == "" {
			fail(w, 400, "Type your first name so the checklist can be signed.")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		var chk rec
		for _, c := range s.regLoad(regDefs["stchecklists"]) {
			if c["id"] == in.ID {
				chk = c
			}
		}
		if chk == nil {
			fail(w, 404, "That checklist was not found.")
			return
		}
		ticked := map[int]bool{}
		for _, n := range in.Done {
			ticked[n] = true
		}
		total, done, missed := 0, 0, []string{}
		for _, l := range parseChecklist(chk["items"]) {
			if l.Kind != "item" {
				continue
			}
			total++
			if ticked[l.N] {
				done++
			} else {
				missed = append(missed, l.Text)
			}
		}
		now := time.Now()
		if _, err := s.regAdd(regDefs["stchecklog"], rec{"date": now.Format(dayFmt), "station": chk["station"], "kind": chk["kind"], "by": strings.TrimSpace(in.By), "time": now.Format("3:04 PM"),
			"done": fmt.Sprintf("%d of %d", done, total), "missed": strings.Join(missed, "\n"), "notes": in.Notes}); err != nil {
			fail(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"done": done, "total": total, "missed": len(missed)})
	})
	mux.HandleFunc("GET /api/stchecklists", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		lists, logs := s.regLoad(regDefs["stchecklists"]), s.regLoad(regDefs["stchecklog"])
		s.mu.Unlock()
		today := time.Now().Format(dayFmt)
		fin := map[string]rec{}
		for _, l := range logs {
			if l["date"] == today {
				fin[l["station"]+"|"+l["kind"]] = l
			}
		}
		out := []map[string]any{}
		for _, c := range lists {
			if c["active"] == "No" {
				continue
			}
			m := map[string]any{"id": c["id"], "station": c["station"], "kind": c["kind"], "title": c["title"], "lines": parseChecklist(c["items"])}
			if l, ok := fin[c["station"]+"|"+c["kind"]]; ok {
				m["finished"] = map[string]string{"by": l["by"], "time": l["time"], "done": l["done"]}
			}
			out = append(out, m)
		}
		writeJSON(w, 200, map[string]any{"lists": out})
	})
}

// stationBook is every SOP, guide and checklist for one station, in reading
// order, each starting on its own page.
func (s *store) stationBook(station string) *report {
	rep := &report{Title: station + " Station Book", Subtitle: "SOPs, guides and checklists - " + time.Now().Format("January 2, 2006")}
	order := func(k string) int {
		for i, v := range guideKinds {
			if v == k {
				return i
			}
		}
		return len(guideKinds)
	}
	gs := []rec{}
	for _, r := range s.regLoad(regDefs["guides"]) {
		if r["station"] == station {
			gs = append(gs, r)
		}
	}
	sort.SliceStable(gs, func(i, j int) bool { return order(gs[i]["kind"]) < order(gs[j]["kind"]) })
	for _, g := range gs {
		rep.Sections = append(rep.Sections, reportSection{Heading: g["title"], Group: true}, reportSection{HTML: mdHTML(g["body"]), Text: mdPlain(g["body"])})
	}
	for _, c := range s.regSorted(regDefs["stchecklists"]) {
		if c["station"] != station {
			continue
		}
		if cr, ok := s.guideReport("checklist:" + c["id"]); ok {
			rep.Sections = append(rep.Sections, reportSection{Heading: "Checklist: " + c["title"], Group: true})
			rep.Sections = append(rep.Sections, cr.Sections...)
		}
	}
	if len(rep.Sections) == 0 {
		rep.Sections = append(rep.Sections, reportSection{Note: "Nothing has been written for this station yet."})
	}
	return rep
}
