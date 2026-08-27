package loader

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/DUE-NAVIGATION/be/internal/model"
)

// Store 는 메모리에 올려둔 제도 데이터다.
//
// ★ 읽기 전용이다. 사용자 입력은 여기에 들어오지 않는다 (설계 원칙 2).
// 시작할 때 한 번 읽고, 이후에는 판정할 때마다 그대로 꺼내 쓴다.
//
// 여러 요청이 동시에 읽으므로 뮤텍스로 감싼다. 개발 모드에서 다시 읽을 때
// 쓰기가 일어나기 때문이다 — 읽기만 한다면 뮤텍스가 필요 없다.
type Store struct {
	mu        sync.RWMutex
	programs  []model.Program
	relations []model.Relation
	problems  []Problem

	dir string
}

// Problem 은 읽다가 발견한 문제다.
//
// ★ 제도 하나가 깨져도 서버를 죽이지 않는다. 나머지 제도는 정상 동작해야 하고,
// 문제는 로그와 이 목록에 남겨 나중에 고치게 한다.
// 발표 도중 제도 JSON 하나 때문에 데모가 멈추면 안 된다.
type Problem struct {
	File   string `json:"file"`
	Reason string `json:"reason"`
}

func (p Problem) String() string {
	return fmt.Sprintf("%s: %s", p.File, p.Reason)
}

// New 는 제도 디렉터리를 읽어 Store 를 만든다.
//
// 반환된 error 는 "디렉터리 자체를 못 읽었다" 는 뜻이다.
// 개별 파일의 문제는 error 가 아니라 Problems() 로 보고한다.
func New(dir string) (*Store, error) {
	s := &Store{dir: dir}
	if err := s.Reload(); err != nil {
		return nil, err
	}
	return s, nil
}

// Reload 는 디렉터리를 다시 읽는다.
//
// 개발 중 팀원이 제도 JSON 을 고치면 서버를 껐다 켜지 않고 반영하기 위한 것이다.
// 제도 데이터 작성이 이 프로젝트 최대 병목이라, 이 왕복을 줄이는 게 실제로 시간을 아낀다.
func (s *Store) Reload() error {
	paths, err := filepath.Glob(filepath.Join(s.dir, "*.json"))
	if err != nil {
		return fmt.Errorf("제도 디렉터리를 읽지 못했습니다 (%s): %w", s.dir, err)
	}
	sort.Strings(paths) // 읽는 순서를 고정한다. 판정은 결정론적이어야 한다

	var (
		programs  []model.Program
		relations []model.Relation
		problems  []Problem
	)
	seen := map[string]string{} // 제도 id → 처음 나온 파일

	for _, path := range paths {
		name := filepath.Base(path)

		// _ 로 시작하는 파일은 스키마·예시 등 제도가 아닌 파일로 본다
		if len(name) > 0 && name[0] == '_' {
			continue
		}

		raw, err := os.ReadFile(path)
		if err != nil {
			problems = append(problems, Problem{name, fmt.Sprintf("파일을 읽지 못했습니다: %v", err)})
			continue
		}

		var p model.Program
		if err := json.Unmarshal(raw, &p); err != nil {
			problems = append(problems, Problem{name, fmt.Sprintf("JSON 형식이 잘못됐습니다: %v", err)})
			continue
		}

		if errs := ValidateProgram(p); len(errs) > 0 {
			for _, e := range errs {
				problems = append(problems, Problem{name, e})
			}
			continue
		}

		if first, dup := seen[p.ID]; dup {
			problems = append(problems, Problem{
				name, fmt.Sprintf("id %q 가 %s 와 중복입니다", p.ID, first),
			})
			continue
		}
		seen[p.ID] = name

		programs = append(programs, p)
	}

	// 제도 간 관계는 별도 파일에 둔다. 없어도 정상이다 (중복수급 관계가 아직 없을 수 있다)
	relPath := filepath.Join(s.dir, "..", "relations.json")
	if raw, err := os.ReadFile(relPath); err == nil {
		if err := json.Unmarshal(raw, &relations); err != nil {
			problems = append(problems, Problem{"relations.json",
				fmt.Sprintf("JSON 형식이 잘못됐습니다: %v", err)})
			relations = nil
		} else {
			relations = filterRelations(relations, seen, &problems)
		}
	}

	s.mu.Lock()
	s.programs, s.relations, s.problems = programs, relations, problems
	s.mu.Unlock()

	return nil
}

// filterRelations 는 존재하지 않는 제도를 가리키는 관계를 걸러낸다.
// 남겨두면 판정에서 조용히 무시되어 "왜 배타가 안 걸리지" 를 찾기 어려워진다.
func filterRelations(in []model.Relation, known map[string]string, problems *[]Problem) []model.Relation {
	out := make([]model.Relation, 0, len(in))
	for _, r := range in {
		switch {
		case r.From == "" || r.To == "":
			*problems = append(*problems, Problem{"relations.json", "from 또는 to 가 비었습니다"})
		case known[r.From] == "":
			*problems = append(*problems, Problem{"relations.json",
				fmt.Sprintf("from %q 는 존재하지 않는 제도입니다", r.From)})
		case known[r.To] == "":
			*problems = append(*problems, Problem{"relations.json",
				fmt.Sprintf("to %q 는 존재하지 않는 제도입니다", r.To)})
		default:
			out = append(out, r)
		}
	}
	return out
}

// Programs 는 읽어둔 제도 목록의 복사본을 돌려준다.
//
// ★ 복사본인 이유: 호출자가 슬라이스를 만지작거려도 원본이 오염되지 않게 하기 위해서다.
// 제도 데이터는 모든 판정이 공유하는 단일 원본이다.
func (s *Store) Programs() []model.Program {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]model.Program(nil), s.programs...)
}

// Relations 는 제도 간 관계의 복사본을 돌려준다.
func (s *Store) Relations() []model.Relation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]model.Relation(nil), s.relations...)
}

// Problems 는 읽는 중 발견한 문제 목록을 돌려준다.
func (s *Store) Problems() []Problem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Problem(nil), s.problems...)
}

// Count 는 정상적으로 읽힌 제도 수다.
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.programs)
}
