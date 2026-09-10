package model

// ConditionStatus 는 조건 단위 판정이다.
//
// ★ UNKNOWN 은 "입력값이 없어 판정 불가" 다. FAIL 이 아니다.
// 사람들은 자기 소득을 정확히 모른다. 이 구분이 서비스의 신뢰를 만든다.
type ConditionStatus string

const (
	StatusPass    ConditionStatus = "PASS"
	StatusFail    ConditionStatus = "FAIL"
	StatusUnknown ConditionStatus = "UNKNOWN"
)

// 조건이 속한 그룹. 화면이 판정을 어떻게 읽어야 하는지가 여기서 갈린다.
//
// ★ None 은 배제 조건이라 뜻이 뒤집힌다.
// "재직 중이 아닐 것" 조건에서 엔진의 PASS 는 "재직 중이다 = 탈락" 이고,
// FAIL 은 "재직 중이 아니다 = 통과" 다. 그룹을 알려주지 않으면 화면이
// 이용자에게 정반대를 보여준다. 실제로 그 버그가 있었다.
const (
	GroupAll  = "all"
	GroupAny  = "any"
	GroupNone = "none"
	// 시설의 관할 지역. 뜻이 뒤집히지 않는 일반 조건이다
	GroupCoverage = "coverage"
)

// ConditionResult 는 조건 하나의 판정 근거다.
//
// ★ 설명 가능성의 핵심 — "왜 해당/미해당인지" 를 조건 단위로 보여주기 위해
// 모든 조건에 대해 반드시 남긴다.
type ConditionResult struct {
	Condition Condition `json:"condition"`
	// 이 조건이 속한 그룹 (all / any / none / coverage).
	// ★ 화면은 이 값을 보고 none 그룹의 표시를 뒤집는다
	Group string `json:"group"`
	// ★ 엔진 관점의 판정이다. 뒤집지 않는다 — 규칙 엔진이 실제로 무엇을
	// 보았는지가 원자료로 남아야 디버깅이 된다
	Status ConditionStatus `json:"status"`
	// 사용자 입력의 실제 값. 화면의 "입력: 29세". 모르면 nil
	Actual any `json:"actual,omitempty"`
	// 사람 말 사유
	Reason string `json:"reason"`
}

// MatchStatus 는 제도별 판정 결과다.
type MatchStatus string

const (
	// 확인된 조건이 전부 충족
	MatchEligible MatchStatus = "ELIGIBLE"
	// 명시적으로 탈락
	MatchIneligible MatchStatus = "INELIGIBLE"
	// 탈락은 아니지만 확인이 더 필요
	MatchNeedsInfo MatchStatus = "NEEDS_INFO"
)

// MatchResult 는 제도 하나에 대한 판정 결과다.
type MatchResult struct {
	Program Program     `json:"program"`
	Status  MatchStatus `json:"status"`
	// 조건 단위 근거. 항상 채운다
	Conditions []ConditionResult `json:"conditions"`
	// 연간 예상 수령액 (원). 산정 불가면 0
	EstimatedAmount int64 `json:"estimatedAmount"`
	// 판정에 더 필요한 필드 이름
	MissingFields []string `json:"missingFields"`
}

// Summary 는 결과 화면 상단의 요약이다.
// "확인된 것 6건 · 연 4,800,000원 · 추가 확인 3건"
type Summary struct {
	EligibleCount     int   `json:"eligibleCount"`
	NeedsInfoCount    int   `json:"needsInfoCount"`
	IneligibleCount   int   `json:"ineligibleCount"`
	TotalAnnualAmount int64 `json:"totalAnnualAmount"`
	// 중복수급 배제로 제거된 제도 id (Phase 2)
	ExcludedByConflict []string `json:"excludedByConflict,omitempty"`
}

// Confidence 는 AI 추출의 필드별 확신도다 (Phase 4).
//
// ★ 값은 대문자다. 이 프로젝트의 모든 enum 이 UPPER_SNAKE 이고
// (PASS / ELIGIBLE / MONTHLY_RENT …), 프론트 타입도 그 규칙을 따른다.
type Confidence string

const (
	ConfidenceHigh   Confidence = "HIGH"
	ConfidenceMedium Confidence = "MEDIUM"
	ConfidenceLow    Confidence = "LOW"
)

// Disclaimer 는 모든 성공 응답에 포함되는 고지다.
//
// ★ 이 서비스는 안내 도구이며 공식 판정이 아니다. 지우지 마라.
const Disclaimer = "실제 수급 여부는 관할 기관의 심사로 결정됩니다"

// DocumentDisclaimer 는 문서 번역 응답에 포함되는 고지다 (Phase 6).
const DocumentDisclaimer = "원문의 법적 효력이 우선합니다"
