// Command server 는 DUE 백엔드 HTTP 서버다.
//
// 지금은 /healthz 만 응답한다.
// Phase 5 에서 internal/handler 의 실제 엔드포인트가 붙는다.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	addr := ":" + env("PORT", "8080")
	origins := parseOrigins(env("CORS_ALLOWED_ORIGINS", "http://localhost:3000"))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz)

	srv := &http.Server{
		Addr:              addr,
		Handler:           withCORS(origins, mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       20 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// 종료 신호를 받으면 진행 중인 요청을 마치고 내려간다.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("서버 시작",
			"addr", addr,
			"demoMode", env("DEMO_MODE", "false"),
			"corsAllowedOrigins", strings.Join(origins, ","),
		)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("서버 종료", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("종료 신호 수신. 정리 중")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("정상 종료 실패", "err", err)
	}
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":  "ok",
		"service": "due-api",
		// 설계 원칙 2 — 아무것도 저장하지 않는다
		"storesUserData": false,
	})
}

// ── CORS ────────────────────────────────────────────────────

// withCORS 는 허용된 오리진의 브라우저 요청만 통과시킨다.
//
// ★ 라이브러리를 쓰지 않고 직접 쓴 이유
//
//	외부 의존성이 0개인 상태를 유지하면 `go run` 한 번으로 뜬다.
//	현장 와이파이에서 go mod download 가 실패해 데모가 막히는 상황을 만들지 않는다.
//	필요한 기능이 오리진 허용 + 프리플라이트뿐이라 rs/cors 는 과하다.
//
// ★ 자격증명(쿠키)을 허용하지 않는다. 로그인이 없으므로 필요 없고,
// Allow-Credentials 를 켜는 순간 오리진 검사 실수가 곧바로 취약점이 된다.
func withCORS(allowed []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// 오리진이 허용 목록에 있을 때만 헤더를 붙인다. 와일드카드를 쓰지 않는다.
		if origin != "" && originAllowed(allowed, origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			// 오리진마다 응답이 다르므로 캐시가 섞이지 않게 알린다.
			w.Header().Add("Vary", "Origin")
		}

		// 프리플라이트는 여기서 끝낸다. 핸들러까지 내려보내지 않는다.
		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			w.Header().Add("Vary", "Access-Control-Request-Method")
			w.Header().Add("Vary", "Access-Control-Request-Headers")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// parseOrigins 는 쉼표로 구분된 오리진 목록을 자른다. 빈 항목은 버린다.
func parseOrigins(raw string) []string {
	out := []string{}
	for _, o := range strings.Split(raw, ",") {
		if o = strings.TrimSpace(o); o != "" {
			out = append(out, strings.TrimSuffix(o, "/"))
		}
	}
	return out
}

// originAllowed 는 정확히 일치하는 오리진만 허용한다.
// 접두사 비교를 쓰지 않는다 — localhost:3000 이 localhost:30000 을 통과시킨다.
func originAllowed(allowed []string, origin string) bool {
	for _, a := range allowed {
		if a == origin {
			return true
		}
	}
	return false
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
