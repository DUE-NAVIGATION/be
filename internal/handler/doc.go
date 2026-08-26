// Package handler 는 HTTP 핸들러다 (Phase 5).
//
// 규칙:
//   - 요청·응답 필드는 camelCase (프론트가 TypeScript)
//   - 에러는 { error: { code, message } } 로 통일
//   - 모든 성공 응답에 model.Disclaimer 포함
//   - ★ 요청 로그에 사용자 입력 원문을 남기지 않는다. 경로·상태코드·소요시간만
package handler
