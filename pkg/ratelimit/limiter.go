package ratelimit

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// Simple token bucket rate limiter per remote IP.
// All standard library.

type bucket struct {
	tokens chan struct{}
}

type Limiter struct {
	mu      sync.Mutex
	perMin  int
	burst   int
	buckets map[string]*bucket
}

func New(perMin, burst int) *Limiter {
	if perMin <= 0 {
		perMin = 60
	}
	if burst <= 0 {
		burst = 120
	}
	return &Limiter{perMin: perMin, burst: burst, buckets: make(map[string]*bucket)}
}

func (l *Limiter) get(ip string) *bucket {
	l.mu.Lock()
	defer l.mu.Unlock()
	b := l.buckets[ip]
	if b == nil {
		b = &bucket{tokens: make(chan struct{}, l.burst)}
		// fill burst
		for i := 0; i < l.burst; i++ {
			b.tokens <- struct{}{}
		}
		// refill goroutine
		interval := time.Minute / time.Duration(l.perMin)
		go func() {
			t := time.NewTicker(interval)
			defer t.Stop()
			for range t.C {
				select {
				case b.tokens <- struct{}{}:
				default:
				}
			}
		}()
		l.buckets[ip] = b
	}
	return b
}

func (l *Limiter) Allow(r *http.Request) bool {
	ip := clientIP(r)
	b := l.get(ip)
	select {
	case <-b.tokens:
		return true
	default:
		return false
	}
}

func clientIP(r *http.Request) string {
	// best effort: X-Forwarded-For first IP, else RemoteAddr host
	if xf := r.Header.Get("X-Forwarded-For"); xf != "" {
		parts := xf
		if idx := indexByte(parts, ','); idx >= 0 {
			parts = parts[:idx]
		}
		return trimSpace(parts)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}
