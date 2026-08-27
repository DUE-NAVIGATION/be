package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"

	"github.com/DUE-NAVIGATION/be/internal/model"
)

// extractSystem 은 구조화 프롬프트다.
//
// ★ 이 문구는 코드 리뷰 대상이다. 고칠 때는 왜 고치는지 함께 남긴다.
// ★ 판정을 시키지 않는다. 제도명을 언급하지 못하게 한다. 계산을 시키지 않는다.
const extractSystem = `당신은 복지 상담 접수 담당자입니다. 사용자가 자기 상황을 편하게 말하면,
그 말에서 사실만 뽑아 정해진 항목으로 옮깁니다.

## 절대 하지 말 것
- 자격을 판정하지 마세요. "해당됩니다", "받을 수 있습니다" 같은 말을 하지 마세요.
- 제도 이름을 언급하지 마세요. (청년월세, 기초생활수급 등)
- 언급되지 않은 값을 추측해서 채우지 마세요.
  "월세 산다"고만 했으면 보증금과 월세 금액은 비워 둡니다.
- 값을 계산하지 마세요. "연봉 3600만원"이라고 하면 월 소득을 나누지 말고
  incomeMonthly 를 비워 두고 followUpQuestions 로 되물으세요.

## 항목 (해당하는 것만 채웁니다. 나머지는 아예 넣지 마세요)
householdSize · age · incomeMonthly · assets · housingType · deposit ·
monthlyRent · employmentStatus · isSingleParent · childrenAges ·
hasDisability · disabilityLevel · isPregnant · basicLivelihoodType ·
receivingPrograms · region

## 값의 형태
housingType: MONTHLY_RENT / JEONSE / OWNED / PUBLIC_LEASE / FREE_USE / OTHER
employmentStatus: EMPLOYED / SELF_EMPLOYED / LOST_JOB / UNEMPLOYED /
                  STUDENT / RETIRED / ON_LEAVE / OTHER
disabilityLevel: SEVERE / MILD
basicLivelihoodType: LIVELIHOOD / MEDICAL / HOUSING / EDUCATION / NONE
금액: 원 단위 정수. "80만원" → 800000
나이: 만 나이 정수

## 확신도 (confidence)
- HIGH: 사용자가 그대로 말했다 ("아이는 7살")
- MEDIUM: 말에서 분명히 따라 나온다 ("혼자 애 키운다" → isSingleParent)
- LOW: 애매하다. 이때는 followUpQuestions 로 확인 질문을 함께 넣으세요.

## 되묻기 (followUpQuestions)
판정에 꼭 필요한데 비어 있는 항목을 물어보세요. 최대 3개.
- 가장 중요한 것부터: 가구원수 → 월 소득 → 재산
- 한 번에 하나씩, 짧고 쉬운 말로
- 예: "월 소득이 어느 정도인가요?" (O)
      "소득인정액을 알려주세요" (X — 일반인이 모르는 말입니다)

## 가려진 정보
입력에 [주민등록번호] [전화번호] 같은 표시가 있으면, 그 자리에 민감정보가
있었지만 의도적으로 제거된 것입니다. 그 값을 묻거나 추측하지 마세요.`

const extractToolName = "record_context"

const extractToolDesc = "사용자의 말에서 뽑아낸 상황 정보를 기록합니다. 판정하지 않습니다."

