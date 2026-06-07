// pool.go — Searcher that fans requests across a pool of underlying Searchers,
// rotating round-robin and parking members that hit a quota or transient
// failure for a configurable cooldown period.
//
// Designed for the common case of stitching together several free-tier
// RapidAPI keys into one effective searcher, but works with any mix of
// Searcher implementations.
package twitter

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// PoolMember pairs a Searcher with a human-readable name (for logs).
type PoolMember struct {
	Name     string
	Searcher Searcher
}

// Pool is a Searcher composed of multiple member Searchers. It tries members
// in round-robin order, skipping any that are currently in cooldown.
//
// A member enters cooldown when it returns a *QuotaError (quota / throttle).
// Other errors are returned to the caller without parking the member; only
// quota-shaped failures justify trying the next provider.
type Pool struct {
	mu       sync.Mutex
	members  []*memberState
	next     int           // round-robin cursor
	cooldown time.Duration // how long to park a quota-exhausted member
}

type memberState struct {
	PoolMember
	cooldownUntil time.Time
}

// NewPool constructs a Pool. Cooldown defaults to 1 hour if zero is passed.
func NewPool(members []PoolMember, cooldown time.Duration) *Pool {
	if cooldown <= 0 {
		cooldown = time.Hour
	}
	states := make([]*memberState, len(members))
	for i, m := range members {
		states[i] = &memberState{PoolMember: m}
	}
	return &Pool{members: states, cooldown: cooldown}
}

// Len returns the number of pool members (handy for tests/diagnostics).
func (p *Pool) Len() int { return len(p.members) }

// Search calls the next available pool member. If a member returns a
// QuotaError it is parked for the cooldown window and the next member is
// tried. The error from the *last* attempted member is returned if every
// member fails or is in cooldown.
func (p *Pool) Search(ctx context.Context, query string, max int) ([]Tweet, error) {
	if len(p.members) == 0 {
		return nil, errors.New("pool: no members")
	}

	tried := 0
	var lastErr error
	for tried < len(p.members) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		m := p.pickNext()
		if m == nil {
			// All members are in cooldown. Surface the most recent error if
			// we have one, otherwise a generic "all parked" message.
			if lastErr != nil {
				return nil, fmt.Errorf("pool: all members in cooldown; last error: %w", lastErr)
			}
			return nil, errors.New("pool: all members in cooldown")
		}
		tried++

		tweets, err := m.Searcher.Search(ctx, query, max)
		if err == nil {
			log.Printf("pool: %s served %d tweet(s)", m.Name, len(tweets))
			return tweets, nil
		}

		lastErr = err
		if IsQuotaError(err) {
			p.parkMember(m, err)
			continue
		}
		// Non-quota error: don't retry blindly across providers — different
		// providers can fail for different reasons (auth, schema mismatch,
		// network) and silently rotating would hide real bugs.
		return nil, fmt.Errorf("pool: %s failed: %w", m.Name, err)
	}
	return nil, fmt.Errorf("pool: all %d member(s) exhausted; last error: %w", len(p.members), lastErr)
}

// pickNext returns the next member that is not currently in cooldown,
// advancing the round-robin cursor. Returns nil if every member is parked.
func (p *Pool) pickNext() *memberState {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	for i := 0; i < len(p.members); i++ {
		idx := (p.next + i) % len(p.members)
		m := p.members[idx]
		if now.After(m.cooldownUntil) {
			p.next = (idx + 1) % len(p.members)
			return m
		}
	}
	return nil
}

func (p *Pool) parkMember(m *memberState, cause error) {
	d := p.cooldownFor(cause)
	p.mu.Lock()
	m.cooldownUntil = time.Now().Add(d)
	p.mu.Unlock()
	log.Printf("pool: parking %s for %s (%v)", m.Name, d, cause)
}

// cooldownFor uses a short backoff for per-minute "too many requests" 429s
// and the configured cooldown for real daily/monthly quota exhaustion.
func (p *Pool) cooldownFor(cause error) time.Duration {
	var q *QuotaError
	if errors.As(cause, &q) && q.Status == 429 {
		b := strings.ToLower(q.Body)
		if strings.Contains(b, "too many requests") ||
			strings.Contains(b, "rate limit") ||
			strings.Contains(b, "too many request") {
			if d := 2 * time.Minute; d < p.cooldown {
				return d
			}
		}
	}
	return p.cooldown
}
