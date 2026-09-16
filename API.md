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
| `TOO_MANY_REQUESTS` | 429 | AI 엔드포인트 호출이 1분 상한을 넘음 → 수동 입력으로 폴백 |
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
  "facilityCount": 2258,
  "medianIncomeYear": 2026,
  "aiEnabled": true
}
```

`facilityCount` 가 `0` 이면 시설 데이터를 못 읽은 것이다. 첫 화면에 "기관 N곳" 으로 쓴다.

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
| `region` | string | 시도 (예: `"서울특별시"`) |
| `district` | string | 시군구 (예: `"관악구"`). ★ 시설 판정의 1순위 조건 |
| `crisisSignals` | (`SELF_HARM` \| `VIOLENCE`)[] | 지금 안전이 걱정되는 신호 |

`householdIncomePct` 는 **보내지 않는다.** 서버가 계산해서 채운다.

> ★ `region` · `district` 는 **`GET /api/regions` 가 준 값 그대로** 보낼 것.
> "수원" 이나 "장안구" 처럼 자유 입력하면 데이터의 `"수원시"` 와 어긋나 갈 수 있는 곳이
> 전부 "관할 밖" 으로 빠진다.
>
> ★ `crisisSignals` 는 **판정을 바꾸지 않는다.** 그 신호에 응답하는 기관(`facility.crisis`)을
> 맨 위로 올리는 데만 쓴다 (`facilities[].urgent`). 가장 민감한 입력이라 저장·로그에 남지 않고,
> 기관에 보내는 문의 문구에도 넣지 않는다.

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
  "facilities": [ /* 아래 "시설" 참조 */ ],
  "facilitySummary": {
    "availableCount": 73,
    "needsInfoCount": 0,
    "outOfScope": 2185,
    "reachableNow": 73
  },
  "incomePct": 19.05,
  "medianIncomeYear": 2026,
  "disclaimer": "실제 수급 여부는 관할 기관의 심사로 결정됩니다"
}
```

### 시설 (`facilities`) ★ 결과 화면의 주인공

제도와 같은 규칙 엔진으로 판정한다. 다른 점은 **관할 지역이 조건 맨 앞에 붙는다**는 것이다.

```json
{
  "facility": {
    "id": "hotline-109",
    "name": "자살예방 상담전화",
    "type": "HOTLINE",
    "sector": "PUBLIC",
    "operator": "",
    "crisis": ["SELF_HARM"],
    "summary": "힘든 마음을 24시간 언제든 이야기할 수 있습니다.",
    "coverage": { "scope": "NATIONWIDE" },
    "location": { "sido": "", "sigungu": "", "roadAddress": "" },
    "contact": { "phone": "109", "hours": "24시간 연중무휴", "always": true },
    "services": ["자살예방 상담"],
    "fee": "무료 (국번없이 109)",
    "source": { "url": "...", "revisedAt": "2026-09-10" }
  },
  "status": "ELIGIBLE",
  "conditions": [
    {
      "condition": { "field": "region", "op": "eq", "label": "전국 누구나 이용 가능" },
      "group": "coverage",
      "status": "PASS",
      "reason": "지역 제한이 없습니다"
    }
  ],
  "missingFields": [],
  "urgent": true
}
```

| 필드 | 뜻 |
| --- | --- |
| `sector` | `PUBLIC` 공공 / `PRIVATE` 민간. **설치 주체 기준** — 구청이 세우고 법인에 위탁했으면 공공. 화면은 이 값으로 구역을 나눈다 |
| `operator` | 실제 운영 법인 (위탁이면 수탁 법인). 확인 안 되면 빈 값 |
| `crisis` | 이 기관이 응답하는 위기 신호 |
| `coverage.scope` | `NATIONWIDE` / `SIDO` / `SIGUNGU`. 거주지를 모르면 관할이 `UNKNOWN` → `NEEDS_INFO` |
| `contact.always` | 24시간 운영. 밤에 급한 사람에게 먼저 보여야 한다 |
| `urgent` | 사용자의 `crisisSignals` 에 응답하는 곳. 화면 맨 위에 따로 둔다 |

