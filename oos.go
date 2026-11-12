package main

// Equipment out-of-service holds. Taking equipment out of service needs a
// reason and a name. It stays out of service, with an alert at the top of every
// screen, until someone on the manager list clears it and says what was checked
// or fixed. A verified repair on a work order does not lift the hold. Each hold
// is kept in Maintenance Data\oos.json as a permanent record.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type OOSRecord struct {
	ID          string `json:"id"`
	EquipmentID string `json:"equipmentId"`
	Reason      string `json:"reason"`
	IssueID     string `json:"issueId,omitempty"`
	RemovedBy   string `json:"removedBy"`
	RemovedAt   string `json:"removedAt"`
	ClearedBy   string `json:"clearedBy,omitempty"`
	ClearedAt   string `json:"clearedAt,omitempty"`
	ClearNote   string `json:"clearNote,omitempty"`
	// Details printed on the out-of-service tag.
	AssetID     string `json:"assetId,omitempty"`
	Location    string `json:"location,omitempty"`
	Safety      string `json:"safety,omitempty"`     // Yes / No: safety hazard
	FoodSafety  string `json:"foodSafety,omitempty"` // Yes / No
	PowerOff    string `json:"powerOff,omitempty"`   // Yes / No: power or gas disconnected
	Lockout     string `json:"lockout,omitempty"`    // Yes / No: lockout / tagout used
	WorkOrder   string `json:"workOrder,omitempty"`
	Vendor      string `json:"vendor,omitempty"`
	TagPriority string `json:"tagPriority,omitempty"` // High / Medium / Low
	Tag         string `json:"tag,omitempty"`         // saved tag image file name
}

// tagFields are the details typed on the tag form.
type tagFields struct {
	AssetID     string `json:"assetId"`
	Location    string `json:"location"`
	Safety      string `json:"safety"`
	FoodSafety  string `json:"foodSafety"`
	PowerOff    string `json:"powerOff"`
	Lockout     string `json:"lockout"`
	WorkOrder   string `json:"workOrder"`
	Vendor      string `json:"vendor"`
	TagPriority string `json:"tagPriority"`
}

func yesNo(v string) string {
	if v == "Yes" || v == "No" {
		return v
	}
	return ""
}

func (t tagFields) apply(r *OOSRecord) {
	r.Location = strings.TrimSpace(t.Location)
	r.AssetID, r.WorkOrder, r.Vendor = strings.TrimSpace(t.AssetID), strings.TrimSpace(t.WorkOrder), strings.TrimSpace(t.Vendor)
	r.Safety, r.FoodSafety, r.PowerOff, r.Lockout = yesNo(t.Safety), yesNo(t.FoodSafety), yesNo(t.PowerOff), yesNo(t.Lockout)
	if t.TagPriority == "High" || t.TagPriority == "Medium" || t.TagPriority == "Low" {
		r.TagPriority = t.TagPriority
	}
}

func (s *store) tagsDir() string { return filepath.Join(filepath.Dir(s.dir), "Out of Service Tags") }

func (s *store) oosList() []*OOSRecord {
	out := []*OOSRecord{}
	s.readMaint("oos.json", &out)
	return out
}

// holdStart puts e out of service and records why. The caller holds s.mu and
// saves the equipment list. A unit that already has an open hold keeps it.
func (s *store) holdStart(e *Equipment, reason, by, issueID string) {
	e.Status = "Out of Service"
	all := s.oosList()
	for _, r := range all {
		if r.EquipmentID == e.ID && r.ClearedAt == "" {
			return
		}
	}
	all = append(all, &OOSRecord{ID: newID(), EquipmentID: e.ID, Reason: reason, IssueID: issueID, RemovedBy: by, RemovedAt: stamp()})
	_ = s.writeMaint("oos.json", all)
}

func (s *store) canClear(name string, managers []string) bool {
	if len(managers) == 0 {
		return true
	}
	for _, m := range managers {
		if strings.EqualFold(strings.TrimSpace(m), strings.TrimSpace(name)) {
			return true
		}
	}
	return false
}

