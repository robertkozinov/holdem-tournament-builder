package http

import (
	"math"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"
	"holdem-tournament-builder/internal/transport/response"
)

type rateClass int

const (
	generalRate rateClass = iota
	registerRate
	loginRate
)

type rateLimitOptions struct {
	rate  rate.Limit
	burst int
}

type clientBuckets struct {
	buckets  [3]*rate.Limiter
	lastSeen time.Time
}

type ipRateLimiter struct {
	mu              sync.Mutex
	now             func() time.Time
	limits          [3]rateLimitOptions
	clients         map[string]*clientBuckets
	maxClients      int
	idle            time.Duration
	cleanupInterval time.Duration
	nextCleanup     time.Time
}

type requestSafety struct {
	limiter *ipRateLimiter
	hashing chan struct{}
}

func newRequestSafety(now func() time.Time) *requestSafety {
	return &requestSafety{
		limiter: &ipRateLimiter{
			now:             now,
			limits:          [3]rateLimitOptions{{rate: 2, burst: 60}, {rate: rate.Limit(5) / 60, burst: 3}, {rate: rate.Limit(10) / 60, burst: 10}},
			clients:         make(map[string]*clientBuckets),
			maxClients:      10000,
			idle:            10 * time.Minute,
			cleanupInterval: time.Minute,
		},
		hashing: make(chan struct{}, 2),
	}
}

func (l *ipRateLimiter) allow(ip string, class rateClass) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if !now.Before(l.nextCleanup) {
		for key, client := range l.clients {
			if now.Sub(client.lastSeen) >= l.idle {
				delete(l.clients, key)
			}
		}
		l.nextCleanup = now.Add(l.cleanupInterval)
	}
	client := l.clients[ip]
	if client == nil {
		if len(l.clients) >= l.maxClients {
			return false, l.nextCleanup.Sub(now)
		}
		client = &clientBuckets{}
		for i, options := range l.limits {
			client.buckets[i] = rate.NewLimiter(options.rate, options.burst)
		}
		l.clients[ip] = client
	}
	client.lastSeen = now
	reservation := client.buckets[class].ReserveN(now, 1)
	if !reservation.OK() {
		return false, time.Minute
	}
	delay := reservation.DelayFrom(now)
	if delay > 0 {
		reservation.CancelAt(now)
		return false, delay
	}
	return true, 0
}

func requestIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		return ip.Unmap().String()
	}
	return "unknown"
}

func (s *requestSafety) rateLimit(class rateClass) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Context().Err() != nil {
				response.WriteError(w, http.StatusServiceUnavailable, "request unavailable")
				return
			}
			if allowed, retry := s.limiter.allow(requestIP(r), class); !allowed {
				seconds := max(1, int(math.Ceil(retry.Seconds())))
				w.Header().Set("Retry-After", strconv.Itoa(seconds))
				response.WriteError(w, http.StatusTooManyRequests, "too many requests")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (s *requestSafety) limitHashing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case s.hashing <- struct{}{}:
			defer func() { <-s.hashing }()
		default:
			response.WriteError(w, http.StatusServiceUnavailable, "authentication temporarily unavailable")
			return
		}
		if r.Context().Err() != nil {
			response.WriteError(w, http.StatusServiceUnavailable, "request unavailable")
			return
		}
		next.ServeHTTP(w, r)
	})
}