- 정렬은 **위기 응답 → 이용 가능 → 확인 필요 → 관할 밖**, 같은 상태에서는 24시간 → 전화 가능 → id 순
- ★ **관할 밖(`INELIGIBLE`)은 최대 20건만 담긴다.** 전국 데이터에서 관할 밖은 2천 곳이 넘고,
  전부 보내면 응답이 4.5MB 가 된다(실측 → 193KB). 전체 건수는 `facilitySummary.outOfScope` 에 있다.
  **"관할 밖 N곳" 은 목록 길이가 아니라 이 값으로 표시할 것**
- `sector` 가 없는 시설을 공공으로 취급하지 말 것. 확인되지 않았다는 뜻이다

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
| `facilitySummary.availableCount` · `reachableNow` | 상단 **"지금 연락하실 수 있는 곳 N곳 · N곳은 바로 전화"** |
| `facilitySummary.outOfScope` | "관할 지역이 아닌 곳 N곳" (접어 둔다) |
| `facilities[].urgent` | 맨 위 **"지금 바로 이야기할 수 있는 곳"** 상자 |

- `results` 는 **해당 → 확인필요 → 미해당** 순, 같은 상태에서는 금액이 큰 것부터 이미 정렬되어 있다
- **`INELIGIBLE` 도 함께 내려간다.** 접어두더라도 "왜 안 되는지" 를 볼 수 있어야 한다
- `summary.totalAnnualAmount` 에는 **`ELIGIBLE` 만** 더해진다. 확인필요 제도의 금액은 빠진다

### 알아둘 것 — `none`(배제) 조건의 표시

조건마다 **`group`** 이 붙는다 — `all` · `any` · `none` · `coverage`.

배제 조건(`group: "none"`, 예: "재직 중이 아닐 것")은 엔진 관점에서 **`PASS` 가 "배제에 걸렸다"**,
즉 이용자에게는 **탈락**이라는 뜻이다. 의미가 뒤집혀 있다.

- 서버는 **엔진 관점의 원자료를 그대로** 내려보낸다 (`status` 를 뒤집지 않는다)
- **화면이 `group === "none"` 일 때 `PASS`/`FAIL` 을 뒤집어 그린다.** `UNKNOWN` 은 그대로 둔다
- `reason` 문구는 이미 이용자 관점으로 쓰여 있다 ("여기에 해당하지 않습니다")

프론트 구현은 `FE/lib/format.ts` 의 `displayStatus` 다. 실제로 이 표시가 반대로 나가
"통과인데 붉은 FAIL" 로 보이던 버그가 있었고, 테스트로 고정해 두었다.

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

## GET /api/facilities

서버가 읽고 있는 **시설 전체**. 판정하지 않는다. 데이터 작성자 확인용·디버깅용.

```json
{
  "facilities": [ /* 위 facility 와 같은 모양 */ ],
  "count": 2258,
  "problems": [{ "file": "seoul-x.json", "reason": "연락할 방법이 하나도 없습니다" }],
  "disclaimer": "실제 수급 여부는 관할 기관의 심사로 결정됩니다"
}
```

★ 2천 건이 전부 내려온다(약 2MB). **화면에서 쓰지 말 것** — 판정 결과는 `/api/evaluate` 에서 온다.

---

## GET /api/regions

입력 화면의 **시·군·구 선택 목록**. 서버가 들고 있는 시설의 관할에서 만든다.

```json
{
  "regions": [
    { "sido": "경기도", "sigungu": ["가평군", "고양시", "과천시"] },
    { "sido": "세종특별자치시", "sigungu": [] }
  ]
}
```

