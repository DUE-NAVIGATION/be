package handler

import (
	"log/slog"
	"net/http"
	"time"
)

// 요청 본문 상한. 판정 입력은 아무리 커도 몇 KB 다.
// 문서 이미지(Phase 6)는 따로 상한을 둔다.
const maxBodyBytes = 1 << 20 // 1MB

// limitBody 는 본문 크기를 제한한다.
// 상한을 넘으면 읽는 쪽(decodeJSON)에서 MaxBytesError 로 잡힌다.
func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		}
		next.ServeHTTP(w, r)
	})
}

// statusRecorder 는 로그에 쓸 상태코드와 응답 크기를 기억한다.
// http.ResponseWriter 는 나중에 상태코드를 되물을 방법이 없어서 필요하다.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK // WriteHeader 없이 바로 쓰면 200 이다
	}
	n, err := s.ResponseWriter.Write(b)
	s.bytes += n
	return n, err
}

// logRequests 는 요청을 한 줄로 남긴다.
//
// ★ 남기는 것: 메서드, 경로, 상태코드, 소요시간, 응답 크기.
// ★ 남기지 않는 것: 요청 본문, 쿼리 문자열, 헤더, IP.
//
// 사용자 입력에는 소득·가족관계·질병 정보가 들어 있다. 로그에 남기는 순간
// "저장하지 않는다" 는 약속이 깨진다. 디버깅이 조금 불편한 것을 감수한다.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}

		next.ServeHTTP(rec, r)

		if rec.status == 0 {
			rec.status = http.StatusOK
		}
		slog.Info("요청",
			"method", r.Method,
			"path", r.URL.Path, // ★ r.URL.String() 이 아니다. 쿼리를 뺀다
			"status", rec.status,
			"ms", time.Since(start).Milliseconds(),
			"bytes", rec.bytes,
		)
	})
}

// recoverPanic 은 핸들러가 터져도 서버를 살려둔다.
//
// 규칙 엔진은 panic 하지 않도록 만들었지만, 그 약속이 깨지는 날에도
// 프로세스가 죽으면 안 된다. 발표 중 500 은 넘어갈 수 있어도 서버 종료는 못 넘어간다.
func recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				// ★ 요청 본문이 아니라 어디서 터졌는지만 남긴다
				slog.Error("핸들러에서 panic 발생",
					"method", r.Method, "path", r.URL.Path, "panic", v)
				writeError(w, http.StatusInternalServerError, CodeInternal,
					"서버에서 처리하지 못했습니다")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
