package rules_test

import (
	"testing"

	"github.com/DUE-NAVIGATION/be/internal/model"
	"github.com/DUE-NAVIGATION/be/internal/rules"
)

func strp(s string) *string { return &s }
func intp(n int) *int       { return &n }

// 관악구 지역아동센터. 그 동네 살고 18세 미만 자녀가 있으면 이용할 수 있다.
func childCenter() model.Facility {
	return model.Facility{
		ID:   "gwanak-child-center",
		Name: "관악 지역아동센터",
		Type: model.FacilityChildCenter,
		Coverage: model.Coverage{
			Scope:   model.CoverageSigungu,
			Sido:    "서울특별시",
			Sigungu: "관악구",
		},
		Eligibility: model.Eligibility{
			All: []model.Condition{
				{Field: model.FieldChildrenAges, Op: model.OpExists, Label: "양육 중인 자녀"},
			},
		},
		Contact: model.Contact{Phone: "02-000-0000"},
	}
}

// 전국 상담 창구. 조건이 없다 — 누구나 전화할 수 있다.
func hotline() model.Facility {
	return model.Facility{
		ID:       "hotline-129",
		Name:     "보건복지상담센터",
		Type:     model.FacilityHotline,
		Coverage: model.Coverage{Scope: model.CoverageNationwide},
		Contact:  model.Contact{Phone: "129", Always: true},
	}
}

