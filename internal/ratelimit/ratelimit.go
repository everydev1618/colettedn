package ratelimit

import (
	"sync"
	"time"
)

// DenialReason indicates why a request was denied
type DenialReason string

const (
	NotDenied    DenialReason = ""
	MinuteLimit  DenialReason = "minute_limit"
	DailyLimit   DenialReason = "daily_limit"
)

// Result contains detailed information about a rate limit check
type Result struct {
	Allowed        bool
	Reason         DenialReason
	RetryAfter     time.Duration
	DailyRemaining int
	DailyUsed      int
	MinuteUsed     int
}

type Limiter struct {
	mu sync.RWMutex

	// Per-minute rate limiting
	requests    map[string][]time.Time
	perMinute   int

	// Daily caps
	dailyCounts map[string]*dailyCount
	dailyLimit  int
}

type dailyCount struct {
	count int
	date  string // YYYY-MM-DD
}

type Config struct {
	PerMinute  int // Max requests per minute per IP
	DailyLimit int // Max requests per day per IP
}

func New(cfg Config) *Limiter {
	if cfg.PerMinute == 0 {
		cfg.PerMinute = 5
	}
	if cfg.DailyLimit == 0 {
		cfg.DailyLimit = 30
	}

	l := &Limiter{
		requests:    make(map[string][]time.Time),
		perMinute:   cfg.PerMinute,
		dailyCounts: make(map[string]*dailyCount),
		dailyLimit:  cfg.DailyLimit,
	}

	// Start cleanup goroutine
	go l.cleanup()

	return l
}

// Allow checks if the request should be allowed and records it if so.
// isPro indicates if the user has a pro subscription (unlimited daily searches).
func (l *Limiter) Allow(ip string, isPro bool) Result {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	today := now.Format("2006-01-02")

	// Track daily usage for analytics, but only enforce for non-pro users
	dc := l.dailyCounts[ip]
	if dc == nil || dc.date != today {
		dc = &dailyCount{count: 0, date: today}
		l.dailyCounts[ip] = dc
	}

	// Check per-minute rate (applies to everyone to prevent abuse)
	cutoff := now.Add(-time.Minute)
	var recent []time.Time
	for _, t := range l.requests[ip] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}

	// Daily limit only applies to non-pro users
	if !isPro && dc.count >= l.dailyLimit {
		// Calculate time until midnight UTC
		tomorrow := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
		return Result{
			Allowed:        false,
			Reason:         DailyLimit,
			RetryAfter:     tomorrow.Sub(now),
			DailyRemaining: 0,
			DailyUsed:      dc.count,
			MinuteUsed:     len(recent),
		}
	}

	if len(recent) >= l.perMinute {
		// Find when the oldest request in the window expires
		oldest := recent[0]
		retryAfter := oldest.Add(time.Minute).Sub(now)
		return Result{
			Allowed:        false,
			Reason:         MinuteLimit,
			RetryAfter:     retryAfter,
			DailyRemaining: l.dailyLimit - dc.count,
			DailyUsed:      dc.count,
			MinuteUsed:     len(recent),
		}
	}

	// Allow the request
	l.requests[ip] = append(recent, now)
	dc.count++

	dailyRemaining := l.dailyLimit - dc.count
	if isPro {
		dailyRemaining = -1 // Indicate unlimited
	}

	return Result{
		Allowed:        true,
		Reason:         NotDenied,
		DailyRemaining: dailyRemaining,
		DailyUsed:      dc.count,
		MinuteUsed:     len(recent) + 1,
	}
}

// DailyLimit returns the configured daily limit
func (l *Limiter) DailyLimit() int {
	return l.dailyLimit
}

// cleanup periodically removes old data
func (l *Limiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		l.mu.Lock()
		now := time.Now()
		cutoff := now.Add(-time.Minute)
		today := now.Format("2006-01-02")

		// Clean old minute-window data
		for ip, times := range l.requests {
			var recent []time.Time
			for _, t := range times {
				if t.After(cutoff) {
					recent = append(recent, t)
				}
			}
			if len(recent) == 0 {
				delete(l.requests, ip)
			} else {
				l.requests[ip] = recent
			}
		}

		// Clean old daily counts
		for ip, dc := range l.dailyCounts {
			if dc.date != today {
				delete(l.dailyCounts, ip)
			}
		}

		l.mu.Unlock()
	}
}
