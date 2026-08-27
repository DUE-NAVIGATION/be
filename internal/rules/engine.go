package rules

import (
	"fmt"

	"github.com/DUE-NAVIGATION/be/internal/model"
)

// evaluated 는 조건 하나의 판정 결과와, 그 판정을 왜 내렸는지에 대한 내부 표시다.
//
// UNKNOWN 은 두 가지 이유로 발생하는데 둘을 반드시 구분해야 한다.
//
//	사용자 입력 부족 → 사용자에게 물어본다 (MissingFields 에 담는다)
//	제도 JSON 오류   → 사용자에게 물어볼 수 없다. 우리 데이터 버그다
//
// "incomeMontly 를 알려주세요" 라고 물을 수는 없다.
type evaluated struct {
	res          model.ConditionResult
	missingInput bool
}

// Evaluate 는 제도 하나를 사용자 상황과 대조해 판정한다.
//
// ★ 이 함수가 이 프로젝트의 심장이다. 판정은 여기서만 일어난다.
// 순수 함수다 — 부수효과 없음, I/O 없음, 전역 상태 없음, panic 없음.
//
// 판정 규칙
//
//	All  전부 PASS 여야 통과 (AND)
//	Any  하나라도 PASS 면 통과 (OR)
//	None 하나라도 PASS 면 탈락 (배제 조건)
//
// ★ 값이 없는 조건은 FAIL 이 아니라 UNKNOWN 이다. 값이 없다고 탈락시키지 않는다.
// UNKNOWN 이 남으면 결과는 NEEDS_INFO 이고, 부족한 필드가 MissingFields 에 담긴다.
//
// 우선순위: 명시적 탈락(INELIGIBLE)이 확인필요(NEEDS_INFO)보다 앞선다.
// 나이가 확실히 초과라면 소득을 몰라도 그 제도는 해당되지 않는다.
func Evaluate(p model.Program, ctx model.UserContext) model.MatchResult {
	all := evaluateGroup(p.Eligibility.All, ctx)
	any := evaluateGroup(p.Eligibility.Any, ctx)
	none := evaluateGroup(p.Eligibility.None, ctx)

	evals := make([]evaluated, 0, len(all)+len(any)+len(none))
	evals = append(evals, all...)
	evals = append(evals, any...)
	evals = append(evals, none...)

	conditions := make([]model.ConditionResult, 0, len(evals))
	for _, e := range evals {
		conditions = append(conditions, e.res)
	}

	status := decide(p.Eligibility, all, any, none)

	// 부족한 입력값은 "확인 필요" 일 때만 의미가 있다.
	// 이미 탈락이 확정된 제도에 대해 사용자에게 더 묻지 않는다.
	missing := []string{}
	if status == model.MatchNeedsInfo {
		missing = missingFields(evals)
	}

	return model.MatchResult{
		Program:       p,
		Status:        status,
		Conditions:    conditions,
		MissingFields: missing,
		// 금액은 Phase 2 의 Estimate 가 채운다. 여기서 계산하지 않는다.
		EstimatedAmount: 0,
	}
}

