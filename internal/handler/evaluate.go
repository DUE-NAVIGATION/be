package handler

import (
	"net/http"
	"sort"

	"github.com/DUE-NAVIGATION/be/internal/model"
	"github.com/DUE-NAVIGATION/be/internal/rules"
)

// EvaluateRequest 는 POST /api/evaluate 의 요청이다.
//
//	{ "context": { "householdSize": 2, "age": 34, ... } }
type EvaluateRequest struct {
	Context model.UserContext `json:"context"`
}

// EvaluateResponse 는 결과 화면이 그릴 모든 것이다.
type EvaluateResponse struct {
	Results []model.MatchResult `json:"results"`
	Summary model.Summary       `json:"summary"`
	// 계산된 중위소득 대비 비율(%). 계산할 수 없었으면 null
	IncomePct *float64 `json:"incomePct"`
	// 기준중위소득 표의 기준연도. 화면에 "2026년 기준" 으로 표시한다
	MedianIncomeYear int `json:"medianIncomeYear"`
	// ★ 모든 성공 응답에 붙는다. 지우지 말 것
	Disclaimer string `json:"disclaimer"`
}

// evaluate 는 사용자 상황을 받아 제도별 판정 결과를 돌려준다.
//
// 이 핸들러가 이 서버의 본체다. 순서가 중요하다.
//
//  1. 소득 비율 계산 → 2) 제도별 판정 → 3) 금액 산정
//     → 4) 중복수급 정리 → 5) 요약
//
// 3번이 4번보다 먼저다. 금액을 모르면 "어느 조합이 최대인가" 를 정할 수 없다.
func (a *API) evaluate(w http.ResponseWriter, r *http.Request) {
	var req EvaluateRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	programs := a.Programs.Programs()
	if len(programs) == 0 {
		// 제도가 없으면 판정 자체가 성립하지 않는다. 빈 결과로 속이지 않는다.
		writeError(w, http.StatusServiceUnavailable, CodeInternal,
			"제도 데이터가 아직 준비되지 않았습니다")
		return
	}

	// 1) 소득 비율을 채운다. 계산할 수 없으면 채우지 않는다 —
	//    그러면 해당 조건이 UNKNOWN 이 되어 "확인 필요" 로 간다. 0% 로 단정하지 않는다.
	ctx := a.Income.WithIncomePct(req.Context)

	// 2) 제도마다 판정한다
	results := make([]model.MatchResult, 0, len(programs))
	for _, p := range programs {
		results = append(results, rules.Evaluate(p, ctx))
	}

	// 3) 연간 예상 수령액을 채운다
	results = rules.WithEstimates(results)

	// 4) 동시에 받을 수 없는 제도를 정리한다
	resolved := rules.ResolveConflicts(results, a.Programs.Relations())

	// 5) 화면 상단 요약
	summary := rules.Summarize(resolved.Results)
	summary.ExcludedByConflict = resolved.Excluded

	sortResults(resolved.Results)

	writeJSON(w, http.StatusOK, EvaluateResponse{
		Results:          resolved.Results,
		Summary:          summary,
		IncomePct:        ctx.HouseholdIncomePct,
		MedianIncomeYear: a.Income.Table.Year,
		Disclaimer:       model.Disclaimer,
	})
}

// sortResults 는 화면에 보일 순서로 정렬한다.
//
//	해당 → 확인필요 → 미해당, 같은 상태 안에서는 금액이 큰 것부터.
//
// 프론트가 다시 정렬할 필요가 없게 하고, 같은 입력이면 항상 같은 순서가 나오게 한다.
func sortResults(rs []model.MatchResult) {
	sort.SliceStable(rs, func(i, j int) bool {
		si, sj := statusOrder(rs[i].Status), statusOrder(rs[j].Status)
		if si != sj {
			return si < sj
		}
		if rs[i].EstimatedAmount != rs[j].EstimatedAmount {
			return rs[i].EstimatedAmount > rs[j].EstimatedAmount
		}
		// 금액까지 같으면 id 로 고정한다. 순서가 실행마다 달라지면 안 된다
		return rs[i].Program.ID < rs[j].Program.ID
	})
}

func statusOrder(s model.MatchStatus) int {
	switch s {
	case model.MatchEligible:
		return 0
	case model.MatchNeedsInfo:
		return 1
	}
	return 2
}
