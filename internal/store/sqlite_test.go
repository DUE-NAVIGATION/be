package store_test

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/DUE-NAVIGATION/be/internal/loader"
	"github.com/DUE-NAVIGATION/be/internal/model"
	"github.com/DUE-NAVIGATION/be/internal/rules"
	"github.com/DUE-NAVIGATION/be/internal/store"
)

// dataDir 는 실제 제도 데이터다. 픽스처를 따로 만들지 않는다 —
// 진짜 데이터로 왕복해야 의미가 있다.
func dataDir() string { return filepath.Join("..", "..", "data") }

// seeded 는 실제 JSON 을 임시 DB 로 옮겨 담고 두 저장소를 함께 돌려준다.
func seeded(t *testing.T) (*loader.Store, *store.Store) {
	t.Helper()

	src, err := loader.New(filepath.Join(dataDir(), "programs"))
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "test.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Seed(db, src.Programs(), src.Relations()); err != nil {
		t.Fatal(err)
	}
	db.Close()

	dst, err := store.New(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { dst.Close() })

	return src, dst
}

// ★ 이 테스트가 이 패키지의 존재 이유다.
//
// 제도 데이터를 DB 로 옮기면서 조건 하나, 금액 하나라도 달라지면
// 같은 사람이 같은 상황을 넣었는데 다른 답을 받는다. 그건 버그가 아니라 사고다.
func TestSQLiteRoundTripKeepsProgramsIdentical(t *testing.T) {
	src, dst := seeded(t)

	want, got := src.Programs(), dst.Programs()
	if len(want) != len(got) {
		t.Fatalf("제도 수가 다르다: JSON %d건, DB %d건", len(want), len(got))
	}

	// loader 도 store 도 id 순으로 돌려준다. 순서까지 같아야 화면이 같다
	for i := range want {
		if !reflect.DeepEqual(want[i], got[i]) {
			t.Errorf("%s 가 왕복에서 달라졌다\nJSON: %+v\n  DB: %+v",
				want[i].ID, want[i], got[i])
		}
	}
}

func TestSQLiteRoundTripKeepsRelationsIdentical(t *testing.T) {
	src, dst := seeded(t)

	want, got := src.Relations(), dst.Relations()

	// nil 과 빈 슬라이스는 뜻이 같다. 관계가 0건인 지금 상태에서
	// DeepEqual 로 통째로 비교하면 그 차이만으로 실패한다
	if len(want) != len(got) {
		t.Fatalf("관계 수가 다르다: JSON %d건, DB %d건", len(want), len(got))
	}
	for i := range want {
		if !reflect.DeepEqual(want[i], got[i]) {
			t.Errorf("관계 %d번이 왕복에서 달라졌다\nJSON: %+v\n  DB: %+v",
				i, want[i], got[i])
		}
	}
}

// ★ 판정까지 같아야 한다. 구조가 같아도 엔진에 들어가는 순간 달라질 수 있다
// (조건 순서가 뒤집히면 근거표의 줄 순서가 달라진다).
func TestJudgementIsIdenticalFromEitherSource(t *testing.T) {
	src, dst := seeded(t)

	two := 2
	age := 33
	income := int64(2_560_000)
	single := true
	lostJob := model.EmploymentLostJob

	ctx := model.UserContext{
		HouseholdSize:    &two,
		Age:              &age,
		IncomeMonthly:    &income,
		IsSingleParent:   &single,
		ChildrenAges:     []int{7},
		EmploymentStatus: &lostJob,
	}

	fromJSON := evaluateAll(src.Programs(), ctx)
	fromDB := evaluateAll(dst.Programs(), ctx)

	if len(fromJSON) != len(fromDB) {
		t.Fatalf("판정 건수가 다르다: %d vs %d", len(fromJSON), len(fromDB))
	}
	for i := range fromJSON {
		if !reflect.DeepEqual(fromJSON[i], fromDB[i]) {
			t.Errorf("%s 의 판정이 저장소에 따라 달라졌다\nJSON: %+v\n  DB: %+v",
				fromJSON[i].Program.ID, fromJSON[i], fromDB[i])
		}
	}
}

func evaluateAll(ps []model.Program, ctx model.UserContext) []model.MatchResult {
	out := make([]model.MatchResult, 0, len(ps))
	for _, p := range ps {
		out = append(out, rules.Evaluate(p, ctx))
	}
	return rules.WithEstimates(out)
}

// 씨딩은 통째로 다시 쓴다. 두 번 돌려도 늘어나지 않아야 한다.
func TestSeedIsIdempotent(t *testing.T) {
	src, err := loader.New(filepath.Join(dataDir(), "programs"))
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "twice.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for i := 0; i < 2; i++ {
		if err := store.Seed(db, src.Programs(), src.Relations()); err != nil {
			t.Fatalf("%d번째 씨딩 실패: %v", i+1, err)
		}
	}

	dst, err := store.New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer dst.Close()

	if want, got := src.Count(), dst.Count(); want != got {
		t.Errorf("두 번 씨딩 후 제도 수 = %d, want %d", got, want)
	}
}

// ★★ 이 DB 에 사용자 테이블이 생기면 실패한다.
//
// 설계 원칙 2 는 문서가 아니라 테스트로 지킨다. 누군가 "결과 저장" 이나
// "사용자 이력" 테이블을 추가하면 여기서 막힌다.
func TestSchemaHasNoUserTables(t *testing.T) {
	path := filepath.Join(t.TempDir(), "schema.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rows, err := db.Query(
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	allowed := map[string]bool{"programs": true, "conditions": true, "relations": true}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		if !allowed[name] {
			t.Errorf("★ 허용되지 않은 테이블: %q — 이 DB 에는 제도 데이터만 담는다. "+
				"사용자 입력을 저장하면 설계 원칙 2가 무너진다", name)
		}
	}
}
