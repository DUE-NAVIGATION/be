package model

// UserContext 는 판정 입력값이다.
//
// ★ 모든 스칼라 필드가 포인터인 이유
//
//	이 프로젝트에서 가장 중요한 구분은 "모르는 값" 과 "0" 이다.
//	소득 0원(무소득)과 소득 미입력은 완전히 다른 판정으로 이어진다.
//	- 소득 0원      → 대부분의 저소득 제도에 해당 (ELIGIBLE)
//	- 소득 미입력   → 판정 불가 (NEEDS_INFO). 탈락시키면 안 된다
//
//	포인터는 encoding/json 이 이 구분을 공짜로 해준다.
//	필드가 없으면 nil, "incomeMonthly": 0 이면 0을 가리키는 포인터다.
//
// ※ 이 구조체는 요청 처리 중 메모리에만 존재한다. 디스크에 쓰지 않는다.
// ※ 이름·주소·주민번호 등 식별정보를 여기에 넣지 않는다.
type UserContext struct {
	// 가구원 수 (본인 포함)
	HouseholdSize *int `json:"householdSize,omitempty"`
	// 만 나이
	Age *int `json:"age,omitempty"`
	// 월 소득 (원). 소득평가액 기준
	IncomeMonthly *int64 `json:"incomeMonthly,omitempty"`
	// 재산 총액 (원). 소득환산 대상
	Assets *int64 `json:"assets,omitempty"`

	HousingType *HousingType `json:"housingType,omitempty"`
	// 보증금 (원)
	Deposit *int64 `json:"deposit,omitempty"`
	// 월세 (원)
	MonthlyRent *int64 `json:"monthlyRent,omitempty"`

	EmploymentStatus *EmploymentStatus `json:"employmentStatus,omitempty"`
	// 한부모 가구 여부
	IsSingleParent *bool `json:"isSingleParent,omitempty"`
	// 자녀 나이 목록 (만 나이). nil = 모름, []int{} = 자녀 없음
	ChildrenAges  []int `json:"childrenAges,omitempty"`
	HasDisability *bool `json:"hasDisability,omitempty"`
	// 현재 수급 중인 제도 id 또는 급여 코드. 배제 조건 판정용
	ReceivingPrograms []string `json:"receivingPrograms,omitempty"`
	// 거주 지역 (시도 단위)
	Region *string `json:"region,omitempty"`

	// ★ 파생값 — income 엔진이 계산해 채운다. 사용자가 직접 입력하지 않는다.
	// 중위소득 대비 비율(%). 대부분의 제도 자격이 이 값 하나로 갈린다.
	HouseholdIncomePct *float64 `json:"householdIncomePct,omitempty"`
}

// HousingType 은 주거 형태다.
type HousingType string

const (
	HousingMonthlyRent HousingType = "MONTHLY_RENT" // 월세
	HousingJeonse      HousingType = "JEONSE"       // 전세
	HousingOwned       HousingType = "OWNED"        // 자가
	HousingPublicLease HousingType = "PUBLIC_LEASE" // 공공임대
	HousingFreeUse     HousingType = "FREE_USE"     // 무상거주
	HousingOther       HousingType = "OTHER"
)

// EmploymentStatus 는 취업 상태다.
type EmploymentStatus string

const (
	EmploymentEmployed     EmploymentStatus = "EMPLOYED"      // 재직
	EmploymentSelfEmployed EmploymentStatus = "SELF_EMPLOYED" // 자영업
	EmploymentLostJob      EmploymentStatus = "LOST_JOB"      // 실직
	EmploymentUnemployed   EmploymentStatus = "UNEMPLOYED"    // 미취업
	EmploymentStudent      EmploymentStatus = "STUDENT"       // 학생
	EmploymentRetired      EmploymentStatus = "RETIRED"       // 은퇴
	EmploymentOnLeave      EmploymentStatus = "ON_LEAVE"      // 휴직
	EmploymentOther        EmploymentStatus = "OTHER"
)

// ── 포인터 리터럴 헬퍼 ────────────────────────────────────────
// 테스트와 제도 데이터 작성에서 &42 를 쓸 수 없으므로 필요하다.

func Int(v int) *int           { return &v }
func Int64(v int64) *int64     { return &v }
func Float(v float64) *float64 { return &v }
func Bool(v bool) *bool        { return &v }
func Str(v string) *string     { return &v }

func Housing(v HousingType) *HousingType              { return &v }
func Employment(v EmploymentStatus) *EmploymentStatus { return &v }
