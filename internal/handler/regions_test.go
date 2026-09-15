package handler

import (
	"reflect"
	"testing"

	"github.com/DUE-NAVIGATION/be/internal/model"
)

func fac(scope model.CoverageScope, sido, sigungu string) model.Facility {
	return model.Facility{Coverage: model.Coverage{Scope: scope, Sido: sido, Sigungu: sigungu}}
}

// ★ 입력 화면이 고를 수 있는 값은 데이터에 실제로 있는 관할뿐이어야 한다.
func TestBuildRegions(t *testing.T) {
	got := buildRegions([]model.Facility{
		fac(model.CoverageSigungu, "서울특별시", "관악구"),
		fac(model.CoverageSigungu, "서울특별시", "강남구"),
		fac(model.CoverageSigungu, "서울특별시", "관악구"), // 중복은 한 번만
		fac(model.CoverageSigungu, "경기도", "수원시"),
		fac(model.CoverageSido, "세종특별자치시", ""), // 시군구 없는 시도도 목록에 남는다
		fac(model.CoverageNationwide, "", ""),  // 상담 전화는 지역을 늘리지 않는다
	})

	want := []Region{
		{Sido: "경기도", Sigungu: []string{"수원시"}},
		{Sido: "서울특별시", Sigungu: []string{"강남구", "관악구"}},
		{Sido: "세종특별자치시", Sigungu: []string{}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildRegions =\n %#v\nwant\n %#v", got, want)
	}
}

func TestBuildRegionsEmpty(t *testing.T) {
	if got := buildRegions(nil); len(got) != 0 {
		t.Errorf("시설이 없으면 빈 목록이어야 한다: %#v", got)
	}
}
