// Command server 는 DUE 백엔드 HTTP 서버다.
//
// 시작할 때 기준중위소득 표와 제도 데이터를 메모리로 읽고, 그 뒤로는 파일을 보지 않는다.
// 사용자 입력은 요청이 살아 있는 동안만 존재한다 (설계 원칙 2).
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/DUE-NAVIGATION/be/internal/ai"
	"github.com/DUE-NAVIGATION/be/internal/handler"
	"github.com/DUE-NAVIGATION/be/internal/income"
	"github.com/DUE-NAVIGATION/be/internal/loader"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	dataDir := env("DATA_DIR", "data")
	addr := ":" + env("PORT", "8080")
	origins := parseOrigins(env("CORS_ALLOWED_ORIGINS", "http://localhost:3000"))

	// ── 기준중위소득 표 ──────────────────────────────────────
	// 이 표가 없으면 소득 판정 자체가 불가능하다. 없으면 뜨지 않는 게 맞다.
	table, err := income.LoadTable(filepath.Join(dataDir, "median-income.json"))
	if err != nil {
		slog.Error("기준중위소득 표를 읽지 못했습니다", "err", err)
		os.Exit(1)
	}

	// ── 제도 데이터 ─────────────────────────────────────────
	// 제도는 하나가 깨져도 나머지로 동작해야 한다. 경고만 남기고 계속 간다.
	store, err := loader.New(filepath.Join(dataDir, "programs"))
	if err != nil {
		slog.Error("제도 디렉터리를 읽지 못했습니다", "err", err)
		os.Exit(1)
	}
	for _, p := range store.Problems() {
		slog.Warn("제도를 건너뛰었습니다", "file", p.File, "reason", p.Reason)
	}
	if store.Count() == 0 {
		slog.Warn("읽힌 제도가 없습니다. /api/evaluate 가 동작하지 않습니다",
			"guide", "data/programs/README.md")
	}

	// ── AI 게이트웨이 ───────────────────────────────────────
	// ★ 키가 없어도 서버는 뜬다. 판정은 AI 없이 완전히 동작하고,
	// 대화형 입력만 막힌다. 데모 중 키가 만료돼도 결과 화면은 살아 있어야 한다.
	demoMode := env("DEMO_MODE", "false") == "true"

	// 데모 모드면 미리 뽑아둔 응답을 쓴다.
	// ★ 이게 있으면 API 키도 네트워크도 없이 데모가 완주된다.
	var demoCache ai.Cache
	if demoMode {
		fc, err := ai.LoadCache(filepath.Join(dataDir, "demo-cache.json"))
		if err != nil {
			// 뜨지 못할 이유는 아니지만, 발표 당일에야 알면 늦는다. 크게 남긴다
			slog.Error("DEMO_MODE 인데 데모 캐시를 쓸 수 없습니다", "err", err)
		} else {
			demoCache = fc
			for _, op := range fc.Ops() {
				slog.Info("데모 캐시", "op", op,
					"항목", fc.Count(op), "기본값", fc.HasDefault(op))
			}
		}
	}

	aiClient := ai.New(ai.Config{
		APIKey:   os.Getenv("ANTHROPIC_API_KEY"), // ★ 코드에 넣지 않는다
		Model:    env("ANTHROPIC_MODEL", ""),
		Timeout:  envDuration("AI_TIMEOUT_SECONDS", 8*time.Second),
		DemoMode: demoMode,
		Cache:    demoCache,
	})
	switch {
	case demoCache != nil:
		slog.Info("데모 모드입니다. AI 응답은 캐시에서 나갑니다 (API 호출 없음)")
	case !aiClient.Enabled():
		slog.Warn("ANTHROPIC_API_KEY 가 없습니다. /api/extract 와 /api/explain 은 503 을 돌려줍니다",
			"대안", "DEMO_MODE=true 로 켜면 캐시로 동작합니다",
			"영향", "판정(/api/evaluate)은 정상 동작합니다")
	}

	api := &handler.API{
		Programs: store,
		Income:   income.Calculator{Table: table},
		AI:       aiClient,
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           withCORS(origins, api.Routes()),
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
			"programs", store.Count(),
			"aiEnabled", aiClient.Enabled(),
			"medianIncomeYear", table.Year,
			"demoMode", demoMode,
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

// envDuration 은 초 단위 환경변수를 읽는다. 잘못된 값이면 기본값을 쓴다.
func envDuration(key string, fallback time.Duration) time.Duration {
	n, err := strconv.Atoi(os.Getenv(key))
	if err != nil || n <= 0 {
		return fallback
	}
	return time.Duration(n) * time.Second
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
