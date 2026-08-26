// Package ai 는 Claude API 게이트웨이다 (Phase 4 / 6).
//
// AI 의 역할은 딱 둘이다.
//  1. 자연어 → 구조화된 UserContext
//  2. 판정 결과 → 사람 말 설명
//
// ★ "이 사람이 이 제도에 해당하나요?" 를 LLM 에 묻는 코드를 작성하지 마라.
// ★ 시크릿 필터(sanitize.go)를 먼저 만들고 나서 LLM 을 붙인다. 순서를 바꾸지 마라.
// ★ API 키는 환경변수로만 받는다. 코드에 넣지 않는다.
package ai
