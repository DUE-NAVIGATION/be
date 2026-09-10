# DUE API — 복지 사각지대 내비게이터 백엔드

## 프로젝트 정체

사용자가 자기 상황을 자연어로 말하면 **이용할 수 있는 시설로 연결해 주고**,
받을 수 있는 복지제도를 함께 알려주는 서비스의 백엔드.

**2026-09-10 방향 전환** (사회복지학과 교수 자문) — 무게중심이 "얼마 받을 수 있나"
에서 **"어디로 가면 되나"** 로 옮겨졌다. 제도를 알아도 어디에 물어야 할지 모르면
결국 도달하지 못한다. 시설(`internal/model/facility.go`)이 결과 화면의 주인공이고,
제도는 그 아래 부가 정보로 붙는다. 판정 엔진은 둘이 공유한다.

Go로 작성하며 REST API만 제공한다. 프론트(Next.js)는 `FE` 저장소에 있고
**2026-09-08 부터 같은 사람이 맡는다** — 계약(`internal/model` ↔ `FE/types`)을
한쪽만 고치면 런타임에 깨지므로 반드시 같이 고친다.

이름의 뜻: due = "마땅히 지급되어야 할". "이건 원래 당신 것입니다."

## ★ 최상위 설계 원칙 (절대 어기지 말 것)

### 1. 판정은 AI가 하지 않는다

- 자격 판정과 금액 계산은 결정론적 규칙 엔진이 한다
- AI의 역할은 딱 둘: ①자연어 → 구조화된 입력값 ②판정 결과 → 사람 말 설명
- LLM에게 "이 사람이 이 제도에 해당하나요?"를 묻는 코드를 절대 작성하지 마라
- 이유: 환각, 비일관성, 근거 부재. 심사에서 가장 먼저 공격당하는 지점이다

### 2. 사용자에 관한 것은 아무것도 저장하지 않는다

- ★ 사용자 입력을 디스크에 영속화하지 않는다. 이 원칙은 그대로다
- 제도 데이터는 시작 시 메모리로 로드한다 (읽기 전용)
- **2026-09-08 변경** — 과제 요건으로 제도 데이터용 DB(SQLite)를 추가했다.
  담기는 것은 "제도가 무엇인가" 뿐이고 "누가 무엇을 물었는가" 는 담기지 않는다.
  사용자 테이블이 생기면 테스트가 막는다 (`TestSchemaHasNoUserTables`)
- 진실의 원본은 여전히 `data/programs/*.json` 이다. DB 는 `cmd/seed` 가 옮겨 담은
  사본이고, 방향은 JSON → DB 한 쪽뿐이다. 제도 데이터는 git 에서 리뷰되어야 한다 —
  누가 언제 어떤 근거로 금액을 바꿨는지가 남지 않으면 심사에서 답할 말이 없다
- 로그에 사용자 입력 원문을 남기지 않는다
- LLM에 보낼 때도 최소 항목만. 주민번호·계좌번호는 전송 전에 제거
- 이것이 발표의 핵심 장면이다. 편의를 위해 저장 기능을 추가하지 마라

### 3. 단정하지 않는다

- 결과는 ELIGIBLE / INELIGIBLE / NEEDS_INFO 세 가지
- 입력값이 없는 조건은 FAIL이 아니라 UNKNOWN으로 처리한다
- 조건별 판정 근거를 응답에 항상 포함한다 (설명 가능성)

## 기술 스택

- Go 1.22+
- 라우팅: 표준 net/http (Go 1.22 라우팅 패턴)
- CORS: 표준 라이브러리로 직접 작성 (`cmd/server/main.go` 의 `withCORS`).
  오리진 허용 + 프리플라이트가 전부라 rs/cors 는 과하다
- ★ 외부 의존성을 최소로 유지한다. 현재 **1개**: `modernc.org/sqlite`
  (순수 Go, cgo 불필요 → distroless 이미지에 그대로 들어간다).
  다른 것을 늘리기 전에 표준 라이브러리로 되는지 먼저 확인할 것
- ★ 그래서 **발표 전에 `go mod download` 를 미리 돌려 모듈 캐시를 채워둔다.**
  현장 와이파이에서 의존성을 받다 막히면 데모가 끝난다. Docker 이미지를
  미리 빌드해 두면 이 위험은 사라진다
- DB 는 제도 데이터 전용이다. ORM 은 쓰지 않는다 — `database/sql` 로 직접 쓴다.
  테이블이 3개뿐이라 ORM 이 벌어주는 게 없다
