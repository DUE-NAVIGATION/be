package rules

import (
	"testing"

	"github.com/DUE-NAVIGATION/be/internal/model"
)

func TestAnnualAmount(t *testing.T) {
	tests := []struct {
		name    string
		benefit model.Benefit
		want    int64
		wantOK  bool
	}{
		{
			name:    "월 20만원 × 12개월 (기획서 예시)",
			benefit: model.Benefit{Type: model.BenefitMonthly, Amount: 200000, Months: 12},
			want:    2400000,
			wantOK:  true,
		},
		{
			name:    "months 가 없으면 계속 지급으로 보고 12개월",
			benefit: model.Benefit{Type: model.BenefitMonthly, Amount: 100000},
			want:    1200000,
			wantOK:  true,
		},
		{
			name:    "지급 기간이 1년 미만이면 그 기간만",
			benefit: model.Benefit{Type: model.BenefitMonthly, Amount: 300000, Months: 6},
			want:    1800000,
			wantOK:  true,
		},
		{
			name:    "★ 24개월 지원이어도 연 금액은 12개월치만",
			benefit: model.Benefit{Type: model.BenefitMonthly, Amount: 200000, Months: 24},
			want:    2400000,
			wantOK:  true,
		},
		{
			name:    "1회성",
			benefit: model.Benefit{Type: model.BenefitOnce, Amount: 500000},
			want:    500000,
			wantOK:  true,
		},
		{
			name:    "연 정액",
			benefit: model.Benefit{Type: model.BenefitYearly, Amount: 1200000},
			want:    1200000,
			wantOK:  true,
		},
		{
			name:    "★ 요금 감면율은 금액으로 환산하지 않는다",
			benefit: model.Benefit{Type: model.BenefitRate, RatePct: 30},
			want:    0,
			wantOK:  false,
		},
		{
			name:    "★ 현물·서비스는 금액이 없다",
			benefit: model.Benefit{Type: model.BenefitInKind, Note: "돌봄 서비스 주 3회"},
			want:    0,
			wantOK:  false,
		},
		{
			name:    "금액이 0이면 산정 불가",
			benefit: model.Benefit{Type: model.BenefitMonthly, Amount: 0, Months: 12},
			want:    0,
			wantOK:  false,
		},
		{
			name:    "음수 금액은 산정 불가",
			benefit: model.Benefit{Type: model.BenefitOnce, Amount: -1000},
			want:    0,
			wantOK:  false,
		},
		{
			name:    "알 수 없는 급여 형태 → 지어내지 않는다",
			benefit: model.Benefit{Type: model.BenefitType("WEEKLY"), Amount: 10000},
			want:    0,
			wantOK:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := AnnualAmount(tt.benefit)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, 기대값 %v", ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("금액 = %d, 기대값 %d", got, tt.want)
			}
		})
	}
}

func TestWithEstimates(t *testing.T) {
	results := []model.MatchResult{
		{
			Program: model.Program{ID: "a", Benefit: model.Benefit{Type: model.BenefitMonthly, Amount: 200000, Months: 12}},
			Status:  model.MatchEligible,
		},
		{
			// 확인 필요여도 금액은 채운다 — "확인되면 얼마" 를 보여줘야 한다
			Program: model.Program{ID: "b", Benefit: model.Benefit{Type: model.BenefitOnce, Amount: 500000}},
			Status:  model.MatchNeedsInfo,
		},
		{
			Program: model.Program{ID: "c", Benefit: model.Benefit{Type: model.BenefitRate, RatePct: 30}},
			Status:  model.MatchEligible,
		},
	}

	got := WithEstimates(results)

	if got[0].EstimatedAmount != 2400000 {
		t.Errorf("a = %d, 기대값 2400000", got[0].EstimatedAmount)
	}
	if got[1].EstimatedAmount != 500000 {
		t.Errorf("b(확인필요) = %d, 기대값 500000", got[1].EstimatedAmount)
	}
	if got[2].EstimatedAmount != 0 {
		t.Errorf("c(감면율) = %d, 기대값 0", got[2].EstimatedAmount)
	}

	// 원본을 건드리지 않는다
	if results[0].EstimatedAmount != 0 {
		t.Error("원본 슬라이스가 변경됐다")
	}
}

func TestSummarize(t *testing.T) {
	results := []model.MatchResult{
		{Status: model.MatchEligible, EstimatedAmount: 2400000},
		{Status: model.MatchEligible, EstimatedAmount: 1200000},
		{Status: model.MatchNeedsInfo, EstimatedAmount: 9999999}, // 합계에 들어가면 안 된다
		{Status: model.MatchNeedsInfo},
		{Status: model.MatchIneligible, EstimatedAmount: 8888888},
	}

	got := Summarize(results)

	if got.EligibleCount != 2 {
		t.Errorf("EligibleCount = %d, 기대값 2", got.EligibleCount)
	}
	if got.NeedsInfoCount != 2 {
		t.Errorf("NeedsInfoCount = %d, 기대값 2", got.NeedsInfoCount)
	}
	if got.IneligibleCount != 1 {
		t.Errorf("IneligibleCount = %d, 기대값 1", got.IneligibleCount)
	}
	// ★ 확인 필요·미해당 금액은 절대 더하지 않는다
	if got.TotalAnnualAmount != 3600000 {
		t.Errorf("TotalAnnualAmount = %d, 기대값 3600000", got.TotalAnnualAmount)
	}
}

func TestSummarizeEmpty(t *testing.T) {
	got := Summarize(nil)
	if got.EligibleCount != 0 || got.TotalAnnualAmount != 0 {
		t.Errorf("빈 결과의 요약이 이상하다: %+v", got)
	}
}
