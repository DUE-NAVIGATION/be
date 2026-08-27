# DUE API 문서

프론트가 이 문서만 보고 붙일 수 있게 쓴다. 새 엔드포인트나 에러 코드를 추가하면 **여기도 같이 고친다.**

기본 주소 — 개발 `http://localhost:8080`

## 공통 규칙

- 요청·응답 필드는 전부 **camelCase** (프론트가 TypeScript)
- 요청 본문 상한 **1MB**
- 모든 **성공** 응답에 `disclaimer` 가 들어 있다 — 화면에 반드시 노출할 것
- 모든 **실패** 응답은 같은 모양이다

```json
{ "error": { "code": "INVALID_JSON", "message": "요청 본문이 비어 있습니다" } }
```

| code | HTTP | 언제 |
| --- | --- | --- |
| `INVALID_JSON` | 400 | JSON 형식 오류, 빈 본문, 모르는 항목, 타입 불일치 |
| `INVALID_REQUEST` | 400 | 값이 규칙에 맞지 않음 |
| `BODY_TOO_LARGE` | 413 | 본문 1MB 초과 |
| `NOT_FOUND` | 404 | 없는 경로 |
| `NOT_IMPLEMENTED` | 501 | 아직 안 만든 기능 (Phase 6 문서 번역) |
| `AI_UNAVAILABLE` | 503 | AI 키가 없어 쓸 수 없음 → 수동 입력으로 폴백 |
| `AI_FAILED` | 502 | AI 호출 실패 → 수동 입력으로 폴백 |
| `INTERNAL` | 500 · 503 | 서버 오류, 제도 데이터 없음 |

> **모르는 항목은 조용히 무시되지 않는다.** `incomeMontly` 처럼 오타를 보내면 400 과 함께
> 어느 항목이 문제인지 알려준다. 오타가 조용히 무시되면 판정이 틀린 채로 화면에 나간다.

---

## GET /healthz

서버가 살아 있는지 확인한다.

```json
{
  "status": "ok",
  "service": "due-api",
  "storesUserData": false,
  "programCount": 3,
  "medianIncomeYear": 2026,
  "aiEnabled": true
}
```

`aiEnabled` 가 `false` 면 `/api/extract` · `/api/explain` 이 503 을 돌려준다.
대화형 입력 대신 **수동 입력 폼**을 띄워야 한다.

---

## POST /api/evaluate ★ 핵심

사용자 상황을 받아 제도별 판정 결과를 돌려준다.

### 요청

```json
{
  "context": {
    "householdSize": 2,
    "age": 33,
    "incomeMonthly": 800000,
    "housingType": "MONTHLY_RENT",
    "monthlyRent": 400000,
    "employmentStatus": "LOST_JOB",
    "isSingleParent": true,
    "childrenAges": [7]
  }
}
```

**`context` 의 모든 항목은 생략 가능하다.** 모르는 값을 억지로 채우지 말고 **아예 빼서** 보낼 것.

> ★ `0` 과 "모름" 은 완전히 다르다.
> `"incomeMonthly": 0` 은 **무소득**이라는 정보이고, 항목을 빼면 **모른다**는 뜻이다.
> 무소득인 사람을 "모름" 으로 보내면 받을 수 있는 제도가 전부 "확인 필요" 로 밀린다.

| 항목 | 형식 | 비고 |
| --- | --- | --- |
| `householdSize` | number | 가구원 수 (본인 포함) |
| `age` | number | 만 나이 |
| `incomeMonthly` | number | 월 소득 (원) |
| `assets` | number | 재산 총액 (원) |
| `housingType` | `MONTHLY_RENT` `JEONSE` `OWNED` `PUBLIC_LEASE` `FREE_USE` `OTHER` | |
| `deposit` / `monthlyRent` | number | 보증금 / 월세 (원) |
| `employmentStatus` | `EMPLOYED` `SELF_EMPLOYED` `LOST_JOB` `UNEMPLOYED` `STUDENT` `RETIRED` `ON_LEAVE` `OTHER` | |
| `isSingleParent` | boolean | |
| `childrenAges` | number[] | `[]` = 자녀 없음, 생략 = 모름 |
| `hasDisability` | boolean | |
| `disabilityLevel` | `SEVERE` `MILD` | |
| `isPregnant` | boolean | |
| `basicLivelihoodType` | `LIVELIHOOD` `MEDICAL` `HOUSING` `EDUCATION` `NONE` | `NONE` = 수급자 아님(정보), 생략 = 모름 |
| `receivingPrograms` | string[] | 현재 받는 제도 |
| `region` | string | 시도 |

