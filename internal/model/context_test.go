package model_test

import (
	"encoding/json"
	"testing"

	"github.com/DUE-NAVIGATION/be/internal/model"
)

// 이 프로젝트에서 가장 중요한 구분을 고정한다.
// "소득 0원(무소득)" 과 "소득 미입력" 이 절대 같아지면 안 된다.
func TestUnknownIsNotZero(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantValue any
		wantKnown bool
	}{
		{
			name:      "필드 자체가 없으면 모름",
			body:      `{}`,
			wantValue: nil,
			wantKnown: false,
		},
		{
			name:      "0 은 아는 값이다 — 무소득",
			body:      `{"incomeMonthly":0}`,
			wantValue: int64(0),
			wantKnown: true,
		},
		{
			name:      "null 은 모름",
			body:      `{"incomeMonthly":null}`,
			wantValue: nil,
			wantKnown: false,
		},
		{
			name:      "값이 있으면 그대로",
			body:      `{"incomeMonthly":1800000}`,
			wantValue: int64(1800000),
			wantKnown: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ctx model.UserContext
			if err := json.Unmarshal([]byte(tt.body), &ctx); err != nil {
				t.Fatalf("언마샬 실패: %v", err)
			}

			got, known, exists := model.Lookup(ctx, model.FieldIncomeMonthly)
			if !exists {
				t.Fatal("incomeMonthly 필드가 없다고 나왔다")
			}
			if known != tt.wantKnown {
				t.Errorf("known = %v, 기대값 %v", known, tt.wantKnown)
			}
			if got != tt.wantValue {
				t.Errorf("value = %v(%T), 기대값 %v(%T)", got, got, tt.wantValue, tt.wantValue)
			}
		})
	}
}

// 오타 난 필드는 UNKNOWN 이 아니라 "그런 필드 없음" 으로 구분되어야 한다.
// 제도 JSON 의 흔한 실수를 cmd/validate 가 잡으려면 이 구분이 필요하다.
func TestUnknownFieldIsDistinguishable(t *testing.T) {
	ctx := model.UserContext{Age: model.Int(29)}

	if _, _, exists := model.Lookup(ctx, "incomeMontly"); exists { // 오타
		t.Error("오타 난 필드가 존재한다고 나왔다")
	}
	if _, known, exists := model.Lookup(ctx, model.FieldAge); !exists || !known {
		t.Error("age 를 못 읽었다")
	}
	if model.FieldExists("nope") {
		t.Error("FieldExists 가 없는 필드를 true 로 답했다")
	}
	for _, f := range model.KnownFields() {
		if _, _, exists := model.Lookup(model.UserContext{}, f); !exists {
			t.Errorf("KnownFields 에 있는 %q 를 Lookup 이 모른다 — 목록과 switch 가 어긋났다", f)
		}
	}
}

// 빈 슬라이스는 "없다는 정보" 이므로 모름이 아니다.
func TestEmptySliceIsKnown(t *testing.T) {
	var nilCase model.UserContext
	if _, known, _ := model.Lookup(nilCase, model.FieldChildrenAges); known {
		t.Error("nil 슬라이스가 아는 값으로 나왔다")
	}

	emptyCase := model.UserContext{ChildrenAges: []int{}}
	if _, known, _ := model.Lookup(emptyCase, model.FieldChildrenAges); !known {
		t.Error("빈 슬라이스(자녀 없음)가 모름으로 나왔다")
	}
}

// 프론트가 TypeScript 이므로 JSON 키는 camelCase 여야 한다.
// 모르는 값은 응답에서 아예 빠진다 (omitempty).
func TestJSONShape(t *testing.T) {
	ctx := model.UserContext{
		HouseholdSize:    model.Int(2),
		IsSingleParent:   model.Bool(true),
		EmploymentStatus: model.Employment(model.EmploymentLostJob),
		HousingType:      model.Housing(model.HousingMonthlyRent),
	}

	b, err := json.Marshal(ctx)
	if err != nil {
		t.Fatalf("마샬 실패: %v", err)
	}

	got := string(b)
	want := `{"householdSize":2,"housingType":"MONTHLY_RENT","employmentStatus":"LOST_JOB","isSingleParent":true}`
	if got != want {
		t.Errorf("JSON 이 다르다\n got: %s\nwant: %s", got, want)
	}
}
