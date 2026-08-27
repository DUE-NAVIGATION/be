package rules

import "github.com/DUE-NAVIGATION/be/internal/model"

// AnnualAmount 는 급여 정의로부터 "첫 1년 동안 받게 되는 금액"(원)을 구한다.
//
// ★ 전부 정수 연산이다. 부동소수점을 쓰지 않는다 — 금액이 1원씩 어긋나면
// 화면에 뜬 숫자와 합계가 맞지 않고, 심사에서 바로 티가 난다.
//
// 두 번째 반환값은 "산정할 수 있었는가" 다. false 면 합계에 넣지 않는다.
// RATE(요금 감면율)나 IN_KIND(현물·서비스)는 금액으로 환산할 수 없다.
// 이 구분이 없으면 데모의 "연 480만원" 이 오염된다.
func AnnualAmount(b model.Benefit) (int64, bool) {
	switch b.Type {
	case model.BenefitMonthly:
		if b.Amount <= 0 {
			return 0, false
		}
		// months 가 없으면 계속 지급되는 급여로 보고 12개월로 환산한다.
		// 지급 기간이 1년을 넘으면(예: 24개월) 첫 1년치만 센다 —
		// "연 얼마" 라고 말하면서 2년치를 보여주면 과장이다.
		months := b.Months
		if months <= 0 {
			months = 12
		}
		if months > 12 {
			months = 12
		}
		return b.Amount * int64(months), true

	case model.BenefitOnce, model.BenefitYearly:
		if b.Amount <= 0 {
			return 0, false
		}
		return b.Amount, true

	case model.BenefitRate, model.BenefitInKind:
		// 감면율·현물은 금액으로 환산하지 않는다. 화면에는 급여 내용을 그대로 보여준다
		return 0, false
	}

	// 알 수 없는 급여 형태. 제도 JSON 오류이므로 지어내지 않는다
	return 0, false
}

// WithEstimates 는 판정 결과에 연간 예상 수령액을 채워 돌려준다.
//
// 확인 필요(NEEDS_INFO)인 제도에도 금액을 채운다 —
// "확인되면 연 240만원" 을 보여줘야 사용자가 정보를 더 입력할 이유가 생긴다.
// 다만 합계에는 넣지 않는다. 그건 Summarize 가 판단한다.
func WithEstimates(results []model.MatchResult) []model.MatchResult {
	out := make([]model.MatchResult, 0, len(results))
	for _, r := range results {
		amount, ok := AnnualAmount(r.Program.Benefit)
		if !ok {
			amount = 0
		}
		r.EstimatedAmount = amount
		out = append(out, r)
	}
	return out
}

// Summarize 는 결과 화면 상단의 요약을 만든다.
// "확인된 것 6건 · 연 4,800,000원 · 추가 확인 3건"
//
// ★ 합계에는 ELIGIBLE 만 넣는다.
// 확인이 필요한 제도의 금액까지 더하면 받지도 못할 금액을 약속하는 셈이 된다.
func Summarize(results []model.MatchResult) model.Summary {
	var s model.Summary
	for _, r := range results {
		switch r.Status {
		case model.MatchEligible:
			s.EligibleCount++
			s.TotalAnnualAmount += r.EstimatedAmount
		case model.MatchNeedsInfo:
			s.NeedsInfoCount++
		case model.MatchIneligible:
			s.IneligibleCount++
		}
	}
	return s
}
