package model

// 시설 — 이용자가 실제로 찾아가거나 전화할 수 있는 곳.
//
// ★ 제도(Program)는 "무엇을 받을 수 있는가" 이고, 시설(Facility)은 "어디로 가면
// 되는가" 다. 사각지대에 있는 사람에게 더 급한 건 대개 후자다 — 제도를 알아도
// 어디에 물어야 할지 모르면 결국 도달하지 못한다.
//
// 자격 판정 구조는 제도와 똑같이 Eligibility 를 쓴다. 규칙 엔진을 그대로
// 재사용하기 위해서다. 시설이라고 판정 방식이 다를 이유가 없다.

// FacilityType 은 시설 종류다.
//
// 전국사회복지시설표준데이터(사회보장정보원)의 "시설종류명" 을 이 값으로 옮긴다.
// 원문 문자열을 그대로 쓰지 않는 이유: 지자체마다 표기가 조금씩 다르고
// ("지역아동센터" / "지역아동센터(일반)"), 화면에서 묶어 보여줘야 하기 때문이다.
type FacilityType string

const (
	FacilityChildCenter      FacilityType = "CHILD_CENTER"      // 지역아동센터
	FacilityCommunityWelfare FacilityType = "COMMUNITY_WELFARE" // 종합사회복지관
	FacilityElderly          FacilityType = "ELDERLY"           // 노인복지관·경로당
	FacilityDisability       FacilityType = "DISABILITY"        // 장애인복지관
	FacilityMentalHealth     FacilityType = "MENTAL_HEALTH"     // 정신건강복지센터
	FacilitySelfSufficiency  FacilityType = "SELF_SUFFICIENCY"  // 지역자활센터
	FacilityShelter          FacilityType = "SHELTER"           // 쉼터·보호시설
	FacilitySingleParent     FacilityType = "SINGLE_PARENT"     // 한부모가족복지시설
	FacilityFamilyCenter     FacilityType = "FAMILY_CENTER"     // 가족센터·건강가정지원센터
	FacilityJobCenter        FacilityType = "JOB_CENTER"        // 고용복지플러스센터
	FacilityHotline          FacilityType = "HOTLINE"           // 전화 상담 창구
	FacilityCommunityCenter  FacilityType = "COMMUNITY_CENTER"  // 행정복지센터(주민센터)
	FacilityOther            FacilityType = "OTHER"
)

// KnownFacilityTypes 는 지원하는 시설 종류다. cmd/validate 가 쓴다.
func KnownFacilityTypes() []FacilityType {
	return []FacilityType{
		FacilityChildCenter, FacilityCommunityWelfare, FacilityElderly,
		FacilityDisability, FacilityMentalHealth, FacilitySelfSufficiency,
		FacilityShelter, FacilitySingleParent, FacilityFamilyCenter,
		FacilityJobCenter, FacilityHotline, FacilityCommunityCenter,
		FacilityOther,
	}
}

func FacilityTypeIsKnown(t FacilityType) bool {
	for _, k := range KnownFacilityTypes() {
		if k == t {
			return true
		}
	}
	return false
}

// CoverageScope 는 이 시설이 누구를 받는가의 범위다.
type CoverageScope string

const (
	// 전국 누구나. 상담 전화가 대부분 여기다
	CoverageNationwide CoverageScope = "NATIONWIDE"
	// 시도 거주자. 예: 서울시민 대상
	CoverageSido CoverageScope = "SIDO"
	// 시군구 거주자. 지역아동센터·행정복지센터가 여기다
	CoverageSigungu CoverageScope = "SIGUNGU"
)

// Coverage 는 관할 범위다.
//
// ★ 시설은 물리적인 장소다. 제도와 달리 "어디 사는가" 가 1순위 조건이 된다.
// 관악구 주민센터를 강남구 사람에게 안내하면 그건 안내가 아니라 헛걸음이다.
type Coverage struct {
	Scope CoverageScope `json:"scope"`
	// Scope 가 SIDO / SIGUNGU 일 때 채운다. NATIONWIDE 면 비운다
	Sido string `json:"sido,omitempty"`
	// Scope 가 SIGUNGU 일 때 채운다
	Sigungu string `json:"sigungu,omitempty"`
	// 관할을 벗어나도 이용할 수 있는 예외 (예: "긴급한 경우 타 지역 주민도 상담 가능")
	Note string `json:"note,omitempty"`
}

// Serves 는 이 관할이 사용자의 거주지를 포함하는지 본다.
//
// 반환값 세 가지의 뜻이 다르다.
//
//	PASS    관할 안이다
//	FAIL    관할 밖이다 — 안내하면 헛걸음이 된다
//	UNKNOWN 거주지를 모른다 — 관할 밖이라고 단정하지 않는다 (설계 원칙 3)
//
// ★ 거주지를 모른다고 시설을 숨기지 않는다. 물어보면 될 일이다.
func (c Coverage) Serves(ctx UserContext) ConditionStatus {
	switch c.Scope {
	case CoverageNationwide:
		return StatusPass

	case CoverageSido:
		if ctx.Region == nil {
			return StatusUnknown
		}
		if *ctx.Region == c.Sido {
			return StatusPass
		}
		return StatusFail

	case CoverageSigungu:
		// 시도가 이미 다르면 시군구를 몰라도 관할 밖이 확정이다
		if ctx.Region != nil && *ctx.Region != c.Sido {
			return StatusFail
		}
		if ctx.District == nil {
			return StatusUnknown
		}
		if *ctx.District == c.Sigungu && (ctx.Region == nil || *ctx.Region == c.Sido) {
			return StatusPass
		}
		return StatusFail
	}

	// 알 수 없는 범위는 데이터 오류다. 단정하지 않는다
	return StatusUnknown
}

