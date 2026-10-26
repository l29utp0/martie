package channer

import (
	"sync"
	"time"

	"github.com/loynet/ptchan-gateway/clients/go"
	"golang.org/x/time/rate"
)

const donorNoticeCooldown = 12 * time.Hour

type limitResult uint8

const (
	limitAllowed limitResult = iota
	limitGlobal
	limitThread
	limitDonorNotice
)

type threadLimit struct {
	limiter         *rate.Limiter
	lastSeen        time.Time
	lastDonorNotice time.Time
}

// Limiter applies the process-wide and per-thread reply budgets as one
// decision, so a denied request never consumes capacity from the other budget.
type Limiter struct {
	mu sync.Mutex

	global      *rate.Limiter
	threadRate  rate.Limit
	threadBurst int
	threads     map[gateway.ThreadRef]*threadLimit
}

func NewLimiter(globalPerHour, globalBurst, threadPerHour, threadBurst int) *Limiter {
	return &Limiter{
		global:      hourlyLimiter(globalPerHour, globalBurst),
		threadRate:  hourlyRate(threadPerHour),
		threadBurst: threadBurst,
		threads:     make(map[gateway.ThreadRef]*threadLimit),
	}
}

func (l *Limiter) allow(thread gateway.ThreadRef, now time.Time, donorNotice bool) limitResult {
	l.mu.Lock()
	defer l.mu.Unlock()

	for ref, limit := range l.threads {
		if now.Sub(limit.lastSeen) >= time.Hour && now.Sub(limit.lastDonorNotice) >= donorNoticeCooldown {
			delete(l.threads, ref)
		}
	}

	if limit := l.threads[thread]; donorNotice && limit != nil && !limit.lastDonorNotice.IsZero() && now.Sub(limit.lastDonorNotice) < donorNoticeCooldown {
		return limitDonorNotice
	}

	if l.global.TokensAt(now) < 1 {
		if limit := l.threads[thread]; limit != nil {
			limit.lastSeen = now
		}
		return limitGlobal
	}

	limit := l.threads[thread]
	if limit == nil {
		limit = &threadLimit{
			limiter: rate.NewLimiter(l.threadRate, l.threadBurst),
		}
		l.threads[thread] = limit
	}
	limit.lastSeen = now
	if limit.limiter.TokensAt(now) < 1 {
		return limitThread
	}

	// Reserve before posting so concurrent requests and uncertain outcomes cannot
	// produce repeated notices. Failed attempts also retain the cooldown.
	if donorNotice {
		limit.lastDonorNotice = now
	}
	l.global.AllowN(now, 1)
	limit.limiter.AllowN(now, 1)
	return limitAllowed
}

func hourlyLimiter(requests, burst int) *rate.Limiter {
	return rate.NewLimiter(hourlyRate(requests), burst)
}

func hourlyRate(requests int) rate.Limit {
	return rate.Limit(float64(requests) / time.Hour.Seconds())
}
