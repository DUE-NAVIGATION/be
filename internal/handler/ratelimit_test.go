package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ★ 이 테스트가 이 파일의 존재 이유다. AI 호출은 돈이 든다.
func TestRateLimiterStopsAtLimit(t *testing.T) {
	l := newRateLimiter(3)

	for i := 0; i < 3; i++ {
		if !l.allow("1.1.1.1") {
			t.Fatalf("%d번째 요청이 막혔다. 상한 안에서는 통과해야 한다", i+1)
		}
	}
	if l.allow("1.1.1.1") {
		t.Error("상한을 넘겼는데 통과했다")
	}
	// 다른 사람은 영향을 받지 않는다
	if !l.allow("2.2.2.2") {
		t.Error("다른 요청자가 남의 상한에 걸렸다")
	}
}

func TestRateLimiterRefillsAfterWindow(t *testing.T) {
	now := time.Now()
	l := newRateLimiter(1)
	l.now = func() time.Time { return now }

	if !l.allow("1.1.1.1") || l.allow("1.1.1.1") {
		t.Fatal("1회 상한이 지켜지지 않았다")
	}

	now = now.Add(61 * time.Second)
	if !l.allow("1.1.1.1") {
		t.Error("1분이 지나면 다시 통과해야 한다")
	}
}

func TestRateLimiterZeroMeansUnlimited(t *testing.T) {
	l := newRateLimiter(0)
	for i := 0; i < 100; i++ {
		if !l.allow("1.1.1.1") {
			t.Fatalf("%d번째에서 막혔다. 0 은 무제한이어야 한다", i+1)
		}
	}
}

// 프록시 뒤에서 모두가 한 사람으로 묶이면, 한 명이 다 쓰면 전부 막힌다.
func TestClientIPUsesForwardedFor(t *testing.T) {
	tests := []struct {
		name, xff, remote, want string
	}{
		{"프록시 뒤 (여러 단계)", "203.0.113.9, 10.0.0.1", "10.0.0.1:443", "203.0.113.9"},
		{"프록시 한 단계", "203.0.113.9", "10.0.0.1:443", "203.0.113.9"},
		{"직접 연결", "", "203.0.113.9:54321", "203.0.113.9"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/api/extract", nil)
			r.RemoteAddr = tt.remote
			if tt.xff != "" {
				r.Header.Set("X-Forwarded-For", tt.xff)
			}
			if got := clientIP(r); got != tt.want {
				t.Errorf("clientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}

// 막을 때도 같은 에러 형식이어야 한다. 프론트가 한 가지 모양만 안다.
func TestLimitAIRespondsWith429(t *testing.T) {
	a := &API{}
	l := newRateLimiter(1)
	h := a.limitAI(l, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/extract", nil)
		r.RemoteAddr = "1.1.1.1:1234"
		h(rec, r)

		if i == 0 && rec.Code != http.StatusOK {
			t.Fatalf("첫 요청 = %d, want 200", rec.Code)
		}
		if i == 1 {
			if rec.Code != http.StatusTooManyRequests {
				t.Errorf("두 번째 요청 = %d, want 429", rec.Code)
			}
			if body := rec.Body.String(); !strings.Contains(body, CodeTooManyRequests) {
				t.Errorf("에러 코드가 없다: %s", body)
			}
		}
	}
}
