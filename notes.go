package main

import ()

// Note is a handoff note one manager leaves for the next. Notes are never
// deleted; finished ones are marked done and kept as a record.
type Note struct {
	ID     string `json:"id"`
	At     string `json:"at"`
	By     string `json:"by"`
	Text   string `json:"text"`
	Done   bool   `json:"done"`
	DoneAt string `json:"doneAt"`
	DoneBy string `json:"doneBy"`
}

// ------------------------------------------------------ file fingerprints
