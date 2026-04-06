// limiter.go — per-key token bucket used to enforce RPM caps.
//
// The bucket is a classic leaky-bucket with integer tokens: each
// virtual key gets `rpm` tokens that refill linearly over a 60 second
// window. The implementation is sync.Map-keyed so concurrent requests
// on different keys never contend on a shared mutex, while contention
// within a single key is bounded to a short critical section.

package keys

import (
	"sync"
	"time"
)

// Limiter is a map of keyID -> bucket. Buckets are created lazily on
// first Allow() call so an idle key costs nothing.
type Limiter struct {
	buckets sync.Map // keyID -> *bucket
}

type bucket struct {
	mu      sync.Mutex
	tokens  float64
	cap     float64
	refill  float64 // tokens per second
	lastRef time.Time
}

// NewLimiter returns an empty Limiter.
func NewLimiter() *Limiter { return &Limiter{} }

// Allow reports whether a request may proceed for keyID given its
// configured RPM. It consumes one token on success and is safe for
// concurrent use.
func (l *Limiter) Allow(keyID string, rpm int) bool {
	if rpm <= 0 {
		return true
	}
	v, _ := l.buckets.LoadOrStore(keyID, &bucket{
		tokens:  float64(rpm),
		cap:     float64(rpm),
		refill:  float64(rpm) / 60.0,
		lastRef: time.Now(),
	})
	b := v.(*bucket)

	b.mu.Lock()
	defer b.mu.Unlock()

	// If the operator changed the RPM for this key mid-flight, resize
	// the bucket in place so new limits take effect immediately.
	newCap := float64(rpm)
	if b.cap != newCap {
		b.cap = newCap
		b.refill = newCap / 60.0
		if b.tokens > b.cap {
			b.tokens = b.cap
		}
	}

	now := time.Now()
	elapsed := now.Sub(b.lastRef).Seconds()
	if elapsed > 0 {
		b.tokens += elapsed * b.refill
		if b.tokens > b.cap {
			b.tokens = b.cap
		}
		b.lastRef = now
	}
	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

// Forget drops the bucket for keyID so the next Allow call starts
// fresh. Called when a key is deleted.
func (l *Limiter) Forget(keyID string) {
	l.buckets.Delete(keyID)
}
