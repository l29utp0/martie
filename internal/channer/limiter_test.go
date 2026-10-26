package channer

import (
	"testing"
	"time"

	"github.com/loynet/ptchan-gateway/clients/go"
)

func TestLimiterKeepsThreadAndGlobalBudgetsIndependent(t *testing.T) {
	now := time.Now()
	first := gateway.ThreadRef{Board: "i", ThreadID: 1}
	second := gateway.ThreadRef{Board: "i", ThreadID: 2}
	limit := NewLimiter(3, 3, 2, 2)

	if got := limit.allow(first, now, false); got != limitAllowed {
		t.Fatalf("first request = %v, want allowed", got)
	}
	if got := limit.allow(first, now, false); got != limitAllowed {
		t.Fatalf("second request = %v, want allowed", got)
	}
	if got := limit.allow(first, now, false); got != limitThread {
		t.Fatalf("third request = %v, want thread limit", got)
	}
	if got := limit.allow(second, now, false); got != limitAllowed {
		t.Fatalf("other thread after denial = %v, want allowed", got)
	}
	if got := limit.allow(gateway.ThreadRef{Board: "i", ThreadID: 3}, now, false); got != limitGlobal {
		t.Fatalf("request after global budget = %v, want global limit", got)
	}
}

func TestLimiterDiscardsInactiveThreads(t *testing.T) {
	now := time.Now()
	first := gateway.ThreadRef{Board: "i", ThreadID: 1}
	second := gateway.ThreadRef{Board: "i", ThreadID: 2}
	limit := NewLimiter(10, 10, 2, 2)

	limit.allow(first, now, false)
	limit.allow(second, now.Add(30*time.Minute), false)
	limit.allow(second, now.Add(time.Hour), false)

	if _, ok := limit.threads[first]; ok {
		t.Fatal("inactive thread bucket was retained")
	}
	if _, ok := limit.threads[second]; !ok {
		t.Fatal("active thread bucket was discarded")
	}
}

func TestDonorNoticeCooldown(t *testing.T) {
	now := time.Now()
	thread := gateway.ThreadRef{Board: "i", ThreadID: 1}
	limit := NewLimiter(20, 20, 20, 20)
	for _, tt := range []struct {
		name   string
		ref    gateway.ThreadRef
		at     time.Time
		notice bool
		want   limitResult
	}{
		{"first notice", thread, now, true, limitAllowed},
		{"repeat notice", thread, now, true, limitDonorNotice},
		{"donor still allowed", thread, now, false, limitAllowed},
		{"another thread", gateway.ThreadRef{Board: "i", ThreadID: 2}, now, true, limitAllowed},
		{"another board", gateway.ThreadRef{Board: "a", ThreadID: 1}, now, true, limitAllowed},
		{"after bucket expiry", thread, now.Add(2 * time.Hour), true, limitDonorNotice},
		{"just before cooldown expires", thread, now.Add(12*time.Hour - time.Nanosecond), true, limitDonorNotice},
		{"cooldown expires", thread, now.Add(12 * time.Hour), true, limitAllowed},
		{"new window", thread, now.Add(12 * time.Hour), true, limitDonorNotice},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := limit.allow(tt.ref, tt.at, tt.notice); got != tt.want {
				t.Fatalf("allow = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSuppressedDonorNoticeDoesNotConsumeBudgets(t *testing.T) {
	now := time.Now()
	thread := gateway.ThreadRef{Board: "i", ThreadID: 1}
	limit := NewLimiter(2, 2, 2, 2)
	if got := limit.allow(thread, now, true); got != limitAllowed {
		t.Fatal(got)
	}
	for range 10 {
		if got := limit.allow(thread, now, true); got != limitDonorNotice {
			t.Fatal(got)
		}
	}
	if got := limit.allow(thread, now, false); got != limitAllowed {
		t.Fatalf("donor after suppressed notices = %v", got)
	}
}

func TestRateLimitedDonorNoticeDoesNotStartCooldown(t *testing.T) {
	now := time.Now()
	thread := gateway.ThreadRef{Board: "i", ThreadID: 1}
	limit := NewLimiter(1, 1, 1, 1)
	if got := limit.allow(thread, now, false); got != limitAllowed {
		t.Fatal(got)
	}
	if got := limit.allow(thread, now, true); got != limitGlobal {
		t.Fatal(got)
	}
	if got := limit.allow(thread, now.Add(time.Hour), true); got != limitAllowed {
		t.Fatalf("notice after refill = %v", got)
	}
}
