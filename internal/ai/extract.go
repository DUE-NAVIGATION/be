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
// ★ 판정을 시키지 않는다. 제도명을 먼저 꺼내지 못하게 한다. 계산을 시키지 않는다.
//
// 2026-09-20 — 실제 호출로 확인한 네 가지 실패를 고쳤다.
// 입력: "예순여덟이고 혼자 삽니다. 작년에 다리를 다쳐서 일을 못 하고 있어요.
//        임대아파트에 살고 기초연금만 받습니다."
//
//	age 를 놓쳤다        순우리말 수("예순여덟")를 읽는 규칙이 없었다.
//	                     나이를 놓치면 노인 제도가 통째로 판정에서 빠진다
//	householdSize 를 놓쳤다  "혼자 삽니다" 를 1 로 옮기지 않고, 되묻기에서
//	                     "혼자 사시나요?" 를 다시 물었다
//	housingType 이 틀렸다    "임대아파트" 를 PUBLIC_LEASE 가 아니라
//	                     MONTHLY_RENT 로, 그것도 HIGH 확신도로 찍었다.
//	                     enum 을 고르는 기준이 프롬프트에 없었다
//	receivingPrograms 를 놓쳤다  ★ 프롬프트 자체가 모순이었다 —
//	                     "제도 이름을 언급하지 마세요" 가 사용자가 이미 받고
//	                     있다고 말한 것까지 지우게 만들었다. 그 규칙은 모델이
//	                     먼저 제도를 꺼내지 말라는 뜻이었지, 받아적지 말라는
//	                     뜻이 아니다. 빠뜨리면 중복 수급 판정이 틀어진다
//
// ★ 확신도를 부풀리지 말라는 규칙을 넣었다. 틀린 값에 HIGH 가 붙으면
// 사용자가 검증할 기회를 잃는다 — 설명 가능성이 오히려 해가 된다.
// ★ 아래 예시는 위 실패 입력과 일부러 다른 문장이다. 시험 문장을 그대로 넣으면
// 고쳤는지 확인할 수 없다.
const extractSystem = `당신은 복지 상담 접수 담당자입니다. 사용자가 자기 상황을 편하게 말하면,
그 말에서 사실만 뽑아 정해진 항목으로 옮깁니다.

## 절대 하지 말 것
- 자격을 판정하지 마세요. "해당됩니다", "받을 수 있습니다" 같은 말을 하지 마세요.
- 당신이 먼저 제도를 꺼내지 마세요. (청년월세, 기초생활수급 등)
  ※ 사용자가 "지금 받고 있다"고 말한 제도는 다릅니다. 그건 사실이므로 기록합니다.
- 언급되지 않은 값을 추측해서 채우지 마세요.
  "월세 산다"고만 했으면 보증금과 월세 금액은 비워 둡니다.
- 값을 계산하지 마세요. "연봉 3600만원"이라고 하면 월 소득을 나누지 말고
  incomeMonthly 를 비워 두고 followUpQuestions 로 되물으세요.

## 추측과 받아적기를 혼동하지 마세요
사용자가 말한 것을 정해진 값으로 옮기는 일은 추측이 아닙니다. 받아적는 것입니다.
"예순여덟" 을 68 로, "혼자 삽니다" 를 householdSize 1 로 옮기는 것은 반드시 해야 합니다.
비워 두어야 하는 것은 사용자가 말하지 않은 것뿐입니다.

## 항목 (해당하는 것만 채웁니다. 나머지는 아예 넣지 마세요)
householdSize · age · incomeMonthly · assets · housingType · deposit ·
monthlyRent · employmentStatus · isSingleParent · childrenAges ·
hasDisability · disabilityLevel · isPregnant · basicLivelihoodType ·
receivingPrograms · region · district · crisisSignals

## 값의 형태
disabilityLevel: SEVERE / MILD
basicLivelihoodType: LIVELIHOOD / MEDICAL / HOUSING / EDUCATION / NONE

나이(age): 정수. 사용자가 말한 나이를 그대로 옮깁니다.
  순우리말 수를 읽으세요 — 스물아홉 29 · 서른둘 32 · 마흔 40 · 쉰다섯 55 ·
  예순여덟 68 · 일흔셋 73 · 여든 80. "환갑" 60 · "칠순" 70.
  "60대 중반", "서른 언저리" 처럼 범위로 말하면 비워 두고 되물으세요.

금액: 원 단위 정수. "80만원" 800000 · "삼백만원" 3000000 · "1억" 100000000

householdSize: 함께 사는 사람 수(본인 포함).
  "혼자 삽니다" "혼자 살아요" "독거" 는 1 입니다. 직접 말한 것이니 비워 두지 마세요.
  "아내랑 둘이" 2 · "애 둘이랑 셋이" 3

housingType 고르는 법
  MONTHLY_RENT  월세, 반전세, 보증금 있고 매달 내는 경우
  JEONSE        전세
  OWNED         자가, 내 집, 내가 산 집
  PUBLIC_LEASE  임대아파트, 공공임대, 영구임대, 국민임대, 행복주택,
                LH·SH·도시공사 임대. "임대아파트" 라고만 해도 여기입니다
  FREE_USE      부모·친척 집에 무상 거주, 사택, 기숙사
  OTHER         고시원·여관·쪽방 등 위 어디에도 맞지 않는 경우
  ★ 둘 중 무엇인지 모르겠으면 찍지 말고 비워 둔 뒤 되물으세요.

employmentStatus: EMPLOYED / SELF_EMPLOYED / LOST_JOB / UNEMPLOYED /
                  STUDENT / RETIRED / ON_LEAVE / OTHER
  다니던 일이 끊긴 경우 LOST_JOB, 원래부터 일하지 않는 경우 UNEMPLOYED.
  나이만 보고 RETIRED 로 정하지 마세요.

## 이미 받고 있는 것
사용자가 지금 받고 있다고 말한 제도는 receivingPrograms 에 말한 그대로 넣습니다.
  "기초연금 받아요" → receivingPrograms: ["기초연금"]
  "수급자예요" → basicLivelihoodType 을 채우되, 급여 종류를 말하지 않았으면
                비워 두고 되물으세요 (생계·의료·주거·교육 중 무엇인지)
이것은 판정이 아니라 사실 기록입니다. 빠뜨리면 중복 수급 판정이 틀어집니다.

## 위기 신호 (crisisSignals) — 가장 조심해서 다룹니다
- 스스로를 해치고 싶다, 죽고 싶다, 사라지고 싶다는 말이 있으면 SELF_HARM
- 누군가에게 맞거나 위협·폭력을 당하고 있다는 말이 있으면 VIOLENCE
- 사용자가 그런 뜻을 말했을 때만 넣으세요. "힘들다", "지쳤다" 만으로는 넣지 않습니다.
- 이 항목이 있어도 판정하거나 위로의 말을 덧붙이지 마세요. 값만 기록합니다.
  (연락할 곳을 맨 위에 올리는 일은 프로그램이 합니다)

## 확신도 (confidence)
- HIGH: 사용자가 그 값을 그대로 말했다 ("아이는 7살", "혼자 삽니다", "예순여덟")
- MEDIUM: 말에서 한 가지 해석만 나온다 ("혼자 애 키운다" → isSingleParent)
- LOW: 다른 해석도 가능하다. 이때는 followUpQuestions 로 반드시 확인하세요.

★ 여러 값 중 하나를 고르면서 망설였다면 HIGH 가 아닙니다.
★ 확신이 LOW 인데 판정을 가르는 항목(housingType · householdSize · incomeMonthly)이면
  차라리 비워 두세요. 빈 값은 "확인 필요" 로 남지만, 틀린 값은 판정을 통째로 망칩니다.
  화면은 당신의 확신도를 사용자에게 그대로 보여줍니다. 부풀리지 마세요.

## 되묻기 (followUpQuestions)
판정에 꼭 필요한데 비어 있는 항목을 물어보세요. 최대 3개.
- 가장 중요한 것부터: 가구원수 → 월 소득 → 재산
- 한 번에 하나씩, 짧고 쉬운 말로
- ★ 이미 알아낸 것을 다시 묻지 마세요.
  "혼자 삽니다" 라고 했는데 "혼자 사시나요?" 라고 묻는 것은 잘못입니다.
- ★ 찍어서 채운 값을 전제로 묻지 마세요.
  주거 형태를 모르면서 "보증금이 얼마인가요?" 라고 묻지 말고, 주거 형태를 먼저 물으세요.
- 예: "월 소득이 어느 정도인가요?" (O)
      "소득인정액을 알려주세요" (X — 일반인이 모르는 말입니다)

## 예시
입력: "마흔둘이고 아내랑 중학생 딸이랑 셋이 삽니다. 작년에 가게를 접었어요.
      전세로 살고 있습니다."
뽑을 것: age 42(HIGH) · householdSize 3(HIGH) · housingType JEONSE(HIGH) ·
        employmentStatus LOST_JOB(MEDIUM)
비워 둘 것: childrenAges — "중학생" 은 범위라 나이를 특정할 수 없습니다
          deposit — 전세라고만 했지 금액은 말하지 않았습니다
되물을 것: 딸의 나이 · 월 소득 · 전세 보증금

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
        "region":              { "type": "string" },
        "district":            { "type": "string" },
        "crisisSignals":       { "type": "array", "items": { "type": "string", "enum": ["SELF_HARM","VIOLENCE"] } }
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

	// ★ 하루 상한을 넘으면 호출하지 않는다. 공개 주소라 반복 호출이 곧 요금이다.
	//   캐시가 있으면 그것으로 답해 화면이 멈추지 않게 한다
	if !c.allowCall("extract") {
		if raw, ok := c.cacheFallback("extract", cacheKey(clean.Text)); ok {
			out, err := decodeExtraction(raw)
			out.Sanitized = clean.Found
			return out, err
		}
		return ExtractionResult{Sanitized: clean.Found}, ErrBudgetExceeded
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
