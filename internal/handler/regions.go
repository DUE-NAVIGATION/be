package handler

import (
	"net/http"
	"sort"

	"github.com/DUE-NAVIGATION/be/internal/model"
)

// RegionsResponse 는 GET /api/regions 의 응답이다 — 입력 화면의 시·군·구 선택 목록.
//
// ★ 시·군·구를 자유 입력으로 받으면 "수원" · "장안구" 처럼 데이터의 "수원시" 와
// 어긋나, 갈 수 있는 곳이 전부 "관할 밖" 으로 빠진다. 서버가 실제로 들고 있는
// 시설의 관할에서 목록을 만들어 주면, 고른 값이 반드시 데이터와 맞는다.
type RegionsResponse struct {
	Regions []Region `json:"regions"`
}

// Region 은 시도 하나와 그 안의 시군구 목록이다.
// 시군구가 비어 있으면 시도 전체가 관할이다 (예: 세종특별자치시).
type Region struct {
	Sido    string   `json:"sido"`
	Sigungu []string `json:"sigungu"`
}

func (a *API) regions(w http.ResponseWriter, _ *http.Request) {
	var fs []model.Facility
	if a.Facilities != nil {
		fs = a.Facilities.Facilities()
	}
	writeJSON(w, http.StatusOK, RegionsResponse{Regions: buildRegions(fs)})
}

// buildRegions 는 시설의 관할에서 시도 → 시군구 목록을 만든다.
//
// 전국 대상(NATIONWIDE) 시설은 지역을 늘리지 않는다 — 상담 전화는 어디서나 된다.
// 순서는 이름순이다. 입력 화면이 시도 순서를 따로 가지고 있다.
func buildRegions(fs []model.Facility) []Region {
	bySido := map[string]map[string]bool{}
	for _, f := range fs {
		c := f.Coverage
		if c.Sido == "" || c.Scope == model.CoverageNationwide {
			continue
		}
		if bySido[c.Sido] == nil {
			bySido[c.Sido] = map[string]bool{}
		}
		if c.Scope == model.CoverageSigungu && c.Sigungu != "" {
			bySido[c.Sido][c.Sigungu] = true
		}
	}

	out := make([]Region, 0, len(bySido))
	for sido, set := range bySido {
		list := make([]string, 0, len(set))
		for sg := range set {
			list = append(list, sg)
		}
		sort.Strings(list)
		out = append(out, Region{Sido: sido, Sigungu: list})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Sido < out[j].Sido })
	return out
}