// extractSchema 는 도구 입력 스키마다. 이 모양이 아니면 모델이 답할 수 없다.
//
// ★ householdIncomePct 는 넣지 않는다. 계산 엔진이 채우는 파생값이고,
// AI 가 이 값을 만들어내면 판정 근거가 오염된다.
var extractSchema = json.RawMessage(`{
  "type": "object",
  "properties": {
    "extracted": {
      "type": "object",
      "properties": {
        "householdSize":       { "type": "integer", "minimum": 1 },
        "age":                 { "type": "integer", "minimum": 0, "maximum": 130 },
        "incomeMonthly":       { "type": "integer", "minimum": 0 },
        "assets":              { "type": "integer", "minimum": 0 },
        "housingType":         { "type": "string", "enum": ["MONTHLY_RENT","JEONSE","OWNED","PUBLIC_LEASE","FREE_USE","OTHER"] },
        "deposit":             { "type": "integer", "minimum": 0 },
        "monthlyRent":         { "type": "integer", "minimum": 0 },
        "employmentStatus":    { "type": "string", "enum": ["EMPLOYED","SELF_EMPLOYED","LOST_JOB","UNEMPLOYED","STUDENT","RETIRED","ON_LEAVE","OTHER"] },
        "isSingleParent":      { "type": "boolean" },
        "childrenAges":        { "type": "array", "items": { "type": "integer", "minimum": 0 } },
        "hasDisability":       { "type": "boolean" },
        "disabilityLevel":     { "type": "string", "enum": ["SEVERE","MILD"] },
        "isPregnant":          { "type": "boolean" },
        "basicLivelihoodType": { "type": "string", "enum": ["LIVELIHOOD","MEDICAL","HOUSING","EDUCATION","NONE"] },
        "receivingPrograms":   { "type": "array", "items": { "type": "string" } },
        "region":              { "type": "string" }
      },
      "additionalProperties": false
    },
    "confidence": {
      "type": "object",
      "additionalProperties": { "type": "string", "enum": ["HIGH","MEDIUM","LOW"] }
    },
    "followUpQuestions": {
      "type": "array",
      "items": { "type": "string" },
      "maxItems": 3
    }
  },
  "required": ["extracted", "confidence", "followUpQuestions"],
  "additionalProperties": false
}`)

// ExtractionResult 는 자연어에서 뽑아낸 상황이다.
type ExtractionResult struct {
	Extracted         model.UserContext           `json:"extracted"`
	Confidence        map[string]model.Confidence `json:"confidence"`
	FollowUpQuestions []string                    `json:"followUpQuestions"`
	// Sanitized 는 전송 전에 가린 민감정보의 종류·건수다. 값은 들어 있지 않다
	Sanitized map[Kind]int `json:"sanitized,omitempty"`
}

// Extract 는 자연어를 판정 입력값으로 옮긴다.
//
// 순서가 중요하다.
//
//  1. 시크릿 필터 → 2) LLM 호출 → 3) 결과 검증
//
// ★ 1번을 건너뛰는 경로를 만들지 마라. 한 번 나간 정보는 되돌릴 수 없다.
func (c *Client) Extract(ctx context.Context, text string) (ExtractionResult, error) {
	// 1) 무조건 먼저 거른다
	clean := Sanitize(text)

	// 데모 모드: 미리 뽑아둔 응답이 있으면 그걸 쓴다 (Phase 7)
	if raw, ok := c.cached("extract", cacheKey(clean.Text)); ok {
		out, err := decodeExtraction(raw)
		out.Sanitized = clean.Found
		return out, err
	}

	// 2) 도구 스키마를 강제해 호출
	raw, err := c.callTool(ctx, extractSystem, clean.Text,
		extractToolName, extractToolDesc, extractSchema)
	if err != nil {
		return ExtractionResult{}, err
	}

	// 3) 스키마를 통과했어도 한 번 더 본다
	out, err := decodeExtraction(raw)
	if err != nil {
		return ExtractionResult{}, err
	}
	out.Sanitized = clean.Found
	return out, nil
}

