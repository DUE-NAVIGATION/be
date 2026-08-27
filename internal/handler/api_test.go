package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DUE-NAVIGATION/be/internal/income"
	"github.com/DUE-NAVIGATION/be/internal/loader"
	"github.com/DUE-NAVIGATION/be/internal/model"
)

// 실제 데이터로 서버를 세운다. 테스트용 가짜 제도를 만들지 않는 이유:
// 배포되는 제도 JSON 이 깨지면 여기서 잡혀야 한다.
func newTestAPI(t *testing.T) http.Handler {
	t.Helper()

	dataDir := filepath.Join("..", "..", "data")

	table, err := income.LoadTable(filepath.Join(dataDir, "median-income.json"))
	if err != nil {
		t.Fatalf("기준중위소득 표를 읽지 못했다: %v", err)
	}
	store, err := loader.New(filepath.Join(dataDir, "programs"))
	if err != nil {
		t.Fatalf("제도를 읽지 못했다: %v", err)
	}

	api := &API{Programs: store, Income: income.Calculator{Table: table}}
	return api.Routes()
}

func do(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

// ── /healthz ────────────────────────────────────────────────

func TestHealthzSaysItStoresNothing(t *testing.T) {
	rec := do(t, newTestAPI(t), http.MethodGet, "/healthz", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	// 설계 원칙 2. 프론트의 연결 확인 화면이 service 를 읽는다
	for _, want := range []string{`"service":"due-api"`, `"storesUserData":false`} {
		if !strings.Contains(body, want) {
			t.Errorf("응답에 %s 가 없다: %s", want, body)
		}
	}
}

// ── /api/evaluate ───────────────────────────────────────────

func decodeEvaluate(t *testing.T, rec *httptest.ResponseRecorder) EvaluateResponse {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200. body: %s", rec.Code, rec.Body.String())
	}
	var got EvaluateResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("응답을 파싱하지 못했다: %v\n%s", err, rec.Body.String())
	}
	return got
}

// 기획서의 데모 시나리오를 그대로 통과시킨다.
// "혼자 애 키우는데 일이 끊겼어요. 아이는 7살이고 월세 살아요."
func TestEvaluateDemoScenario(t *testing.T) {
	body := `{"context":{
		"householdSize": 2,
		"age": 33,
		"incomeMonthly": 800000,
		"housingType": "MONTHLY_RENT",
		"monthlyRent": 400000,
		"employmentStatus": "LOST_JOB",
		"isSingleParent": true,
		"childrenAges": [7]
	}}`

	got := decodeEvaluate(t, do(t, newTestAPI(t), http.MethodPost, "/api/evaluate", body))

	if len(got.Results) == 0 {
		t.Fatal("판정 결과가 비었다")
	}
	if got.Disclaimer != model.Disclaimer {
		t.Errorf("고지 문구가 빠졌다: %q", got.Disclaimer)
	}
	if got.MedianIncomeYear != 2026 {
		t.Errorf("기준연도 = %d, want 2026", got.MedianIncomeYear)
	}
	// 2인 가구 월 80만원 → 기준중위소득(419만) 대비 약 19%
	if got.IncomePct == nil {
		t.Fatal("소득 비율이 계산되지 않았다")
	}
	if *got.IncomePct < 18 || *got.IncomePct > 20 {
		t.Errorf("소득 비율 = %.1f%%, 약 19%% 가 나와야 한다", *got.IncomePct)
	}

	// 한부모 아동양육비는 해당이어야 한다 (한부모 + 소득 65% 이하 + 자녀 있음)
	for _, r := range got.Results {
		if r.Program.ID != "single-parent-child-care" {
			continue
		}
		if r.Status != model.MatchEligible {
			t.Errorf("한부모 아동양육비 = %s, want ELIGIBLE", r.Status)
			for _, c := range r.Conditions {
				t.Logf("  %s → %s (%s)", c.Condition.Field, c.Status, c.Reason)
			}
		}
		if r.EstimatedAmount != 230000*12 {
			t.Errorf("예상액 = %d, want %d", r.EstimatedAmount, 230000*12)
		}
	}

	if got.Summary.EligibleCount+got.Summary.NeedsInfoCount+got.Summary.IneligibleCount !=
		len(got.Results) {
		t.Error("요약의 건수 합이 결과 수와 다르다")
	}
}

