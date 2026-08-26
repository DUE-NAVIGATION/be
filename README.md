# DUE API

복지 사각지대 내비게이터 백엔드. **Go 1.22+**, DB 없음, REST API만 제공한다.

> due = "마땅히 지급되어야 할". **이건 원래 당신 것입니다.**

프론트(Next.js)는 별도 저장소: https://github.com/DUE-NAVIGATION/FE
프로젝트 전체 설명·설계: https://github.com/DUE-NAVIGATION/.github

## 시작하기

```bash
cp .env.example .env      # 값을 채운다. ★ API 키를 코드에 넣지 않는다
go run ./cmd/server       # http://localhost:8080
```

```bash
curl http://localhost:8080/healthz
# {"service":"due-api","status":"ok","storesUserData":false}
```

| 명령어                   | 설명                          |
| ------------------------ | ----------------------------- |
| `go run ./cmd/server`    | 개발 서버                     |
| `go test ./...`          | 테스트                        |
| `go vet ./...`           | 정적 검사                     |
| `gofmt -l .`             | 포맷 확인 (출력이 없어야 정상) |
| `go run ./cmd/validate`  | 제도 JSON 검증 (Phase 3)      |

## 폴더 구조

```
cmd/server/        진입점
cmd/validate/      제도 JSON 검증 CLI (Phase 3)
internal/
  rules/           규칙 엔진 (순수 함수) ★ 핵심
  income/          소득 계산 엔진
  ai/              Claude 호출, 스키마 검증, 시크릿 필터
  handler/         HTTP 핸들러
  model/           공용 타입
  loader/          제도 JSON 로더
data/
  programs/*.json  제도 정의
```

## API 계약

프론트와 합의된 인터페이스. 필드는 전부 **camelCase**(프론트가 TypeScript).

| 메서드 | 경로             | 요청            | 응답                                             |
| ------ | ---------------- | --------------- | ------------------------------------------------ |
| POST   | `/api/extract`   | `{ text }`      | `{ extracted, confidence, followUpQuestions }`    |
| POST   | `/api/evaluate`  | `{ context }`   | `{ results[], summary }`                          |
| POST   | `/api/explain`   | `{ results }`   | `{ explanation }`                                 |
| POST   | `/api/document`  | `{ imageBase64 }` | `{ summary, todos[], deadline, ... }`           |
| GET    | `/api/programs`  | —               | `{ programs[] }`                                  |
| GET    | `/healthz`       | —               | `200`                                             |

- 에러는 `{ error: { code, message } }` 로 통일
- 모든 성공 응답에 `disclaimer` 포함 — "실제 수급 여부는 관할 기관의 심사로 결정됩니다"

> Phase 0 기준으로 `/healthz` 만 구현돼 있다. 나머지는 Phase 5에서 붙는다.
> 요청·응답 예시는 그때 이 문서에 채운다.

## 설계 원칙 (요약)

1. **판정은 AI가 하지 않는다** — 자격 판정·금액 계산은 `internal/rules`의 결정론적 규칙 엔진.
   AI는 ①자연어 → 구조화 ②판정 결과 → 사람 말 설명, 딱 둘만 한다
2. **아무것도 저장하지 않는다** — DB 없음. 제도 JSON만 시작 시 메모리로 로드(읽기 전용).
   로그에 사용자 입력 원문을 남기지 않는다
3. **단정하지 않는다** — 입력값이 없는 조건은 `FAIL`이 아니라 `UNKNOWN`,
   결과는 `NEEDS_INFO`. 조건별 근거를 응답에 항상 포함한다

### "모르는 값"과 "0"

`UserContext`의 스칼라 필드가 전부 포인터인 이유다.

- 소득 **0원**(무소득) → 대부분의 저소득 제도에 해당
- 소득 **미입력** → 판정 불가. 탈락시키면 안 된다

사람들은 자기 소득을 정확히 모른다. 이 구분이 서비스의 신뢰를 만든다.
`internal/model/context_test.go`가 이 동작을 고정한다.

전체 규칙은 [CLAUDE.md](CLAUDE.md) 참조.

## 현재 상태

**Phase 0 완료** — 폴더 구조, 공용 타입, `/healthz`.

다음은 **Phase 1 규칙 엔진** (최우선). 제도 JSON 작성은 Phase 1과 **병렬로** 시작해야 한다 —
코딩보다 오래 걸리는 최대 병목이다.