// decide 는 그룹별 판정 결과를 모아 최종 상태를 정한다.
func decide(e model.Eligibility, all, any, none []evaluated) model.MatchStatus {
	// 자격요건이 하나도 없으면 판정 근거가 없다.
	// ELIGIBLE 로 단정하지 않는다 — 제도 JSON 이 덜 채워졌을 가능성이 크다.
	if len(e.All) == 0 && len(e.Any) == 0 && len(e.None) == 0 {
		return model.MatchNeedsInfo
	}

	// ── 1. 명시적 탈락이 최우선 ──────────────────────────────
	// 배제 조건이 하나라도 성립하면 탈락이다.
	if count(none, model.StatusPass) > 0 {
		return model.MatchIneligible
	}
	// All 중 하나라도 확실히 실패하면 탈락이다.
	if count(all, model.StatusFail) > 0 {
		return model.MatchIneligible
	}
	// Any 가 있는데 전부 확실히 실패했으면 탈락이다.
	if len(any) > 0 &&
		count(any, model.StatusPass) == 0 &&
		count(any, model.StatusUnknown) == 0 {
		return model.MatchIneligible
	}

	// ── 2. 확인 필요 ────────────────────────────────────────
	// 배제 조건을 판정하지 못했다면 배제 여부를 단정할 수 없다.
	if count(none, model.StatusUnknown) > 0 {
		return model.MatchNeedsInfo
	}
	if count(all, model.StatusUnknown) > 0 {
		return model.MatchNeedsInfo
	}
	// Any 는 하나라도 PASS 면 나머지를 몰라도 통과다.
	if len(any) > 0 && count(any, model.StatusPass) == 0 {
		return model.MatchNeedsInfo
	}

	return model.MatchEligible
}

func evaluateGroup(conds []model.Condition, ctx model.UserContext) []evaluated {
	out := make([]evaluated, 0, len(conds))
	for _, c := range conds {
		out = append(out, evaluateCondition(c, ctx))
	}
	return out
}

func count(es []evaluated, s model.ConditionStatus) int {
	n := 0
	for _, e := range es {
		if e.res.Status == s {
			n++
		}
	}
	return n
}

// missingFields 는 사용자에게 더 물어봐야 할 필드를 순서를 유지한 채 중복 없이 모은다.
func missingFields(es []evaluated) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, e := range es {
		if !e.missingInput {
			continue
		}
		f := e.res.Condition.Field
		if seen[f] {
			continue
		}
		seen[f] = true
		out = append(out, f)
	}
	return out
}

// evaluateCondition 은 조건 하나를 판정한다.
//
// 반드시 Reason 을 채운다 — 사용자에게 "왜 해당/미해당인지" 를 조건 단위로
// 보여줘야 하기 때문이다. 이게 이 서비스의 설명 가능성이다.
func evaluateCondition(c model.Condition, ctx model.UserContext) evaluated {
	actual, known, exists := model.Lookup(ctx, c.Field)

	// 제도 JSON 이 없는 필드를 참조한다. 데이터 버그이므로 사용자에게 묻지 않는다.
	if !exists {
		return dataError(c, nil, fmt.Sprintf("제도 데이터 오류: '%s' 는 없는 필드입니다", c.Field))
	}
	if !model.OpIsKnown(c.Op) {
		return dataError(c, actualOrNil(actual, known),
			fmt.Sprintf("제도 데이터 오류: '%s' 는 알 수 없는 연산자입니다", c.Op))
	}

	// exists 는 "값을 아는가" 를 묻는다.
	// 모른다고 해서 "없다" 로 단정하지 않는다 — 우리가 모르는 것과 실제로 없는 것은 다르다.
	if c.Op == model.OpExists {
		if known {
			return pass(c, actual, "값이 입력되었습니다")
		}
		return needsInput(c)
	}

	if !known {
		return needsInput(c)
	}

	ok, reason, decidable := apply(c.Op, actual, c.Value)
	if !decidable {
		// 값은 아는데 판정이 안 된다 = 제도 JSON 이 잘못됐거나 타입이 안 맞는다.
		// 사용자에게 다시 물어도 해결되지 않는다.
		return dataError(c, actual, reason)
	}
	if ok {
		return pass(c, actual, reason)
	}
	return fail(c, actual, reason)
}

