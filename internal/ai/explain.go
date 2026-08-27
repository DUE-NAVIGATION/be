package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/DUE-NAVIGATION/be/internal/model"
)

// explainSystem 은 결과 설명 프롬프트다.
//
// ★ 판정은 이미 끝났다. 이 단계는 설명만 한다.
// 뒤집거나 새 제도를 언급하면 규칙 엔진을 쓰는 의미가 사라진다.
const explainSystem = `당신은 복지 상담원입니다. 이미 나온 판정 결과를 사용자가 이해할 수 있게
설명합니다. 판정은 이미 끝났습니다. 당신은 설명만 합니다.

## 절대 하지 말 것
- ★ 판정을 뒤집지 마세요. 결과가 "확인필요"인데 "받으실 수 있습니다"라고
  쓰면 안 됩니다.
- ★ 주어진 목록에 없는 제도를 언급하지 마세요.
- 금액을 새로 계산하거나 바꾸지 마세요. 주어진 숫자를 그대로 쓰세요.
- "반드시", "확실히", "보장됩니다" 같은 단정적 표현을 쓰지 마세요.

## 어떻게 쓸 것인가
- 중학생이 읽을 수 있는 문장으로. 한자어를 풀어 쓰세요.
  "소득인정액" → "소득과 재산을 합쳐 계산한 금액"
- 3~5문장. 길게 쓰지 마세요.
- 순서: ①받을 수 있는 것 ②확인이 더 필요한 것과 무엇을 알려주면 되는지
        ③다음에 할 일
- 확인필요 항목은 "아직 알 수 없다"이지 "안 된다"가 아닙니다.
  그 차이가 드러나게 쓰세요.

## 예시
"입력하신 내용으로는 한부모가족 아동양육비를 받으실 수 있을 것으로 보입니다.
 연 276만원 정도입니다. 청년월세 지원은 주거급여를 받고 계신지 확인되면
 판단할 수 있습니다. 주민센터나 복지로에서 신청하실 수 있습니다."`

const explainToolName = "write_explanation"

const explainToolDesc = "이미 나온 판정 결과를 사용자가 읽을 설명문으로 옮깁니다. 판정하지 않습니다."

var explainSchema = json.RawMessage(`{
  "type": "object",
  "properties": {
    "explanation": { "type": "string", "minLength": 10, "maxLength": 1000 }
  },
  "required": ["explanation"],
  "additionalProperties": false
}`)

// Explain 은 판정 결과를 사람이 읽는 설명문으로 바꾼다.
//
// ★ 사용자의 상황(UserContext)을 통째로 보내지 않는다.
// 설명을 쓰는 데 필요한 것은 "무엇이 어떻게 판정됐는가" 뿐이다.
// 소득·가족관계를 다시 내보낼 이유가 없다 (설계 원칙 2).
func (c *Client) Explain(ctx context.Context, results []model.MatchResult, summary model.Summary) (string, error) {
	brief := briefResults(results, summary)

	if raw, ok := c.cached("explain", cacheKey(brief)); ok {
		return decodeExplanation(raw)
	}

	raw, err := c.callTool(ctx, explainSystem, brief,
		explainToolName, explainToolDesc, explainSchema)
	if err != nil {
		return "", err
	}
	return decodeExplanation(raw)
}

func decodeExplanation(raw json.RawMessage) (string, error) {
	var wire struct {
		Explanation string `json:"explanation"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return "", ErrUnavailable
	}
	if strings.TrimSpace(wire.Explanation) == "" {
		return "", ErrUnavailable
	}
	return strings.TrimSpace(wire.Explanation), nil
}

// briefResults 는 판정 결과를 설명에 필요한 만큼만 줄여서 적는다.
//
// 여기 적히지 않은 것은 모델이 알 수 없다 — 그게 목적이다.
// 모델이 언급할 수 있는 제도는 이 목록에 있는 것뿐이다.
func briefResults(results []model.MatchResult, summary model.Summary) string {
	var b strings.Builder

	fmt.Fprintf(&b, "판정 요약: 해당 %d건, 확인필요 %d건, 미해당 %d건, 해당 제도 연간 합계 %d원\n\n",
		summary.EligibleCount, summary.NeedsInfoCount, summary.IneligibleCount,
		summary.TotalAnnualAmount)

	write := func(title string, want model.MatchStatus, withReasons bool) {
		var lines []string
		for _, r := range results {
			if r.Status != want {
				continue
			}
			line := "- " + r.Program.Name
			if r.EstimatedAmount > 0 {
				line += fmt.Sprintf(" (연 %d원)", r.EstimatedAmount)
			}
			if withReasons {
				if reasons := unknownReasons(r); reasons != "" {
					line += " — 확인이 필요한 것: " + reasons
				}
			}
			lines = append(lines, line)
		}
		if len(lines) == 0 {
			return
		}
		b.WriteString(title + "\n" + strings.Join(lines, "\n") + "\n\n")
	}

	write("[해당]", model.MatchEligible, false)
	write("[확인필요]", model.MatchNeedsInfo, true)

	// 미해당은 이름만. 왜 안 되는지까지 길게 쓰면 설명문이 부정적으로 흐른다
	write("[미해당]", model.MatchIneligible, false)

	b.WriteString("위 목록에 있는 제도만 언급하세요. 금액은 위에 적힌 숫자를 그대로 쓰세요.")
	return b.String()
}

// unknownReasons 는 "무엇을 알려주면 판단할 수 있는지" 를 사람 말로 모은다.
func unknownReasons(r model.MatchResult) string {
	var out []string
	seen := map[string]bool{}
	for _, c := range r.Conditions {
		if c.Status != model.StatusUnknown {
			continue
		}
		name := c.Condition.Label
		if name == "" {
			name = c.Condition.Field
		}
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return strings.Join(out, ", ")
}
