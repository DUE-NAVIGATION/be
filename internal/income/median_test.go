package income

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/DUE-NAVIGATION/be/internal/model"
)

// 2026년 기준중위소득 (보건복지부 고시 제2025-135호)
func testTable() Table {
	return Table{
		Year: 2026,
		Source: model.Source{
			URL:       "https://www.mohw.go.kr/",
			RevisedAt: "2026-01-01",
			Agency:    "보건복지부",
		},
		ByHouseholdSize: map[string]int64{
			"1": 2564238,
			"2": 4199292,
			"3": 5359036,
			"4": 6494738,
			"5": 7556719,
			"6": 8555952,
		},
		ExtraPerPerson: 999233,
	}
}

func TestMedianIncome(t *testing.T) {
	tbl := testTable()

	tests := []struct {
		name          string
		householdSize int
		want          int64
		wantOK        bool
	}{
		{"1인", 1, 2564238, true},
		{"2인", 2, 4199292, true},
		{"3인", 3, 5359036, true},
		{"4인", 4, 6494738, true},
		{"5인", 5, 7556719, true},
		{"6인", 6, 8555952, true},
		{"7인 — 표에 없으므로 6인 + 증가액", 7, 8555952 + 999233, true},
		{"9인 — 3인분 증가", 9, 8555952 + 999233*3, true},
		{"0인 — 계산 불가", 0, 0, false},
		{"음수 — 계산 불가", -1, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tbl.MedianIncome(tt.householdSize)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, 기대값 %v", ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("금액 = %d, 기대값 %d", got, tt.want)
			}
		})
	}
}

// 표에 구멍이 있으면 지어내지 않고 계산 불가로 답해야 한다.
func TestMedianIncomeWithGap(t *testing.T) {
	tbl := Table{
		Year:            2026,
		Source:          model.Source{URL: "x", RevisedAt: "y"},
		ByHouseholdSize: map[string]int64{"1": 1000, "4": 4000},
		ExtraPerPerson:  500,
	}
	if _, ok := tbl.MedianIncome(2); ok {
		t.Error("표에 없는 2인 가구를 계산해버렸다")
	}
	if got, ok := tbl.MedianIncome(5); !ok || got != 4500 {
		t.Errorf("5인 = %d(%v), 기대값 4500(true)", got, ok)
	}
}

func TestRatio(t *testing.T) {
	calc := Calculator{Table: testTable()}

	tests := []struct {
		name    string
		ctx     model.UserContext
		wantPct float64
		wantOK  bool
	}{
		{
			name: "4인 가구, 소득이 정확히 중위소득 → 100%",
			ctx: model.UserContext{
				HouseholdSize: model.Int(4),
				IncomeMonthly: model.Int64(6494738),
			},
			wantPct: 100,
			wantOK:  true,
		},
		{
			name: "1인 가구, 중위소득의 절반 → 50%",
			ctx: model.UserContext{
				HouseholdSize: model.Int(1),
				IncomeMonthly: model.Int64(1282119),
			},
			wantPct: 50,
			wantOK:  true,
		},
		{
			name: "★ 소득 0원은 계산된다 → 0%",
			ctx: model.UserContext{
				HouseholdSize: model.Int(2),
				IncomeMonthly: model.Int64(0),
			},
			wantPct: 0,
			wantOK:  true,
		},
		{
			name: "★ 소득 미입력은 계산 불가 — 0% 로 단정하지 않는다",
			ctx: model.UserContext{
				HouseholdSize: model.Int(2),
			},
			wantOK: false,
		},
		{
			name: "가구원수 미입력은 계산 불가",
			ctx: model.UserContext{
				IncomeMonthly: model.Int64(1000000),
			},
			wantOK: false,
		},
		{
			name:   "아무것도 모르면 계산 불가",
			ctx:    model.UserContext{},
			wantOK: false,
		},
		{
			name: "환산 파라미터가 없으면 재산은 반영되지 않는다",
			ctx: model.UserContext{
				HouseholdSize: model.Int(1),
				IncomeMonthly: model.Int64(1282119),
				Assets:        model.Int64(500000000), // 5억
			},
			wantPct: 50, // 재산이 있어도 그대로. 반영 안 됨을 고정한다
			wantOK:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := calc.Ratio(tt.ctx)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, 기대값 %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if math.Abs(got-tt.wantPct) > 0.01 {
				t.Errorf("비율 = %.4f%%, 기대값 %.4f%%", got, tt.wantPct)
			}
		})
	}
}

