package rules_test

import (
	"strings"
	"testing"

	"github.com/DUE-NAVIGATION/be/internal/model"
	"github.com/DUE-NAVIGATION/be/internal/rules"
)

// 배제 조건이 있는 제도. "재직 중이면 안 된다".
func withExclusion() model.Program {
	return model.Program{
		ID:   "job-support",
		Name: "구직 지원",
		Eligibility: model.Eligibility{
			All: []model.Condition{
				{Field: model.FieldAge, Op: model.OpGte, Value: 19, Label: "만 19세 이상"},
			},
			None: []model.Condition{
				{
					Field: model.FieldEmploymentStatus,
					Op:    model.OpEq,
					Value: "EMPLOYED",
					Label: "재직 중이 아닐 것",
				},
			},
		},
		Benefit: model.Benefit{Type: model.BenefitMonthly, Amount: 500000, Months: 6},
	}
}

func find(rs []model.ConditionResult, field string) *model.ConditionResult {
	for i := range rs {
		if rs[i].Condition.Field == field {
			return &rs[i]
		}
	}
	return nil
}

// ★ 이 테스트가 이 파일의 존재 이유다.
//
// 화면이 조건의 뜻을 정반대로 보여주던 버그가 있었다.
// "재직 중이 아닐 것" 조건에 실직자가 붉은 FAIL 로 표시됐다 —
// 실제로는 배제에 걸리지 않아 통과인데도.
//
// 엔진의 Status 는 원자료로 그대로 두고, Group 으로 화면이 뒤집을 수 있게 한다.
func TestNoneGroupIsMarked(t *testing.T) {
	employed := model.EmploymentEmployed
	lostJob := model.EmploymentLostJob
	age := 33

	tests := []struct {
		name string
		emp  model.EmploymentStatus
		// 엔진 관점 — 뒤집지 않은 원자료
		wantStatus model.ConditionStatus
		// 제도 전체 판정
		wantMatch model.MatchStatus
	}{
		{
			name:       "★ 실직자는 배제에 걸리지 않는다 (엔진 FAIL = 이용자에겐 통과)",
			emp:        lostJob,
			wantStatus: model.StatusFail,
			wantMatch:  model.MatchEligible,
		},
		{
			name:       "★ 재직자는 배제에 걸린다 (엔진 PASS = 이용자에겐 탈락)",
			emp:        employed,
			wantStatus: model.StatusPass,
			wantMatch:  model.MatchIneligible,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			emp := tt.emp
			got := rules.Evaluate(withExclusion(), model.UserContext{
				Age: &age, EmploymentStatus: &emp,
			})

			if got.Status != tt.wantMatch {
				t.Errorf("제도 판정 = %q, want %q", got.Status, tt.wantMatch)
			}

			c := find(got.Conditions, model.FieldEmploymentStatus)
			if c == nil {
				t.Fatal("배제 조건이 근거에 없다")
			}
			if c.Group != model.GroupNone {
				t.Errorf("Group = %q, want %q — 화면이 표시를 뒤집을 수 없다",
					c.Group, model.GroupNone)
			}
			if c.Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q (엔진 관점 원자료는 뒤집지 않는다)",
					c.Status, tt.wantStatus)
			}
		})
	}
}

// 모든 조건에 그룹이 붙어야 한다. 하나라도 비면 화면이 판단할 수 없다.
func TestEveryConditionHasGroup(t *testing.T) {
	age := 33
	got := rules.Evaluate(withExclusion(), model.UserContext{Age: &age})

	for _, c := range got.Conditions {
		if c.Group == "" {
			t.Errorf("%s 조건에 Group 이 없다", c.Condition.Field)
		}
	}
}

// ★ 사유 문구도 이용자 관점이어야 한다.
// 표시만 뒤집고 문구를 그대로 두면 "통과인데 사유는 탈락 이유" 가 된다.
func TestExcludeReasonIsWrittenForTheUser(t *testing.T) {
	age := 33

	lostJob := model.EmploymentLostJob
	pass := rules.Evaluate(withExclusion(), model.UserContext{
		Age: &age, EmploymentStatus: &lostJob,
	})
	c := find(pass.Conditions, model.FieldEmploymentStatus)
	if !strings.Contains(c.Reason, "해당하지 않습니다") {
		t.Errorf("배제에 안 걸린 경우 사유 = %q", c.Reason)
	}
	// 원래 문구("EMPLOYED 이(가) 아닙니다")가 그대로 새어나오면 안 된다
	if strings.Contains(c.Reason, "아닙니다 (입력") {
		t.Errorf("엔진 관점 문구가 그대로 노출됐다: %q", c.Reason)
	}

	employed := model.EmploymentEmployed
	fail := rules.Evaluate(withExclusion(), model.UserContext{
		Age: &age, EmploymentStatus: &employed,
	})
	c = find(fail.Conditions, model.FieldEmploymentStatus)
	if !strings.Contains(c.Reason, "이용할 수 없습니다") {
		t.Errorf("배제에 걸린 경우 사유 = %q", c.Reason)
	}
}

// 시설의 관할 조건은 뜻이 뒤집히지 않는다. coverage 그룹으로 구분한다.
func TestCoverageConditionHasItsOwnGroup(t *testing.T) {
	got := rules.EvaluateFacility(childCenter(), model.UserContext{
		Region: strp("서울특별시"), District: strp("관악구"), ChildrenAges: []int{7},
	})

	if got.Conditions[0].Group != model.GroupCoverage {
		t.Errorf("관할 조건 Group = %q, want %q",
			got.Conditions[0].Group, model.GroupCoverage)
	}
	if got.Conditions[0].Status != model.StatusPass {
		t.Errorf("관할 조건 Status = %q, want PASS", got.Conditions[0].Status)
	}
}
