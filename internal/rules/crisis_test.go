package rules_test

import (
	"testing"

	"github.com/DUE-NAVIGATION/be/internal/model"
	"github.com/DUE-NAVIGATION/be/internal/rules"
)

func crisisLine(id string, offers ...model.CrisisSignal) model.Facility {
	f := hotline()
	f.ID = id
	f.Crisis = offers
	return f
}

// ★ 위기 신호는 판정을 바꾸지 않는다. 응답하는 곳을 표시할 뿐이다.
func TestCrisisSignalMarksUrgent(t *testing.T) {
	selfHarm := model.UserContext{CrisisSignals: []model.CrisisSignal{model.CrisisSelfHarm}}

	if m := rules.EvaluateFacility(crisisLine("109", model.CrisisSelfHarm), selfHarm); !m.Urgent {
		t.Error("자해 신호에 109 가 맨 위로 올라와야 한다")
	}
	if m := rules.EvaluateFacility(crisisLine("1366", model.CrisisViolence), selfHarm); m.Urgent {
		t.Error("다른 신호에 응답하는 곳은 올리지 않는다")
	}
	if m := rules.EvaluateFacility(crisisLine("109", model.CrisisSelfHarm), model.UserContext{}); m.Urgent {
		t.Error("신호가 없으면 아무것도 올리지 않는다")
	}
	m := rules.EvaluateFacility(crisisLine("109", model.CrisisSelfHarm), selfHarm)
	if m.Status != model.MatchEligible {
		t.Errorf("판정은 그대로여야 한다: %q", m.Status)
	}
}

// 갈 수 없는 곳(관할 밖)은 위기라도 올리지 않는다. 헛걸음이 된다.
func TestUrgentIsNotRaisedWhenOutOfScope(t *testing.T) {
	f := childCenter()
	f.Crisis = []model.CrisisSignal{model.CrisisViolence}
	m := rules.EvaluateFacility(f, model.UserContext{
		Region: strp("부산광역시"), District: strp("해운대구"), ChildrenAges: []int{7},
		CrisisSignals: []model.CrisisSignal{model.CrisisViolence},
	})
	if m.Status != model.MatchIneligible || m.Urgent {
		t.Errorf("관할 밖은 올리지 않는다: status=%q urgent=%v", m.Status, m.Urgent)
	}
}

func TestSortPutsUrgentFirst(t *testing.T) {
	always := hotline()
	always.ID = "a-always"
	plain := hotline()
	plain.ID = "b-urgent"
	plain.Contact.Always = false
	ms := []model.FacilityMatch{
		{Facility: always, Status: model.MatchEligible},
		{Facility: plain, Status: model.MatchEligible, Urgent: true},
	}
	rules.SortFacilities(ms)
	if !ms[0].Urgent {
		t.Errorf("위기 신호에 응답하는 곳이 24시간 창구보다도 먼저여야 한다: %s", ms[0].Facility.ID)
	}
}
