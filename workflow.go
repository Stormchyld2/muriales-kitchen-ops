package main

// Daily workflow: the manager's day as timed blocks of checks, ticked off and
// saved per day, plus a handoff log of open issues with an owner and due date.
// Weekly and monthly overlay checks are saved once per week or month.

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

func init() {
	regDefs["dwitems"] = &collDef{Name: "dwitems", Title: "Daily Workflow Items", NewLabel: "Add Workflow Item", Prefix: "DW-", Digits: 3, Deletable: true, Starter: "dw_items.json", Report: "dw-blank",
		Intro:    "The checks on the daily workflow, in order. Block is the heading they sit under (items with the same block text are grouped, in the order of their IDs). Start and End are 24-hour times like 07:20 and let the Today screen highlight the block you are in. Frequency Weekly and Monthly checks are ticked once per week or month.",
		ListCols: []string{"id", "block", "text", "frequency", "active"},
		Fields:   []fieldDef{txt("block", "Block (time and heading)", true), txt("start", "Start (24-hour, optional)", false), txt("end", "End (24-hour, optional)", false), lng("text", "Check", true), sel("frequency", "How often", []string{"Daily", "Weekly", "Monthly"}, true), yn("active", "Active?")}}
	regDefs["dwlog"] = &collDef{Name: "dwlog", Title: "Daily Workflow Log", NewLabel: "Add Entry", Prefix: "DWL-", Digits: 6, Desc: true, Deletable: true, Report: "dw-today",
		Intro:    "Every check that was ticked, with the period it counts for, who ticked it and when.",
		ListCols: []string{"id", "period", "itemId", "by", "time"},
		Fields:   []fieldDef{txt("period", "Day, week or month", true), txt("itemId", "Item", true), txt("by", "Ticked by", false), txt("time", "Time", false)}}
	regDefs["dwhandoff"] = &collDef{Name: "dwhandoff", Title: "Handoff Log", NewLabel: "Add Handoff Item", Prefix: "HO-", Digits: 4, Desc: true, Deletable: true, Report: "dw-handoff",
		Intro:    "Open issues to hand to the next manager: what it is, who owns it, the next action and when it is due. Overdue ones show on the dashboard until they are marked Done.",
		ListCols: []string{"id", "issue", "priority", "owner", "nextAction", "due", "status"},
		Fields:   []fieldDef{lng("issue", "Issue", true), sel("priority", "Priority", []string{"High", "Medium", "Low"}, true), txt("owner", "Assigned to", true), lng("nextAction", "Next action", false), dat("due", "Due", false), sel("status", "Status", []string{"Open", "In Progress", "Done"}, true), dat("date", "Added on", false), lng("notes", "Notes", false)}}
}

// periodKey is the day, week (its Monday) or month an item is ticked for.
func periodKey(freq string, day time.Time) string {
	switch freq {
	case "Weekly":
		return "W" + mondayOf(day).Format(dayFmt)
	case "Monthly":
		return "M" + day.Format("2006-01")
	}
	return day.Format(dayFmt)
}

type dwItem struct {
	ID    string `json:"id"`
	Text  string `json:"text"`
	Freq  string `json:"freq"`
	Done  bool   `json:"done"`
	By    string `json:"by,omitempty"`
	Time  string `json:"time,omitempty"`
	LogID string `json:"logId,omitempty"`
}

type dwBlock struct {
	Block   string   `json:"block"`
	Start   string   `json:"start"`
	End     string   `json:"end"`
	Overlay bool     `json:"overlay"`
	Items   []dwItem `json:"items"`
}

// dwDay lays out one day: the daily blocks and the weekly/monthly overlays.
func (s *store) dwDay(day time.Time) (blocks []dwBlock, done, total int) {
	logged := map[string]rec{}
	for _, l := range s.regLoad(regDefs["dwlog"]) {
		logged[l["period"]+"|"+l["itemId"]] = l
	}
	idx := map[string]int{}
	for _, it := range s.regSorted(regDefs["dwitems"]) {
		if it["active"] == "No" {
			continue
		}
		freq := firstNonEmpty(it["frequency"], "Daily")
		key := it["block"]
		bi, ok := idx[key]
		if !ok {
			bi = len(blocks)
			idx[key] = bi
			blocks = append(blocks, dwBlock{Block: key, Start: it["start"], End: it["end"]})
		}
		if freq != "Daily" {
			blocks[bi].Overlay = true
		}
		l, isDone := logged[periodKey(freq, day)+"|"+it["id"]]
		blocks[bi].Items = append(blocks[bi].Items, dwItem{ID: it["id"], Text: it["text"], Freq: freq, Done: isDone, By: l["by"], Time: l["time"], LogID: l["id"]})
		total++
		if isDone {
			done++
		}
	}
	return
}

func (s *store) dwProgress() (done, total int) {
	_, d, t := s.dwDay(time.Now())
	return d, t
}

