package income

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/DUE-NAVIGATION/be/internal/model"
)

// Table 은 기준중위소득 표다. data/median-income.json 과 1:1 로 대응한다.
//
// ★ 기준연도와 출처를 데이터와 함께 들고 다닌다. 화면에 "2026년 기준 · 출처 보기" 를
// 띄워야 하고, 심사에서 반드시 물어보기 때문이다.
type Table struct {
	Year   int          `json:"year"`
	Source model.Source `json:"source"`

	// 가구원수 → 월 기준중위소득 (원). JSON 의 키는 문자열이다
	ByHouseholdSize map[string]int64 `json:"byHouseholdSize"`

	// 표에 없는 큰 가구는 1인 증가시마다 이 금액을 더한다
	ExtraPerPerson int64 `json:"extraPerPerson"`

	// 재산의 소득환산 파라미터. 확인되지 않았으면 nil 이다
	PropertyConversion *PropertyConversion `json:"propertyConversion"`
}

// PropertyConversion 은 재산을 월 소득으로 환산하는 파라미터다.
//
// ★ 이 값들은 지역과 재산 종류에 따라 다르다. 추측해서 채우지 마라.
// 틀린 값을 넣으면 금액이 틀린 채로 데모가 나간다.
type PropertyConversion struct {
	// 기본재산액 (원). 이 금액까지는 환산하지 않는다
	BasicDeduction int64 `json:"basicDeduction"`
	// 월 소득환산율 (%)
	MonthlyRatePct float64      `json:"monthlyRatePct"`
	Source         model.Source `json:"source"`
}

// LoadTable 은 기준중위소득 표를 파일에서 읽는다.
//
// 이 표가 없으면 소득 판정 자체가 불가능하므로, 실패하면 에러를 그대로 올린다.
// (제도 JSON 과 다르다. 제도는 하나 깨져도 나머지가 동작해야 하지만, 이건 없으면 안 된다)
func LoadTable(path string) (Table, error) {
	var t Table

	raw, err := os.ReadFile(path)
	if err != nil {
		return t, fmt.Errorf("기준중위소득 표를 읽지 못했습니다 (%s): %w", path, err)
	}
	if err := json.Unmarshal(raw, &t); err != nil {
		return t, fmt.Errorf("기준중위소득 표의 형식이 잘못됐습니다 (%s): %w", path, err)
	}
	if err := t.Validate(); err != nil {
		return t, fmt.Errorf("기준중위소득 표가 유효하지 않습니다 (%s): %w", path, err)
	}
	return t, nil
}

// Validate 는 표가 쓸 만한 상태인지 확인한다.
func (t Table) Validate() error {
	if t.Year <= 0 {
		return fmt.Errorf("기준연도가 없습니다")
	}
	if len(t.ByHouseholdSize) == 0 {
		return fmt.Errorf("가구원수별 금액이 비어 있습니다")
	}
	if t.Source.URL == "" || t.Source.RevisedAt == "" {
		return fmt.Errorf("출처(url, revisedAt)가 없습니다")
	}
	for k, v := range t.ByHouseholdSize {
		size, err := strconv.Atoi(k)
		if err != nil || size <= 0 {
			return fmt.Errorf("가구원수 키 %q 가 잘못됐습니다", k)
		}
		if v <= 0 {
			return fmt.Errorf("%d인 가구 금액이 잘못됐습니다: %d", size, v)
		}
	}
	return nil
}

// MedianIncome 은 가구원수에 해당하는 월 기준중위소득을 돌려준다.
//
// 표에 없는 큰 가구는 가장 큰 가구원수 금액에 ExtraPerPerson 을 더해 산정한다.
// 고시의 "1인 증가시마다 ... 더하여 산정" 규칙과 같은 방식이다.
func (t Table) MedianIncome(householdSize int) (int64, bool) {
	if householdSize <= 0 {
		return 0, false
	}
	if v, ok := t.ByHouseholdSize[strconv.Itoa(householdSize)]; ok {
		return v, true
	}

	maxSize, maxValue := 0, int64(0)
	for k, v := range t.ByHouseholdSize {
		size, err := strconv.Atoi(k)
		if err != nil {
			continue
		}
		if size > maxSize {
			maxSize, maxValue = size, v
		}
	}
	if maxSize == 0 || householdSize < maxSize {
		// 표보다 작은데 키가 없다 = 표에 구멍이 있다. 지어내지 않는다
		return 0, false
	}
	if t.ExtraPerPerson <= 0 {
		return 0, false
	}
	return maxValue + t.ExtraPerPerson*int64(householdSize-maxSize), true
}

// Calculator 는 표를 들고 소득 비율을 계산한다.
//
// 전역 변수 대신 값을 들고 다니는 이유: 테스트에서 연도별 표를 갈아끼울 수 있고,
// 표를 아직 안 읽은 상태로 계산이 호출되는 사고를 컴파일 단계에서 막을 수 있다.
type Calculator struct {
	Table Table
}

// Ratio 는 중위소득 대비 비율(%)을 계산한다.
//
//	소득인정액   = 소득평가액 + 재산의 소득환산액
//	중위소득비율 = 소득인정액 / 기준중위소득(가구원수) × 100
//
// ★ 이 값 하나로 대부분의 제도 자격이 갈린다. AI 에게 맡기지 않고 코드로 계산한다.
//
// 두 번째 반환값은 "계산할 수 있었는가" 다. 입력이 부족하면 false 이고,
// 이때 호출자는 0% 로 단정하지 말고 "확인 필요" 로 넘겨야 한다.
func (c Calculator) Ratio(ctx model.UserContext) (float64, bool) {
	if ctx.HouseholdSize == nil || ctx.IncomeMonthly == nil {
		return 0, false
	}
	median, ok := c.Table.MedianIncome(*ctx.HouseholdSize)
	if !ok || median <= 0 {
		return 0, false
	}

	recognized := *ctx.IncomeMonthly + c.propertyIncome(ctx.Assets)
	if recognized < 0 {
		recognized = 0
	}

	return float64(recognized) / float64(median) * 100, true
}

// propertyIncome 은 재산의 월 소득환산액을 구한다.
//
// 환산 파라미터가 확인되지 않았거나(nil) 재산을 모르면 0 을 돌려준다.
// ★ 여기서 0 은 "재산이 없다" 가 아니라 "이번 계산에 반영하지 않았다" 는 뜻이다.
// 파라미터를 채우기 전까지 재산이 많은 사람의 비율이 실제보다 낮게 나온다.
func (c Calculator) propertyIncome(assets *int64) int64 {
	pc := c.Table.PropertyConversion
	if pc == nil || assets == nil {
		return 0
	}
	excess := *assets - pc.BasicDeduction
	if excess <= 0 {
		return 0
	}
	return int64(float64(excess) * pc.MonthlyRatePct / 100)
}

// WithIncomePct 는 계산된 비율을 UserContext 에 채워 돌려준다.
// 제도 조건이 householdIncomePct 를 참조하므로, 판정 직전에 한 번 통과시킨다.
//
// 계산이 불가능하면 원본을 그대로 돌려준다 — 그러면 그 조건은 UNKNOWN 이 되고
// 결과는 "확인 필요" 가 된다. 값을 지어내지 않는다.
func (c Calculator) WithIncomePct(ctx model.UserContext) model.UserContext {
	pct, ok := c.Ratio(ctx)
	if !ok {
		return ctx
	}
	ctx.HouseholdIncomePct = model.Float(pct)
	return ctx
}