// decodeExtraction 은 모델이 준 JSON 을 우리 타입으로 옮기고 다듬는다.
//
// 스키마를 강제했어도 검증을 한 번 더 하는 이유:
// 스키마는 모양만 맞춰줄 뿐, "AI 가 채우면 안 되는 값" 까지 막아주지는 않는다.
func decodeExtraction(raw json.RawMessage) (ExtractionResult, error) {
	var wire struct {
		Extracted         json.RawMessage   `json:"extracted"`
		Confidence        map[string]string `json:"confidence"`
		FollowUpQuestions []string          `json:"followUpQuestions"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return ExtractionResult{}, ErrUnavailable
	}

	out := ExtractionResult{
		Confidence:        map[string]model.Confidence{},
		FollowUpQuestions: []string{},
	}

	// 모르는 항목이 오면 통째로 버리지 않고, 아는 항목만 남긴다.
	// 하나 틀렸다고 전부 버리면 사용자가 다시 입력해야 한다.
	if len(wire.Extracted) > 0 {
		dec := json.NewDecoder(bytes.NewReader(wire.Extracted))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&out.Extracted); err != nil {
			// 모르는 항목이 섞였다. 관대한 해석으로 한 번 더 시도한다
			_ = json.Unmarshal(wire.Extracted, &out.Extracted)
		}
	}

	// ★ 파생값은 AI 가 채울 수 없다. 무조건 지운다
	out.Extracted.HouseholdIncomePct = nil

	// 잘못된 enum 값은 버린다. 남겨두면 판정에서 "제도 데이터 오류" 로 새어
	// 원인을 엉뚱한 곳에서 찾게 된다. 비워두면 "확인 필요" 가 되어 안전하다
	dropInvalidEnums(&out.Extracted)

	for k, v := range wire.Confidence {
		if !model.FieldExists(k) {
			continue // 우리가 모르는 항목의 확신도는 의미가 없다
		}
		switch model.Confidence(strings.ToUpper(v)) {
		case model.ConfidenceHigh, model.ConfidenceMedium, model.ConfidenceLow:
			out.Confidence[k] = model.Confidence(strings.ToUpper(v))
		}
	}

	for _, q := range wire.FollowUpQuestions {
		if q = strings.TrimSpace(q); q == "" {
			continue
		}
		out.FollowUpQuestions = append(out.FollowUpQuestions, q)
		if len(out.FollowUpQuestions) == followUpQuestions {
			break // 한 번에 너무 많이 물으면 사용자가 지친다
		}
	}

	return out, nil
}

func dropInvalidEnums(ctx *model.UserContext) {
	if ctx.HousingType != nil && !validHousing(*ctx.HousingType) {
		ctx.HousingType = nil
	}
	if ctx.EmploymentStatus != nil && !validEmployment(*ctx.EmploymentStatus) {
		ctx.EmploymentStatus = nil
	}
	if ctx.DisabilityLevel != nil {
		if v := *ctx.DisabilityLevel; v != model.DisabilitySevere && v != model.DisabilityMild {
			ctx.DisabilityLevel = nil
		}
	}
	if ctx.BasicLivelihoodType != nil && !validBasicLivelihood(*ctx.BasicLivelihoodType) {
		ctx.BasicLivelihoodType = nil
	}
}

func validHousing(v model.HousingType) bool {
	switch v {
	case model.HousingMonthlyRent, model.HousingJeonse, model.HousingOwned,
		model.HousingPublicLease, model.HousingFreeUse, model.HousingOther:
		return true
	}
	return false
}

func validEmployment(v model.EmploymentStatus) bool {
	switch v {
	case model.EmploymentEmployed, model.EmploymentSelfEmployed, model.EmploymentLostJob,
		model.EmploymentUnemployed, model.EmploymentStudent, model.EmploymentRetired,
		model.EmploymentOnLeave, model.EmploymentOther:
		return true
	}
	return false
}

func validBasicLivelihood(v model.BasicLivelihoodType) bool {
	switch v {
	case model.BasicLivelihood, model.BasicLivelihoodMedical,
		model.BasicLivelihoodHousing, model.BasicLivelihoodEducation,
		model.BasicLivelihoodNone:
		return true
	}
	return false
}

// cacheKey 는 데모 캐시에서 쓸 키다. 공백 차이로 캐시가 빗나가지 않게 다듬는다.
func cacheKey(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
