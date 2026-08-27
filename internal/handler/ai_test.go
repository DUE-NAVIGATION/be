package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DUE-NAVIGATION/be/internal/ai"
	"github.com/DUE-NAVIGATION/be/internal/income"
	"github.com/DUE-NAVIGATION/be/internal/loader"
	"github.com/DUE-NAVIGATION/be/internal/model"
)

// newTestAPIWithAI 는 가짜 Claude 서버를 붙인 API 를 세운다.
// 실제 키 없이 /api/extract · /api/explain 전 구간을 검증한다.
func newTestAPIWithAI(t *testing.T, respond func(w http.ResponseWriter, body string)) (http.Handler, func() string) {
	t.Helper()

	var lastBody string
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(b)
		lastBody = string(b)
		respond(w, lastBody)
	}))
	t.Cleanup(fake.Close)

	dataDir := filepath.Join("..", "..", "data")
	table, err := income.LoadTable(filepath.Join(dataDir, "median-income.json"))
	if err != nil {
		t.Fatal(err)
	}
	store, err := loader.New(filepath.Join(dataDir, "programs"))
	if err != nil {
		t.Fatal(err)
	}

	api := &API{
		Programs: store,
		Income:   income.Calculator{Table: table},
		AI:       ai.New(ai.Config{APIKey: "test-key", BaseURL: fake.URL}),
	}
	return api.Routes(), func() string { return lastBody }
}

func toolUse(name, input string) string {
	return `{"content":[{"type":"tool_use","name":"` + name + `","input":` + input + `}],"stop_reason":"tool_use"}`
}

// ── /api/extract ────────────────────────────────────────────

func TestExtractEndpoint(t *testing.T) {
	h, sent := newTestAPIWithAI(t, func(w http.ResponseWriter, _ string) {
		w.Write([]byte(toolUse("record_context", `{
			"extracted": {"householdSize":2,"isSingleParent":true,"childrenAges":[7],
			              "employmentStatus":"LOST_JOB","housingType":"MONTHLY_RENT"},
			"confidence": {"isSingleParent":"HIGH"},
			"followUpQuestions": ["월 소득이 어느 정도인가요?"]
		}`)))
	})

	rec := do(t, h, http.MethodPost, "/api/extract",
		`{"text":"혼자 애 키우는데 일이 끊겼어요. 아이는 7살이고 월세 살아요."}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d. body: %s", rec.Code, rec.Body.String())
	}

	var got ExtractResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("파싱 실패: %v", err)
	}
	if got.Extracted.HouseholdSize == nil || *got.Extracted.HouseholdSize != 2 {
		t.Errorf("householdSize = %v", got.Extracted.HouseholdSize)
	}
	if len(got.FollowUpQuestions) != 1 {
		t.Errorf("되묻기 = %v", got.FollowUpQuestions)
	}
	if got.Disclaimer != model.Disclaimer {
		t.Error("고지 문구가 빠졌다")
	}
	if sent() == "" {
		t.Error("AI 로 아무것도 나가지 않았다")
	}
}

// ★ 구조화 결과를 그대로 판정에 넣을 수 있어야 한다.
// 이 둘이 맞지 않으면 대화형 입력이 화면까지 이어지지 않는다.
func TestExtractOutputFeedsEvaluate(t *testing.T) {
	h, _ := newTestAPIWithAI(t, func(w http.ResponseWriter, _ string) {
		w.Write([]byte(toolUse("record_context", `{
			"extracted": {"householdSize":2,"age":33,"incomeMonthly":800000,
			              "isSingleParent":true,"childrenAges":[7],"housingType":"MONTHLY_RENT"},
			"confidence": {}, "followUpQuestions": []
		}`)))
	})

	rec := do(t, h, http.MethodPost, "/api/extract", `{"text":"상황 설명"}`)
	var ext ExtractResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &ext); err != nil {
		t.Fatal(err)
	}

	// 구조화 결과를 그대로 판정 요청으로 넘긴다
	ctxJSON, err := json.Marshal(map[string]any{"context": ext.Extracted})
	if err != nil {
		t.Fatal(err)
	}
	evalRec := do(t, h, http.MethodPost, "/api/evaluate", string(ctxJSON))
	if evalRec.Code != http.StatusOK {
		t.Fatalf("판정 실패 code=%d body=%s", evalRec.Code, evalRec.Body.String())
	}

	var out EvaluateResponse
	if err := json.Unmarshal(evalRec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Summary.EligibleCount == 0 {
		t.Error("구조화 → 판정 흐름에서 해당 제도가 하나도 안 나왔다")
	}
}

// ★ 민감정보는 AI 로 나가기 전에 걸러진다. 무엇을 걸렀는지는 응답에 남는다.
func TestExtractEndpointSanitizes(t *testing.T) {
	h, sent := newTestAPIWithAI(t, func(w http.ResponseWriter, _ string) {
		w.Write([]byte(toolUse("record_context",
			`{"extracted":{},"confidence":{},"followUpQuestions":[]}`)))
	})

	rec := do(t, h, http.MethodPost, "/api/extract",
		`{"text":"저는 900101-1234567 이고 010-1234-5678 입니다"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}

	if strings.Contains(sent(), "900101-1234567") {
		t.Error("★ 주민등록번호가 AI 로 나갔다")
	}

	var got ExtractResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.Sanitized[ai.KindResidentID] == 0 {
		t.Errorf("가린 내역이 응답에 없다: %v", got.Sanitized)
	}
	// ★ 응답에도 원문이 있으면 안 된다
	if strings.Contains(rec.Body.String(), "900101-1234567") {
		t.Error("★ 응답에 원문이 들어 있다")
	}
}

func TestExtractRejectsBadInput(t *testing.T) {
	h, _ := newTestAPIWithAI(t, func(w http.ResponseWriter, _ string) {
		t.Error("잘못된 입력인데 AI 를 호출했다")
	})

	tests := []struct {
		name string
		body string
	}{
		{"빈 문자열", `{"text":""}`},
		{"공백만", `{"text":"   "}`},
		{"너무 김", `{"text":"` + strings.Repeat("가", 2001) + `"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, h, http.MethodPost, "/api/extract", tt.body)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("code = %d, want 400", rec.Code)
			}
			if got := decodeErr(t, rec); got.Error.Code != CodeInvalidRequest {
				t.Errorf("code = %q", got.Error.Code)
			}
		})
	}
}

// AI 가 실패하면 프론트가 수동 입력으로 갈 수 있는 에러를 준다.
func TestExtractFailsGracefully(t *testing.T) {
	h, _ := newTestAPIWithAI(t, func(w http.ResponseWriter, _ string) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"type":"error","error":{"type":"overloaded_error","message":"과부하"}}`))
	})

	rec := do(t, h, http.MethodPost, "/api/extract", `{"text":"월세 살아요"}`)
	if rec.Code != http.StatusBadGateway {
		t.Errorf("code = %d, want 502", rec.Code)
	}
	got := decodeErr(t, rec)
	if got.Error.Code != CodeAIFailed {
		t.Errorf("code = %q, want %q", got.Error.Code, CodeAIFailed)
	}
	// ★ 내부 오류 문구를 그대로 흘리지 않는다
	if strings.Contains(got.Error.Message, "overloaded") {
		t.Errorf("내부 오류가 새어나갔다: %q", got.Error.Message)
	}
}