- 마이그레이션 도구를 쓰지 않는다. `schema.sql` 을 `CREATE TABLE IF NOT EXISTS`
  로 매번 적용한다. DB 가 생성물이라 스키마가 바뀌면 다시 만들면 된다
- GraphQL 쓰지 않는다. 엔드포인트가 5개뿐이라 과설계다
- AI: Claude API를 net/http로 직접 호출 (SDK 불필요)

## 폴더 구조

    be/
      cmd/server/main.go        진입점 (라우팅 · CORS · 그레이스풀 셧다운)
      cmd/validate/main.go      제도·시설 JSON 검증 CLI
      cmd/seed/main.go          제도 JSON → SQLite 씨딩 CLI
      internal/
        rules/                  규칙 엔진 (순수 함수) ★ 핵심
        income/                 소득 계산 엔진
        ai/                     Claude 호출, 스키마 검증, 시크릿 필터
        handler/                HTTP 핸들러
        model/                  공용 타입
        loader/                 제도·시설 JSON 로더
        store/                  제도 SQLite 저장소 (loader 와 같은 인터페이스)
      data/
        programs/*.json         제도 정의 ★ 원본
        facilities/*.json       시설 정의 (파일 하나에 배열로 여러 건)
        median-income.json      기준중위소득 표
        due.db                  제도 DB (생성물. 커밋하지 않는다)
      go.mod

## API 계약 (프론트와 합의된 인터페이스)

    POST /api/extract     { text }         → { extracted, confidence, followUpQuestions }
    POST /api/evaluate    { context }      → { results[], summary,
                                              facilities[], facilitySummary }
    POST /api/explain     { results }      → { explanation }
    POST /api/document    { imageBase64 }  → { summary, todos[], deadline, ... }
    GET  /api/programs                     → { programs[] }
    GET  /api/facilities                   → { facilities[], problems[] }
    GET  /healthz                          → { status, service, storesUserData }

- ★ 헬스체크만 `/api` 접두어가 없다. 프론트 `lib/api.ts` 의 `getHealth` 와 맞춰야 한다.
  한쪽을 바꾸면 첫 화면이 "백엔드에 연결할 수 없습니다"로 뜬다
- 요청·응답 필드는 camelCase로 통일 (프론트가 TypeScript)
- enum 값은 UPPER_SNAKE로 통일 (`ELIGIBLE` `MONTHLY_RENT` `HIGH`).
  제도 JSON의 연산자(`between` `lte` …)만 예외적으로 소문자다 — 사람이 손으로 쓰기 때문
- `internal/model/*`는 프론트 `types/index.ts`의 원본이다. 한쪽만 고치면 런타임에 깨진다
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
- 사용자 데이터를 담는 테이블 추가 (제도 데이터용 DB 만 허용)
- ORM·마이그레이션 도구 도입
- GraphQL
- 인증·세션·사용자 관리
- 제도·시설 데이터를 Go 코드에 하드코딩 (반드시 JSON)
- ★ 시설 전화번호·주소를 지어내기. 확인되지 않으면 비워 둔다 —
  틀린 번호로 전화하게 만드는 것은 안내하지 않는 것보다 나쁘다
- API 키를 코드에 삽입 (환경변수만)

## 진행 현황

- [x] Phase 0 — 셋업 + 타입 정의
- [x] Phase 1 — 규칙 엔진 `internal/rules` ★ 최우선
- [x] Phase 2 — 소득 계산 + 중복수급
- [x] Phase 3 — 제도 로더 + 검증 CLI (제도 데이터는 3건. 팀이 계속 추가)
- [x] Phase 4 — AI 계층 (시크릿 필터 → 구조화 → 설명)
- [x] Phase 5 — HTTP 계층 (extract/explain/document 는 Phase 4 에서)
- [x] Phase 8 — 시설 연결 (모델·관할 판정·로더·검증·API). 2026-09-10 방향 전환
- [ ] Phase 8-2 — 시설 화면 (전화 걸기·준비물·문의 스크립트) ★ 다음
- [ ] Phase 8-3 — 공공데이터 → 시설 JSON 임포터, 실제 지역 데이터
- [ ] Phase 6 — 문서 번역 (여유 시)
- [x] Phase 7 — 데모 안정화 (캐시·Docker·체크리스트). 배포·리허설은 발표 전
