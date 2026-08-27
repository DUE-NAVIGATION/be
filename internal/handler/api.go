package handler

import (
	"net/http"

	"github.com/DUE-NAVIGATION/be/internal/ai"
	"github.com/DUE-NAVIGATION/be/internal/income"
	"github.com/DUE-NAVIGATION/be/internal/loader"
)

// API 는 HTTP 계층이 판정에 쓰는 것들을 들고 있다.
//
// ★ 상태를 여기 담지 않는다. Programs 는 읽기 전용이고,
// Income 은 기준중위소득 표를 들고 있는 값이다.
// 사용자 입력은 요청이 살아 있는 동안만 스택 위에 존재한다 (설계 원칙 2).
type API struct {
	Programs *loader.Store
	Income   income.Calculator
	// AI 는 없어도 서버가 뜬다. 없으면 해당 엔드포인트만 503 을 돌려주고,
	// 프론트는 수동 입력으로 넘어간다. 판정은 AI 없이도 완전히 동작한다
	AI *ai.Client
}

// Routes 는 전체 라우팅과 공통 처리를 조립해 돌려준다.
//
// 공통 처리는 바깥부터 안쪽 순서로 적용된다.
//
//	recoverPanic → logRequests → limitBody → 라우터 → 핸들러
//
// panic 복구를 가장 바깥에 두는 이유: 로깅이나 본문 제한에서 문제가 생겨도
// 서버가 죽지 않아야 한다. 발표 중 500 은 참을 수 있지만 프로세스 종료는 안 된다.
func (a *API) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", a.health)
	mux.HandleFunc("GET /api/programs", a.programs)
	mux.HandleFunc("POST /api/evaluate", a.evaluate)

	mux.HandleFunc("POST /api/extract", a.extract)
	mux.HandleFunc("POST /api/explain", a.explain)

	// Phase 6 에서 붙는다. 경로와 에러 형식만 확정해 둔다.
	mux.HandleFunc("POST /api/document", notImplemented("문서 번역"))

	// 등록되지 않은 경로도 같은 에러 형식으로 답한다.
	// 프론트가 오타를 냈을 때 HTML 404 를 받으면 원인을 찾기 어렵다.
	mux.HandleFunc("/", a.notFound)

	return recoverPanic(logRequests(limitBody(mux)))
}

func (a *API) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"service": "due-api",
		// 설계 원칙 2 — 아무것도 저장하지 않는다
		"storesUserData":   false,
		"programCount":     a.Programs.Count(),
		"medianIncomeYear": a.Income.Table.Year,
		// 프론트가 대화형 입력을 띄울지 수동 입력 폼을 띄울지 판단한다
		"aiEnabled": a.AI.Enabled(),
	})
}

func (a *API) notFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, CodeNotFound,
		"그런 경로가 없습니다: "+r.Method+" "+r.URL.Path)
}

func notImplemented(what string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotImplemented, CodeNotImplemented,
			what+" 기능은 아직 준비 중입니다 (Phase 4)")
	}
}