// ★ 관할 판정의 세 상태. 이 표가 시설 기능의 핵심이다.
//
// 특히 UNKNOWN 이 FAIL 로 새면 안 된다 — 거주지를 안 적었다는 이유로
// 갈 수 있는 곳이 "해당 없음" 으로 사라지면 이 서비스는 실패한다.
func TestCoverageServes(t *testing.T) {
	tests := []struct {
		name     string
		coverage model.Coverage
		ctx      model.UserContext
		want     model.ConditionStatus
	}{
		{
			name:     "전국 대상은 지역을 몰라도 통과",
			coverage: model.Coverage{Scope: model.CoverageNationwide},
			ctx:      model.UserContext{},
			want:     model.StatusPass,
		},
		{
			name:     "시도 일치",
			coverage: model.Coverage{Scope: model.CoverageSido, Sido: "서울특별시"},
			ctx:      model.UserContext{Region: strp("서울특별시")},
			want:     model.StatusPass,
		},
		{
			name:     "시도 불일치는 관할 밖",
			coverage: model.Coverage{Scope: model.CoverageSido, Sido: "서울특별시"},
			ctx:      model.UserContext{Region: strp("부산광역시")},
			want:     model.StatusFail,
		},
		{
			name:     "★ 시도를 모르면 관할 밖이 아니라 확인 필요",
			coverage: model.Coverage{Scope: model.CoverageSido, Sido: "서울특별시"},
			ctx:      model.UserContext{},
			want:     model.StatusUnknown,
		},
		{
			name: "시군구 일치",
			coverage: model.Coverage{
				Scope: model.CoverageSigungu, Sido: "서울특별시", Sigungu: "관악구",
			},
			ctx: model.UserContext{
				Region: strp("서울특별시"), District: strp("관악구"),
			},
			want: model.StatusPass,
		},
		{
			name: "같은 시도 다른 시군구는 관할 밖",
			coverage: model.Coverage{
				Scope: model.CoverageSigungu, Sido: "서울특별시", Sigungu: "관악구",
			},
			ctx: model.UserContext{
				Region: strp("서울특별시"), District: strp("강남구"),
			},
			want: model.StatusFail,
		},
		{
			name: "★ 시도가 이미 다르면 시군구를 몰라도 관할 밖이 확정",
			coverage: model.Coverage{
				Scope: model.CoverageSigungu, Sido: "서울특별시", Sigungu: "관악구",
			},
			ctx:  model.UserContext{Region: strp("부산광역시")},
			want: model.StatusFail,
		},
		{
			name: "★ 시도만 맞고 시군구를 모르면 확인 필요",
			coverage: model.Coverage{
				Scope: model.CoverageSigungu, Sido: "서울특별시", Sigungu: "관악구",
			},
			ctx:  model.UserContext{Region: strp("서울특별시")},
			want: model.StatusUnknown,
		},
		{
			name: "아무것도 모르면 확인 필요",
			coverage: model.Coverage{
				Scope: model.CoverageSigungu, Sido: "서울특별시", Sigungu: "관악구",
			},
			ctx:  model.UserContext{},
			want: model.StatusUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.coverage.Serves(tt.ctx); got != tt.want {
				t.Errorf("Serves() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEvaluateFacility(t *testing.T) {
	tests := []struct {
		name string
		f    model.Facility
		ctx  model.UserContext
		want model.MatchStatus
	}{
		{
			name: "관할 안 + 조건 충족 → 이용 가능",
			f:    childCenter(),
			ctx: model.UserContext{
				Region: strp("서울특별시"), District: strp("관악구"),
				ChildrenAges: []int{7},
			},
			want: model.MatchEligible,
		},
		{
			name: "★ 관할 밖이면 조건이 다 맞아도 갈 수 없다",
			f:    childCenter(),
			ctx: model.UserContext{
				Region: strp("서울특별시"), District: strp("강남구"),
				ChildrenAges: []int{7},
			},
			want: model.MatchIneligible,
		},
		{
			name: "★ 거주지를 모르면 확인 필요 — 숨기지 않는다",
			f:    childCenter(),
			ctx:  model.UserContext{ChildrenAges: []int{7}},
			want: model.MatchNeedsInfo,
		},
		{
			name: "관할 안이지만 이용 대상 조건을 모르면 확인 필요",
			f:    childCenter(),
			ctx: model.UserContext{
				Region: strp("서울특별시"), District: strp("관악구"),
			},
			want: model.MatchNeedsInfo,
		},
		{
			name: "★ 이용 대상 조건이 없는 전국 창구는 아무 정보 없이도 이용 가능",
			f:    hotline(),
			ctx:  model.UserContext{},
			want: model.MatchEligible,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rules.EvaluateFacility(tt.f, tt.ctx)
			if got.Status != tt.want {
				t.Errorf("Status = %q, want %q", got.Status, tt.want)
			}
			// 근거는 언제나 남는다. 첫 줄은 관할이다
			if len(got.Conditions) == 0 {
				t.Fatal("근거가 비었다")
			}
			if got.Conditions[0].Reason == "" {
				t.Error("관할 조건에 사유가 없다")
			}
		})
	}
}

// ★ "어디 사시는지 알려주시면" 을 화면이 말할 수 있어야 한다.
func TestEvaluateFacilityAsksForRegion(t *testing.T) {
	got := rules.EvaluateFacility(childCenter(),
		model.UserContext{ChildrenAges: []int{7}})

	if got.Status != model.MatchNeedsInfo {
		t.Fatalf("Status = %q", got.Status)
	}

	var asksDistrict bool
	for _, f := range got.MissingFields {
		if f == model.FieldDistrict {
			asksDistrict = true
		}
	}
	if !asksDistrict {
		t.Errorf("시군구를 물어야 하는데 MissingFields = %v", got.MissingFields)
	}
}

// 전국 창구는 물어볼 것이 없다. MissingFields 가 오염되면 화면이
// "지역을 알려주세요" 를 쓸데없이 띄운다.
func TestNationwideFacilityAsksNothing(t *testing.T) {
	got := rules.EvaluateFacility(hotline(), model.UserContext{})
	if len(got.MissingFields) != 0 {
		t.Errorf("MissingFields = %v, want 빈 목록", got.MissingFields)
	}
}

// ★ 밤에 급한 사람이 이 화면을 본다. 24시간 창구가 위로 와야 한다.
func TestSortPutsAlwaysOpenFirst(t *testing.T) {
	daytime := hotline()
	daytime.ID = "daytime"
	daytime.Contact.Always = false

	ms := []model.FacilityMatch{
		{Facility: daytime, Status: model.MatchEligible},
		{Facility: hotline(), Status: model.MatchEligible},
	}
	rules.SortFacilities(ms)

	if ms[0].Facility.ID != "hotline-129" {
		t.Errorf("맨 앞 = %q, want 24시간 창구", ms[0].Facility.ID)
	}
}

func TestSortPutsAvailableBeforeOutOfScope(t *testing.T) {
	ms := []model.FacilityMatch{
		{Facility: childCenter(), Status: model.MatchIneligible},
		{Facility: hotline(), Status: model.MatchNeedsInfo},
		{Facility: childCenter(), Status: model.MatchEligible},
	}
	rules.SortFacilities(ms)

	want := []model.MatchStatus{
		model.MatchEligible, model.MatchNeedsInfo, model.MatchIneligible,
	}
	for i, w := range want {
		if ms[i].Status != w {
			t.Errorf("%d번째 = %q, want %q", i, ms[i].Status, w)
		}
	}
}

func TestSummarizeFacilities(t *testing.T) {
	noPhone := childCenter()
	noPhone.ID = "no-phone"
	noPhone.Contact = model.Contact{Website: "https://example.kr"}

	got := rules.SummarizeFacilities([]model.FacilityMatch{
		{Facility: hotline(), Status: model.MatchEligible},
		{Facility: noPhone, Status: model.MatchEligible},
		{Facility: childCenter(), Status: model.MatchNeedsInfo},
		{Facility: childCenter(), Status: model.MatchIneligible},
	})

	if got.AvailableCount != 2 {
		t.Errorf("AvailableCount = %d, want 2", got.AvailableCount)
	}
	if got.NeedsInfoCount != 1 {
		t.Errorf("NeedsInfoCount = %d, want 1", got.NeedsInfoCount)
	}
	if got.OutOfScope != 1 {
		t.Errorf("OutOfScope = %d, want 1", got.OutOfScope)
	}
	// ★ 전화번호가 없는 곳은 "지금 연락 가능" 에 세지 않는다
	if got.ReachableNow != 1 {
		t.Errorf("ReachableNow = %d, want 1 (전화 있는 곳만)", got.ReachableNow)
	}
}

// 나이 조건이 있는 시설에서 나이를 모르면 확인 필요여야 한다.
// (제도 판정과 규칙이 같은지 확인)
func TestFacilityUnknownAgeIsNeedsInfo(t *testing.T) {
	f := hotline()
	f.Eligibility = model.Eligibility{
		All: []model.Condition{
			{Field: model.FieldAge, Op: model.OpGte, Value: 65, Label: "만 65세 이상"},
		},
	}

	if got := rules.EvaluateFacility(f, model.UserContext{}); got.Status != model.MatchNeedsInfo {
		t.Errorf("Status = %q, want NEEDS_INFO", got.Status)
	}

	old := intp(70)
	if got := rules.EvaluateFacility(f, model.UserContext{Age: old}); got.Status != model.MatchEligible {
		t.Errorf("Status = %q, want ELIGIBLE", got.Status)
	}
}
