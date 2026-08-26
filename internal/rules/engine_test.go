package rules

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/DUE-NAVIGATION/be/internal/model"
)

// ── 테스트 헬퍼 ──────────────────────────────────────────────

func prog(e model.Eligibility) model.Program {
	return model.Program{
		ID:          "test-program",
		Name:        "테스트 제도",
		Category:    model.CategoryOther,
		Eligibility: e,
		Benefit:     model.Benefit{Type: model.BenefitMonthly, Amount: 200000, Months: 12},
		Source:      model.Source{URL: "https://example.test", RevisedAt: "2026-01-01"},
	}
}

func cond(field string, op model.Op, value any) model.Condition {
	return model.Condition{Field: field, Op: op, Value: value}
}

// jsonCond 는 제도 JSON 을 거쳐 들어온 조건을 흉내낸다.
// encoding/json 은 숫자를 float64, 배열을 []any 로 만들기 때문에,
// Go 리터럴로 쓴 조건만 테스트하면 실제 동작을 놓친다.
func jsonCond(raw string) model.Condition {
	var c model.Condition
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		panic(err) // 테스트 픽스처의 오류이므로 즉시 드러내야 한다
	}
	return c
}

// ── Evaluate 테이블 테스트 ───────────────────────────────────

func TestEvaluate(t *testing.T) {
	tests := []struct {
		name        string
		program     model.Program
		ctx         model.UserContext
		wantStatus  model.MatchStatus
		wantMissing []string
	}{
		{
			name: "all 전부 통과 → ELIGIBLE",
			program: prog(model.Eligibility{All: []model.Condition{
				cond(model.FieldAge, model.OpBetween, []any{19, 34}),
				cond(model.FieldHousingType, model.OpIn, []any{"MONTHLY_RENT"}),
			}}),
			ctx: model.UserContext{
				Age:         model.Int(29),
				HousingType: model.Housing(model.HousingMonthlyRent),
			},
			wantStatus:  model.MatchEligible,
			wantMissing: nil,
		},
		{
			name: "all 중 하나 실패 → INELIGIBLE",
			program: prog(model.Eligibility{All: []model.Condition{
				cond(model.FieldAge, model.OpBetween, []any{19, 34}),
				cond(model.FieldDeposit, model.OpLte, 50000000),
			}}),
			ctx: model.UserContext{
				Age:     model.Int(29),
				Deposit: model.Int64(90000000), // 초과
			},
			wantStatus:  model.MatchIneligible,
			wantMissing: nil,
		},
		{
			name: "값 누락 → NEEDS_INFO + MissingFields 정확",
			program: prog(model.Eligibility{All: []model.Condition{
				cond(model.FieldAge, model.OpBetween, []any{19, 34}),
				cond(model.FieldHouseholdIncomePct, model.OpLte, 60),
				cond(model.FieldDeposit, model.OpLte, 50000000),
			}}),
			ctx:         model.UserContext{Age: model.Int(29)},
			wantStatus:  model.MatchNeedsInfo,
			wantMissing: []string{"householdIncomePct", "deposit"},
		},
		{
			name: "none 조건 발동 → INELIGIBLE",
			program: prog(model.Eligibility{
				All: []model.Condition{cond(model.FieldAge, model.OpGte, 19)},
				None: []model.Condition{
					cond(model.FieldReceivingPrograms, model.OpContains, "HOUSING_BENEFIT"),
				},
			}),
			ctx: model.UserContext{
				Age:               model.Int(29),
				ReceivingPrograms: []string{"HOUSING_BENEFIT"},
			},
			wantStatus:  model.MatchIneligible,
			wantMissing: nil,
		},
		{
			name: "none 조건 미발동 → ELIGIBLE",
			program: prog(model.Eligibility{
				All: []model.Condition{cond(model.FieldAge, model.OpGte, 19)},
				None: []model.Condition{
					cond(model.FieldReceivingPrograms, model.OpContains, "HOUSING_BENEFIT"),
				},
			}),
			ctx: model.UserContext{
				Age:               model.Int(29),
				ReceivingPrograms: []string{"YOUTH_ALLOWANCE"},
			},
			wantStatus:  model.MatchEligible,
			wantMissing: nil,
		},
		{
			name: "any 일부 통과 → ELIGIBLE",
			program: prog(model.Eligibility{Any: []model.Condition{
				cond(model.FieldIsSingleParent, model.OpEq, true),
				cond(model.FieldHasDisability, model.OpEq, true),
			}}),
			ctx: model.UserContext{
				IsSingleParent: model.Bool(true),
				HasDisability:  model.Bool(false),
			},
			wantStatus:  model.MatchEligible,
			wantMissing: nil,
		},
		{
			name: "any 전부 실패 → INELIGIBLE",
			program: prog(model.Eligibility{Any: []model.Condition{
				cond(model.FieldIsSingleParent, model.OpEq, true),
				cond(model.FieldHasDisability, model.OpEq, true),
			}}),
			ctx: model.UserContext{
				IsSingleParent: model.Bool(false),
				HasDisability:  model.Bool(false),
			},
			wantStatus:  model.MatchIneligible,
			wantMissing: nil,
		},
		{
			name: "any 에 통과가 없고 UNKNOWN 이 있으면 → NEEDS_INFO",
			program: prog(model.Eligibility{Any: []model.Condition{
				cond(model.FieldIsSingleParent, model.OpEq, true),
				cond(model.FieldHasDisability, model.OpEq, true),
			}}),
			ctx:         model.UserContext{IsSingleParent: model.Bool(false)},
			wantStatus:  model.MatchNeedsInfo,
			wantMissing: []string{"hasDisability"},
		},
		{
			name: "between 하한 경계 포함 → PASS",
			program: prog(model.Eligibility{All: []model.Condition{
				cond(model.FieldAge, model.OpBetween, []any{19, 34}),
			}}),
			ctx:        model.UserContext{Age: model.Int(19)},
			wantStatus: model.MatchEligible,
		},
		{
			name: "between 상한 경계 포함 → PASS",
			program: prog(model.Eligibility{All: []model.Condition{
				cond(model.FieldAge, model.OpBetween, []any{19, 34}),
			}}),
			ctx:        model.UserContext{Age: model.Int(34)},
			wantStatus: model.MatchEligible,
		},
		{
			name: "between 범위 밖 → INELIGIBLE",
			program: prog(model.Eligibility{All: []model.Condition{
				cond(model.FieldAge, model.OpBetween, []any{19, 34}),
			}}),
			ctx:        model.UserContext{Age: model.Int(35)},
			wantStatus: model.MatchIneligible,
		},
		{
			name: "★ 소득 0원은 아는 값이다 → 판정된다 (NEEDS_INFO 아님)",
			program: prog(model.Eligibility{All: []model.Condition{
				cond(model.FieldIncomeMonthly, model.OpLte, 1000000),
			}}),
			ctx:        model.UserContext{IncomeMonthly: model.Int64(0)},
			wantStatus: model.MatchEligible,
		},
		{
			name: "★ 소득 미입력은 탈락이 아니라 확인필요",
			program: prog(model.Eligibility{All: []model.Condition{
				cond(model.FieldIncomeMonthly, model.OpLte, 1000000),
			}}),
			ctx:         model.UserContext{},
			wantStatus:  model.MatchNeedsInfo,
			wantMissing: []string{"incomeMonthly"},
		},
		{
			name: "명시적 탈락은 확인필요보다 우선한다",
			program: prog(model.Eligibility{All: []model.Condition{
				cond(model.FieldAge, model.OpLte, 34),
				cond(model.FieldDeposit, model.OpLte, 50000000),
			}}),
			ctx:        model.UserContext{Age: model.Int(50)}, // deposit 은 모름
			wantStatus: model.MatchIneligible,
		},
		{
			name: "between 인데 value 가 배열 2개가 아님 → UNKNOWN (panic 아님)",
			program: prog(model.Eligibility{All: []model.Condition{
				cond(model.FieldAge, model.OpBetween, []any{19}),
			}}),
			ctx:         model.UserContext{Age: model.Int(29)},
			wantStatus:  model.MatchNeedsInfo,
			wantMissing: nil, // 사용자 입력 부족이 아니라 제도 JSON 오류다
		},
		{
			name: "알 수 없는 연산자 → UNKNOWN (panic 아님)",
			program: prog(model.Eligibility{All: []model.Condition{
				cond(model.FieldAge, model.Op("morethan"), 19),
			}}),
			ctx:         model.UserContext{Age: model.Int(29)},
			wantStatus:  model.MatchNeedsInfo,
			wantMissing: nil,
		},
		{
			name: "UserContext 에 없는 필드 참조 → UNKNOWN, MissingFields 에 넣지 않는다",
			program: prog(model.Eligibility{All: []model.Condition{
				cond("incomeMontly", model.OpLte, 1000000), // 오타
			}}),
			ctx:         model.UserContext{IncomeMonthly: model.Int64(500000)},
			wantStatus:  model.MatchNeedsInfo,
			wantMissing: nil,
		},
		{
			name: "타입 불일치 (문자열에 lte) → UNKNOWN",
			program: prog(model.Eligibility{All: []model.Condition{
				cond(model.FieldRegion, model.OpLte, 100),
			}}),
			ctx:         model.UserContext{Region: model.Str("서울")},
			wantStatus:  model.MatchNeedsInfo,
			wantMissing: nil,
		},
		{
			name:        "완전히 빈 Eligibility → NEEDS_INFO (단정하지 않는다)",
			program:     prog(model.Eligibility{}),
			ctx:         model.UserContext{Age: model.Int(29)},
			wantStatus:  model.MatchNeedsInfo,
			wantMissing: nil,
		},
		{
			name: "exists — 값이 있으면 PASS",
			program: prog(model.Eligibility{All: []model.Condition{
				cond(model.FieldHasDisability, model.OpExists, nil),
			}}),
			ctx:        model.UserContext{HasDisability: model.Bool(false)},
			wantStatus: model.MatchEligible,
		},
		{
			name: "exists — 값을 모르면 UNKNOWN (없다고 단정하지 않는다)",
			program: prog(model.Eligibility{All: []model.Condition{
				cond(model.FieldHasDisability, model.OpExists, nil),
			}}),
			ctx:         model.UserContext{},
			wantStatus:  model.MatchNeedsInfo,
			wantMissing: []string{"hasDisability"},
		},
		{
			name: "contains — 자녀 나이 배열에 포함",
			program: prog(model.Eligibility{All: []model.Condition{
				cond(model.FieldChildrenAges, model.OpContains, 7),
			}}),
			ctx:        model.UserContext{ChildrenAges: []int{7, 12}},
			wantStatus: model.MatchEligible,
		},
		{
			name: "contains — 빈 배열은 아는 값이므로 FAIL (확인필요 아님)",
			program: prog(model.Eligibility{All: []model.Condition{
				cond(model.FieldChildrenAges, model.OpContains, 7),
			}}),
			ctx:        model.UserContext{ChildrenAges: []int{}},
			wantStatus: model.MatchIneligible,
		},
		{
			name: "MissingFields 는 중복을 제거한다",
			program: prog(model.Eligibility{All: []model.Condition{
				cond(model.FieldDeposit, model.OpGte, 0),
				cond(model.FieldDeposit, model.OpLte, 50000000),
			}}),
			ctx:         model.UserContext{},
			wantStatus:  model.MatchNeedsInfo,
			wantMissing: []string{"deposit"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Evaluate(tt.program, tt.ctx)

			if got.Status != tt.wantStatus {
				t.Errorf("Status = %s, 기대값 %s", got.Status, tt.wantStatus)
				for _, c := range got.Conditions {
					t.Logf("  %s %s → %s (%s)", c.Condition.Field, c.Condition.Op, c.Status, c.Reason)
				}
			}

			if len(tt.wantMissing) > 0 || len(got.MissingFields) > 0 {
				if !reflect.DeepEqual(got.MissingFields, tt.wantMissing) {
					t.Errorf("MissingFields = %v, 기대값 %v", got.MissingFields, tt.wantMissing)
				}
			}

			// 설명 가능성 — 모든 조건에 대해 근거를 남겨야 한다
			wantConds := len(tt.program.Eligibility.All) +
				len(tt.program.Eligibility.Any) +
				len(tt.program.Eligibility.None)
			if len(got.Conditions) != wantConds {
				t.Errorf("ConditionResult 개수 = %d, 기대값 %d", len(got.Conditions), wantConds)
			}
			for _, c := range got.Conditions {
				if c.Reason == "" {
					t.Errorf("조건 %s 에 Reason 이 비어 있다", c.Condition.Field)
				}
			}
		})
	}
}

