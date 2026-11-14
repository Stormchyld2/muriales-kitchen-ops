package main

import (
	"sync"
	"sync/atomic"
	"time"
)

// A phone that lost Wi-Fi sends its saves later, stamped with the time they were
// really entered. While such a save is handled, nowT returns that time so the
// entry carries the right date and clock time.
var clockAt atomic.Pointer[time.Time]

func nowT() time.Time {
	if p := clockAt.Load(); p != nil {
		return *p
	}
	return time.Now()
}

// replayMu makes a queued save, its duplicate check and its clock override one step.
var replayMu sync.Mutex