func (s *store) oosRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/maint/equipment/{id}/out-of-service", func(w http.ResponseWriter, r *http.Request) {
		var in2 struct {
			Reason, By, IssueID string
			tagFields
		}
		if json.NewDecoder(r.Body).Decode(&in2) != nil || strings.TrimSpace(in2.Reason) == "" || strings.TrimSpace(in2.By) == "" {
			fail(w, 400, "Enter the reason and your name.")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		eq, is, _ := s.loadMaint()
		for _, e := range eq {
			if e.ID != r.PathValue("id") {
				continue
			}
			if e.Status == "Out of Service" {
				fail(w, 400, "This equipment is already out of service.")
				return
			}
			s.holdStart(e, strings.TrimSpace(in2.Reason), strings.TrimSpace(in2.By), in2.IssueID)
			all := s.oosList()
			for _, rec := range all {
				if rec.EquipmentID == e.ID && rec.ClearedAt == "" {
					in2.tagFields.apply(rec)
				}
			}
			_ = s.writeMaint("oos.json", all)
			if err := s.writeMaint("equipment.json", eq); err != nil {
				s.maintFail(w, err)
				return
			}
			s.writeLog(eq, is)
			writeJSON(w, 200, s.maintPayload())
			return
		}
		fail(w, 404, "That equipment was not found.")
	})

	mux.HandleFunc("POST /api/maint/equipment/{id}/clear", func(w http.ResponseWriter, r *http.Request) {
		var in2 struct {
			By, Note string
			Safe     bool
		}
		if json.NewDecoder(r.Body).Decode(&in2) != nil || strings.TrimSpace(in2.By) == "" {
			fail(w, 400, "Enter your name.")
			return
		}
		if strings.TrimSpace(in2.Note) == "" {
			fail(w, 400, "Write what was checked or repaired.")
			return
		}
		if !in2.Safe {
			fail(w, 400, "Confirm the equipment has been checked and is safe for food production.")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		eq, is, st := s.loadMaint()
		if !s.canClear(in2.By, st.Managers) {
			fail(w, 403, "Only an authorized manager can clear equipment for service. Authorized: "+strings.Join(st.Managers, ", ")+". The list is in Settings.")
			return
		}
		for _, e := range eq {
			if e.ID != r.PathValue("id") {
				continue
			}
			if e.Status != "Out of Service" {
				fail(w, 400, "This equipment is not out of service.")
				return
			}
			by, now := strings.TrimSpace(in2.By), stamp()
			all := s.oosList()
			closed := false
			for _, rec := range all {
				if rec.EquipmentID == e.ID && rec.ClearedAt == "" {
					rec.ClearedBy, rec.ClearedAt, rec.ClearNote = by, now, strings.TrimSpace(in2.Note)
					closed = true
				}
			}
			if !closed { // taken out of service before holds were recorded
				all = append(all, &OOSRecord{ID: newID(), EquipmentID: e.ID, Reason: "Not recorded", RemovedAt: now, RemovedBy: "", ClearedBy: by, ClearedAt: now, ClearNote: strings.TrimSpace(in2.Note)})
			}
			e.Status = "Operational"
			for _, p := range is {
				if p.EquipmentID == e.ID && p.Status != "Verified" && p.Status != "Closed" {
					e.Status = "Needs Attention" // work orders are still open
				}
			}
			if err := s.writeMaint("oos.json", all); err != nil {
				s.maintFail(w, err)
				return
			}
			if err := s.writeMaint("equipment.json", eq); err != nil {
				s.maintFail(w, err)
				return
			}
			s.writeLog(eq, is)
			writeJSON(w, 200, s.maintPayload())
			return
		}
		fail(w, 404, "That equipment was not found.")
	})
}

// oosMarkdown lists holds for the Maintenance Log.md file.
func (s *store) oosMarkdown(eq []*Equipment) string {
	names := map[string]string{}
	for _, e := range eq {
		names[e.ID] = e.Name
	}
	var b strings.Builder
	b.WriteString("\n## Out of service\n\n")
	n := 0
	all := s.oosList()
	for i := len(all) - 1; i >= 0; i-- {
		r := all[i]
		state := "**OUT OF SERVICE**"
		if r.ClearedAt != "" {
			state = "cleared " + r.ClearedAt[:10] + " by " + r.ClearedBy + " (" + r.ClearNote + ")"
		}
		fmt.Fprintf(&b, "- %s: %s, removed %s by %s. %s. %s\n", names[r.EquipmentID], r.Reason, r.RemovedAt[:10], r.RemovedBy, state, daysOut(r))
		n++
	}
	if n == 0 {
		b.WriteString("None.\n")
	}
	return b.String()
}