// apply 는 연산자를 실행한다.
//
// 반환값
//
//	ok        조건을 충족하는가
//	reason    사람 말 사유. 항상 채운다
//	decidable 판정할 수 있었는가. false 면 UNKNOWN 이다 (타입 불일치 등)
func apply(op model.Op, actual, value any) (ok bool, reason string, decidable bool) {
	switch op {
	case model.OpBetween:
		lo, hi, err := pair(value)
		if err != nil {
			return false, "제도 데이터 오류: " + err.Error(), false
		}
		n, isNum := number(actual)
		if !isNum {
			return false, fmt.Sprintf("판정 불가 — 숫자가 아닌 값(%v)에 범위 조건을 쓸 수 없습니다", actual), false
		}
		// 양 끝을 포함한다. "만 19~34세" 는 19세와 34세를 포함한다.
		if n >= lo && n <= hi {
			return true, fmt.Sprintf("%s 은(는) %s ~ %s 범위 안입니다", num(n), num(lo), num(hi)), true
		}
		return false, fmt.Sprintf("%s 은(는) %s ~ %s 범위를 벗어납니다", num(n), num(lo), num(hi)), true

	case model.OpLte, model.OpGte:
		n, isNum := number(actual)
		if !isNum {
			return false, fmt.Sprintf("판정 불가 — 숫자가 아닌 값(%v)에 대소 조건을 쓸 수 없습니다", actual), false
		}
		limit, isLimitNum := number(value)
		if !isLimitNum {
			return false, "제도 데이터 오류: 비교값이 숫자가 아닙니다", false
		}
		if op == model.OpLte {
			if n <= limit {
				return true, fmt.Sprintf("%s 은(는) %s 이하입니다", num(n), num(limit)), true
			}
			return false, fmt.Sprintf("%s 은(는) %s 을(를) 초과합니다", num(n), num(limit)), true
		}
		if n >= limit {
			return true, fmt.Sprintf("%s 은(는) %s 이상입니다", num(n), num(limit)), true
		}
		return false, fmt.Sprintf("%s 은(는) %s 에 미치지 못합니다", num(n), num(limit)), true

	case model.OpEq:
		eq, cmp := equal(actual, value)
		if !cmp {
			return false, fmt.Sprintf("판정 불가 — %v 와(과) %v 는 비교할 수 없습니다", actual, value), false
		}
		if eq {
			return true, fmt.Sprintf("%v 와(과) 일치합니다", value), true
		}
		return false, fmt.Sprintf("%v 이(가) 아닙니다 (입력: %v)", value, actual), true

	case model.OpIn:
		list, isList := slice(value)
		if !isList {
			return false, "제도 데이터 오류: in 의 value 는 배열이어야 합니다", false
		}
		for _, v := range list {
			if eq, cmp := equal(actual, v); cmp && eq {
				return true, fmt.Sprintf("%v 은(는) 허용 목록에 있습니다", actual), true
			}
		}
		return false, fmt.Sprintf("%v 은(는) 허용 목록에 없습니다", actual), true

	case model.OpContains:
		items, isList := slice(actual)
		if !isList {
			return false, fmt.Sprintf("판정 불가 — %v 는 목록이 아니라서 포함 여부를 볼 수 없습니다", actual), false
		}
		if value == nil {
			return false, "제도 데이터 오류: contains 의 value 가 비었습니다", false
		}
		for _, item := range items {
			if eq, cmp := equal(item, value); cmp && eq {
				return true, fmt.Sprintf("%v 을(를) 포함합니다", value), true
			}
		}
		return false, fmt.Sprintf("%v 을(를) 포함하지 않습니다", value), true
	}

	// OpIsKnown 을 통과했으므로 여기 오지 않는다. 방어적으로만 둔다.
	return false, fmt.Sprintf("판정 불가 — 처리되지 않은 연산자 '%s'", op), false
}

// ── 값 변환 헬퍼 ────────────────────────────────────────────
// 제도 JSON 을 거치면 숫자는 float64, 배열은 []any 가 된다.
// Go 리터럴로 쓴 값과 JSON 에서 온 값이 똑같이 동작해야 한다.

// number 는 어떤 수치 표현이든 float64 로 바꾼다.
func number(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	}
	return 0, false
}