// ★ 아무것도 모르면 전부 '확인 필요' 여야 한다. 탈락시키지 않는다.
func TestEvaluateEmptyContextNeedsInfo(t *testing.T) {
	got := decodeEvaluate(t, do(t, newTestAPI(t), http.MethodPost,
		"/api/evaluate", `{"context":{}}`))

	if got.IncomePct != nil {
		t.Errorf("소득 비율이 계산되면 안 된다: %v", *got.IncomePct)
	}
	if got.Summary.IneligibleCount != 0 {
		t.Errorf("미해당 = %d건. 정보가 없다고 탈락시키면 안 된다", got.Summary.IneligibleCount)
	}
	if got.Summary.NeedsInfoCount != len(got.Results) {
		t.Errorf("확인필요 = %d건 / 전체 %d건", got.Summary.NeedsInfoCount, len(got.Results))
	}
	for _, r := range got.Results {
		if len(r.MissingFields) == 0 {
			t.Errorf("%s: 무엇이 부족한지 알려주지 않는다", r.Program.ID)
		}
	}
}

// 확인 필요 제도의 금액은 합계에 들어가면 안 된다.
func TestEvaluateTotalCountsEligibleOnly(t *testing.T) {
	got := decodeEvaluate(t, do(t, newTestAPI(t), http.MethodPost,
		"/api/evaluate", `{"context":{}}`))

	if got.Summary.TotalAnnualAmount != 0 {
		t.Errorf("합계 = %d. 확인 필요뿐인데 금액이 잡혔다", got.Summary.TotalAnnualAmount)
	}
}

// 결과는 해당 → 확인필요 → 미해당 순으로 정렬되어야 한다.
func TestEvaluateResultsAreSorted(t *testing.T) {
	body := `{"context":{"householdSize":1,"age":29,"incomeMonthly":1000000,
		"housingType":"MONTHLY_RENT","assets":0}}`
	got := decodeEvaluate(t, do(t, newTestAPI(t), http.MethodPost, "/api/evaluate", body))

	last := -1
	for _, r := range got.Results {
		cur := statusOrder(r.Status)
		if cur < last {
			t.Errorf("정렬이 깨졌다: %s(%s) 가 뒤에 왔다", r.Program.ID, r.Status)
		}
		last = cur
	}
}

// 같은 요청은 항상 같은 응답이어야 한다. 판정은 결정론적이다.
func TestEvaluateIsDeterministic(t *testing.T) {
	h := newTestAPI(t)
	body := `{"context":{"householdSize":2,"age":30,"incomeMonthly":1200000,
		"housingType":"MONTHLY_RENT","isSingleParent":true,"childrenAges":[7]}}`

	first := do(t, h, http.MethodPost, "/api/evaluate", body).Body.String()
	for i := 0; i < 10; i++ {
		again := do(t, h, http.MethodPost, "/api/evaluate", body).Body.String()
		if again != first {
			t.Fatal("같은 요청에 다른 응답이 나왔다")
		}
	}
}

// ── 에러 형식 ───────────────────────────────────────────────

func decodeErr(t *testing.T, rec *httptest.ResponseRecorder) ErrorBody {
	t.Helper()
	var got ErrorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("에러 응답을 파싱하지 못했다: %v\n%s", err, rec.Body.String())
	}
	if got.Error.Code == "" || got.Error.Message == "" {
		t.Errorf("code/message 가 비었다: %+v", got)
	}
	return got
}

