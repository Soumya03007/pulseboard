package middleware

import (
	"testing"
	"time"
)

func TestIPRateLimiterAllowsThenRecovers(t *testing.T) {
	limiter := NewIPRateLimiter(2, time.Minute)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	limiter.now = func() time.Time { return now }

	if allowed, _ := limiter.allow("127.0.0.1"); !allowed {
		t.Fatal("first request rejected")
	}
	if allowed, _ := limiter.allow("127.0.0.1"); !allowed {
		t.Fatal("second request rejected")
	}
	if allowed, retryAfter := limiter.allow("127.0.0.1"); allowed || retryAfter != 60 {
		t.Fatalf("expected rate limit with 60 seconds, got allowed=%v retry=%d", allowed, retryAfter)
	}
	now = now.Add(time.Minute)
	if allowed, _ := limiter.allow("127.0.0.1"); !allowed {
		t.Fatal("request did not recover after window")
	}
}
