package handler

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// rateLimiter 는 "한 사람이 1분에 몇 번" 을 센다.
//
// ★ AI 엔드포인트에만 붙인다. 공개 주소라 누가 반복 호출하면 그대로 요금이 된다.
// 판정(/api/evaluate)에는 붙이지 않는다 — 돈이 들지 않고, 심사 중에 여러 번
// 눌러보는 것이 정상이다.
//
// 고정 창(fixed window) 방식이다. 토큰 버킷보다 거칠지만, 막으려는 것이
// "한 사람이 수백 번" 이라 이 정도로 충분하다. 해커톤에서 정확도보다 단순함이 낫다.
//
// ★ IP 를 세는 데만 쓰고 로그에 남기지 않는다 (설계 원칙 2).
type rateLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	now    func() time.Time
	seen   map[string]*counter
}

type counter struct {
	start time.Time
	n     int
}

func newRateLimiter(limit int) *rateLimiter {
	return &rateLimiter{
		limit:  limit,
		window: time.Minute,
		now:    time.Now,
		seen:   map[string]*counter{},
	}
}

// allow 는 이번 요청을 받아도 되는지 본다. limit 이 0 이하면 제한하지 않는다.
func (l *rateLimiter) allow(key string) bool {
	if l == nil || l.limit <= 0 {
		return true
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	c := l.seen[key]
	if c == nil || now.Sub(c.start) >= l.window {
		l.seen[key] = &counter{start: now, n: 1}
		l.sweep(now)
		return true
	}
	if c.n >= l.limit {
		return false
	}
	c.n++
	return true
}

// sweep 은 오래된 항목을 버린다. 지우지 않으면 맵이 계속 자란다.
// 항목이 많아질 때만 돌아 평소 요청에 부담을 주지 않는다.
func (l *rateLimiter) sweep(now time.Time) {
	if len(l.seen) < 1000 {
		return
	}
	for k, c := range l.seen {
		if now.Sub(c.start) >= 2*l.window {
			delete(l.seen, k)
		}
	}
}

// limitAI 는 AI 엔드포인트를 감싼다.
func (a *API) limitAI(limiter *rateLimiter, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !limiter.allow(clientIP(r)) {
			writeError(w, http.StatusTooManyRequests, CodeTooManyRequests,
				"요청이 잠깐 몰렸습니다. 1분 뒤에 다시 시도해 주세요. "+
					"직접 입력으로는 지금 바로 판정하실 수 있습니다")
			return
		}
		next(w, r)
	}
}

// clientIP 는 요청자를 구분할 키를 만든다.
//
// Render·Vercel 같은 프록시 뒤에서는 RemoteAddr 가 프록시 주소라 모두 같아진다.
// X-Forwarded-For 의 맨 앞(원래 요청자)을 쓴다. 헤더는 위조할 수 있지만,
// 막으려는 것이 공격이 아니라 요금 폭증이라 이 정도가 적당하다.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if first, _, ok := strings.Cut(xff, ","); ok {
			return strings.TrimSpace(first)
		}
		return strings.TrimSpace(xff)
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