func (s *store) dwAttention(today string) []todoItem {
	out := []todoItem{}
	for _, r := range s.regLoad(regDefs["dwhandoff"]) {
		if r["status"] == "Done" || r["due"] == "" || r["due"] > today {
			continue
		}
		st := "Due Today"
		if r["due"] < today {
			st = "Overdue"
		}
		out = append(out, todoItem{Group: "Handoff", Title: "Handoff: " + clipText(r["issue"], 70), Detail: strings.TrimSpace(r["owner"] + " · " + r["nextAction"]), Due: r["due"], State: st, Kind: "handoff", ID: r["id"]})
	}
	return out
}

func clipText(v string, n int) string {
	v = strings.Join(strings.Fields(v), " ")
	if len(v) > n {
		return v[:n-1] + "…"
	}
	return v
}

func (s *store) workflowReport(id string) (*report, bool) {
	day := time.Now()
	switch id {
	case "dw-today", "dw-blank":
		blocks, done, total := s.dwDay(day)
		title, sub := "Daily Workflow", day.Format("Monday, January 2, 2006")
		note := fmt.Sprintf("%d of %d checks done.", done, total)
		if id == "dw-blank" {
			title, note = "Daily Workflow (blank)", "Tick each box as it is done. Overlay checks are done on their assigned day."
		}
		rep := &report{Title: title, Subtitle: sub}
		for i, b := range blocks {
			rows := [][]string{}
			for _, it := range b.Items {
				mark := "☐"
				if id == "dw-today" && it.Done {
					mark = "☑"
				}
				who := ""
				if id == "dw-today" && it.Done {
					who = strings.TrimSpace(it.By + " " + it.Time)
				}
				rows = append(rows, []string{mark, it.Text, who})
			}
			sec := reportSection{Heading: b.Block, Cols: []string{"", "Check", "Done by"}, Rows: rows}
			if i == 0 {
				sec.Note = note
			}
			rep.Sections = append(rep.Sections, sec)
		}
		return rep, true
	case "dw-handoff":
		rows := [][]string{}
		for _, r := range s.regSorted(regDefs["dwhandoff"]) {
			if r["status"] != "Done" {
				rows = append(rows, []string{r["issue"], r["priority"], r["owner"], r["nextAction"], r["due"], r["status"]})
			}
		}
		return &report{Title: "Quick Handoff Log", Subtitle: day.Format("Monday, January 2, 2006"), Sections: []reportSection{{Note: "Open items only.", Cols: []string{"Issue", "Priority", "Assigned to", "Next action", "Due", "Status"}, Rows: rows}}}, true
	}
	return nil, false
}

func (s *store) workflowRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/dw/today", func(w http.ResponseWriter, r *http.Request) {
		day := time.Now()
		if d, err := time.ParseInLocation(dayFmt, r.URL.Query().Get("date"), time.Local); err == nil {
			day = d
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		blocks, done, total := s.dwDay(day)
		open := 0
		for _, h := range s.regLoad(regDefs["dwhandoff"]) {
			if h["status"] != "Done" {
				open++
			}
		}
		writeJSON(w, 200, map[string]any{"date": day.Format(dayFmt), "blocks": blocks, "done": done, "total": total, "openHandoff": open})
	})
	mux.HandleFunc("POST /api/dw/tick", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Date, ItemID, By string
			On               bool
		}
		if jsonDecode(r, &in) != nil || in.ItemID == "" {
			fail(w, 400, "Could not read that.")
			return
		}
		day, err := time.ParseInLocation(dayFmt, in.Date, time.Local)
		if err != nil {
			day = time.Now()
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.isReadOnly() {
			fail(w, 423, errReadOnly.Error())
			return
		}
		var item rec
		for _, it := range s.regLoad(regDefs["dwitems"]) {
			if it["id"] == in.ItemID {
				item = it
			}
		}
		if item == nil {
			fail(w, 404, "That check no longer exists.")
			return
		}
		period := periodKey(firstNonEmpty(item["frequency"], "Daily"), day)
		c := regDefs["dwlog"]
		rows := s.regLoad(c)
		keep, found := rows[:0:0], false
		for _, l := range rows {
			if l["period"] == period && l["itemId"] == in.ItemID {
				found = true
				if in.On {
					keep = append(keep, l)
				}
				continue
			}
			keep = append(keep, l)
		}
		if in.On && !found {
			if _, err := s.regAdd(c, rec{"period": period, "itemId": in.ItemID, "by": strings.TrimSpace(in.By), "time": time.Now().Format("3:04 PM")}); err != nil {
				fail(w, 500, err.Error())
				return
			}
		} else if !in.On && found {
			if err := s.regSave(c, keep); err != nil {
				fail(w, 500, err.Error())
				return
			}
		}
		blocks, done, total := s.dwDay(day)
		_ = blocks
		writeJSON(w, 200, map[string]any{"done": done, "total": total})
	})
}
