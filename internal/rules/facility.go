package rules

import (
	"sort"

	"github.com/DUE-NAVIGATION/be/internal/model"
)

// EvaluateFacility 는 시설 하나를 사용자 상황과 대조해 판정한다.
//
// ★ 제도 판정(Evaluate)과 규칙은 같다. 다른 점은 딱 둘이다.
//
//  1. 관할 지역이 조건 하나로 맨 앞에 붙는다. 시설은 물리적 장소라
//     관할을 벗어나면 나머지 조건이 다 맞아도 갈 수 없다.
//
//  2. 이용 대상 조건이 비어 있으면 "이용 가능" 이다.
//     제도는 조건이 비어 있으면 NEEDS_INFO 로 둔다 — 제도 JSON 이 덜 채워졌을
//     가능성이 크기 때문이다. 그런데 시설은 다르다. 지역아동센터처럼
//     "그 동네 살면 누구나" 인 곳이 실제로 많다. 그걸 확인필요로 돌리면
//     정작 갈 수 있는 곳을 못 찾는다.
//
// 순수 함수다 — 부수효과 없음, I/O 없음, panic 없음.
func EvaluateFacility(f model.Facility, ctx model.UserContext) model.FacilityMatch {
	coverage := coverageCondition(f, ctx)

	all := evaluateGroup(model.GroupAll, f.Eligibility.All, ctx)
	any := evaluateGroup(model.GroupAny, f.Eligibility.Any, ctx)
	none := evaluateGroup(model.GroupNone, f.Eligibility.None, ctx)

	evals := make([]evaluated, 0, 1+len(all)+len(any)+len(none))
	evals = append(evals, coverage)
	evals = append(evals, all...)
	evals = append(evals, any...)
	evals = append(evals, none...)

	conditions := make([]model.ConditionResult, 0, len(evals))
	for _, e := range evals {
		conditions = append(conditions, e.res)
	}

	status := decideFacility(f.Eligibility, coverage, all, any, none)

	missing := []string{}
	if status == model.MatchNeedsInfo {
		missing = missingFields(evals)
	}

	return model.FacilityMatch{
		Facility:      f,
		Status:        status,
		Conditions:    conditions,
		MissingFields: missing,
	}
}

// coverageCondition 은 관할 판정을 조건 하나로 만든다.
//
// ★ 관할을 코드 안에서 조용히 걸러내지 않고 조건으로 드러내는 이유:
// "왜 우리 동네 센터가 안 나오지" 를 화면이 스스로 답할 수 있어야 한다.
// 근거표에 "관악구 거주자 — 미해당(입력: 강남구)" 한 줄이 남는다.
func coverageCondition(f model.Facility, ctx model.UserContext) evaluated {
	status := f.Coverage.Serves(ctx)

	// 어느 필드를 물어야 하는지. 이 값이 MissingFields 로 나가 화면의
	// "어디 사시는지 알려주시면" 이 된다
	field := ""
	var actual any
	switch f.Coverage.Scope {
	case model.CoverageSido:
		field = model.FieldRegion
		if ctx.Region != nil {
			actual = *ctx.Region
		}
	case model.CoverageSigungu:
		field = model.FieldDistrict
		if ctx.District != nil {
			actual = *ctx.District
		} else if ctx.Region != nil {
			// 시도만 알고 시군구를 모르는 상태. 무엇으로 판단했는지는 남긴다
			actual = *ctx.Region
		}
	}

	res := model.ConditionResult{
		Condition: model.Condition{
			Field: field,
			Op:    model.OpEq,
			Label: f.Coverage.Label(),
		},
		Group:  model.GroupCoverage,
		Status: status,
		Actual: actual,
		Reason: coverageReason(f.Coverage, status, actual),
	}

	return evaluated{
		res: res,
		// 전국 대상 시설은 물어볼 것이 없다. MissingFields 를 오염시키지 않는다
		missingInput: status == model.StatusUnknown && field != "",
	}
}

func coverageReason(c model.Coverage, status model.ConditionStatus, actual any) string {
	switch status {
	case model.StatusPass:
		if c.Scope == model.CoverageNationwide {
			return "지역 제한이 없습니다"
		}
		return "관할 지역에 해당합니다"

	case model.StatusFail:
		if s, ok := actual.(string); ok && s != "" {
			return "관할 밖입니다 (입력: " + s + ")"
		}
		return "관할 밖입니다"

	case model.StatusUnknown:
		if c.Scope == model.CoverageSigungu {
			return "사시는 시군구가 입력되지 않아 판단할 수 없습니다"
		}
		return "사시는 지역이 입력되지 않아 판단할 수 없습니다"
	}
	return ""
}