// ── /api/explain ────────────────────────────────────────────

func TestExplainEndpoint(t *testing.T) {
	h, sent := newTestAPIWithAI(t, func(w http.ResponseWriter, _ string) {
		w.Write([]byte(toolUse("write_explanation",
			`{"explanation":"한부모가족 아동양육비를 받으실 수 있을 것으로 보입니다."}`)))
	})

	// 실제 판정 결과를 받아 그대로 설명 요청으로 넘긴다
	evalRec := do(t, h, http.MethodPost, "/api/evaluate",
		`{"context":{"householdSize":2,"age":33,"incomeMonthly":800000,
		  "isSingleParent":true,"childrenAges":[7],"housingType":"MONTHLY_RENT"}}`)
	var ev EvaluateResponse
	if err := json.Unmarshal(evalRec.Body.Bytes(), &ev); err != nil {
		t.Fatal(err)
	}

	req, _ := json.Marshal(ExplainRequest{Results: ev.Results, Summary: ev.Summary})
	rec := do(t, h, http.MethodPost, "/api/explain", string(req))
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d. body: %s", rec.Code, rec.Body.String())
	}

	var got ExplainResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Explanation == "" {
		t.Error("설명문이 비었다")
	}
	if got.Disclaimer != model.Disclaimer {
		t.Error("고지 문구가 빠졌다")
	}

	// ★ 설명 단계에 사용자의 상황이 섞이면 안 된다
	for _, leak := range []string{"incomeMonthly", "childrenAges", "800000"} {
		if strings.Contains(sent(), leak) {
			t.Errorf("★ 사용자 상황 %q 가 설명 요청에 섞였다", leak)
		}
	}
}

func TestExplainRejectsEmptyResults(t *testing.T) {
	h, _ := newTestAPIWithAI(t, func(w http.ResponseWriter, _ string) {
		t.Error("빈 결과인데 AI 를 호출했다")
	})

	rec := do(t, h, http.MethodPost, "/api/explain", `{"results":[],"summary":{}}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", rec.Code)
	}
}

// AI 가 없어도 /healthz 가 그 사실을 알려줘야 한다.
// 프론트가 대화형 입력을 띄울지 수동 폼을 띄울지 여기서 판단한다.
func TestHealthzReportsAIState(t *testing.T) {
	if body := do(t, newTestAPI(t), http.MethodGet, "/healthz", "").Body.String(); !strings.Contains(body, `"aiEnabled":false`) {
		t.Errorf("키가 없는데 aiEnabled 가 false 가 아니다: %s", body)
	}

	h, _ := newTestAPIWithAI(t, func(w http.ResponseWriter, _ string) {})
	if body := do(t, h, http.MethodGet, "/healthz", "").Body.String(); !strings.Contains(body, `"aiEnabled":true`) {
		t.Errorf("키가 있는데 aiEnabled 가 true 가 아니다: %s", body)
	}
}