`householdIncomePct` 는 **보내지 않는다.** 서버가 계산해서 채운다.

### 응답

```json
{
  "results": [
    {
      "program": {
        "id": "single-parent-child-care",
        "name": "한부모가족 아동양육비",
        "category": "CHILDCARE",
        "summary": "저소득 한부모가족에게 ...",
        "benefit": { "type": "MONTHLY", "amount": 230000, "months": 12 },
        "apply": {
          "channel": ["BOKJIRO", "COMMUNITY_CENTER"],
          "documents": ["한부모가족증명서", "가족관계증명서"],
          "period": "연중 상시 신청"
        },
        "source": {
          "url": "https://...",
          "revisedAt": "2026-01-01",
          "agency": "여성가족부"
        }
      },
      "status": "ELIGIBLE",
      "conditions": [
        {
          "condition": { "field": "isSingleParent", "op": "eq", "value": true, "label": "한부모가족" },
          "status": "PASS",
          "actual": true,
          "reason": "true 와(과) 일치합니다"
        },
        {
          "condition": { "field": "householdIncomePct", "op": "lte", "value": 65, "label": "기준중위소득 65% 이하" },
          "status": "PASS",
          "actual": 19.05,
          "reason": "19.1 은(는) 65 이하입니다"
        }
      ],
      "estimatedAmount": 2760000,
      "missingFields": []
    }
  ],
  "summary": {
    "eligibleCount": 1,
    "needsInfoCount": 2,
    "ineligibleCount": 0,
    "totalAnnualAmount": 2760000,
    "excludedByConflict": []
  },
  "incomePct": 19.05,
  "medianIncomeYear": 2026,
  "disclaimer": "실제 수급 여부는 관할 기관의 심사로 결정됩니다"
}
```

### 화면을 그릴 때

| 필드 | 화면에서 |
| --- | --- |
| `summary.eligibleCount` · `totalAnnualAmount` · `needsInfoCount` | 상단 **"확인된 것 N건 · 연 ○○○원 · 추가 확인 N건"** |
| `status` | `ELIGIBLE` 해당 / `NEEDS_INFO` **확인필요** / `INELIGIBLE` 미해당 |
| `conditions[].status` | `PASS` 충족 / `FAIL` 미충족 / `UNKNOWN` **확인 필요** |
| `conditions[].condition.label` | 조건 이름. **화면에 그대로 쓰라고 만든 문구다** |
| `conditions[].actual` | "입력: 29세" 의 값. 모르면 `null` |
| `missingFields` | 더 물어봐야 할 항목. `NEEDS_INFO` 일 때만 채워진다 |
| `incomePct` | "중위소득 약 19%". 계산 불가면 `null` |
| `estimatedAmount` | **연간** 예상액(원). `RATE`·`IN_KIND` 급여는 `0` |

- `results` 는 **해당 → 확인필요 → 미해당** 순, 같은 상태에서는 금액이 큰 것부터 이미 정렬되어 있다
- **`INELIGIBLE` 도 함께 내려간다.** 접어두더라도 "왜 안 되는지" 를 볼 수 있어야 한다
- `summary.totalAnnualAmount` 에는 **`ELIGIBLE` 만** 더해진다. 확인필요 제도의 금액은 빠진다

### 알아둘 것 — `none`(배제) 조건의 표시

배제 조건(예: "주거급여를 받고 있지 않을 것")은 **`PASS` 가 "배제 조건에 걸렸다"** 는 뜻이라
사용자 화면 기준으로는 의미가 뒤집혀 있다. **지금은 그대로 내려가므로 프론트에서 그대로 그리면 어색하다.**
표시 규칙을 정리해 서버에서 뒤집을 예정이다 (아래 "알려진 과제" 참조).

---

## GET /api/programs

서버가 읽고 있는 제도 목록. 프론트 확인용·디버깅용.

```json
{
  "programs": [ /* 위 program 과 같은 모양 */ ],
  "count": 3,
  "problems": [
    { "file": "energy-voucher.json", "reason": "source.revisedAt 이 없습니다" }
  ],
  "disclaimer": "실제 수급 여부는 관할 기관의 심사로 결정됩니다"
}
```

`problems` 는 **읽다가 건너뛴 파일**이다. 제도 하나가 깨져도 서버는 죽지 않고 나머지로 동작한다.
비어 있으면 이 항목 자체가 없다.

---

## POST /api/extract

자연어를 판정 입력값으로 옮긴다. **판정하지 않는다** — 결과를 `/api/evaluate` 에 그대로 넣으면 판정이 된다.

### 요청