// Label 은 관할을 사람 말로. 근거표의 "조건" 칸에 그대로 쓴다.
func (c Coverage) Label() string {
	switch c.Scope {
	case CoverageNationwide:
		return "전국 누구나 이용 가능"
	case CoverageSido:
		return c.Sido + " 거주자"
	case CoverageSigungu:
		return c.Sido + " " + c.Sigungu + " 거주자"
	}
	return "관할 지역"
}

// Location 은 시설의 물리적 위치다.
//
// 항목 이름은 전국사회복지시설표준데이터를 따른다. 나중에 다른 지자체 데이터를
// 붙일 때 매핑을 다시 짜지 않기 위해서다.
type Location struct {
	Sido    string `json:"sido"`
	Sigungu string `json:"sigungu"`
	// 소재지도로명주소
	RoadAddress string `json:"roadAddress"`
	// 소재지지번주소. 도로명이 없는 오래된 데이터가 있다
	LotAddress string `json:"lotAddress,omitempty"`
	// 위경도. 지도 링크에 쓴다. 없으면 주소로 검색한다
	Lat float64 `json:"lat,omitempty"`
	Lng float64 `json:"lng,omitempty"`
}

// HasAddress 는 지도로 안내할 수 있는 주소가 있는지 본다.
// 전화 상담 창구(HOTLINE)는 주소가 없다.
func (l Location) HasAddress() bool {
	return l.RoadAddress != "" || l.LotAddress != ""
}

// Contact 는 연결 수단이다. 이 구조체가 이 서비스의 결말이다.
//
// ★ 전화번호를 지어내지 마라. 확인되지 않으면 비워 둔다.
// 틀린 번호로 전화하게 만드는 것은 안내하지 않는 것보다 나쁘다.
type Contact struct {
	// 대표 전화. "02-1234-5678" 또는 "129" 같은 단축번호
	Phone string `json:"phone,omitempty"`
	Fax   string `json:"fax,omitempty"`
	Email string `json:"email,omitempty"`
	// 홈페이지
	Website string `json:"website,omitempty"`
	// 온라인 신청·예약 페이지. 있으면 화면에서 바로 보낸다
	ApplyURL string `json:"applyUrl,omitempty"`
	// 운영시간. "평일 09:00~18:00" 처럼 사람이 읽는 문자열.
	// 요일별 구조로 쪼개지 않는 이유: 원본 데이터가 자유 문자열이라
	// 쪼개는 순간 대부분이 파싱 실패로 사라진다
	Hours string `json:"hours,omitempty"`
	// 24시간 운영 여부. 위급한 사람에게 먼저 보여줘야 한다
	Always bool `json:"always,omitempty"`
}

// Reachable 은 연락할 방법이 하나라도 있는지 본다.
//
// ★ 연락할 수 없는 시설은 이 서비스에서 의미가 없다. 로더가 걸러낸다.
func (c Contact) Reachable() bool {
	return c.Phone != "" || c.Email != "" || c.ApplyURL != "" || c.Website != ""
}

// Facility 는 시설 하나다. data/facilities/*.json 과 1:1 로 대응한다.
type Facility struct {
	ID   string       `json:"id"`
	Name string       `json:"name"`
	Type FacilityType `json:"type"`
	// 한 줄 설명. "무엇을 해주는 곳인지" 를 이용자 말로
	Summary string `json:"summary,omitempty"`

	// 이용 대상 조건. 제도와 같은 구조라 규칙 엔진을 그대로 쓴다
	Eligibility Eligibility `json:"eligibility"`
	// 관할 범위. 판정에서 조건 하나로 함께 다뤄진다
	Coverage Coverage `json:"coverage"`

	Location Location `json:"location"`
	Contact  Contact  `json:"contact"`

	// 제공 서비스. "무료급식", "학습지원", "심리상담"
	Services []string `json:"services,omitempty"`
	// 이용료. "무료", "소득별 차등" 처럼 사람이 읽는 문자열
	Fee string `json:"fee,omitempty"`
	// 방문·문의 시 챙겨야 할 것
	Documents []string `json:"documents,omitempty"`

	// 관할행정기관. "어디에 민원을 넣어야 하는가" 이기도 하다
	Authority string `json:"authority,omitempty"`
	Source    Source `json:"source"`
}

// FacilityMatch 는 시설 하나에 대한 판정 결과다.
//
// MatchResult 와 형태를 맞춘다 — 화면이 같은 컴포넌트로 그릴 수 있게.
type FacilityMatch struct {
	Facility Facility    `json:"facility"`
	Status   MatchStatus `json:"status"`
	// 조건 단위 근거. 첫 줄은 항상 관할 지역이다
	Conditions []ConditionResult `json:"conditions"`
	// 판정에 더 필요한 필드 이름. NEEDS_INFO 일 때만 채운다
	MissingFields []string `json:"missingFields"`
}

// FacilitySummary 는 시설 결과의 요약이다.
type FacilitySummary struct {
	AvailableCount int `json:"availableCount"`
	NeedsInfoCount int `json:"needsInfoCount"`
	OutOfScope     int `json:"outOfScope"`
	// 지금 바로 전화할 수 있는 곳의 수. 화면 상단의 행동 유도에 쓴다
	ReachableNow int `json:"reachableNow"`
}
