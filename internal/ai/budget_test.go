package ai

import (
	"testing"
	"time"
)

// ★ 상한을 넘으면 더 쓰지 못한다. 요금이 여기서 멈춘다.
func TestBudgetStopsAtLimit(t *testing.T) {
	b := newBudget(2)

	if !b.spend() || !b.spend() {
		t.Fatal("상한 안에서는 쓸 수 있어야 한다")
	}
	if b.spend() {
		t.Error("상한을 넘어서도 쓸 수 있으면 요금이 멈추지 않는다")
	}
	if used, limit := b.used0(); used != 2 || limit != 2 {
		t.Errorf("used0() = %d/%d, want 2/2", used, limit)
	}
}

// 날짜가 바뀌면 다시 센다.
func TestBudgetResetsNextDay(t *testing.T) {
	day := time.Date(2026, 9, 16, 23, 0, 0, 0, time.Local)
	b := newBudget(1)
	b.now = func() time.Time { return day }

	if !b.spend() {
		t.Fatal("첫 호출은 되어야 한다")
	}
	if b.spend() {
		t.Fatal("같은 날 상한을 넘으면 막아야 한다")
	}

	b.now = func() time.Time { return day.Add(2 * time.Hour) } // 다음 날
	if !b.spend() {
		t.Error("날짜가 바뀌면 다시 쓸 수 있어야 한다")
	}
}

// 상한을 0 으로 두면 제한하지 않는다 (로컬 개발·시연용).
func TestBudgetZeroMeansUnlimited(t *testing.T) {
	b := newBudget(0)
	for i := 0; i < 50; i++ {
		if !b.spend() {
			t.Fatalf("%d번째에서 막혔다. 0 은 무제한이어야 한다", i+1)
		}
	}
}

// nil 이어도 터지지 않는다 — 예산을 안 쓰는 경로가 있다.
func TestBudgetNilIsSafe(t *testing.T) {
	var b *budget
	if !b.spend() {
		t.Error("nil 예산은 막지 않아야 한다")
	}
	if used, limit := b.used0(); used != 0 || limit != 0 {
		t.Errorf("used0() = %d/%d, want 0/0", used, limit)
	}
}
