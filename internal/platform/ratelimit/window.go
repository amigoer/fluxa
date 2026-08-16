// Package ratelimit provides a small in-process counter for endpoints
// that must stay reachable without a session and therefore cannot lean on
// per-account limits alone.
//
// It is deliberately in-memory: the durable limits that protect a
// specific account live in the database next to the thing being limited,
// and this exists to blunt the broader "one caller, many targets" shape
// that no per-account counter can see. A restart clearing it, or a second
// replica keeping its own tally, both cost a little accuracy and neither
// is worth a round trip to Postgres on every unauthenticated request.
package ratelimit

import (
	"sync"
	"time"
)

// sweepThreshold is the number of tracked keys past which Allow does a
// cleanup pass. Without it a stream of one-shot keys -- which is exactly
// what an abusive caller rotating IPs produces -- would grow the map
// without bound.
const sweepThreshold = 4096

type counter struct {
	count      int
	windowEnds time.Time
}

// Window is a fixed-window counter: each key gets limit events per
// window, and the window restarts on the first event after it lapses.
type Window struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	keys   map[string]*counter
}

func NewWindow(limit int, window time.Duration) *Window {
	return &Window{limit: limit, window: window, keys: map[string]*counter{}}
}

// Allow records an event for key and reports whether it is within the
// limit. A rejected event is not counted, so a caller that keeps trying
// during a lapsed window is not held out beyond it.
func (w *Window) Allow(key string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()
	if len(w.keys) > sweepThreshold {
		w.sweep(now)
	}

	c, ok := w.keys[key]
	if !ok || now.After(c.windowEnds) {
		w.keys[key] = &counter{count: 1, windowEnds: now.Add(w.window)}
		return true
	}
	if c.count >= w.limit {
		return false
	}
	c.count++
	return true
}

// sweep drops keys whose window has lapsed. Callers hold the lock.
func (w *Window) sweep(now time.Time) {
	for key, c := range w.keys {
		if now.After(c.windowEnds) {
			delete(w.keys, key)
		}
	}
}
