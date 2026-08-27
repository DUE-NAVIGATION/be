package rules

import (
	"sort"
	"testing"

	"github.com/DUE-NAVIGATION/be/internal/model"
)

// res 는 판정이 끝나고 금액까지 채워진 결과를 흉내낸다.
func res(id string, status model.MatchStatus, amount int64) model.MatchResult {
	return model.MatchResult{
		Program:         model.Program{ID: id, Name: id + " 제도"},
		Status:          status,
		Conditions:      []model.ConditionResult{},
		EstimatedAmount: amount,
		MissingFields:   []string{},
	}
}

func statusOf(t *testing.T, r Resolution, id string) model.MatchResult {
	t.Helper()
	for _, x := range r.Results {
		if x.Program.ID == id {
			return x
		}
	}
	t.Fatalf("결과에 %s 가 없다", id)
	return model.MatchResult{}
}

func TestResolveConflictsExclusive(t *testing.T) {
	tests := []struct {
		name         string
		results      []model.MatchResult
		relations    []model.Relation
		wantEligible []string
		wantExcluded []string
	}{
		{
			name: "배타 둘 중 금액이 큰 쪽을 남긴다",
			results: []model.MatchResult{
				res("housing", model.MatchEligible, 2400000),
				res("rent", model.MatchEligible, 1200000),
			},
			relations: []model.Relation{
				{From: "housing", To: "rent", Type: model.RelationExclusive},
			},
			wantEligible: []string{"housing"},
			wantExcluded: []string{"rent"},
		},
		{
			name: "★ 탐욕법이면 틀리는 배치 — A-B, B-C 에서 A+C 가 최대",
			results: []model.MatchResult{
				res("a", model.MatchEligible, 700000),
				res("b", model.MatchEligible, 1000000), // 혼자서는 가장 크다
				res("c", model.MatchEligible, 700000),
			},
			relations: []model.Relation{
				{From: "a", To: "b", Type: model.RelationExclusive},
				{From: "b", To: "c", Type: model.RelationExclusive},
			},
			// b 하나(100만)보다 a+c(140만)가 크다
			wantEligible: []string{"a", "c"},
			wantExcluded: []string{"b"},
		},
		{
			name: "관계가 없으면 전부 남는다",
			results: []model.MatchResult{
				res("a", model.MatchEligible, 100),
				res("b", model.MatchEligible, 200),
			},
			relations:    nil,
			wantEligible: []string{"a", "b"},
			wantExcluded: nil,
		},
		{
			name: "해당 가능하지 않은 제도는 배타 계산에 끼지 않는다",
			results: []model.MatchResult{
				res("a", model.MatchEligible, 100000),
				res("b", model.MatchIneligible, 9000000),
			},
			relations: []model.Relation{
				{From: "a", To: "b", Type: model.RelationExclusive},
			},
			wantEligible: []string{"a"},
			wantExcluded: nil,
		},
		{
			name: "결과에 없는 제도를 가리키는 관계는 무시한다",
			results: []model.MatchResult{
				res("a", model.MatchEligible, 100000),
			},
			relations: []model.Relation{
				{From: "a", To: "없는제도", Type: model.RelationExclusive},
			},
			wantEligible: []string{"a"},
			wantExcluded: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveConflicts(tt.results, tt.relations)

			var eligible []string
			for _, r := range got.Results {
				if r.Status == model.MatchEligible {
					eligible = append(eligible, r.Program.ID)
				}
			}
			sort.Strings(eligible)

			excluded := append([]string(nil), got.Excluded...)
			sort.Strings(excluded)
			want := append([]string(nil), tt.wantExcluded...)
			sort.Strings(want)

			if !equalStrings(eligible, tt.wantEligible) {
				t.Errorf("남은 제도 = %v, 기대값 %v", eligible, tt.wantEligible)
			}
			if !equalStrings(excluded, want) {
				t.Errorf("제외된 제도 = %v, 기대값 %v", excluded, want)
			}
		})
	}
}

// 밀려난 제도에도 왜 밀렸는지가 남아야 한다. 설명 가능성이 이 서비스의 핵심이다.
func TestExcludedHasReason(t *testing.T) {
	got := ResolveConflicts(
		[]model.MatchResult{
			res("big", model.MatchEligible, 2400000),
			res("small", model.MatchEligible, 600000),
		},
		[]model.Relation{{From: "big", To: "small", Type: model.RelationExclusive}},
	)

	small := statusOf(t, got, "small")
	if small.Status != model.MatchIneligible {
		t.Fatalf("small 상태 = %s, 기대값 INELIGIBLE", small.Status)
	}
	if len(small.Conditions) == 0 {
		t.Fatal("밀려난 이유가 남지 않았다")
	}
	last := small.Conditions[len(small.Conditions)-1]
	if last.Reason == "" {
		t.Error("사유가 비어 있다")
	}
	// 상대 제도 이름이 사유에 있어야 사용자가 납득한다
	if want := "big 제도"; !contains(last.Reason, want) {
		t.Errorf("사유에 상대 제도 이름이 없다: %q", last.Reason)
	}
	// 사용자에게 물어볼 값이 아니므로 MissingFields 에 새면 안 된다
	if len(small.MissingFields) != 0 {
		t.Errorf("MissingFields 가 오염됐다: %v", small.MissingFields)
	}
}

