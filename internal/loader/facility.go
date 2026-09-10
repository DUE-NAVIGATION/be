package loader

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/DUE-NAVIGATION/be/internal/model"
)

// FacilityStore 는 메모리에 올려둔 시설 데이터다.
//
// 제도 Store 와 구조가 같다. 읽기 전용이고, 사용자 입력은 들어오지 않는다.
type FacilityStore struct {
	mu         sync.RWMutex
	facilities []model.Facility
	problems   []Problem

	dir string
}

// NewFacilities 는 시설 디렉터리를 읽어 저장소를 만든다.
//
// 디렉터리가 아예 없으면 빈 저장소를 돌려준다 — 시설 데이터를 아직 안 넣은
// 상태에서도 서버는 떠야 한다. 제도 판정은 시설과 무관하게 동작한다.
func NewFacilities(dir string) (*FacilityStore, error) {
	s := &FacilityStore{dir: dir}
	if err := s.Reload(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *FacilityStore) Reload() error {
	paths, err := filepath.Glob(filepath.Join(s.dir, "*.json"))
	if err != nil {
		return fmt.Errorf("시설 디렉터리를 읽지 못했습니다 (%s): %w", s.dir, err)
	}
	sort.Strings(paths) // 읽는 순서를 고정한다. 판정은 결정론적이어야 한다

	var (
		facilities []model.Facility
		problems   []Problem
	)
	seen := map[string]string{} // 시설 id → 처음 나온 파일

	for _, path := range paths {
		name := filepath.Base(path)
		if len(name) > 0 && name[0] == '_' {
			continue
		}

		raw, err := os.ReadFile(path)
		if err != nil {
			problems = append(problems, Problem{name,
				fmt.Sprintf("파일을 읽지 못했습니다: %v", err)})
			continue
		}

		// 시설은 파일 하나에 여러 건이 들어간다.
		// 지역아동센터만 한 구에 수십 곳이라 파일을 하나씩 만들 수 없다.
		var batch []model.Facility
		if err := json.Unmarshal(raw, &batch); err != nil {
			problems = append(problems, Problem{name,
				fmt.Sprintf("JSON 형식이 잘못됐습니다 (시설 배열이어야 합니다): %v", err)})
			continue
		}

		for i, f := range batch {
			where := fmt.Sprintf("%s[%d]", name, i)

			if errs := ValidateFacility(f); len(errs) > 0 {
				for _, e := range errs {
					problems = append(problems, Problem{where, e})
				}
				continue
			}
			if first, dup := seen[f.ID]; dup {
				problems = append(problems, Problem{where,
					fmt.Sprintf("id %q 가 %s 와 중복입니다", f.ID, first)})
				continue
			}
			seen[f.ID] = where
			facilities = append(facilities, f)
		}
	}

	// id 순으로 고정한다. 같은 입력에 항상 같은 순서가 나와야 한다
	sort.SliceStable(facilities, func(i, j int) bool {
		return facilities[i].ID < facilities[j].ID
	})

	s.mu.Lock()
	s.facilities, s.problems = facilities, problems
	s.mu.Unlock()
	return nil
}

func (s *FacilityStore) Facilities() []model.Facility {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.Facility, len(s.facilities))
	copy(out, s.facilities)
	return out
}

func (s *FacilityStore) Problems() []Problem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Problem, len(s.problems))
	copy(out, s.problems)
	return out
}

func (s *FacilityStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.facilities)
}

// ValidateFacility 는 시설 하나를 검사한다.
//
// ★ 여기서 가장 중요한 검사는 "연락할 방법이 있는가" 다.
// 연락할 수 없는 시설은 이 서비스에서 존재 이유가 없다 — 목록만 늘리고
// 이용자를 막다른 길로 보낸다.
func ValidateFacility(f model.Facility) []string {
	var errs []string

	if strings.TrimSpace(f.ID) == "" {
		errs = append(errs, "id 가 비었습니다")
	}
	if strings.TrimSpace(f.Name) == "" {
		errs = append(errs, "name 이 비었습니다")
	}
	if !model.FacilityTypeIsKnown(f.Type) {
		errs = append(errs, fmt.Sprintf("모르는 시설 종류입니다: %q", f.Type))
	}

	// ★ 연락 수단
	if !f.Contact.Reachable() {
		errs = append(errs, "연락할 방법이 하나도 없습니다 "+
			"(phone · email · applyUrl · website 중 최소 하나). "+
			"연락할 수 없는 시설은 이용자를 막다른 길로 보냅니다")
	}

	errs = append(errs, validateCoverage(f.Coverage)...)

	// 전화 상담 창구가 아니면 주소가 있어야 찾아갈 수 있다
	if f.Type != model.FacilityHotline && !f.Location.HasAddress() {
		errs = append(errs, "주소가 없습니다 (roadAddress 또는 lotAddress). "+
			"전화 상담 창구(HOTLINE)가 아니면 찾아갈 곳이 있어야 합니다")
	}

	// 출처는 심사에서 물어본다
	if strings.TrimSpace(f.Source.RevisedAt) == "" {
		errs = append(errs, "source.revisedAt 이 비었습니다 (데이터 기준일자)")
	}

	// 이용 대상 조건은 제도와 완전히 같은 규칙으로 검사한다.
	// 검사기를 따로 만들지 않는다 — 두 벌이 되면 반드시 어긋난다
	errs = append(errs, validateConditions("all", f.Eligibility.All)...)
	errs = append(errs, validateConditions("any", f.Eligibility.Any)...)
	errs = append(errs, validateConditions("none", f.Eligibility.None)...)

	return errs
}

func validateCoverage(c model.Coverage) []string {
	var errs []string

	switch c.Scope {
	case model.CoverageNationwide:
		if c.Sido != "" || c.Sigungu != "" {
			errs = append(errs, "coverage.scope 가 NATIONWIDE 인데 sido/sigungu 가 채워져 있습니다")
		}

	case model.CoverageSido:
		if strings.TrimSpace(c.Sido) == "" {
			errs = append(errs, "coverage.scope 가 SIDO 인데 sido 가 비었습니다")
		}

	case model.CoverageSigungu:
		if strings.TrimSpace(c.Sido) == "" {
			errs = append(errs, "coverage.scope 가 SIGUNGU 인데 sido 가 비었습니다")
		}
		if strings.TrimSpace(c.Sigungu) == "" {
			errs = append(errs, "coverage.scope 가 SIGUNGU 인데 sigungu 가 비었습니다")
		}

	default:
		errs = append(errs, fmt.Sprintf(
			"모르는 coverage.scope 입니다: %q (NATIONWIDE · SIDO · SIGUNGU)", c.Scope))
	}

	return errs
}
