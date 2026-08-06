package middleware

import (
	"math"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type IPRateLimiter struct {
	mu       sync.Mutex
	limit    int
	window   time.Duration
	requests map[string][]time.Time
	now      func() time.Time
}

func NewIPRateLimiter(limit int, window time.Duration) *IPRateLimiter {
	return &IPRateLimiter{limit: limit, window: window, requests: make(map[string][]time.Time), now: time.Now}
}

func (l *IPRateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		allowed, retryAfter := l.allow(clientIP(c.Request.RemoteAddr))
		if !allowed {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}

func (l *IPRateLimiter) allow(ip string) (bool, int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	cutoff := now.Add(-l.window)
	requests := l.requests[ip]
	first := 0
	for first < len(requests) && !requests[first].After(cutoff) {
		first++
	}
	requests = requests[first:]
	if len(requests) >= l.limit {
		retryAfter := int(math.Ceil(requests[0].Add(l.window).Sub(now).Seconds()))
		if retryAfter < 1 {
			retryAfter = 1
		}
		l.requests[ip] = requests
		return false, retryAfter
	}
	l.requests[ip] = append(requests, now)
	return true, 0
}

func clientIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		return host
	}
	return remoteAddr
}