func TestResolveConflictsReducing(t *testing.T) {
	got := ResolveConflicts(
		[]model.MatchResult{
			res("main", model.MatchEligible, 1000000),
			res("sub", model.MatchEligible, 500000),
		},
		[]model.Relation{
			{From: "main", To: "sub", Type: model.RelationReducing, ReducePct: 30},
		},
	)

	main := statusOf(t, got, "main")
	if main.Status != model.MatchEligible {
		t.Errorf("감액은 탈락이 아니다. 상태 = %s", main.Status)
	}
	if main.EstimatedAmount != 700000 {
		t.Errorf("감액 후 금액 = %d, 기대값 700000", main.EstimatedAmount)
	}
	if statusOf(t, got, "sub").EstimatedAmount != 500000 {
		t.Error("상대 제도 금액이 바뀌었다")
	}
}

// 한쪽이 실제로 받는 게 아니면 깎이지 않는다.
func TestReducingOnlyWhenBothEligible(t *testing.T) {
	got := ResolveConflicts(
		[]model.MatchResult{
			res("main", model.MatchEligible, 1000000),
			res("sub", model.MatchNeedsInfo, 500000),
		},
		[]model.Relation{
			{From: "main", To: "sub", Type: model.RelationReducing, ReducePct: 30},
		},
	)

	if got := statusOf(t, got, "main").EstimatedAmount; got != 1000000 {
		t.Errorf("금액 = %d, 기대값 1000000 (깎이면 안 된다)", got)
	}
}

func TestResolveConflictsPrerequisite(t *testing.T) {
	tests := []struct {
		name       string
		baseStatus model.MatchStatus
		want       model.MatchStatus
	}{
		{"선행이 해당이면 그대로", model.MatchEligible, model.MatchEligible},
		{"선행이 확인필요면 후행도 확인필요", model.MatchNeedsInfo, model.MatchNeedsInfo},
		{"선행이 미해당이면 후행도 미해당", model.MatchIneligible, model.MatchIneligible},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveConflicts(
				[]model.MatchResult{
					res("child", model.MatchEligible, 100000),
					res("parent", tt.baseStatus, 200000),
				},
				[]model.Relation{
					{From: "child", To: "parent", Type: model.RelationPrerequisite},
				},
			)
			if s := statusOf(t, got, "child").Status; s != tt.want {
				t.Errorf("child 상태 = %s, 기대값 %s", s, tt.want)
			}
		})
	}
}

// A → B → C 연쇄에서 C 가 무너지면 A 까지 내려와야 한다.
func TestPrerequisiteChain(t *testing.T) {
	got := ResolveConflicts(
		[]model.MatchResult{
			res("a", model.MatchEligible, 100),
			res("b", model.MatchEligible, 100),
			res("c", model.MatchIneligible, 100),
		},
		[]model.Relation{
			{From: "a", To: "b", Type: model.RelationPrerequisite},
			{From: "b", To: "c", Type: model.RelationPrerequisite},
		},
	)

	if s := statusOf(t, got, "b").Status; s != model.MatchIneligible {
		t.Errorf("b = %s, 기대값 INELIGIBLE", s)
	}
	if s := statusOf(t, got, "a").Status; s != model.MatchIneligible {
		t.Errorf("a = %s, 기대값 INELIGIBLE (연쇄가 전파되지 않았다)", s)
	}
}

// 선행 제도가 결과에 아예 없으면 단정하지 않고 확인필요로 둔다.
func TestPrerequisiteMissingProgram(t *testing.T) {
	got := ResolveConflicts(
		[]model.MatchResult{res("child", model.MatchEligible, 100)},
		[]model.Relation{
			{From: "child", To: "안읽힌제도", Type: model.RelationPrerequisite},
		},
	)

	if s := statusOf(t, got, "child").Status; s != model.MatchNeedsInfo {
		t.Errorf("child = %s, 기대값 NEEDS_INFO", s)
	}
}

// 원본을 건드리지 않아야 재실행이 안전하다.
func TestResolveDoesNotMutateInput(t *testing.T) {
	input := []model.MatchResult{
		res("a", model.MatchEligible, 1000),
		res("b", model.MatchEligible, 2000),
	}
	ResolveConflicts(input, []model.Relation{
		{From: "a", To: "b", Type: model.RelationExclusive},
	})

	for _, r := range input {
		if r.Status != model.MatchEligible {
			t.Errorf("원본 %s 의 상태가 바뀌었다: %s", r.Program.ID, r.Status)
		}
	}
}

// 같은 입력이면 항상 같은 답이 나와야 한다. 판정은 결정론적이어야 한다.
func TestResolveIsDeterministic(t *testing.T) {
	build := func() ([]model.MatchResult, []model.Relation) {
		return []model.MatchResult{
			res("a", model.MatchEligible, 500000),
			res("b", model.MatchEligible, 500000),
			res("c", model.MatchEligible, 500000),
		}, []model.Relation{
			{From: "a", To: "b", Type: model.RelationExclusive},
			{From: "b", To: "c", Type: model.RelationExclusive},
			{From: "a", To: "c", Type: model.RelationExclusive},
		}
	}

	first := ResolveConflicts(build())
	for i := 0; i < 20; i++ {
		again := ResolveConflicts(build())
		if !equalStrings(first.Excluded, again.Excluded) {
			t.Fatalf("실행마다 답이 다르다: %v vs %v", first.Excluded, again.Excluded)
		}
	}
	// 셋이 서로 배타면 하나만 남는다
	if len(first.Excluded) != 2 {
		t.Errorf("제외 = %v, 기대값 2건", first.Excluded)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
