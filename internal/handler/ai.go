package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/DUE-NAVIGATION/be/internal/ai"
	"github.com/DUE-NAVIGATION/be/internal/model"
)

// 자유 입력 상한. 상담 문장이 이보다 길 이유가 없고,
// 길수록 LLM 으로 나가는 양이 늘어난다.
const maxExtractTextRunes = 2000

// ── POST /api/extract ───────────────────────────────────────

type ExtractRequest struct {
	Text string `json:"text"`
}

type ExtractResponse struct {
	Extracted         model.UserContext           `json:"extracted"`
	Confidence        map[string]model.Confidence `json:"confidence"`
	FollowUpQuestions []string                    `json:"followUpQuestions"`
	// 전송 전에 가린 민감정보의 종류·건수. ★ 값은 들어 있지 않다.
	// 화면에 "주민등록번호는 보내지 않았습니다" 를 띄우는 데 쓴다
	Sanitized  map[ai.Kind]int `json:"sanitized,omitempty"`
	Disclaimer string          `json:"disclaimer"`
}

// extract 는 자연어를 판정 입력값으로 옮긴다.
//
// ★ 여기서 판정하지 않는다. 결과를 그대로 /api/evaluate 에 넣으면 판정이 된다.
func (a *API) extract(w http.ResponseWriter, r *http.Request) {
	var req ExtractRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	text := strings.TrimSpace(req.Text)
	if text == "" {
		writeError(w, http.StatusBadRequest, CodeInvalidRequest,
			"text 가 비어 있습니다")
		return
	}
	if len([]rune(text)) > maxExtractTextRunes {
		writeError(w, http.StatusBadRequest, CodeInvalidRequest,
			"입력이 너무 깁니다 (최대 2000자)")
		return
	}
	if !a.AI.Enabled() {
		aiUnavailable(w, "자연어 구조화")
		return
	}

	out, err := a.AI.Extract(r.Context(), text)
	if err != nil {
		writeAIError(w, err, "입력을 이해하지 못했습니다. 직접 입력해 주세요.")
		return
	}

	writeJSON(w, http.StatusOK, ExtractResponse{
		Extracted:         out.Extracted,
		Confidence:        out.Confidence,
		FollowUpQuestions: out.FollowUpQuestions,
		Sanitized:         out.Sanitized,
		Disclaimer:        model.Disclaimer,
	})
}

// ── POST /api/explain ───────────────────────────────────────

type ExplainRequest struct {
	Results []model.MatchResult `json:"results"`
	Summary model.Summary       `json:"summary"`
}

type ExplainResponse struct {
	Explanation string `json:"explanation"`
	Disclaimer  string `json:"disclaimer"`
}

// explain 은 이미 나온 판정 결과를 사람 말로 푼다.
//
// ★ 판정을 다시 하지 않는다. 받은 결과를 그대로 설명할 뿐이다.
func (a *API) explain(w http.ResponseWriter, r *http.Request) {
	var req ExplainRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if len(req.Results) == 0 {
		writeError(w, http.StatusBadRequest, CodeInvalidRequest,
			"results 가 비어 있습니다")
		return
	}
	if !a.AI.Enabled() {
		aiUnavailable(w, "결과 설명 생성")
		return
	}

	text, err := a.AI.Explain(r.Context(), req.Results, req.Summary)
	if err != nil {
		writeAIError(w, err, "설명을 만들지 못했습니다. 조건별 판정 결과를 확인해 주세요.")
		return
	}

	writeJSON(w, http.StatusOK, ExplainResponse{
		Explanation: text,
		Disclaimer:  model.Disclaimer,
	})
}

// ── 공통 ────────────────────────────────────────────────────

// aiUnavailable 은 AI 를 아예 쓸 수 없는 상태다 (키 없음).
// 501(안 만들었음)과 구분한다 — 프론트가 수동 입력으로 폴백해야 하기 때문이다.
func aiUnavailable(w http.ResponseWriter, what string) {
	writeError(w, http.StatusServiceUnavailable, CodeAIUnavailable,
		what+" 기능을 쓸 수 없습니다. 직접 입력해 주세요.")
}

// writeAIError 는 AI 호출 실패를 프론트가 처리할 수 있는 형태로 바꾼다.
//
// ★ 원인 문자열을 그대로 내보내지 않는다. 요청 내용이 섞여 있을 수 있다.
func writeAIError(w http.ResponseWriter, err error, message string) {
	switch {
	case errors.Is(err, ai.ErrNoAPIKey):
		writeError(w, http.StatusServiceUnavailable, CodeAIUnavailable, message)
	default:
		// 시간 초과·형식 오류 모두 사용자 입장에서는 같다 — 직접 입력으로 넘어간다
		writeError(w, http.StatusBadGateway, CodeAIFailed, message)
	}
}