// decideFacility 는 관할과 이용 대상 조건을 합쳐 최종 상태를 정한다.
//
// 우선순위는 제도와 같다 — 명시적 탈락이 확인필요보다 앞선다.
// 관할 밖이 확정이면 나머지를 보지 않는다. 갈 수 없는 곳이기 때문이다.
func decideFacility(
	e model.Eligibility,
	coverage evaluated,
	all, any, none []evaluated,
) model.MatchStatus {
	// ── 1. 관할이 먼저다 ────────────────────────────────────
	if coverage.res.Status == model.StatusFail {
		return model.MatchIneligible
	}

	// ── 2. 이용 대상 조건 ───────────────────────────────────
	// ★ 조건이 하나도 없으면 "그 지역 살면 누구나" 라는 뜻이다.
	//   제도와 달리 여기서는 정상적인 상태다
	hasConditions := len(e.All) > 0 || len(e.Any) > 0 || len(e.None) > 0
	if hasConditions {
		if count(none, model.StatusPass) > 0 {
			return model.MatchIneligible
		}
		if count(all, model.StatusFail) > 0 {
			return model.MatchIneligible
		}
		if len(any) > 0 &&
			count(any, model.StatusPass) == 0 &&
			count(any, model.StatusUnknown) == 0 {
			return model.MatchIneligible
		}
	}

	// ── 3. 확인 필요 ────────────────────────────────────────
	// 관할을 모르면 갈 수 있는지 단정할 수 없다
	if coverage.res.Status == model.StatusUnknown {
		return model.MatchNeedsInfo
	}
	if hasConditions {
		if count(none, model.StatusUnknown) > 0 {
			return model.MatchNeedsInfo
		}
		if count(all, model.StatusUnknown) > 0 {
			return model.MatchNeedsInfo
		}
		if len(any) > 0 && count(any, model.StatusPass) == 0 {
			return model.MatchNeedsInfo
		}
	}

	return model.MatchEligible
}

// SummarizeFacilities 는 시설 결과의 요약을 만든다.
//
// ReachableNow 는 "지금 바로 전화할 수 있는 곳" 의 수다.
// 이용 가능한데 연락할 방법이 없으면 이 서비스에서는 의미가 없다.
func SummarizeFacilities(ms []model.FacilityMatch) model.FacilitySummary {
	var s model.FacilitySummary
	for _, m := range ms {
		switch m.Status {
		case model.MatchEligible:
			s.AvailableCount++
			if m.Facility.Contact.Phone != "" {
				s.ReachableNow++
			}
		case model.MatchNeedsInfo:
			s.NeedsInfoCount++
		case model.MatchIneligible:
			s.OutOfScope++
		}
	}
	return s
}

// SortFacilities 는 화면에 보일 순서로 정렬한다.
//
//	이용 가능 → 확인 필요 → 관할 밖,
//	같은 상태 안에서는 24시간 운영 → 전화 가능 → 이름 순.
//
// ★ 24시간 창구를 위로 올리는 이유: 밤에 급한 사람이 이 화면을 본다.
// 낮에만 여는 복지관을 먼저 보여주면 그 사람에게는 아무 소용이 없다.
func SortFacilities(ms []model.FacilityMatch) {
	sort.SliceStable(ms, func(i, j int) bool {
		a, b := ms[i], ms[j]
		if sa, sb := facilityOrder(a.Status), facilityOrder(b.Status); sa != sb {
			return sa < sb
		}
		if a.Facility.Contact.Always != b.Facility.Contact.Always {
			return a.Facility.Contact.Always
		}
		ap := a.Facility.Contact.Phone != ""
		bp := b.Facility.Contact.Phone != ""
		if ap != bp {
			return ap
		}
		// 끝까지 같으면 id 로 고정한다. 순서가 실행마다 달라지면 안 된다
		return a.Facility.ID < b.Facility.ID
	})
}

func facilityOrder(s model.MatchStatus) int {
	switch s {
	case model.MatchEligible:
		return 0
	case model.MatchNeedsInfo:
		return 1
	}
	return 2
}
