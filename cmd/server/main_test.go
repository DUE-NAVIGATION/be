package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseOrigins(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{"단일", "http://localhost:3000", []string{"http://localhost:3000"}},
		{"공백 포함", "http://a.com , http://b.com", []string{"http://a.com", "http://b.com"}},
		{"빈 항목 제거", "http://a.com,,", []string{"http://a.com"}},
		{"끝 슬래시 제거", "http://a.com/", []string{"http://a.com"}},
		{"전부 비었음", " , ", []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseOrigins(tt.raw)
			if len(got) != len(tt.want) {
				t.Fatalf("길이가 다르다: got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("[%d] got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestOriginAllowedIsExactMatch(t *testing.T) {
	allowed := []string{"http://localhost:3000"}

	tests := []struct {
		origin string
		want   bool
	}{
		{"http://localhost:3000", true},
		// 접두사 비교를 쓰면 통과해버리는 값들. 반드시 막혀야 한다
		{"http://localhost:30000", false},
		{"http://localhost:3000.evil.com", false},
		{"https://localhost:3000", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := originAllowed(allowed, tt.origin); got != tt.want {
			t.Errorf("originAllowed(%q) = %v, want %v", tt.origin, got, tt.want)
		}
	}
}

func TestWithCORS(t *testing.T) {
	allowed := []string{"http://localhost:3000"}
	handler := withCORS(allowed, http.HandlerFunc(healthz))

	t.Run("허용된 오리진에는 헤더를 붙인다", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
			t.Errorf("Allow-Origin = %q", got)
		}
		if got := rec.Header().Get("Vary"); got != "Origin" {
			t.Errorf("Vary = %q, 캐시 오염을 막으려면 Origin 이 있어야 한다", got)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("code = %d, want 200", rec.Code)
		}
	})

	t.Run("허용되지 않은 오리진에는 헤더를 붙이지 않는다", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		req.Header.Set("Origin", "http://evil.com")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("Allow-Origin = %q, 비어 있어야 한다", got)
		}
		// 헤더가 없으면 브라우저가 응답을 읽지 못한다. 서버가 막을 필요는 없다
		if rec.Code != http.StatusOK {
			t.Errorf("code = %d, want 200", rec.Code)
		}
	})

	t.Run("프리플라이트는 204 로 끝낸다", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/api/evaluate", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		req.Header.Set("Access-Control-Request-Method", "POST")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Errorf("code = %d, want 204", rec.Code)
		}
		if got := rec.Header().Get("Access-Control-Allow-Methods"); got == "" {
			t.Error("Allow-Methods 가 비었다")
		}
		if got := rec.Header().Get("Access-Control-Allow-Headers"); got == "" {
			t.Error("Allow-Headers 가 비었다. Content-Type 을 못 보낸다")
		}
	})

	t.Run("자격증명은 허용하지 않는다", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		// 로그인이 없다. 쿠키를 허용할 이유가 없고, 켜는 순간 위험만 는다
		if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "" {
			t.Errorf("Allow-Credentials = %q, 설정하면 안 된다", got)
		}
	})
}

func TestHealthzSaysItStoresNothing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	healthz(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	// 설계 원칙 2. 프론트의 BackendStatus 가 service 를 읽는다
	for _, want := range []string{`"service":"due-api"`, `"storesUserData":false`} {
		if !contains(body, want) {
			t.Errorf("응답에 %s 가 없다: %s", want, body)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
