package model

// Op 는 조건 연산자다. 제도 JSON 의 문자열과 1:1 로 대응한다.
type Op string

const (
	OpBetween  Op = "between"  // value: [min, max] — 양 끝 포함
	OpLte      Op = "lte"      // value: 숫자
	OpGte      Op = "gte"      // value: 숫자
	OpEq       Op = "eq"       // value: 숫자 / 문자열 / 불리언
	OpIn       Op = "in"       // value: 배열 — 대상 값이 배열에 포함
	OpContains Op = "contains" // value: 단일 값 — 대상 배열이 값을 포함
	OpExists   Op = "exists"   // value 무시 — 값이 존재하기만 하면 PASS
)

// KnownOps 는 지원하는 연산자 목록이다. cmd/validate 가 쓴다.
func KnownOps() []Op {
	return []Op{OpBetween, OpLte, OpGte, OpEq, OpIn, OpContains, OpExists}
}

// OpExists 와 이름이 겹치지 않게 OpIsKnown 으로 둔다.
func OpIsKnown(op Op) bool {
	for _, o := range KnownOps() {
		if o == op {
			return true
		}
	}
	return false
}

// Condition 은 자격요건의 최소 단위다.
type Condition struct {
	// 판정 대상 필드. model.Field* 상수 중 하나여야 한다
	Field string `json:"field"`
	Op    Op     `json:"op"`
	// 비교값. op 에 따라 숫자 / 문자열 / 배열
	Value any `json:"value,omitempty"`
	// 화면에 보여줄 사람 말. 예: "만 19~34세"
	Label string `json:"label,omitempty"`
	// 작성자 메모. 판정에 쓰이지 않는다
	Note string `json:"note,omitempty"`
}

// Eligibility 는 자격요건이다.
//
//	All  전부 PASS 여야 통과 (AND)
//	Any  하나라도 PASS 면 통과 (OR)
//	None 하나라도 PASS 면 탈락 (배제 조건)
type Eligibility struct {
	All  []Condition `json:"all,omitempty"`
	Any  []Condition `json:"any,omitempty"`
	None []Condition `json:"none,omitempty"`
}

// BenefitType 은 급여 형태다.
type BenefitType string

const (
	BenefitMonthly BenefitType = "MONTHLY" // 월 정액 × months
	BenefitOnce    BenefitType = "ONCE"    // 1회성
	BenefitYearly  BenefitType = "YEARLY"  // 연 정액
	BenefitRate    BenefitType = "RATE"    // 요금 감면율 — 금액 산정 불가
	BenefitInKind  BenefitType = "IN_KIND" // 현물·서비스 — 금액 없음
)

// Benefit 은 급여 정의다.
//
// ★ 금액은 전부 정수(원 단위)다. 부동소수점을 쓰지 않는다.
// RATE / IN_KIND 는 금액 산정이 불가하므로 총액 합산에서 제외된다 —
// 데모의 "연 480만원" 이 오염되지 않게 한다.
type Benefit struct {
	Type BenefitType `json:"type"`
	// 원 단위. RATE / IN_KIND 면 0
	Amount int64 `json:"amount,omitempty"`
	// MONTHLY 일 때 지급 개월 수
	Months int `json:"months,omitempty"`
	// RATE 일 때 감면율(%)
	RatePct float64 `json:"ratePct,omitempty"`
	// 금액이 가구원수·소득에 따라 달라져 단순 산정이 불가할 때의 설명
	Note string `json:"note,omitempty"`
}

// Apply 는 신청 방법이다.
type Apply struct {
	// 신청 채널. 예: BOKJIRO, COMMUNITY_CENTER
	Channel []string `json:"channel"`
	// 필요 서류
	Documents []string `json:"documents"`
	// 신청 기간 안내
	Period string `json:"period,omitempty"`
}

// Source 는 출처다.
//
// ★ RevisedAt 은 반드시 채운다. 심사에서 물어본다.
type Source struct {
	URL string `json:"url"`
	// 개정일 (YYYY-MM-DD)
	RevisedAt string `json:"revisedAt"`
	// 출처 기관명
	Agency string `json:"agency,omitempty"`
	// 작성자 메모 (근거 문서, 대조가 필요한 사항 등). 판정에는 쓰이지 않는다
	Note string `json:"note,omitempty"`
}

// Category 는 제도 분류다.
type Category string

const (
	CategoryHousing    Category = "HOUSING"    // 주거
	CategoryIncome     Category = "INCOME"     // 소득·생계
	CategoryEmployment Category = "EMPLOYMENT" // 고용
	CategoryChildcare  Category = "CHILDCARE"  // 양육
	CategoryMedical    Category = "MEDICAL"    // 의료
	CategoryEducation  Category = "EDUCATION"  // 교육
	CategoryElderly    Category = "ELDERLY"    // 노인
	CategoryDisability Category = "DISABILITY" // 장애
	CategoryEnergy     Category = "ENERGY"     // 에너지·공과금
	CategoryOther      Category = "OTHER"
)

// Program 은 제도 정의다. data/programs/*.json 과 1:1 로 대응한다.
//
// ★ 제도 데이터를 Go 코드에 하드코딩하지 않는다. 반드시 JSON.
type Program struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Category Category `json:"category"`
	// 한 줄 설명
	Summary     string      `json:"summary,omitempty"`
	Eligibility Eligibility `json:"eligibility"`
	Benefit     Benefit     `json:"benefit"`
	Apply       Apply       `json:"apply"`
	Source      Source      `json:"source"`
}

// RelationType 은 제도 간 관계다 (Phase 2, 중복수급 판정).
type RelationType string

const (
	RelationExclusive    RelationType = "EXCLUSIVE"    // 동시 수급 불가
	RelationReducing     RelationType = "REDUCING"     // 동시 수급 시 감액
	RelationPrerequisite RelationType = "PREREQUISITE" // 선행 조건
)

// Relation 은 두 제도 사이의 관계다.
type Relation struct {
	From string       `json:"from"`
	To   string       `json:"to"`
	Type RelationType `json:"type"`
	// REDUCING 일 때 감액률(%)
	ReducePct float64 `json:"reducePct,omitempty"`
	Reason    string  `json:"reason,omitempty"`
}
