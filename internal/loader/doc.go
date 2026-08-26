// Package loader 는 제도 JSON 로더다 (Phase 3).
//
// 시작 시 data/programs/*.json 을 전부 읽어 메모리에 보관한다 (읽기 전용).
// 로드에 실패한 파일은 서버를 죽이지 말고 경고 로그 후 건너뛴다 —
// 제도 하나가 깨져도 나머지가 동작해야 한다.
//
// 제도 데이터 작성이 이 프로젝트 최대 병목이다. Phase 1 과 병렬로 진행한다.
package loader
