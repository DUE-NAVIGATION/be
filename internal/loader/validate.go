package loader

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/DUE-NAVIGATION/be/internal/model"
)

// 개정일 형식. YYYY-MM-DD 만 받는다.
var revisedAtPattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// ValidateProgram 은 제도 하나를 검사해 문제를 사람 말로 돌려준다.
//
// ★ 이 함수는 로더와 검증 CLI 가 함께 쓴다.
// 두 곳에 규칙을 따로 두면 "validate 는 통과했는데 서버가 안 읽는" 상황이 생긴다.
//
// 에러를 error 가 아니라 []string 으로 돌려주는 이유:
// 첫 번째 문제에서 멈추면 팀원이 고치고 다시 돌리기를 반복해야 한다.
// 한 번에 다 보여줘야 한 번에 고칠 수 있다.
func ValidateProgram(p model.Program) []string {
	var errs []string

	// ── 필수 항목 ────────────────────────────────────────────
	if strings.TrimSpace(p.ID) == "" {
		errs = append(errs, "id 가 없습니다")
	}
	if strings.TrimSpace(p.Name) == "" {
		errs = append(errs, "name 이 없습니다")
	}
	if p.Category == "" {
		errs = append(errs, "category 가 없습니다")
	} else if !isKnownCategory(p.Category) {
		errs = append(errs, fmt.Sprintf("category %q 를 모릅니다. 쓸 수 있는 값: %s",
			p.Category, strings.Join(categoryNames(), ", ")))
	}

	// ── 출처 — 심사에서 반드시 물어본다 ──────────────────────
	if strings.TrimSpace(p.Source.URL) == "" {
		errs = append(errs, "source.url 이 없습니다 (공식 안내 페이지 주소)")
	}
	if strings.TrimSpace(p.Source.RevisedAt) == "" {
		errs = append(errs, "source.revisedAt 이 없습니다 (개정일. 심사에서 물어봅니다)")
	} else if !revisedAtPattern.MatchString(p.Source.RevisedAt) {
		errs = append(errs, fmt.Sprintf("source.revisedAt %q 는 YYYY-MM-DD 형식이어야 합니다",
			p.Source.RevisedAt))
	}

	// ── 자격요건 ────────────────────────────────────────────
	e := p.Eligibility
	if len(e.All) == 0 && len(e.Any) == 0 && len(e.None) == 0 {
		errs = append(errs, "eligibility 가 비었습니다 (all/any/none 중 하나는 있어야 합니다)")
	}
	errs = append(errs, validateConditions("all", e.All)...)
	errs = append(errs, validateConditions("any", e.Any)...)
	errs = append(errs, validateConditions("none", e.None)...)

	// ── 급여 ────────────────────────────────────────────────
	errs = append(errs, validateBenefit(p.Benefit)...)

	return errs
}

func validateConditions(group string, conds []model.Condition) []string {
	var errs []string

	for i, c := range conds {
		where := fmt.Sprintf("eligibility.%s[%d]", group, i)

		// ★ 가장 흔한 실수. 오타 난 필드는 판정에서 조용히 "확인 필요" 로 새기 때문에
		// 사람 눈으로는 발견되지 않는다. 여기서 잡지 못하면 아무도 못 잡는다
		if c.Field == "" {
			errs = append(errs, fmt.Sprintf("%s: field 가 비었습니다", where))
		} else if !model.FieldExists(c.Field) {
			errs = append(errs, fmt.Sprintf("%s: field %q 는 없는 항목입니다.%s",
				where, c.Field, suggestField(c.Field)))
		}

		if c.Op == "" {
			errs = append(errs, fmt.Sprintf("%s: op 가 비었습니다", where))
			continue
		}
		if !model.OpIsKnown(c.Op) {
			errs = append(errs, fmt.Sprintf("%s: op %q 를 모릅니다. 쓸 수 있는 값: %s",
				where, c.Op, strings.Join(opNames(), ", ")))
			continue
		}

		errs = append(errs, validateConditionValue(where, c)...)
	}

	return errs
}

