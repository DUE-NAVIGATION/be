// Package rules 는 규칙 엔진이다. 이 프로젝트의 심장 (Phase 1).
//
// ★ 자격 판정은 여기서만 한다. AI 에게 절대 맡기지 않는다.
//
// 이 패키지의 함수는 순수 함수여야 한다 — 부수효과 없음, 전역 상태 없음,
// I/O 없음, panic 없음. 잘못된 제도 JSON 이 서버를 죽이면 안 된다.
//
// 들어올 것:
//   - Evaluate(p model.Program, ctx model.UserContext) model.MatchResult
//   - 연산자 구현 (between, lte, gte, eq, in, contains, exists)
//   - ResolveConflicts — 중복수급 배제 (Phase 2)
//   - Estimate — 연간 예상 수령액 (Phase 2)
package rules
