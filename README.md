# DUE API

복지 사각지대 내비게이터 백엔드. **Go**, REST API.

> due = "마땅히 지급되어야 할". **이건 원래 당신 것입니다.**

상황을 받아 **연락할 수 있는 기관**과 **받을 수 있는 제도**를 규칙 엔진으로 판정한다.

- 프론트(Next.js): https://github.com/DUE-NAVIGATION/FE
- 프로젝트 설명: https://github.com/DUE-NAVIGATION/.github

## 시작하기

```bash
go run ./cmd/server       # http://localhost:8080
curl localhost:8080/healthz
# {"status":"ok","programCount":3,"facilityCount":2258,"aiEnabled":false,"storesUserData":false,...}
```

**API 키가 없어도 판정은 그대로 동작한다.** 자연어 입력(`/api/extract`)과 쉬운 설명(`/api/explain`)만
503 을 돌려주고, 프론트는 직접 입력 폼으로 진행한다. 키 없이 AI 응답까지 보여주려면
`DEMO_MODE=true` (미리 뽑아둔 응답 사용, [DEMO.md](DEMO.md)).

| 명령어 | 설명 |
| ------ | ---- |
| `go run ./cmd/server` | 서버 |
| `go test ./...` | 테스트 |
| `go run ./cmd/validate` | 제도·시설 JSON 검증 |
| `go run ./cmd/importfacilities -in 원본.csv -out data/facilities/x.json -type CHILD_CENTER` | 공공데이터 CSV → 시설 JSON |
| `go run ./cmd/seed` | 제도 JSON → SQLite |

환경변수는 [.env.example](.env.example) 참고. 프론트 오리진을 `CORS_ALLOWED_ORIGINS` 에 넣어야 브라우저가 응답을 읽는다.

## 폴더 구조

```
cmd/
  server/             진입점 (라우팅 · CORS · 그레이스풀 셧다운)
  validate/           제도·시설 JSON 검증
  importfacilities/   공공데이터 CSV → 시설 JSON
  seed/               제도 JSON → SQLite
internal/
  rules/              ★ 규칙 엔진 (순수 함수) — 제도·시설 판정을 함께 한다
  income/             소득·기준중위소득 계산
  model/              공용 타입 (프론트 types/index.ts 의 원본)
  loader/             JSON 로더 + 검증
  store/              제도 SQLite 저장소
  handler/            HTTP 핸들러
  ai/                 Claude 호출 · 스키마 검증 · 민감정보 필터
data/
  programs/*.json     제도 정의 ★ 원본
  facilities/*.json   시설 — 상담전화 5 · 서울 정신건강복지센터 25 · 전국 지역아동센터 1,747 · 전국 사회복지관 481
  median-income.json  기준중위소득 표
```

## API

| 메서드 | 경로 | 요청 | 응답 |
| ------ | ---- | ---- | ---- |
| POST | `/api/evaluate` | `{ context }` | `{ facilities[], facilitySummary, results[], summary, incomePct, disclaimer }` |
| POST | `/api/extract` | `{ text }` | `{ extracted, confidence, followUpQuestions, sanitized }` |
| POST | `/api/explain` | `{ results, summary }` | `{ explanation }` |
| GET | `/api/programs` | — | `{ programs[] }` |
| GET | `/api/facilities` | — | `{ facilities[], problems[] }` |
| GET | `/api/regions` | — | `{ regions[{ sido, sigungu[] }] }` — 입력 화면의 시·군·구 선택 목록 |
| GET | `/healthz` | — | `{ status, programCount, facilityCount, aiEnabled, storesUserData }` |

자세한 요청·응답은 [API.md](API.md).

## 설계 원칙

1. **판정은 AI 가 하지 않는다.** 자격 판정·금액 계산은 `internal/rules` 의 결정론적 규칙 엔진.
   AI 는 ①자연어 → 입력값 ②판정 결과 → 쉬운 말, 두 가지만 한다
2. **사용자에 관한 것은 저장하지 않는다.** 사용자 테이블이 없다 (`TestSchemaHasNoUserTables` 가 막는다).
   SQLite 에는 제도 데이터만 들어가고, 로그에도 입력 원문을 남기지 않는다
3. **단정하지 않는다.** 입력하지 않은 값의 조건은 `FAIL` 이 아니라 `UNKNOWN`,
   결과는 `NEEDS_INFO`. 조건별 근거가 응답에 항상 들어간다
4. **없는 연락처를 지어내지 않는다.** 연락 수단이 없는 시설은 검증에서 떨어진다

### 시설 판정

- 관할(`coverage`)이 첫 번째 조건이다. 거주지를 모르면 `UNKNOWN` — 숨기지 않고 "알려주시면 확인" 으로 남긴다
- `sector` 로 공공(PUBLIC) / 민간(PRIVATE)을 나눈다. **설치 주체 기준**이며 모르면 검증에서 막힌다
- 정렬: 이용 가능 → 확인 필요 → 관할 밖, 같은 상태에서는 24시간 운영 · 전화 가능한 곳이 먼저

## 배포

**Render** 에 올린다 — `render.yaml`(Blueprint) · `Dockerfile`(distroless, 빌드 중 데이터 검증). 절차는 [DEPLOY.md](DEPLOY.md).
