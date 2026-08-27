package handler

import (
	"net/http"

	"github.com/DUE-NAVIGATION/be/internal/model"
)

// ProgramsResponse 는 GET /api/programs 의 응답이다.
//
// 프론트가 제도 목록을 확인하거나, 제도 데이터를 작성하는 팀원이
// "지금 서버가 뭘 읽고 있는지" 를 확인하는 용도다.
type ProgramsResponse struct {
	Programs []model.Program `json:"programs"`
	Count    int             `json:"count"`
	// 읽다가 건너뛴 파일이 있으면 여기 담긴다.
	// ★ 조용히 빠지면 아무도 모른다. 눈에 보이게 한다
	Problems   []problemView `json:"problems,omitempty"`
	Disclaimer string        `json:"disclaimer"`
}

type problemView struct {
	File   string `json:"file"`
	Reason string `json:"reason"`
}

func (a *API) programs(w http.ResponseWriter, _ *http.Request) {
	programs := a.Programs.Programs()

	problems := make([]problemView, 0)
	for _, p := range a.Programs.Problems() {
		problems = append(problems, problemView{File: p.File, Reason: p.Reason})
	}

	resp := ProgramsResponse{
		Programs:   programs,
		Count:      len(programs),
		Disclaimer: model.Disclaimer,
	}
	if len(problems) > 0 {
		resp.Problems = problems
	}

	writeJSON(w, http.StatusOK, resp)
}