// 제도 JSON 을 거쳐 들어온 값(float64, []any)도 똑같이 동작해야 한다.
// Go 리터럴로만 테스트하면 실제 운영에서 깨진다.
func TestEvaluateFromJSON(t *testing.T) {
	p := prog(model.Eligibility{All: []model.Condition{
		jsonCond(`{"field":"age","op":"between","value":[19,34]}`),
		jsonCond(`{"field":"householdIncomePct","op":"lte","value":60}`),
		jsonCond(`{"field":"housingType","op":"in","value":["MONTHLY_RENT","JEONSE"]}`),
		jsonCond(`{"field":"deposit","op":"lte","value":50000000}`),
	}})

	ctx := model.UserContext{
		Age:                model.Int(29),
		HouseholdIncomePct: model.Float(45),
		HousingType:        model.Housing(model.HousingMonthlyRent),
		Deposit:            model.Int64(20000000),
	}

	got := Evaluate(p, ctx)
	if got.Status != model.MatchEligible {
		t.Errorf("Status = %s, 기대값 ELIGIBLE", got.Status)
		for _, c := range got.Conditions {
			t.Logf("  %s %s → %s (%s)", c.Condition.Field, c.Condition.Op, c.Status, c.Reason)
		}
	}
}

// Actual 은 화면의 "입력: 29세" 에 쓰인다. 아는 값이면 반드시 채워야 한다.
func TestActualIsRecorded(t *testing.T) {
	p := prog(model.Eligibility{All: []model.Condition{
		cond(model.FieldAge, model.OpBetween, []any{19, 34}),
		cond(model.FieldDeposit, model.OpLte, 50000000),
	}})

	got := Evaluate(p, model.UserContext{Age: model.Int(29)})

	if got.Conditions[0].Actual != int64(29) {
		t.Errorf("age 의 Actual = %v(%T), 기대값 29", got.Conditions[0].Actual, got.Conditions[0].Actual)
	}
	if got.Conditions[1].Actual != nil {
		t.Errorf("모르는 값의 Actual 은 nil 이어야 한다, 실제 %v", got.Conditions[1].Actual)
	}
}

