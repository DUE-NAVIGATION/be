package handler

import (
	"net/http"

	"github.com/DUE-NAVIGATION/be/internal/model"
	"github.com/DUE-NAVIGATION/be/internal/rules"
)

// FacilitiesResponse 는 GET /api/facilities 의 응답이다.
//
// 판정 없이 "지금 서버가 어떤 시설을 들고 있는지" 를 확인하는 용도다.
// 시설 데이터를 작성하는 사람이 자기 파일이 읽혔는지 보는 데 쓴다.
type FacilitiesResponse struct {
	Facilities []model.Facility `json:"facilities"`
	Count      int              `json:"count"`
	// 읽다가 건너뛴 것. ★ 조용히 빠지면 아무도 모른다
	Problems   []problemView `json:"problems,omitempty"`
	Disclaimer string        `json:"disclaimer"`
}

func (a *API) facilities(w http.ResponseWriter, _ *http.Request) {
	if a.Facilities == nil {
		writeJSON(w, http.StatusOK, FacilitiesResponse{
			Facilities: []model.Facility{},
			Disclaimer: model.Disclaimer,
		})
		return
	}

	problems := make([]problemView, 0)
	for _, p := range a.Facilities.Problems() {
		problems = append(problems, problemView{File: p.File, Reason: p.Reason})
	}

	resp := FacilitiesResponse{
		Facilities: a.Facilities.Facilities(),
		Count:      a.Facilities.Count(),
		Disclaimer: model.Disclaimer,
	}
	if len(problems) > 0 {
		resp.Problems = problems
	}

	writeJSON(w, http.StatusOK, resp)
}

// evaluateFacilities 는 사용자 상황으로 시설을 판정한다.
//
// ★ /api/evaluate 안에서 제도 판정과 함께 호출된다. 엔드포인트를 나누지 않는
// 이유: 화면이 한 번만 호출하면 되고, 무엇보다 두 결과가 같은 입력에서
// 나왔다는 것이 보장된다. 따로 부르면 그 사이에 사용자가 값을 고칠 수 있다.
func (a *API) evaluateFacilities(ctx model.UserContext) ([]model.FacilityMatch, model.FacilitySummary) {
	if a.Facilities == nil {
		return []model.FacilityMatch{}, model.FacilitySummary{}
	}

	all := a.Facilities.Facilities()
	matches := make([]model.FacilityMatch, 0, len(all))
	for _, f := range all {
		matches = append(matches, rules.EvaluateFacility(f, ctx))
	}

	summary := rules.SummarizeFacilities(matches)
	rules.SortFacilities(matches)

	return matches, summary
}