func daysOut(r *OOSRecord) string {
	from, err := time.Parse(dayFmt, r.RemovedAt[:10])
	if err != nil {
		return ""
	}
	to := time.Now()
	if r.ClearedAt != "" {
		if t, err := time.Parse(dayFmt, r.ClearedAt[:10]); err == nil {
			to = t
		}
	}
	d := int(to.Sub(from).Hours()/24) + 0
	return fmt.Sprintf("%d day%s out", d, map[bool]string{true: "", false: "s"}[d == 1])
}

func (s *store) tagRoutes(mux *http.ServeMux) {
	active := func(all []*OOSRecord, id string) *OOSRecord {
		for _, r := range all {
			if r.EquipmentID == id && r.ClearedAt == "" {
				return r
			}
		}
		return nil
	}
	// Details for the tag of a hold that was started elsewhere (for example
	// by ticking "out of service" when reporting an issue).
	mux.HandleFunc("POST /api/maint/equipment/{id}/tag-details", func(w http.ResponseWriter, r *http.Request) {
		var in2 tagFields
		if json.NewDecoder(r.Body).Decode(&in2) != nil {
			fail(w, 400, "Could not read that.")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		all := s.oosList()
		rec := active(all, r.PathValue("id"))
		if rec == nil {
			fail(w, 400, "This equipment is not out of service.")
			return
		}
		in2.apply(rec)
		if err := s.writeMaint("oos.json", all); err != nil {
			s.maintFail(w, err)
			return
		}
		writeJSON(w, 200, s.maintPayload())
	})
	// The program draws the tag in the window and sends the finished picture here.
	mux.HandleFunc("POST /api/maint/equipment/{id}/tag", func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
		if err != nil || len(b) < 8 || string(b[1:4]) != "PNG" {
			fail(w, 400, "The tag picture was not received.")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		eq, _, _ := s.loadMaint()
		name := ""
		for _, e := range eq {
			if e.ID == r.PathValue("id") {
				name = e.Name
			}
		}
		all := s.oosList()
		rec := active(all, r.PathValue("id"))
		if rec == nil || name == "" {
			fail(w, 400, "This equipment is not out of service.")
			return
		}
		if err := os.MkdirAll(s.tagsDir(), 0o755); err != nil {
			fail(w, 500, "The tags folder could not be made: "+err.Error())
			return
		}
		file := strings.TrimSpace(safeName.ReplaceAllString("Out of Service Tag - "+name+" - "+time.Now().Format("2006-01-02 1504"), "")) + ".png"
		if err := os.WriteFile(filepath.Join(s.tagsDir(), file), b, 0o644); err != nil {
			fail(w, 500, "The tag could not be saved: "+err.Error())
			return
		}
		rec.Tag = file
		if err := s.writeMaint("oos.json", all); err != nil {
			s.maintFail(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"file": file, "folder": s.tagsDir(), "payload": s.maintPayload()})
	})
	mux.HandleFunc("GET /api/oos-tags/{file}", func(w http.ResponseWriter, r *http.Request) {
		f := r.PathValue("file")
		if f != filepath.Base(f) || !strings.HasSuffix(f, ".png") {
			fail(w, 404, "Not found.")
			return
		}
		if r.URL.Query().Get("print") == "1" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, `<!doctype html><html><head><meta charset="utf-8"><title>Out of Service Tag</title><style>@page{size:landscape;margin:8mm}html,body{margin:0;background:#fff}img{width:100%%;height:auto;display:block}.b{margin:8px}@media print{.b{display:none}}</style></head><body><div class="b"><button onclick="print()">Print</button></div><img src="/api/oos-tags/%s" onload="setTimeout(function(){print()},300)"></body></html>`, strings.ReplaceAll(f, " ", "%20"))
			return
		}
		b, err := os.ReadFile(filepath.Join(s.tagsDir(), f))
		if err != nil {
			fail(w, 404, "That tag was not found.")
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(b)
	})
}