// pair 는 between 의 value 를 [최소, 최대] 로 푼다.
func pair(v any) (lo, hi float64, err error) {
	list, ok := slice(v)
	if !ok {
		return 0, 0, fmt.Errorf("between 의 value 는 배열이어야 합니다")
	}
	if len(list) != 2 {
		return 0, 0, fmt.Errorf("between 의 value 는 원소가 2개여야 합니다 (현재 %d개)", len(list))
	}
	lo, okLo := number(list[0])
	hi, okHi := number(list[1])
	if !okLo || !okHi {
		return 0, 0, fmt.Errorf("between 의 value 는 숫자 2개여야 합니다")
	}
	if lo > hi {
		return 0, 0, fmt.Errorf("between 의 최소값이 최대값보다 큽니다")
	}
	return lo, hi, nil
}

// slice 는 어떤 배열 표현이든 []any 로 바꾼다.
func slice(v any) ([]any, bool) {
	switch s := v.(type) {
	case []any:
		return s, true
	case []string:
		out := make([]any, len(s))
		for i, x := range s {
			out[i] = x
		}
		return out, true
	case []int64:
		out := make([]any, len(s))
		for i, x := range s {
			out[i] = x
		}
		return out, true
	case []int:
		out := make([]any, len(s))
		for i, x := range s {
			out[i] = x
		}
		return out, true
	case []float64:
		out := make([]any, len(s))
		for i, x := range s {
			out[i] = x
		}
		return out, true
	}
	return nil, false
}

// equal 은 타입 표현 차이를 흡수해 비교한다.
// cmp=false 면 애초에 비교가 성립하지 않는 조합이다.
func equal(a, b any) (eq bool, cmp bool) {
	if na, ok := number(a); ok {
		if nb, ok := number(b); ok {
			return na == nb, true
		}
		return false, false
	}
	if sa, ok := a.(string); ok {
		if sb, ok := b.(string); ok {
			return sa == sb, true
		}
		return false, false
	}
	if ba, ok := a.(bool); ok {
		if bb, ok := b.(bool); ok {
			return ba == bb, true
		}
		return false, false
	}
	return false, false
}

// num 은 금액·나이를 사람이 읽는 형태로 만든다.
//
// 정수면 소수점을 붙이지 않고, 소수는 첫째 자리에서 끊는다.
// ★ 화면에 보이는 문구에만 쓴다. 판정 비교는 원래 정밀도로 한다 —
// "19.05083047332741 은(는) 65 이하입니다" 같은 문장이 사용자에게 보이면 안 된다.
func num(f float64) string {
	if f == float64(int64(f)) {
		return fmt.Sprintf("%d", int64(f))
	}
	return fmt.Sprintf("%.1f", f)
}

// ── 판정 결과 생성기 ────────────────────────────────────────

func pass(c model.Condition, actual any, reason string) evaluated {
	return evaluated{res: model.ConditionResult{
		Condition: c, Status: model.StatusPass, Actual: actual, Reason: reason,
	}}
}

func fail(c model.Condition, actual any, reason string) evaluated {
	return evaluated{res: model.ConditionResult{
		Condition: c, Status: model.StatusFail, Actual: actual, Reason: reason,
	}}
}

// needsInput 은 사용자가 아직 알려주지 않은 값이다. 물어보면 해결된다.
func needsInput(c model.Condition) evaluated {
	return evaluated{
		res: model.ConditionResult{
			Condition: c, Status: model.StatusUnknown, Reason: "확인 필요 — 입력되지 않았습니다",
		},
		missingInput: true,
	}
}

// dataError 는 제도 JSON 이 잘못된 경우다. 사용자에게 물어도 해결되지 않는다.
// 서버를 죽이지 않고 UNKNOWN 으로 처리하되, MissingFields 에는 넣지 않는다.
func dataError(c model.Condition, actual any, reason string) evaluated {
	return evaluated{res: model.ConditionResult{
		Condition: c, Status: model.StatusUnknown, Actual: actual, Reason: reason,
	}}
}

func actualOrNil(actual any, known bool) any {
	if known {
		return actual
	}
	return nil
}
