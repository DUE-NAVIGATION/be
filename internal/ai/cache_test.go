package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/DUE-NAVIGATION/be/internal/model"
)

func realCache(t *testing.T) *FileCache {
	t.Helper()
	c, err := LoadCache(filepath.Join("..", "..", "data", "demo-cache.json"))
	if err != nil {
		t.Fatalf("데모 캐시를 읽지 못했다: %v", err)
	}
	return c
}

// ★ 배포되는 데모 캐시가 실제로 쓸 수 있는 상태여야 한다.
// 발표 당일에 깨진 걸 알면 늦는다.
func TestRealDemoCacheIsUsable(t *testing.T) {
	c := realCache(t)

	for _, op := range []string{"extract", "explain"} {
		if c.Count(op) == 0 {
			t.Errorf("%s 항목이 하나도 없다", op)
		}
		// 그물이 없으면 리허설과 다른 입력에서 데모가 실패한다
		if !c.HasDefault(op) {
			t.Errorf("%s 에 기본값(*)이 없다", op)
		}
	}

	// 기획서의 데모 시나리오 문장이 반드시 들어 있어야 한다
	const demo = "혼자 애 키우는데 일이 끊겼어요. 아이는 7살이고 월세 살아요."
	raw, ok := c.Lookup("extract", cacheKey(Sanitize(demo).Text))
	if !ok {
		t.Fatal("데모 시나리오 문장이 캐시에 없다")
	}

	got, err := decodeExtraction(raw)
	if err != nil {
		t.Fatalf("캐시 항목을 해석하지 못했다: %v", err)
	}
	if got.Extracted.IsSingleParent == nil || !*got.Extracted.IsSingleParent {
		t.Error("한부모 여부가 빠졌다")
	}
	if got.Extracted.HousingType == nil || *got.Extracted.HousingType != model.HousingMonthlyRent {
		t.Error("주거형태가 빠졌다")
	}
	if len(got.FollowUpQuestions) == 0 {
		t.Error("되묻기가 없다")
	}
}

// 캐시 항목의 키는 공백 차이로 빗나가면 안 된다.
func TestCacheKeyIsNormalized(t *testing.T) {
	c := realCache(t)

	// 파일에 적힌 것과 공백이 다른 입력
	messy := "혼자  애 키우는데 일이 끊겼어요.\n아이는 7살이고 월세 살아요."
	if _, ok := c.Lookup("extract", cacheKey(Sanitize(messy).Text)); !ok {
		t.Error("공백이 다르다고 캐시가 빗나갔다")
	}
}

// 모르는 입력은 기본값으로 내려간다. 데모가 멈추면 안 된다.
func TestCacheFallsBackToDefault(t *testing.T) {
	c := realCache(t)

	raw, ok := c.Lookup("extract", "리허설에 없던 문장입니다")
	if !ok {
		t.Fatal("기본값으로 내려가지 않았다")
	}
	got, err := decodeExtraction(raw)
	if err != nil {
		t.Fatal(err)
	}
	// ★ 기본값은 지어내지 않는다. 비워두고 되묻는다
	if got.Extracted.HouseholdSize != nil || got.Extracted.IncomeMonthly != nil {
		t.Error("기본값이 값을 지어냈다. 비워두고 되물어야 한다")
	}
	if len(got.FollowUpQuestions) == 0 {
		t.Error("기본값에 되묻기가 없다")
	}
}

// ★ DEMO_MODE 에서는 API 를 호출하지 않는다. 키도 네트워크도 없이 돌아야 한다.
func TestDemoModeNeverCallsAPI(t *testing.T) {
	fake := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("★ 데모 모드인데 API 를 호출했다")
	}))
	defer fake.Close()

	c := New(Config{
		BaseURL:  fake.URL,
		DemoMode: true,
		Cache:    realCache(t),
		// ★ 키가 없다. 그래도 동작해야 한다
	})

	if !c.Enabled() {
		t.Fatal("캐시가 있는데 Enabled 가 false 다")
	}

	got, err := c.Extract(context.Background(),
		"혼자 애 키우는데 일이 끊겼어요. 아이는 7살이고 월세 살아요.")
	if err != nil {
		t.Fatalf("Extract 실패: %v", err)
	}
	if got.Extracted.IsSingleParent == nil {
		t.Error("캐시 응답이 반영되지 않았다")
	}

	// 아무 입력이나 넣어도 실패하지 않아야 한다
	if _, err := c.Extract(context.Background(), "완전히 다른 문장"); err != nil {
		t.Errorf("기본값으로도 실패했다: %v", err)
	}

	text, err := c.Explain(context.Background(),
		[]model.MatchResult{{
			Program: model.Program{ID: "a", Name: "테스트 제도"},
			Status:  model.MatchEligible,
		}}, model.Summary{EligibleCount: 1})
	if err != nil {
		t.Fatalf("Explain 실패: %v", err)
	}
	if text == "" {
		t.Error("설명문이 비었다")
	}
}

// ★ 기본 설명문은 특정 제도를 언급하면 안 된다.
// 어떤 판정 결과에도 붙을 수 있으므로, 제도명을 넣으면 화면과 어긋난다.
func TestDefaultExplanationNamesNoProgram(t *testing.T) {
	raw, ok := realCache(t).Lookup("explain", "아무 키")
	if !ok {
		t.Fatal("explain 기본값이 없다")
	}
	text, err := decodeExplanation(raw)
	if err != nil {
		t.Fatal(err)
	}

	// 우리 제도 데이터에 있는 이름이 기본 설명문에 박혀 있으면 안 된다
	for _, name := range []string{"한부모가족 아동양육비", "청년월세", "국민취업지원제도"} {
		if contains(text, name) {
			t.Errorf("★ 기본 설명문에 제도명 %q 가 들어 있다. 판정 결과와 어긋날 수 있다", name)
		}
	}
	if !contains(text, "확인 필요") {
		t.Error("기본 설명문이 '확인 필요' 의 뜻을 설명하지 않는다")
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

func TestLoadCacheErrors(t *testing.T) {
	if _, err := LoadCache("없는파일.json"); err == nil {
		t.Error("없는 파일인데 에러가 없다")
	}
}