func TestErrorShape(t *testing.T) {
	h := newTestAPI(t)

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
		wantCode   string
	}{
		{"깨진 JSON", http.MethodPost, "/api/evaluate", `{"context":`,
			http.StatusBadRequest, CodeInvalidJSON},
		{"빈 본문", http.MethodPost, "/api/evaluate", "",
			http.StatusBadRequest, CodeInvalidJSON},
		{"모르는 항목", http.MethodPost, "/api/evaluate", `{"contxt":{}}`,
			http.StatusBadRequest, CodeInvalidJSON},
		{"타입 불일치", http.MethodPost, "/api/evaluate", `{"context":{"age":"스물아홉"}}`,
			http.StatusBadRequest, CodeInvalidJSON},
		{"없는 경로", http.MethodGet, "/api/없는것", "",
			http.StatusNotFound, CodeNotFound},
		{"아직 없는 기능", http.MethodPost, "/api/extract", `{"text":"안녕"}`,
			http.StatusNotImplemented, CodeNotImplemented},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, h, tt.method, tt.path, tt.body)
			if rec.Code != tt.wantStatus {
				t.Errorf("code = %d, want %d. body: %s",
					rec.Code, tt.wantStatus, rec.Body.String())
			}
			if got := decodeErr(t, rec); got.Error.Code != tt.wantCode {
				t.Errorf("error.code = %q, want %q", got.Error.Code, tt.wantCode)
			}
		})
	}
}

// 오타 난 항목은 조용히 무시되면 안 된다. 무엇이 틀렸는지 알려줘야 한다.
func TestUnknownFieldIsReported(t *testing.T) {
	rec := do(t, newTestAPI(t), http.MethodPost, "/api/evaluate",
		`{"context":{"incomeMontly":1000000}}`)

	got := decodeErr(t, rec)
	if !strings.Contains(got.Error.Message, "incomeMontly") {
		t.Errorf("어느 항목이 문제인지 알려주지 않는다: %q", got.Error.Message)
	}
}

// ── /api/programs ───────────────────────────────────────────

func TestPrograms(t *testing.T) {
	rec := do(t, newTestAPI(t), http.MethodGet, "/api/programs", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}

	var got ProgramsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("파싱 실패: %v", err)
	}
	if got.Count == 0 || len(got.Programs) != got.Count {
		t.Errorf("count = %d, programs = %d", got.Count, len(got.Programs))
	}
	if len(got.Problems) > 0 {
		for _, p := range got.Problems {
			t.Errorf("제도 데이터에 문제가 있다: %s — %s", p.File, p.Reason)
		}
	}
	// 심사에서 반드시 물어보는 항목이다
	for _, p := range got.Programs {
		if p.Source.URL == "" || p.Source.RevisedAt == "" {
			t.Errorf("%s: 출처가 비었다", p.ID)
		}
	}
}

// ── 공통 처리 ───────────────────────────────────────────────

// 캐시에 남으면 "저장하지 않는다" 는 약속이 깨진다.
func TestNoStoreHeader(t *testing.T) {
	h := newTestAPI(t)
	for _, path := range []string{"/healthz", "/api/programs"} {
		rec := do(t, h, http.MethodGet, path, "")
		if got := rec.Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("%s: Cache-Control = %q, want no-store", path, got)
		}
	}
}

func TestBodyTooLarge(t *testing.T) {
	big := `{"context":{"region":"` + strings.Repeat("가", 700_000) + `"}}`
	rec := do(t, newTestAPI(t), http.MethodPost, "/api/evaluate", big)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("code = %d, want 413", rec.Code)
	}
	if got := decodeErr(t, rec); got.Error.Code != CodeBodyTooLarge {
		t.Errorf("code = %q, want %q", got.Error.Code, CodeBodyTooLarge)
	}
}

// panic 이 나도 서버는 살아 있어야 한다.
func TestRecoverPanic(t *testing.T) {
	h := recoverPanic(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("일부러 터뜨림")
	}))

	rec := do(t, h, http.MethodGet, "/boom", "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("code = %d, want 500", rec.Code)
	}
	if got := decodeErr(t, rec); got.Error.Code != CodeInternal {
		t.Errorf("code = %q, want %q", got.Error.Code, CodeInternal)
	}
}