- `sigungu` 가 **빈 배열이면 시도 전체가 관할**이다 (세종특별자치시). 시군구를 묻지 않아도 된다
- 전국 대상 상담 전화는 지역을 늘리지 않는다 — 목록에 나타나지 않는다
- 정렬은 이름순. 시도 순서는 화면이 자기 목록대로 정한다
- 실패해도 화면이 멈추면 안 된다. 목록을 못 받으면 **시군구를 직접 입력**하게 두는 편이 낫다

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
| `AI_UNAVAILABLE` | 503 | 서버에 API 키가 없다 **또는 하루 호출 상한을 넘었다** | 대화형 입력을 아예 숨기고 수동 폼만 |
| `TOO_MANY_REQUESTS` | 429 | 1분에 너무 많이 불렀다 | "잠시 후 다시" 안내 후 수동 폼 |
| `AI_FAILED` | 502 | 호출했지만 실패(시간 초과·형식 오류) | "직접 입력해 주세요" 안내 후 폼 |

`GET /healthz` 의 **`aiEnabled`** 로 시작 시점에 미리 판단할 수 있다.

### 요금을 막는 두 겹의 상한

AI 엔드포인트는 호출마다 돈이 든다. 공개 주소이므로 두 겹으로 막는다.

| 상한 | 환경변수 | 기본값 | 넘으면 |
| --- | --- | --- | --- |
| 요청자(IP)당 1분 | `AI_RATE_PER_MIN` | 6 | **429** `TOO_MANY_REQUESTS` |
| 서버 전체 하루 | `AI_DAILY_LIMIT` | 300 | 데모 캐시로 답한다. 캐시가 없으면 **503** `AI_UNAVAILABLE` |

- 0 으로 두면 제한하지 않는다 (로컬 개발)
- **판정(`/api/evaluate`)에는 상한이 없다.** 돈이 들지 않고, 심사 중에 여러 번 눌러보는 것이 정상이다
- IP 는 세는 데만 쓰고 **로그에 남기지 않는다** (설계 원칙 2)

### 프롬프트 캐싱

시스템 문구와 도구 스키마는 매 호출마다 똑같다(고정 약 1,900 토큰). 마지막 시스템 블록에
캐시 표시를 달아 **도구 스키마까지 함께** 캐시한다. 두 번째 호출부터 그 부분의 입력 비용이
1/10 로 떨어진다 (처음 한 번만 1.25배).

- 캐시 최소 길이는 모델마다 다르다. **Sonnet 5 는 512 토큰이라 걸리고, Haiku 4.5 는
  4,096 토큰이라 우리 프롬프트로는 걸리지 않는다** — 그래서 더 좋은 모델을 같은 비용으로 쓴다
- 먹고 있는지 확인하는 방법은 하나뿐이다. 서버 로그의 `AI 사용량` 줄에서 `캐시읽음` 을 본다.
  계속 0 이면 캐싱이 깨진 것이다 (오류는 나지 않는다)
- 프롬프트를 고치면 캐시가 무효가 된다. 정상이다 — 다음 호출에서 다시 쓴다

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
| 제도 수 | 현재 3건. 30~50건이 목표 — **가장 큰 병목** |
| 소득인정액 | 재산의 소득환산·근로소득공제가 아직 반영되지 않았다. `incomePct` 가 실제보다 낮게 나올 수 있다 (`median-income.json` 의 `propertyConversion` 이 `null`) |
| 중복수급 관계 | 엔진은 완성. 관계 데이터(`data/relations.json`)가 아직 없다 |
| 기관 종류 | 상담 전화 · 정신건강복지센터(서울) · 지역아동센터 · 사회복지관. 노인·장애인 시설은 사회복지시설정보서비스 API 로 확장 예정 |
| 운영시간·이용료 | 기관마다 달라 대부분 비어 있다. 화면이 "전화로 확인" 으로 안내한다 — 지어내지 않는다 |
| 요청 횟수 제한 | 없다. 본문 1MB 상한만 있다. AI 키를 넣고 공개하면 `/api/extract` 남용을 막을 장치가 필요하다 |
| 문서 번역 | `POST /api/document` 는 501 (Phase 6) |

`none` 조건 표시는 **해결됐다** — 조건마다 `group` 이 붙고 화면이 뒤집어 그린다 (위 참조).