// 어떤 입력에도 panic 하지 않아야 한다. 제도 JSON 이 서버를 죽이면 안 된다.
func TestNeverPanics(t *testing.T) {
	nasty := []model.Condition{
		{Field: "", Op: "", Value: nil},
		{Field: model.FieldAge, Op: model.OpBetween, Value: "문자열"},
		{Field: model.FieldAge, Op: model.OpBetween, Value: []any{"a", "b"}},
		{Field: model.FieldAge, Op: model.OpIn, Value: nil},
		{Field: model.FieldAge, Op: model.OpIn, Value: 5},
		{Field: model.FieldChildrenAges, Op: model.OpContains, Value: nil},
		{Field: model.FieldReceivingPrograms, Op: model.OpLte, Value: 3},
		{Field: model.FieldRegion, Op: model.OpBetween, Value: []any{1, 2}},
		{Field: model.FieldAge, Op: model.OpEq, Value: map[string]any{"x": 1}},
		{Field: model.FieldHousingType, Op: model.OpContains, Value: "MONTHLY_RENT"},
	}

	ctxs := []model.UserContext{
		{},
		{Age: model.Int(29), Region: model.Str("서울"), ChildrenAges: []int{7}},
	}

	for _, c := range nasty {
		for _, ctx := range ctxs {
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("panic 발생: 조건 %+v / ctx %+v → %v", c, ctx, r)
					}
				}()
				res := Evaluate(prog(model.Eligibility{All: []model.Condition{c}}), ctx)
				if len(res.Conditions) != 1 || res.Conditions[0].Reason == "" {
					t.Errorf("조건 %+v 의 근거가 비어 있다", c)
				}
			}()
		}
	}
}
