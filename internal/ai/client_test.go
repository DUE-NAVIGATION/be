package ai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DUE-NAVIGATION/be/internal/model"
)

// fakeAnthropic 은 도구 호출 응답을 흉내내는 가짜 서버다.
// 실제 API 키 없이 전 구간을 검증하기 위한 것.
type fakeAnthropic struct {
	*httptest.Server
	calls atomic.Int32
	// 마지막 요청 본문. "무엇이 밖으로 나갔는지" 를 검사하는 데 쓴다
	lastBody atomic.Value
}

func newFake(t *testing.T, handler func(w http.ResponseWriter, body []byte, call int)) *fakeAnthropic {
	t.Helper()
	f := &fakeAnthropic{}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.lastBody.Store(string(body))
		n := int(f.calls.Add(1))

		if got := r.Header.Get("x-api-key"); got == "" {
			t.Error("x-api-key 헤더가 없다")
		}
		if got := r.Header.Get("anthropic-version"); got == "" {
			t.Error("anthropic-version 헤더가 없다")
		}
		handler(w, body, n)
	}))
	t.Cleanup(f.Close)
	return f
}

func (f *fakeAnthropic) body() string {
	v, _ := f.lastBody.Load().(string)
	return v
}

// toolUse 는 도구 호출 응답을 만든다.
func toolUse(name, input string) string {
	return `{"content":[{"type":"tool_use","name":"` + name + `","input":` + input + `}],"stop_reason":"tool_use"}`
}

func testClient(t *testing.T, f *fakeAnthropic) *Client {
	t.Helper()
	return New(Config{
		APIKey:  "test-key",
		BaseURL: f.URL,
		Timeout: 2 * time.Second,
	})
}

// ── Extract ─────────────────────────────────────────────────

func TestExtractDemoScenario(t *testing.T) {
	f := newFake(t, func(w http.ResponseWriter, _ []byte, _ int) {
		w.Write([]byte(toolUse(extractToolName, `{
			"extracted": {
				"householdSize": 2,
				"isSingleParent": true,
				"childrenAges": [7],
				"employmentStatus": "LOST_JOB",
				"housingType": "MONTHLY_RENT"
			},
			"confidence": { "householdSize": "MEDIUM", "isSingleParent": "HIGH" },
			"followUpQuestions": ["월 소득이 어느 정도인가요?"]
		}`)))
	})

	got, err := testClient(t, f).Extract(context.Background(),
		"혼자 애 키우는데 일이 끊겼어요. 아이는 7살이고 월세 살아요.")
	if err != nil {
		t.Fatalf("Extract 실패: %v", err)
	}

	if got.Extracted.HouseholdSize == nil || *got.Extracted.HouseholdSize != 2 {
		t.Errorf("householdSize = %v", got.Extracted.HouseholdSize)
	}
	if got.Extracted.IsSingleParent == nil || !*got.Extracted.IsSingleParent {
		t.Error("isSingleParent 가 반영되지 않았다")
	}
	if got.Confidence["isSingleParent"] != model.ConfidenceHigh {
		t.Errorf("confidence = %v", got.Confidence)
	}
	if len(got.FollowUpQuestions) != 1 {
		t.Errorf("되묻기 = %v", got.FollowUpQuestions)
	}
}

// ★ 시크릿 필터를 반드시 거친다. 원문이 밖으로 나가면 안 된다.
func TestExtractSanitizesBeforeSending(t *testing.T) {
	f := newFake(t, func(w http.ResponseWriter, _ []byte, _ int) {
		w.Write([]byte(toolUse(extractToolName,
			`{"extracted":{},"confidence":{},"followUpQuestions":[]}`)))
	})

	got, err := testClient(t, f).Extract(context.Background(),
		"저는 900101-1234567 이고 010-1234-5678 입니다. 월세 살아요.")
	if err != nil {
		t.Fatal(err)
	}

	sent := f.body()
	for _, secret := range []string{"900101-1234567", "010-1234-5678"} {
		if strings.Contains(sent, secret) {
			t.Errorf("★ %q 가 API 로 나갔다", secret)
		}
	}
	if !strings.Contains(sent, "월세") {
		t.Error("정상 문장이 함께 사라졌다")
	}
	// 무엇을 가렸는지는 알려줘야 화면에 표시할 수 있다
	if got.Sanitized[KindResidentID] != 1 || got.Sanitized[KindPhone] != 1 {
		t.Errorf("가린 내역 = %v", got.Sanitized)
	}
}

