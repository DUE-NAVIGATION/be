package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

// 에러 코드. 프론트가 문자열 비교로 분기할 수 있게 고정한다.
// ★ 새 코드를 추가하면 api/README.md 도 함께 고칠 것.
const (
	CodeInvalidJSON    = "INVALID_JSON"
	CodeInvalidRequest = "INVALID_REQUEST"
	CodeBodyTooLarge   = "BODY_TOO_LARGE"
	CodeNotFound       = "NOT_FOUND"
	CodeNotImplemented = "NOT_IMPLEMENTED"
	CodeInternal       = "INTERNAL"
	// AI 를 쓸 수 없다(키 없음) / 호출했지만 실패했다.
	// 둘 다 프론트는 "수동 입력" 으로 폴백한다
	CodeAIUnavailable = "AI_UNAVAILABLE"
	CodeAIFailed      = "AI_FAILED"
)

// ErrorBody 는 모든 실패 응답의 형태다.
//
//	{ "error": { "code": "INVALID_JSON", "message": "..." } }
//
// 형식을 하나로 고정하는 이유: 프론트가 실패 처리를 한 곳에 모을 수 있다.
type ErrorBody struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	// 판정 결과는 매번 새로 계산한다. 중간 캐시에 남으면 안 된다 (설계 원칙 2).
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(body); err != nil {
		// 여기까지 왔으면 헤더는 이미 나갔다. 응답을 고칠 수 없으니 기록만 남긴다.
		// ★ 사용자 입력이 아니라 오류 자체만 남긴다.
		slog.Error("응답을 쓰지 못했습니다", "err", err)
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, ErrorBody{Error: ErrorDetail{Code: code, Message: message}})
}

// decodeJSON 은 요청 본문을 읽어 v 에 담는다. 실패하면 응답까지 끝내고 false 를 돌려준다.
//
// ★ 실패해도 본문 내용을 로그나 응답에 싣지 않는다. 민감정보가 들어 있다.
// 필드 이름은 프론트 디버깅에 꼭 필요하므로 길이를 잘라서만 노출한다.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields() // 오타 난 필드를 조용히 무시하지 않는다

	if err := dec.Decode(v); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, CodeBodyTooLarge,
				"요청이 너무 큽니다 (최대 1MB)")
			return false
		}
		writeError(w, http.StatusBadRequest, CodeInvalidJSON, decodeMessage(err))
		return false
	}

	// 본문이 두 개 이상 들어오면 뒤엣것이 조용히 무시된다. 그것도 잘못된 요청이다.
	if dec.More() {
		writeError(w, http.StatusBadRequest, CodeInvalidJSON,
			"요청 본문에 JSON 이 두 개 이상 있습니다")
		return false
	}
	return true
}

// decodeMessage 는 JSON 오류를 프론트가 고칠 수 있는 말로 바꾼다.
func decodeMessage(err error) string {
	var syn *json.SyntaxError
	if errors.As(err, &syn) {
		return fmt.Sprintf("JSON 형식이 잘못됐습니다 (%d번째 글자 근처)", syn.Offset)
	}

	var typ *json.UnmarshalTypeError
	if errors.As(err, &typ) {
		field := typ.Field
		if field == "" {
			field = "(최상위)"
		}
		return fmt.Sprintf("%s 항목의 값 형식이 맞지 않습니다 (%s 자리에 %s 이(가) 왔습니다)",
			clip(field), typ.Type.String(), typ.Value)
	}

	if errors.Is(err, io.EOF) {
		return "요청 본문이 비어 있습니다"
	}

	// DisallowUnknownFields 는 문자열 오류만 준다. 필드 이름만 뽑아 쓴다.
	if msg := err.Error(); strings.HasPrefix(msg, "json: unknown field ") {
		return fmt.Sprintf("모르는 항목이 있습니다: %s",
			clip(strings.TrimPrefix(msg, "json: unknown field ")))
	}

	return "JSON 을 읽지 못했습니다"
}

// clip 은 사용자가 보낸 문자열을 응답에 실을 때 길이를 자른다.
func clip(s string) string {
	const max = 80
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
