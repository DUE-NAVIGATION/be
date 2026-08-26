/**
 * 규칙 엔진 — 이 프로젝트의 심장 (Phase 1).
 *
 * <p>★ 자격 판정은 여기서만 한다. AI 에게 절대 맡기지 않는다.
 * 이 패키지의 코드는 부수효과 없는 순수 함수여야 하고, 스프링 빈에 의존하지 않는다.
 * 테스트를 먼저 쓴다.
 *
 * <p>들어올 것: RuleEngine.evaluate(Program, UserContext) -> MatchResult,
 * 연산자 구현, ConflictResolver(중복수급), BenefitEstimator(예상 수령액).
 */
package com.due.rules;