```json
{ "text": "혼자 애 키우는데 일이 끊겼어요. 아이는 7살이고 월세 살아요." }
```

최대 2000자. 빈 문자열이면 400.

### 응답

```json
{
  "extracted": {
    "householdSize": 2, "isSingleParent": true, "childrenAges": [7],
    "employmentStatus": "LOST_JOB", "housingType": "MONTHLY_RENT"
  },
  "confidence": { "householdSize": "MEDIUM", "isSingleParent": "HIGH" },
  "followUpQuestions": ["월 소득이 어느 정도인가요?"],
  "sanitized": { "RESIDENT_ID": 1, "PHONE": 1 },
  "disclaimer": "실제 수급 여부는 관할 기관의 심사로 결정됩니다"
}
```

| 필드 | 화면에서 |
| --- | --- |
| `extracted` | 뽑아낸 값을 카드로 보여주고 **수정 가능**하게. AI 가 잘못 뽑을 수 있다 |
| `confidence` | `LOW` 인 항목은 확인을 유도. `HIGH`/`MEDIUM`/`LOW` |
| `followUpQuestions` | 되묻기. **최대 3개**. 서버가 잘라서 보낸다 |
| `sanitized` | 전송 전에 가린 민감정보의 **종류·건수**. ★ 값은 들어 있지 않다 |

- `extracted` 에는 **알아낸 항목만** 들어 있다. 나머지는 아예 없다 (모른다는 뜻)
- `householdIncomePct` 는 **절대 오지 않는다.** 계산 엔진이 채우는 값이라 AI 가 채워도 서버가 지운다
- `sanitized` 가 있으면 화면에 **"주민등록번호는 전송하지 않았습니다"** 처럼 알려주면 좋다

## POST /api/explain

이미 나온 판정 결과를 사람 말로 푼다. **판정을 다시 하지 않는다.**

### 요청

`/api/evaluate` 의 응답에서 `results` 와 `summary` 를 그대로 넣는다.

```json
{ "results": [ /* evaluate 의 results */ ], "summary": { /* evaluate 의 summary */ } }
```

### 응답

```json
{
  "explanation": "입력하신 내용으로는 한부모가족 아동양육비를 받으실 수 있을 것으로 보입니다. ...",
  "disclaimer": "실제 수급 여부는 관할 기관의 심사로 결정됩니다"
}
```

> **사용자의 상황은 이 단계에서 전송되지 않는다.** 설명에 필요한 것은 "무엇이 어떻게 판정됐는가" 뿐이라, 소득·가족관계는 AI 로 나가지 않는다.

## AI 를 쓸 수 없을 때

두 엔드포인트 모두 실패할 수 있다. **프론트는 반드시 수동 입력 폼으로 폴백해야 한다.**

| code | HTTP | 뜻 | 프론트가 할 일 |
| --- | --- | --- | --- |
| `AI_UNAVAILABLE` | 503 | 서버에 API 키가 없다 | 대화형 입력을 아예 숨기고 수동 폼만 |
| `AI_FAILED` | 502 | 호출했지만 실패(시간 초과·형식 오류) | "직접 입력해 주세요" 안내 후 폼 |

`GET /healthz` 의 **`aiEnabled`** 로 시작 시점에 미리 판단할 수 있다.

## 아직 없는 것 (Phase 6)

| 엔드포인트 | 요청 | 응답 (예정) |
| --- | --- | --- |
| `POST /api/document` | `{ "imageBase64": "..." }` | `{ summary, whatIsIt, todos, deadline, ... }` |

호출하면 **501 + `NOT_IMPLEMENTED`** 가 온다.

## 빠른 확인 (curl)

```bash
curl http://localhost:8080/healthz
```

```bash
curl -X POST http://localhost:8080/api/evaluate -H "Content-Type: application/json" -d '{"context":{"householdSize":2,"age":33,"incomeMonthly":800000,"housingType":"MONTHLY_RENT","isSingleParent":true,"childrenAges":[7]}}'
```

---

## 알려진 과제

| 항목 | 내용 |
| --- | --- |
| `none` 조건 표시 | 배제 조건의 `PASS`/`FAIL` 의미가 화면 기준과 반대다. 서버에서 뒤집을지 결정 필요 |
| 소득인정액 | 재산의 소득환산·근로소득공제가 아직 반영되지 않았다. `incomePct` 가 실제보다 낮게 나올 수 있다 |
| 제도 수 | 현재 3건. 30~50건이 목표 |
| 중복수급 관계 | 엔진은 완성. 관계 데이터(`data/relations.json`)가 아직 비어 있다 |

앞의 두 가지는 **2026-09-09 전문가 자문** 후 확정한다.