// 환산 파라미터가 채워지면 재산이 소득으로 잡혀야 한다.
func TestRatioWithPropertyConversion(t *testing.T) {
	tbl := testTable()
	tbl.PropertyConversion = &PropertyConversion{
		BasicDeduction: 100000000, // 1억
		MonthlyRatePct: 4.17,
		Source:         model.Source{URL: "x", RevisedAt: "y"},
	}
	calc := Calculator{Table: tbl}

	ctx := model.UserContext{
		HouseholdSize: model.Int(1),
		IncomeMonthly: model.Int64(1000000),
		Assets:        model.Int64(200000000), // 2억 → 초과분 1억
	}

	// 초과분 1억 × 4.17% = 417만원/월
	// 소득인정액 = 100만 + 417만 = 517만
	got, ok := calc.Ratio(ctx)
	if !ok {
		t.Fatal("계산되어야 한다")
	}
	want := float64(1000000+4170000) / 2564238 * 100
	if math.Abs(got-want) > 0.01 {
		t.Errorf("비율 = %.4f%%, 기대값 %.4f%%", got, want)
	}

	// 기본재산액 이하면 환산하지 않는다
	ctx.Assets = model.Int64(50000000)
	got, _ = calc.Ratio(ctx)
	want = float64(1000000) / 2564238 * 100
	if math.Abs(got-want) > 0.01 {
		t.Errorf("기본재산액 이하인데 환산됐다: %.4f%%", got)
	}
}

func TestWithIncomePct(t *testing.T) {
	calc := Calculator{Table: testTable()}

	// 계산 가능하면 채운다
	filled := calc.WithIncomePct(model.UserContext{
		HouseholdSize: model.Int(4),
		IncomeMonthly: model.Int64(3247369), // 정확히 절반
	})
	if filled.HouseholdIncomePct == nil {
		t.Fatal("비율이 채워지지 않았다")
	}
	if math.Abs(*filled.HouseholdIncomePct-50) > 0.01 {
		t.Errorf("비율 = %.4f%%, 기대값 50%%", *filled.HouseholdIncomePct)
	}

	// 계산 불가하면 채우지 않는다 — 조건이 UNKNOWN 이 되어 "확인 필요" 로 간다
	untouched := calc.WithIncomePct(model.UserContext{HouseholdSize: model.Int(4)})
	if untouched.HouseholdIncomePct != nil {
		t.Errorf("계산 불가인데 값이 채워졌다: %v", *untouched.HouseholdIncomePct)
	}
}

// 실제 배포되는 데이터 파일이 읽히고 유효한지 확인한다.
// 팀원이 JSON 을 고치다 깨뜨리면 여기서 잡힌다.
func TestLoadRealTable(t *testing.T) {
	path := filepath.Join("..", "..", "data", "median-income.json")

	tbl, err := LoadTable(path)
	if err != nil {
		t.Fatalf("실제 표를 읽지 못했다: %v", err)
	}
	if tbl.Year != 2026 {
		t.Errorf("기준연도 = %d, 기대값 2026", tbl.Year)
	}
	if got, ok := tbl.MedianIncome(4); !ok || got != 6494738 {
		t.Errorf("4인 가구 = %d(%v), 기대값 6494738(true)", got, ok)
	}
	if tbl.Source.URL == "" || tbl.Source.RevisedAt == "" {
		t.Error("출처가 비어 있다 — 심사에서 물어본다")
	}
}

func TestLoadTableErrors(t *testing.T) {
	if _, err := LoadTable(filepath.Join("testdata", "없는파일.json")); err == nil {
		t.Error("없는 파일인데 에러가 없다")
	}
}
