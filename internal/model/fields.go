package model

// 제도 JSON 의 condition.field 에 쓸 수 있는 필드 이름.
//
// ★ 문자열을 그대로 쓰지 말고 이 상수를 쓴다. 오타 난 필드는 조용히 UNKNOWN 이 되어
// "확인 필요"로 새기 때문에, 사람 눈으로는 발견이 어렵다.
// cmd/validate 가 제도 JSON 의 field 를 이 목록과 대조해서 잡아낸다.
const (
	FieldHouseholdSize      = "householdSize"
	FieldAge                = "age"
	FieldIncomeMonthly      = "incomeMonthly"
	FieldAssets             = "assets"
	FieldHousingType        = "housingType"
	FieldDeposit            = "deposit"
	FieldMonthlyRent        = "monthlyRent"
	FieldEmploymentStatus   = "employmentStatus"
	FieldIsSingleParent     = "isSingleParent"
	FieldChildrenAges       = "childrenAges"
	FieldHasDisability      = "hasDisability"
	FieldReceivingPrograms  = "receivingPrograms"
	FieldRegion             = "region"
	FieldHouseholdIncomePct = "householdIncomePct"
)

// KnownFields 는 제도 JSON 에서 참조 가능한 필드 이름을 정렬된 순서로 돌려준다.
// cmd/validate 의 오류 메시지("이런 필드를 쓸 수 있습니다")에 쓴다.
func KnownFields() []string {
	return []string{
		FieldAge,
		FieldAssets,
		FieldChildrenAges,
		FieldDeposit,
		FieldEmploymentStatus,
		FieldHasDisability,
		FieldHouseholdIncomePct,
		FieldHouseholdSize,
		FieldHousingType,
		FieldIncomeMonthly,
		FieldIsSingleParent,
		FieldMonthlyRent,
		FieldReceivingPrograms,
		FieldRegion,
	}
}

// FieldExists 는 그 이름의 필드가 UserContext 에 있는지 알려준다.
func FieldExists(field string) bool {
	for _, f := range KnownFields() {
		if f == field {
			return true
		}
	}
	return false
}

// Lookup 은 필드 이름으로 UserContext 의 값을 꺼낸다.
//
// 반환값
//
//	value  정규화된 값. int64 / float64 / string / bool / []int64 / []string 중 하나
//	known  값을 아는가. false 면 판정은 UNKNOWN 이다 (FAIL 이 아니다)
//	exists 그런 이름의 필드가 있는가. false 면 제도 JSON 이 잘못된 것이다
//
// ★ 리플렉션을 쓰지 않는다. 필드가 15개뿐이고, 명시적인 switch 가
// 컴파일 타임 검사를 받으며 validate CLI 와 목록을 공유할 수 있기 때문이다.
func Lookup(ctx UserContext, field string) (value any, known bool, exists bool) {
	switch field {
	case FieldHouseholdSize:
		if ctx.HouseholdSize == nil {
			return nil, false, true
		}
		return int64(*ctx.HouseholdSize), true, true

	case FieldAge:
		if ctx.Age == nil {
			return nil, false, true
		}
		return int64(*ctx.Age), true, true

	case FieldIncomeMonthly:
		if ctx.IncomeMonthly == nil {
			return nil, false, true
		}
		return *ctx.IncomeMonthly, true, true

	case FieldAssets:
		if ctx.Assets == nil {
			return nil, false, true
		}
		return *ctx.Assets, true, true

	case FieldHousingType:
		if ctx.HousingType == nil {
			return nil, false, true
		}
		return string(*ctx.HousingType), true, true

	case FieldDeposit:
		if ctx.Deposit == nil {
			return nil, false, true
		}
		return *ctx.Deposit, true, true

	case FieldMonthlyRent:
		if ctx.MonthlyRent == nil {
			return nil, false, true
		}
		return *ctx.MonthlyRent, true, true

	case FieldEmploymentStatus:
		if ctx.EmploymentStatus == nil {
			return nil, false, true
		}
		return string(*ctx.EmploymentStatus), true, true

	case FieldIsSingleParent:
		if ctx.IsSingleParent == nil {
			return nil, false, true
		}
		return *ctx.IsSingleParent, true, true

	case FieldChildrenAges:
		// nil = 모름. 빈 슬라이스 = "자녀 없음" 이라는 정보이므로 known 이다.
		if ctx.ChildrenAges == nil {
			return nil, false, true
		}
		ages := make([]int64, 0, len(ctx.ChildrenAges))
		for _, a := range ctx.ChildrenAges {
			ages = append(ages, int64(a))
		}
		return ages, true, true

	case FieldHasDisability:
		if ctx.HasDisability == nil {
			return nil, false, true
		}
		return *ctx.HasDisability, true, true

	case FieldReceivingPrograms:
		if ctx.ReceivingPrograms == nil {
			return nil, false, true
		}
		return append([]string(nil), ctx.ReceivingPrograms...), true, true

	case FieldRegion:
		if ctx.Region == nil {
			return nil, false, true
		}
		return *ctx.Region, true, true

	case FieldHouseholdIncomePct:
		if ctx.HouseholdIncomePct == nil {
			return nil, false, true
		}
		return *ctx.HouseholdIncomePct, true, true
	}

	return nil, false, false
}