// ★ AI 가 파생값을 채워도 무시한다. 판정 근거가 오염되면 안 된다.
func TestExtractRejectsDerivedValue(t *testing.T) {
	f := newFake(t, func(w http.ResponseWriter, _ []byte, _ int) {
		w.Write([]byte(toolUse(extractToolName, `{
			"extracted": { "householdIncomePct": 45.0, "age": 29 },
			"confidence": {}, "followUpQuestions": []
		}`)))
	})

	got, err := testClient(t, f).Extract(context.Background(), "스물아홉이에요")
	if err != nil {
		t.Fatal(err)
	}
	if got.Extracted.HouseholdIncomePct != nil {
		t.Errorf("★ AI 가 채운 파생값이 살아남았다: %v", *got.Extracted.HouseholdIncomePct)
	}
	if got.Extracted.Age == nil || *got.Extracted.Age != 29 {
		t.Error("정상 항목까지 버려졌다")
	}
}

// 잘못된 enum 값은 버린다. 비워두면 "확인 필요" 가 되어 안전하다.
func TestExtractDropsInvalidEnum(t *testing.T) {
	f := newFake(t, func(w http.ResponseWriter, _ []byte, _ int) {
		w.Write([]byte(toolUse(extractToolName, `{
			"extracted": { "housingType": "고시원", "employmentStatus": "백수", "age": 33 },
			"confidence": {}, "followUpQuestions": []
		}`)))
	})

	got, err := testClient(t, f).Extract(context.Background(), "고시원 살아요")
	if err != nil {
		t.Fatal(err)
	}
	if got.Extracted.HousingType != nil {
		t.Errorf("모르는 주거형태가 살아남았다: %v", *got.Extracted.HousingType)
	}
	if got.Extracted.EmploymentStatus != nil {
		t.Errorf("모르는 취업상태가 살아남았다: %v", *got.Extracted.EmploymentStatus)
	}
	if got.Extracted.Age == nil {
		t.Error("정상 항목까지 버려졌다")
	}
}

// 되묻기는 3개를 넘지 않는다. 한 번에 많이 물으면 사용자가 지친다.
func TestExtractLimitsFollowUps(t *testing.T) {
	f := newFake(t, func(w http.ResponseWriter, _ []byte, _ int) {
		w.Write([]byte(toolUse(extractToolName, `{
			"extracted": {}, "confidence": {},
			"followUpQuestions": ["1?","2?","3?","4?","5?"]
		}`)))
	})

	got, _ := testClient(t, f).Extract(context.Background(), "안녕하세요")
	if len(got.FollowUpQuestions) != 3 {
		t.Errorf("되묻기 %d개, 최대 3개여야 한다", len(got.FollowUpQuestions))
	}
}

// 첫 호출이 실패하면 한 번 더 시도한다.
func TestRetriesOnce(t *testing.T) {
	f := newFake(t, func(w http.ResponseWriter, _ []byte, call int) {
		if call == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"type":"error","error":{"type":"api_error","message":"일시 오류"}}`))
			return
		}
		w.Write([]byte(toolUse(extractToolName,
			`{"extracted":{"age":30},"confidence":{},"followUpQuestions":[]}`)))
	})

	got, err := testClient(t, f).Extract(context.Background(), "서른입니다")
	if err != nil {
		t.Fatalf("재시도로 성공해야 한다: %v", err)
	}
	if got.Extracted.Age == nil || *got.Extracted.Age != 30 {
		t.Error("재시도 응답이 반영되지 않았다")
	}
	if n := f.calls.Load(); n != 2 {
		t.Errorf("호출 %d회, 2회여야 한다", n)
	}
}

// 두 번 다 실패하면 에러를 올린다. 프론트가 수동 입력으로 폴백한다.
func TestFailsAfterRetry(t *testing.T) {
	f := newFake(t, func(w http.ResponseWriter, _ []byte, _ int) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"type":"error","error":{"type":"overloaded_error","message":"과부하"}}`))
	})

	_, err := testClient(t, f).Extract(context.Background(), "안녕하세요")
	if !errors.Is(err, ErrUnavailable) {
		t.Errorf("err = %v, ErrUnavailable 이어야 한다", err)
	}
	if n := f.calls.Load(); n != 2 {
		t.Errorf("호출 %d회, 2회여야 한다 (무한 재시도 금지)", n)
	}
}

