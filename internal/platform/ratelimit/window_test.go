package ratelimit

import (
	"testing"
	"time"
)

func TestWindowAllowsUpToTheLimit(t *testing.T) {
	w := NewWindow(3, time.Minute)

	for i := range 3 {
		if !w.Allow("a") {
			t.Fatalf("event %d rejected while under the limit", i+1)
		}
	}
	if w.Allow("a") {
		t.Error("the fourth event was allowed past a limit of 3")
	}
}

func TestWindowKeysAreIndependent(t *testing.T) {
	w := NewWindow(1, time.Minute)

	if !w.Allow("a") || !w.Allow("b") {
		t.Fatal("separate keys must not share a budget")
	}
	if w.Allow("a") {
		t.Error("key a was allowed a second event")
	}
}

func TestWindowResetsAfterItLapses(t *testing.T) {
	w := NewWindow(1, 50*time.Millisecond)

	if !w.Allow("a") {
		t.Fatal("first event rejected")
	}
	if w.Allow("a") {
		t.Fatal("second event allowed inside the window")
	}

	time.Sleep(60 * time.Millisecond)
	if !w.Allow("a") {
		t.Error("the window never reopened")
	}
}

func TestWindowSweepsLapsedKeys(t *testing.T) {
	w := NewWindow(1, time.Millisecond)

	// Past the sweep threshold, a stream of one-shot keys must not keep
	// growing the map -- that stream is what an abusive caller looks like.
	for i := range sweepThreshold + 100 {
		w.Allow(string(rune(i)))
	}
	time.Sleep(5 * time.Millisecond)
	w.Allow("trigger-sweep")

	w.mu.Lock()
	size := len(w.keys)
	w.mu.Unlock()
	if size > sweepThreshold {
		t.Errorf("tracked %d keys after a sweep, want the lapsed ones dropped", size)
	}
}
