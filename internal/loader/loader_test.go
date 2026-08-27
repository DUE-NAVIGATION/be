package loader

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/DUE-NAVIGATION/be/internal/model"
)

const testDir = "testdata/programs"

// 깨진 제도가 섞여 있어도 정상 제도는 읽혀야 한다.
// ★ 제도 하나 때문에 서버가 죽거나 전부 안 읽히면 발표 중에 데모가 멈춘다.
func TestLoadSkipsBrokenFiles(t *testing.T) {
	s, err := New(testDir)
	if err != nil {
		t.Fatalf("디렉터리를 읽지 못했다: %v", err)
	}

	if s.Count() != 1 {
		t.Errorf("정상 제도 = %d건, 기대값 1건", s.Count())
		for _, p := range s.Programs() {
			t.Logf("  읽힌 제도: %s", p.ID)
		}
	}
	if len(s.Problems()) == 0 {
		t.Fatal("깨진 파일이 여럿인데 문제가 하나도 보고되지 않았다")
	}

	programs := s.Programs()
	if len(programs) != 1 || programs[0].ID != "good-program" {
		t.Errorf("읽힌 제도가 기대와 다르다: %+v", programs)
	}
}

// 문제 보고가 어느 파일의 무슨 문제인지 짚어줘야 한다.
// 비개발자 팀원이 이걸 보고 고쳐야 하기 때문이다.
func TestProblemsAreSpecific(t *testing.T) {
	s, err := New(testDir)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		file     string
		contains string
	}{
		{"broken-json.json", "JSON"},
		{"typo-field.json", "incomeMontly"},
		{"bad-op.json", "morethan"},
		{"bad-between.json", "2개"},
		{"no-source.json", "revisedAt"},
		{"zz-duplicate-id.json", "중복"},
	}

	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			var found bool
			for _, p := range s.Problems() {
				if p.File == tt.file && strings.Contains(p.Reason, tt.contains) {
					found = true
					t.Logf("  %s", p)
				}
			}
			if !found {
				t.Errorf("%s 에서 %q 를 포함한 문제 보고를 찾지 못했다", tt.file, tt.contains)
			}
		})
	}
}

// 오타 난 필드에는 비슷한 이름을 귀띔해야 한다. 눈으로 찾기 가장 어려운 실수다.
func TestTypoSuggestion(t *testing.T) {
	s, err := New(testDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range s.Problems() {
		if p.File != "typo-field.json" {
			continue
		}
		if strings.Contains(p.Reason, "incomeMonthly") {
			return // 올바른 이름을 제안했다
		}
	}
	t.Error("오타에 대해 incomeMonthly 를 제안하지 않았다")
}

// _ 로 시작하는 파일은 제도가 아니다 (스키마·예시 등).
func TestUnderscoreFilesIgnored(t *testing.T) {
	s, err := New(testDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range s.Problems() {
		if strings.HasPrefix(p.File, "_") {
			t.Errorf("_ 로 시작하는 파일을 읽으려 했다: %s", p)
		}
	}
}

// 없는 디렉터리는 에러가 아니라 빈 결과다 — 아직 제도를 안 만든 상태도 정상이다.
func TestEmptyDirectory(t *testing.T) {
	s, err := New(filepath.Join("testdata", "없는디렉터리"))
	if err != nil {
		t.Fatalf("빈 디렉터리에서 에러가 났다: %v", err)
	}
	if s.Count() != 0 {
		t.Errorf("제도 = %d건, 기대값 0건", s.Count())
	}
}

// Programs 가 복사본을 주지 않으면 호출자가 원본을 오염시킬 수 있다.
func TestProgramsReturnsCopy(t *testing.T) {
	s, err := New(testDir)
	if err != nil {
		t.Fatal(err)
	}

	got := s.Programs()
	if len(got) == 0 {
		t.Fatal("제도가 없다")
	}
	got[0] = model.Program{ID: "오염됨"}

	again := s.Programs()
	if again[0].ID == "오염됨" {
		t.Error("Programs 가 내부 슬라이스를 그대로 넘겨줬다")
	}
}

// 실제 배포되는 제도 데이터가 전부 유효해야 한다.
// 팀원이 제도를 추가하다 깨뜨리면 여기서 잡힌다.
func TestRealProgramsAreValid(t *testing.T) {
	s, err := New(filepath.Join("..", "..", "data", "programs"))
	if err != nil {
		t.Fatalf("제도 디렉터리를 읽지 못했다: %v", err)
	}

	for _, p := range s.Problems() {
		t.Errorf("제도 데이터에 문제가 있다 — %s", p)
	}
	if s.Count() == 0 {
		t.Error("제도가 하나도 없다. 판정할 대상이 없으면 서비스가 성립하지 않는다")
	}

	// 심사에서 반드시 물어보는 항목이다
	for _, p := range s.Programs() {
		if p.Source.URL == "" || p.Source.RevisedAt == "" {
			t.Errorf("%s: 출처(url/revisedAt)가 비어 있다", p.ID)
		}
	}
}
