package main

// Inspection items: things found on a walk-through that need doing. They are
// kept in Maintenance Data\findings.json, shown on their own tab, and appear
// on the master to-do list until they are marked done. An item that needs a
// repair can carry a MaintainX work order number.

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

var (
	findingStatuses   = []string{"Open", "In Progress", "Work Order Submitted", "Done"}
	findingCategories = []string{"Equipment", "Cleaning", "Facility / Grounds", "Food Safety", "Safety", "Other"}
)

type Finding struct {
	ID          string  `json:"id"`
	Number      int     `json:"number"`
	Date        string  `json:"date"` // day it was found
	Area        string  `json:"area"`
	EquipmentID string  `json:"equipmentId"`
	Category    string  `json:"category"`
	What        string  `json:"what"` // what needs to be done
	Priority    string  `json:"priority"`
	FoundBy     string  `json:"foundBy"`
	AssignedTo  string  `json:"assignedTo"`
	Due         string  `json:"due"`
	Status      string  `json:"status"`
	WorkOrder   string  `json:"workOrder"`
	DoneAt      string  `json:"doneAt"`
	DoneBy      string  `json:"doneBy"`
	History     []Entry `json:"history"`
}

func (s *store) findingList() []*Finding {
	out := []*Finding{}
	s.readMaint("findings.json", &out)
	return out
}

func (s *store) findingRoutes(mux *http.ServeMux) {
	reply := func(w http.ResponseWriter, all []*Finding) {
		writeJSON(w, 200, map[string]any{"findings": all, "statuses": findingStatuses, "categories": findingCategories})
	}
	mux.HandleFunc("GET /api/maint/findings", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		reply(w, s.findingList())
	})
	mux.HandleFunc("POST /api/maint/findings", func(w http.ResponseWriter, r *http.Request) {
		var in2 Finding
		if json.NewDecoder(r.Body).Decode(&in2) != nil || strings.TrimSpace(in2.What) == "" || strings.TrimSpace(in2.FoundBy) == "" {
			fail(w, 400, "Enter what needs to be done and your name.")
			return
		}
		if in2.Due != "" {
			if _, err := time.Parse(dayFmt, in2.Due); err != nil {
				fail(w, 400, "Enter a valid due date.")
				return
			}
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		all := s.findingList()
		f := in2
		f.ID, f.Status = newID(), "Open"
		f.What, f.FoundBy, f.Area = strings.TrimSpace(f.What), strings.TrimSpace(f.FoundBy), strings.TrimSpace(f.Area)
		if f.Date == "" {
			f.Date = time.Now().Format(dayFmt)
		}
		if f.Priority = fixPriority(f.Priority); !oneOf(priorities, f.Priority) {
			f.Priority = priorities[2]
		}
		if !oneOf(findingCategories, f.Category) {
			f.Category = "Other"
		}
		for _, x := range all {
			if x.Number >= f.Number {
				f.Number = x.Number + 1
			}
		}
		if f.Number == 0 {
			f.Number = 1
		}
		f.DoneAt, f.DoneBy = "", ""
		f.History = []Entry{{stamp(), f.FoundBy, "Found on inspection."}}
		all = append(all, &f)
		if err := s.writeMaint("findings.json", all); err != nil {
			fail(w, 500, "It could not be saved: "+err.Error())
			return
		}
		reply(w, all)
	})
	mux.HandleFunc("PUT /api/maint/findings/{id}", func(w http.ResponseWriter, r *http.Request) {
		var in2 struct {
			Status     *string `json:"status"`
			AssignedTo *string `json:"assignedTo"`
			Due        *string `json:"due"`
			Priority   *string `json:"priority"`
			WorkOrder  *string `json:"workOrder"`
			Note       string  `json:"note"`
			By         string  `json:"by"`
		}
		if json.NewDecoder(r.Body).Decode(&in2) != nil || strings.TrimSpace(in2.By) == "" {
			fail(w, 400, "Enter your name.")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		all := s.findingList()
		by, t := strings.TrimSpace(in2.By), stamp()
		for _, f := range all {
			if f.ID != r.PathValue("id") {
				continue
			}
			add := func(text string) { f.History = append(f.History, Entry{t, by, text}) }
			if in2.Status != nil && *in2.Status != f.Status {
				if !oneOf(findingStatuses, *in2.Status) {
					fail(w, 400, "Unknown status.")
					return
				}
				f.Status = *in2.Status
				add("Status changed to " + f.Status + ".")
				if f.Status == "Done" {
					f.DoneAt, f.DoneBy = t[:10], by
				} else {
					f.DoneAt, f.DoneBy = "", ""
				}
			}
			if in2.AssignedTo != nil && strings.TrimSpace(*in2.AssignedTo) != f.AssignedTo {
				f.AssignedTo = strings.TrimSpace(*in2.AssignedTo)
				add("Assigned to " + f.AssignedTo + ".")
			}
			if in2.Due != nil && *in2.Due != f.Due {
				if *in2.Due != "" {
					if _, err := time.Parse(dayFmt, *in2.Due); err != nil {
						fail(w, 400, "Enter a valid due date.")
						return
					}
				}
				f.Due = *in2.Due
				add("Due date set to " + f.Due + ".")
			}
			if in2.Priority != nil {
				if p := fixPriority(*in2.Priority); oneOf(priorities, p) && p != f.Priority {
					f.Priority = p
					add("Priority changed to " + p + ".")
				}
			}
			if in2.WorkOrder != nil && strings.TrimSpace(*in2.WorkOrder) != f.WorkOrder {
				f.WorkOrder = strings.TrimSpace(*in2.WorkOrder)
				add("Work order set to " + f.WorkOrder + ".")
			}
			if n := strings.TrimSpace(in2.Note); n != "" {
				add(n)
			}
			if err := s.writeMaint("findings.json", all); err != nil {
				fail(w, 500, "It could not be saved: "+err.Error())
				return
			}
			reply(w, all)
			return
		}
		fail(w, 404, "That item was not found.")
	})
}
