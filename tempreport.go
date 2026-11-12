package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func (s *store) tempReportsDir() string {
	return filepath.Join(filepath.Dir(s.dir), "Temperature Reports")
}

// dailyTempReport builds the printable temperature report for one day.
func (s *store) dailyTempReport(date string) (*report, error) {
	day, err := time.Parse(dayFmt, date)
	if err != nil {
		return nil, fmt.Errorf("Choose a date for the report.")
	}
	area := map[string]string{}
	for _, p := range s.tempPoints() {
		area[p.ID] = p.Area
	}
	var views []tempView
	for _, v := range s.tempViews(date, "") {
		if len(v.At) >= 10 && v.At[:10] == date {
			views = append(views, v)
		}
	}
	sort.SliceStable(views, func(i, j int) bool {
		if views[i].At != views[j].At {
			return views[i].At < views[j].At
		}
		return views[i].Point < views[j].Point
	})
	rows := [][]string{}
	bad, open := 0, 0
	for _, v := range views {
		isBad := v.Status == "Too Warm" || v.Status == "Too Cold"
		status := v.Status
		if isBad {
			bad++
			if strings.TrimSpace(v.Action) == "" {
				open++
				status += " (no action recorded)"
			}
		}
		at := v.At
		if t, err := time.Parse("2006-01-02 15:04", v.At); err == nil {
			at = t.Format("3:04 PM")
		}
		rows = append(rows, []string{at, v.Point, area[v.PointID], fmt.Sprintf("%g°F", v.Temp), v.Limit, status, v.By, strings.TrimSpace(v.Notes + " " + v.Action)})
	}
	sum := fmt.Sprintf("%d reading%s. %d out of range", len(views), map[bool]string{true: "", false: "s"}[len(views) == 1], bad)
	if open > 0 {
		sum += fmt.Sprintf(", %d with no corrective action recorded", open)
	}
	sum += "."
	if len(views) == 0 {
		sum = "No temperatures were recorded on this day."
	}
	return &report{Title: "Daily Temperature Report", Subtitle: day.Format("Monday, January 2, 2006"), Sections: []reportSection{
		{Note: sum + " Safe ranges follow the FDA Food Code unless a different range was set for the unit.", Cols: []string{"Time", "Unit", "Area", "Temp", "Safe Range", "Status", "Checked By", "Notes / Corrective Action"}, Rows: rows},
		{Heading: "Manager Review", Note: "Reviewed by: ______________________________      Date: ________________      Signature: ______________________________"},
	}}, nil
}

func (s *store) tempReportRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/temp-reports", func(w http.ResponseWriter, r *http.Request) {
		type item struct{ File, Date, Saved string }
		out := []item{}
		ents, _ := os.ReadDir(s.tempReportsDir())
		for _, e := range ents {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".html") {
				continue
			}
			d := strings.TrimSuffix(strings.TrimPrefix(e.Name(), "Temperature Report "), ".html")
			info, _ := e.Info()
			saved := ""
			if info != nil {
				saved = info.ModTime().Format("1/2/2006 3:04 PM")
			}
			out = append(out, item{e.Name(), d, saved})
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Date > out[j].Date })
		writeJSON(w, 200, map[string]any{"reports": out, "folder": s.tempReportsDir()})
	})
	// Saves (or refreshes) the report for one day in the Temperature Reports folder.
	mux.HandleFunc("POST /api/temp-reports", func(w http.ResponseWriter, r *http.Request) {
		var in2 struct{ Date string }
		if jsonDecode(r, &in2) != nil {
			fail(w, 400, "Could not read that.")
			return
		}
		s.mu.Lock()
		rep, err := s.dailyTempReport(in2.Date)
		s.mu.Unlock()
		if err != nil {
			fail(w, 400, err.Error())
			return
		}
		if err := os.MkdirAll(s.tempReportsDir(), 0o755); err != nil {
			fail(w, 500, "The reports folder could not be made: "+err.Error())
			return
		}
		file := "Temperature Report " + in2.Date + ".html"
		if err := os.WriteFile(filepath.Join(s.tempReportsDir(), file), []byte(rep.html(false)), 0o644); err != nil {
			fail(w, 500, "The report could not be saved: "+err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"file": file, "folder": s.tempReportsDir()})
	})
	mux.HandleFunc("GET /api/temp-reports/{file}", func(w http.ResponseWriter, r *http.Request) {
		f := r.PathValue("file")
		if f != filepath.Base(f) || !strings.HasSuffix(f, ".html") || !strings.HasPrefix(f, "Temperature Report ") {
			fail(w, 404, "Not found.")
			return
		}
		b, err := os.ReadFile(filepath.Join(s.tempReportsDir(), f))
		if err != nil {
			fail(w, 404, "That report was not found.")
			return
		}
		page := string(b)
		if r.URL.Query().Get("print") == "1" {
			page = strings.Replace(page, "</body>", `<script>onload=function(){setTimeout(function(){print()},300)}</script></body>`, 1)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(page))
	})
}