func validateConditionValue(where string, c model.Condition) []string {
	var errs []string

	switch c.Op {
	case model.OpExists:
		// value 를 보지 않는다. 있어도 무시된다

	case model.OpBetween:
		list, ok := c.Value.([]any)
		if !ok {
			errs = append(errs, fmt.Sprintf("%s: between 의 value 는 [최소, 최대] 배열이어야 합니다", where))
			break
		}
		if len(list) != 2 {
			errs = append(errs, fmt.Sprintf("%s: between 의 value 는 원소가 2개여야 합니다 (현재 %d개)",
				where, len(list)))
			break
		}
		lo, okLo := list[0].(float64)
		hi, okHi := list[1].(float64)
		if !okLo || !okHi {
			errs = append(errs, fmt.Sprintf("%s: between 의 value 는 숫자 2개여야 합니다", where))
			break
		}
		if lo > hi {
			errs = append(errs, fmt.Sprintf("%s: between 의 최소값(%v)이 최대값(%v)보다 큽니다", where, lo, hi))
		}

	case model.OpLte, model.OpGte:
		if _, ok := c.Value.(float64); !ok {
			errs = append(errs, fmt.Sprintf("%s: %s 의 value 는 숫자여야 합니다", where, c.Op))
		}

	case model.OpIn:
		list, ok := c.Value.([]any)
		if !ok {
			errs = append(errs, fmt.Sprintf("%s: in 의 value 는 배열이어야 합니다", where))
			break
		}
		if len(list) == 0 {
			errs = append(errs, fmt.Sprintf("%s: in 의 value 가 빈 배열입니다", where))
		}

	case model.OpEq, model.OpContains:
		if c.Value == nil {
			errs = append(errs, fmt.Sprintf("%s: %s 의 value 가 없습니다", where, c.Op))
			break
		}
		if _, isList := c.Value.([]any); isList {
			errs = append(errs, fmt.Sprintf("%s: %s 의 value 는 배열이 아니라 값 하나여야 합니다 (배열은 in 을 쓰세요)",
				where, c.Op))
		}
	}

	return errs
}

func validateBenefit(b model.Benefit) []string {
	var errs []string

	switch b.Type {
	case "":
		errs = append(errs, "benefit.type 이 없습니다")

	case model.BenefitMonthly:
		if b.Amount <= 0 {
			errs = append(errs, "benefit.amount 는 0보다 커야 합니다 (월 지급액)")
		}
		if b.Months < 0 {
			errs = append(errs, "benefit.months 가 음수입니다")
		}

	case model.BenefitOnce, model.BenefitYearly:
		if b.Amount <= 0 {
			errs = append(errs, "benefit.amount 는 0보다 커야 합니다")
		}

	case model.BenefitRate:
		if b.RatePct <= 0 || b.RatePct > 100 {
			errs = append(errs, "benefit.ratePct 는 0 초과 100 이하여야 합니다 (감면율 %)")
		}

	case model.BenefitInKind:
		if strings.TrimSpace(b.Note) == "" {
			errs = append(errs, "benefit.note 에 어떤 현물·서비스인지 적어주세요 (금액이 없으므로 설명이 필요합니다)")
		}

	default:
		errs = append(errs, fmt.Sprintf("benefit.type %q 를 모릅니다. 쓸 수 있는 값: MONTHLY, ONCE, YEARLY, RATE, IN_KIND",
			b.Type))
	}

	return errs
}

// suggestField 는 오타로 보이는 필드에 대해 비슷한 이름을 귀띔한다.
// "incomeMontly" 같은 실수를 눈으로 찾는 건 사람에게 특히 어렵다.
func suggestField(typo string) string {
	var near []string
	for _, f := range model.KnownFields() {
		if editDistanceWithin(strings.ToLower(typo), strings.ToLower(f), 2) {
			near = append(near, f)
		}
	}
	if len(near) == 0 {
		return fmt.Sprintf(" 쓸 수 있는 항목: %s", strings.Join(model.KnownFields(), ", "))
	}
	return fmt.Sprintf(" 혹시 %s 인가요?", strings.Join(near, " 또는 "))
}

// editDistanceWithin 은 두 문자열의 편집 거리가 max 이하인지 본다.
// 완전한 거리 값이 필요 없으므로 임계값을 넘으면 바로 끊는다.
func editDistanceWithin(a, b string, max int) bool {
	if a == b {
		return true
	}
	if abs(len(a)-len(b)) > max {
		return false
	}

	// 고전적인 동적 계획법. 이전 행만 들고 있으면 된다
	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}

	for i := 1; i <= len(a); i++ {
		curr[0] = i
		best := curr[0]
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min3(curr[j-1]+1, prev[j]+1, prev[j-1]+cost)
			if curr[j] < best {
				best = curr[j]
			}
		}
		if best > max {
			return false // 이 행 전체가 임계값을 넘었다. 더 볼 필요 없다
		}
		prev, curr = curr, prev
	}

	return prev[len(b)] <= max
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}

func isKnownCategory(c model.Category) bool {
	for _, k := range knownCategories() {
		if k == c {
			return true
		}
	}
	return false
}

func knownCategories() []model.Category {
	return []model.Category{
		model.CategoryHousing, model.CategoryIncome, model.CategoryEmployment,
		model.CategoryChildcare, model.CategoryMedical, model.CategoryEducation,
		model.CategoryElderly, model.CategoryDisability, model.CategoryEnergy,
		model.CategoryOther,
	}
}

func categoryNames() []string {
	out := make([]string, 0, len(knownCategories()))
	for _, c := range knownCategories() {
		out = append(out, string(c))
	}
	return out
}

func opNames() []string {
	ops := model.KnownOps()
	out := make([]string, 0, len(ops))
	for _, o := range ops {
		out = append(out, string(o))
	}
	return out
}
