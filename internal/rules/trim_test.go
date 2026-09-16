package rules_test

import (
	"testing"

	"github.com/DUE-NAVIGATION/be/internal/model"
	"github.com/DUE-NAVIGATION/be/internal/rules"
)

func match(id string, st model.MatchStatus) model.FacilityMatch {
	f := hotline()
	f.ID = id
	return model.FacilityMatch{Facility: f, Status: st}
}

// ★ 갈 수 있는 곳은 하나도 버리지 않는다. 관할 밖만 줄인다.
func TestTrimOutOfScope(t *testing.T) {
	ms := []model.FacilityMatch{
		match("ok-1", model.MatchEligible),
		match("ask-1", model.MatchNeedsInfo),
		match("out-1", model.MatchIneligible),
		match("out-2", model.MatchIneligible),
		match("out-3", model.MatchIneligible),
	}

	got := rules.TrimOutOfScope(ms, 2)

	var ids []string
	for _, m := range got {
		ids = append(ids, m.Facility.ID)
	}
	want := []string{"ok-1", "ask-1", "out-1", "out-2"}
	if len(ids) != len(want) {
		t.Fatalf("남은 것 = %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Errorf("남은 것 = %v, want %v", ids, want)
			break
		}
	}
}

func TestTrimOutOfScopeKeepsEverythingUnderLimit(t *testing.T) {
	ms := []model.FacilityMatch{
		match("ok-1", model.MatchEligible),
		match("out-1", model.MatchIneligible),
	}
	if got := rules.TrimOutOfScope(ms, 20); len(got) != 2 {
		t.Errorf("한도보다 적으면 그대로여야 한다: %d건", len(got))
	}
}
