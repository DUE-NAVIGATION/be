# DUE API — 복지 사각지대 내비게이터 백엔드

## 프로젝트 정체

사용자가 자기 상황을 자연어로 말하면 받을 수 있는 복지제도를 찾아주는 서비스의
백엔드. Go로 작성한다. 프론트(Next.js)는 다른 팀원이 담당하므로,
이 프로젝트는 REST API만 제공한다.

이름의 뜻: due = "마땅히 지급되어야 할". "이건 원래 당신 것입니다."

## ★ 최상위 설계 원칙 (절대 어기지 말 것)

### 1. 판정은 AI가 하지 않는다

- 자격 판정과 금액 계산은 결정론적 규칙 엔진이 한다
- AI의 역할은 딱 둘: ①자연어 → 구조화된 입력값 ②판정 결과 → 사람 말 설명
- LLM에게 "이 사람이 이 제도에 해당하나요?"를 묻는 코드를 절대 작성하지 마라
- 이유: 환각, 비일관성, 근거 부재. 심사에서 가장 먼저 공격당하는 지점이다

### 2. 아무것도 저장하지 않는다

- DB를 쓰지 않는다. 사용자 입력을 디스크에 영속화하지 않는다
- 제도 데이터만 시작 시 JSON에서 메모리로 로드한다 (읽기 전용)
- 로그에 사용자 입력 원문을 남기지 않는다
- LLM에 보낼 때도 최소 항목만. 주민번호·계좌번호는 전송 전에 제거
- 이것이 발표의 핵심 장면이다. 편의를 위해 저장 기능을 추가하지 마라

### 3. 단정하지 않는다

- 결과는 ELIGIBLE / INELIGIBLE / NEEDS_INFO 세 가지
- 입력값이 없는 조건은 FAIL이 아니라 UNKNOWN으로 처리한다
- 조건별 판정 근거를 응답에 항상 포함한다 (설명 가능성)

## 기술 스택

- Go 1.22+
- 라우팅: 표준 net/http (Go 1.22 라우팅 패턴) 또는 chi
- CORS: rs/cors
- DB 없음. ORM 없음. 저장하지 않으므로 필요 없다
- GraphQL 쓰지 않는다. 엔드포인트가 5개뿐이라 과설계다
- AI: Claude API를 net/http로 직접 호출 (SDK 불필요)

## 폴더 구조

    api/
      cmd/server/main.go        진입점
      cmd/validate/main.go      제도 JSON 검증 CLI
      internal/
        rules/                  규칙 엔진 (순수 함수) ★ 핵심
        income/                 소득 계산 엔진
        ai/                     Claude 호출, 스키마 검증, 시크릿 필터
        handler/                HTTP 핸들러
        model/                  공용 타입
        loader/                 제도 JSON 로더
      data/
        programs/*.json         제도 정의
        median-income.json      기준중위소득 표
      go.mod

## API 계약 (프론트와 합의된 인터페이스)

    POST /api/extract     { text }         → { extracted, confidence, followUpQuestions }
    POST /api/evaluate    { context }      → { results[], summary }
    POST /api/explain     { results }      → { explanation }
    POST /api/document    { imageBase64 }  → { summary, todos[], deadline, ... }
    GET  /api/programs                     → { programs[] }
    GET  /healthz                          → 200

- 요청·응답 필드는 camelCase로 통일 (프론트가 TypeScript)
- 에러는 { error: { code, message } } 형태로 일관되게
- 모든 성공 응답에 disclaimer 포함:
  "실제 수급 여부는 관할 기관의 심사로 결정됩니다"

## 작업 방식

1. 코드 변경 시 파일 전체 출력. 부분 스니펫 금지
2. 규칙 엔진과 계산 엔진은 테스트를 먼저 쓴다 (테이블 주도 테스트)
3. LLM 프롬프트도 코드 리뷰 대상 — 초안을 보여주고 진행
4. 해커톤이다. 스코프를 넓히지 마라

## 하지 말 것

- LLM에게 자격 판정을 맡기는 코드
- DB·ORM·마이그레이션 도입
- GraphQL
- 인증·세션·사용자 관리
- 제도 데이터를 Go 코드에 하드코딩 (반드시 JSON)
- API 키를 코드에 삽입 (환경변수만)

## 진행 현황

- [x] Phase 0 — 셋업 + 타입 정의
- [ ] Phase 1 — 규칙 엔진 `internal/rules` ★ 최우선
- [ ] Phase 2 — 소득 계산 + 중복수급
- [ ] Phase 3 — 제도 로더 + 검증 CLI (제도 JSON 작성은 팀원이 병렬로)
- [ ] Phase 4 — AI 계층 (시크릿 필터 → 구조화 → 설명)
- [ ] Phase 5 — HTTP 계층
- [ ] Phase 6 — 문서 번역 (여유 시)
- [ ] Phase 7 — 데모 안정화 ★ 반드시
