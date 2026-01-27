package ratelimit

import (
	"testing"
	"time"
)

func TestLimiterPerMinute(t *testing.T) {
	l := New(Config{PerMinute: 3, DailyLimit: 100})

	ip := "192.168.1.1"

	// First 3 requests should be allowed
	for i := 0; i < 3; i++ {
		result := l.Allow(ip, false)
		if !result.Allowed {
			t.Errorf("request %d should be allowed", i+1)
		}
	}

	// 4th request should be denied (minute limit)
	result := l.Allow(ip, false)
	if result.Allowed {
		t.Error("4th request should be denied (per-minute limit)")
	}
	if result.Reason != MinuteLimit {
		t.Errorf("expected reason MinuteLimit, got %v", result.Reason)
	}
	if result.RetryAfter <= 0 {
		t.Error("RetryAfter should be positive")
	}
}

func TestLimiterDailyLimit(t *testing.T) {
	l := New(Config{PerMinute: 100, DailyLimit: 3})

	ip := "192.168.1.2"

	// First 3 requests should be allowed
	for i := 0; i < 3; i++ {
		result := l.Allow(ip, false)
		if !result.Allowed {
			t.Errorf("request %d should be allowed", i+1)
		}
		if result.DailyRemaining != 3-i-1 {
			t.Errorf("expected DailyRemaining=%d, got %d", 3-i-1, result.DailyRemaining)
		}
	}

	// 4th request should be denied (daily limit)
	result := l.Allow(ip, false)
	if result.Allowed {
		t.Error("4th request should be denied (daily limit)")
	}
	if result.Reason != DailyLimit {
		t.Errorf("expected reason DailyLimit, got %v", result.Reason)
	}
}

func TestLimiterProUserBypassesDaily(t *testing.T) {
	l := New(Config{PerMinute: 100, DailyLimit: 2})

	ip := "192.168.1.3"

	// Exhaust daily limit for this IP
	l.Allow(ip, false)
	l.Allow(ip, false)

	// Non-pro should be denied
	result := l.Allow(ip, false)
	if result.Allowed {
		t.Error("non-pro user should be denied after daily limit")
	}

	// Pro user should be allowed (bypasses daily limit)
	result = l.Allow(ip, true)
	if !result.Allowed {
		t.Error("pro user should bypass daily limit")
	}
	if result.DailyRemaining != -1 {
		t.Errorf("pro user should have DailyRemaining=-1 (unlimited), got %d", result.DailyRemaining)
	}
}

func TestLimiterProUserStillHasMinuteLimit(t *testing.T) {
	l := New(Config{PerMinute: 2, DailyLimit: 100})

	ip := "192.168.1.4"

	// Pro users still have per-minute limit (abuse prevention)
	l.Allow(ip, true)
	l.Allow(ip, true)

	result := l.Allow(ip, true)
	if result.Allowed {
		t.Error("pro user should still be subject to per-minute limit")
	}
	if result.Reason != MinuteLimit {
		t.Errorf("expected MinuteLimit, got %v", result.Reason)
	}
}

func TestLimiterDifferentIPs(t *testing.T) {
	l := New(Config{PerMinute: 1, DailyLimit: 100})

	// Each IP should have its own limit
	result1 := l.Allow("10.0.0.1", false)
	result2 := l.Allow("10.0.0.2", false)

	if !result1.Allowed || !result2.Allowed {
		t.Error("different IPs should have separate limits")
	}
}

func TestLimiterDailyLimitMethod(t *testing.T) {
	l := New(Config{DailyLimit: 50})
	if l.DailyLimit() != 50 {
		t.Errorf("expected DailyLimit()=50, got %d", l.DailyLimit())
	}
}

func TestLimiterDefaultConfig(t *testing.T) {
	l := New(Config{}) // Empty config should use defaults

	if l.perMinute != 5 {
		t.Errorf("expected default perMinute=5, got %d", l.perMinute)
	}
	if l.dailyLimit != 3 {
		t.Errorf("expected default dailyLimit=3, got %d", l.dailyLimit)
	}
}

func TestLimiterUsageTracking(t *testing.T) {
	l := New(Config{PerMinute: 10, DailyLimit: 100})

	ip := "192.168.1.5"

	result := l.Allow(ip, false)
	if result.DailyUsed != 1 {
		t.Errorf("expected DailyUsed=1, got %d", result.DailyUsed)
	}
	if result.MinuteUsed != 1 {
		t.Errorf("expected MinuteUsed=1, got %d", result.MinuteUsed)
	}

	result = l.Allow(ip, false)
	if result.DailyUsed != 2 {
		t.Errorf("expected DailyUsed=2, got %d", result.DailyUsed)
	}
}

func TestLimiterRetryAfterCalculation(t *testing.T) {
	l := New(Config{PerMinute: 1, DailyLimit: 100})

	ip := "192.168.1.6"

	l.Allow(ip, false)
	result := l.Allow(ip, false)

	if result.Allowed {
		t.Fatal("expected request to be denied")
	}

	// RetryAfter should be less than or equal to 1 minute
	if result.RetryAfter > time.Minute {
		t.Errorf("RetryAfter should be <= 1 minute, got %v", result.RetryAfter)
	}
	if result.RetryAfter <= 0 {
		t.Error("RetryAfter should be positive")
	}
}
