package loader

import (
	"strings"
	"testing"

	"github.com/DUE-NAVIGATION/be/internal/model"
)

func validFacility() model.Facility {
	return model.Facility{
		ID:       "t",
		Name:     "테스트센터",
		Type:     model.FacilityHotline,
		Sector:   model.SectorPublic,
		Coverage: model.Coverage{Scope: model.CoverageNationwide},
		Contact:  model.Contact{Phone: "129"},
		Source:   model.Source{RevisedAt: "2026-09-11"},
	}
}

func TestValidateFacilityAcceptsValid(t *testing.T) {
	if errs := ValidateFacility(validFacility()); len(errs) > 0 {
		t.Fatalf("유효한 시설이 걸렸다: %v", errs)
	}
}

// ★ 공공/민간을 모르면 화면이 어느 구역에 둘지 정할 수 없다
func TestValidateFacilityRequiresSector(t *testing.T) {
	for _, s := range []model.Sector{"", "NGO"} {
		f := validFacility()
		f.Sector = s
		errs := ValidateFacility(f)
		if len(errs) == 0 || !strings.Contains(strings.Join(errs, " "), "sector") {
			t.Errorf("sector %q 가 통과했다: %v", s, errs)
		}
	}
}
