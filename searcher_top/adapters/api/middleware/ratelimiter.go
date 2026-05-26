package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type ipLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func RateLimiter(rps float64, burst int) func(http.Handler) http.Handler {
	type store struct {
		mu       sync.Mutex
		limiters map[string]*ipLimiter
	}

	s := &store{
		limiters: make(map[string]*ipLimiter),
	}

	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			s.mu.Lock()
			for ip, l := range s.limiters {
				if time.Since(l.lastSeen) > 5*time.Minute {
					delete(s.limiters, ip)
				}
			}
			s.mu.Unlock()
		}
	}()

	getLimiter := func(ip string) *rate.Limiter {
		s.mu.Lock()
		defer s.mu.Unlock()

		l, ok := s.limiters[ip]
		if !ok {
			l = &ipLimiter{
				limiter: rate.NewLimiter(rate.Limit(rps), burst),
			}
			s.limiters[ip] = l
		}
		l.lastSeen = time.Now()
		return l.limiter
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := extractIP(r)

			if !getLimiter(ip).Allow() {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"rate limit exceeded"}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func extractIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		parts := strings.SplitN(forwarded, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