// 키가 없으면 호출조차 하지 않는다.
func TestNoAPIKey(t *testing.T) {
	c := New(Config{BaseURL: "http://127.0.0.1:1"})
	if c.Enabled() {
		t.Error("키가 없는데 Enabled 가 true 다")
	}
	if _, err := c.Extract(context.Background(), "안녕"); !errors.Is(err, ErrNoAPIKey) {
		t.Errorf("err = %v, ErrNoAPIKey 여야 한다", err)
	}
}

// 시간이 지나면 포기한다. 데모에서 멈춰 있으면 안 된다.
func TestTimeout(t *testing.T) {
	f := newFake(t, func(w http.ResponseWriter, _ []byte, _ int) {
		time.Sleep(300 * time.Millisecond)
		w.Write([]byte(toolUse(extractToolName,
			`{"extracted":{},"confidence":{},"followUpQuestions":[]}`)))
	})
	c := New(Config{APIKey: "k", BaseURL: f.URL, Timeout: 50 * time.Millisecond})

	if _, err := c.Extract(context.Background(), "안녕"); err == nil {
		t.Error("시간 초과인데 성공했다")
	}
}

// 호출을 강제로 취소하면 재시도하지 않는다.
func TestContextCancelStopsRetry(t *testing.T) {
	f := newFake(t, func(w http.ResponseWriter, _ []byte, _ int) {
		time.Sleep(200 * time.Millisecond)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	c := New(Config{APIKey: "k", BaseURL: f.URL, Timeout: 5 * time.Second})
	if _, err := c.Extract(ctx, "안녕"); err == nil {
		t.Error("취소됐는데 성공했다")
	}
	if n := f.calls.Load(); n > 1 {
		t.Errorf("취소 후에도 %d회 호출했다", n)
	}
}

// 도구 사용을 강제해야 형식이 흔들리지 않는다.
func TestForcesToolUse(t *testing.T) {
	f := newFake(t, func(w http.ResponseWriter, _ []byte, _ int) {
		w.Write([]byte(toolUse(extractToolName,
			`{"extracted":{},"confidence":{},"followUpQuestions":[]}`)))
	})
	_, _ = testClient(t, f).Extract(context.Background(), "안녕하세요")

	var req map[string]any
	if err := json.Unmarshal([]byte(f.body()), &req); err != nil {
		t.Fatalf("요청을 파싱하지 못했다: %v", err)
	}
	choice, ok := req["tool_choice"].(map[string]any)
	if !ok || choice["type"] != "tool" || choice["name"] != extractToolName {
		t.Errorf("tool_choice = %v, 도구를 강제해야 한다", req["tool_choice"])
	}
	if req["temperature"] != float64(0) {
		t.Errorf("temperature = %v, 구조화는 0 이어야 한다", req["temperature"])
	}
}

// ── Explain ─────────────────────────────────────────────────

func explainResults() ([]model.MatchResult, model.Summary) {
	results := []model.MatchResult{
		{
			Program:         model.Program{ID: "a", Name: "한부모가족 아동양육비"},
			Status:          model.MatchEligible,
			EstimatedAmount: 2760000,
			Conditions:      []model.ConditionResult{},
		},
		{
			Program: model.Program{ID: "b", Name: "청년월세 한시 특별지원"},
			Status:  model.MatchNeedsInfo,
			Conditions: []model.ConditionResult{{
				Condition: model.Condition{Field: "receivingPrograms", Label: "주거급여를 받고 있지 않을 것"},
				Status:    model.StatusUnknown,
				Reason:    "확인 필요",
			}},
		},
	}
	return results, model.Summary{EligibleCount: 1, NeedsInfoCount: 1, TotalAnnualAmount: 2760000}
}

func TestExplain(t *testing.T) {
	f := newFake(t, func(w http.ResponseWriter, _ []byte, _ int) {
		w.Write([]byte(toolUse(explainToolName,
			`{"explanation":"한부모가족 아동양육비를 받으실 수 있을 것으로 보입니다."}`)))
	})

	results, summary := explainResults()
	got, err := testClient(t, f).Explain(context.Background(), results, summary)
	if err != nil {
		t.Fatalf("Explain 실패: %v", err)
	}
	if got == "" {
		t.Error("설명문이 비었다")
	}

	sent := f.body()
	// 제도 이름과 금액은 들어가야 한다 — 그래야 모델이 지어내지 않는다
	for _, want := range []string{"한부모가족 아동양육비", "2760000", "주거급여를 받고 있지 않을 것"} {
		if !strings.Contains(sent, want) {
			t.Errorf("요청에 %q 가 없다", want)
		}
	}
}

// ★ 설명 단계에는 사용자의 상황을 보내지 않는다.
// 설명에 필요한 것은 "무엇이 어떻게 판정됐는가" 뿐이다.
func TestExplainDoesNotSendUserContext(t *testing.T) {
	f := newFake(t, func(w http.ResponseWriter, _ []byte, _ int) {
		w.Write([]byte(toolUse(explainToolName, `{"explanation":"설명입니다."}`)))
	})

	results, summary := explainResults()
	_, _ = testClient(t, f).Explain(context.Background(), results, summary)

	sent := f.body()
	for _, leak := range []string{"incomeMonthly", "householdSize", "childrenAges", "isSingleParent"} {
		if strings.Contains(sent, leak) {
			t.Errorf("★ 사용자 상황 항목 %q 가 설명 요청에 섞였다", leak)
		}
	}
}

func TestExplainRejectsEmpty(t *testing.T) {
	f := newFake(t, func(w http.ResponseWriter, _ []byte, _ int) {
		w.Write([]byte(toolUse(explainToolName, `{"explanation":"   "}`)))
	})

	results, summary := explainResults()
	if _, err := testClient(t, f).Explain(context.Background(), results, summary); err == nil {
		t.Error("빈 설명문을 받아들였다")
	}
}

// ── 프롬프트 자체를 고정한다 ────────────────────────────────
//
// 프롬프트는 코드 리뷰 대상이다. 핵심 금지문이 실수로 빠지면 여기서 잡힌다.
func TestPromptsKeepTheirGuardrails(t *testing.T) {
	tests := []struct {
		name   string
		prompt string
		musts  []string
	}{
		{"구조화", extractSystem, []string{
			"자격을 판정하지 마세요",
			"제도 이름을 언급하지 마세요",
			"추측해서 채우지 마세요",
			"값을 계산하지 마세요",
		}},
		{"설명", explainSystem, []string{
			"판정을 뒤집지 마세요",
			"주어진 목록에 없는 제도를 언급하지 마세요",
			"금액을 새로 계산하거나 바꾸지 마세요",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, must := range tt.musts {
				if !strings.Contains(tt.prompt, must) {
					t.Errorf("★ 프롬프트에서 %q 가 사라졌다", must)
				}
			}
		})
	}
}

// 데모 캐시가 있으면 API 를 부르지 않는다 (Phase 7 대비).
type stubCache map[string]json.RawMessage

func (c stubCache) Lookup(op, key string) (json.RawMessage, bool) {
	v, ok := c[op+"|"+key]
	return v, ok
}

func TestDemoCacheSkipsAPI(t *testing.T) {
	f := newFake(t, func(w http.ResponseWriter, _ []byte, _ int) {
		t.Error("★ 캐시가 있는데 API 를 호출했다")
	})

	c := New(Config{
		APIKey:   "k",
		BaseURL:  f.URL,
		DemoMode: true,
		Cache: stubCache{
			"extract|월세 살아요": json.RawMessage(
				`{"extracted":{"housingType":"MONTHLY_RENT"},"confidence":{},"followUpQuestions":[]}`),
		},
	})

	got, err := c.Extract(context.Background(), "월세 살아요")
	if err != nil {
		t.Fatal(err)
	}
	if got.Extracted.HousingType == nil || *got.Extracted.HousingType != model.HousingMonthlyRent {
		t.Errorf("캐시 응답이 반영되지 않았다: %+v", got.Extracted)
	}
	if n := f.calls.Load(); n != 0 {
		t.Errorf("API 를 %d회 호출했다", n)
	}
}
