# 제도 JSON 작성 가이드

여기에 파일 하나를 추가하면 제도 하나가 늘어납니다. **코드를 고칠 필요는 없습니다.**

작성 후 반드시 검사하세요.

```bash
cd api
go run ./cmd/validate
```

`✔ 전부 통과했습니다.` 가 나오면 끝입니다. 문제가 있으면 **어느 파일의 무슨 문제인지 한국어로 알려줍니다.**

---

## 1. 시작하기 — 가장 빠른 방법

`youth-monthly-rent.json` 을 복사해서 이름을 바꾸고 내용을 채우세요.

- **파일명**: `제도이름-영문.json` (예: `energy-voucher.json`)
- **파일 하나에 제도 하나**
- `_` 로 시작하는 파일은 제도가 아닌 것으로 보고 무시합니다

---

## 2. 전체 모양

```json
{
  "id": "energy-voucher",
  "name": "에너지바우처",
  "category": "ENERGY",
  "summary": "여름·겨울 전기·가스 요금을 지원합니다.",

  "eligibility": {
    "all":  [ { "field": "...", "op": "...", "value": ..., "label": "..." } ],
    "any":  [ ... ],
    "none": [ ... ]
  },

  "benefit": { "type": "MONTHLY", "amount": 200000, "months": 12 },

  "apply": {
    "channel": ["BOKJIRO", "COMMUNITY_CENTER"],
    "documents": ["임대차계약서", "가족관계증명서"],
    "period": "연중 상시 신청"
  },

  "source": {
    "url": "https://공식-안내-페이지-주소",
    "revisedAt": "2026-01-01",
    "agency": "보건복지부"
  }
}
```

| 항목 | 설명 |
| --- | --- |
| `id` | 영문 소문자와 `-`. **다른 제도와 겹치면 안 됩니다** |
| `name` | 화면에 보이는 공식 제도명 |
| `category` | 아래 분류 중 하나 |
| `summary` | 한 줄 설명 (선택) |
| `eligibility` | 자격요건. **이 문서의 핵심** |
| `benefit` | 얼마를 받는가 |
| `apply` | 어디서 어떻게 신청하는가 |
| `source` | **출처와 개정일. 심사에서 반드시 물어봅니다** |

**분류(`category`)** — `HOUSING`(주거) `INCOME`(소득·생계) `EMPLOYMENT`(고용) `CHILDCARE`(양육) `MEDICAL`(의료) `EDUCATION`(교육) `ELDERLY`(노인) `DISABILITY`(장애) `ENERGY`(에너지·공과금) `OTHER`

---

## 3. 자격요건 — `eligibility`

요건을 **조건 하나씩 쪼개서** 적습니다. 쪼갤수록 사용자에게 "왜 해당/미해당인지"를 항목별로 보여줄 수 있습니다.

| 묶음 | 뜻 |
| --- | --- |
| `all` | **전부** 충족해야 해당 (그리고) |
| `any` | **하나라도** 충족하면 해당 (또는) |
| `none` | **하나라도** 해당하면 **탈락** (제외 조건) |

셋 중 최소 하나는 있어야 합니다. 대부분은 `all` 만 씁니다.

### 조건 하나의 모양

```json
{
  "field": "age",
  "op": "between",
  "value": [19, 34],
  "label": "만 19~34세",
  "note": "작성자 메모. 판정에 쓰이지 않습니다"
}
```

- `label` 은 **화면에 그대로 보이는 문구**입니다. 사용자가 읽을 말로 적어주세요
- `note` 는 애매한 요건이나 확인이 필요한 사항을 남기는 칸입니다. **억지로 조건을 만들지 말고 여기에 적어주세요**

### `field` — 무엇을 볼 것인가

| field | 뜻 | 값의 형태 |
| --- | --- | --- |
| `age` | 만 나이 | 숫자 |
| `householdSize` | 가구원 수 | 숫자 |
| `incomeMonthly` | 월 소득 (원) | 숫자 |
| `householdIncomePct` | **기준중위소득 대비 비율(%)** | 숫자 |
| `assets` | 재산 총액 (원) | 숫자 |
| `housingType` | 주거 형태 | `MONTHLY_RENT` `JEONSE` `OWNED` `PUBLIC_LEASE` `FREE_USE` `OTHER` |
| `deposit` | 보증금 (원) | 숫자 |
| `monthlyRent` | 월세 (원) | 숫자 |
| `employmentStatus` | 취업 상태 | `EMPLOYED` `SELF_EMPLOYED` `LOST_JOB` `UNEMPLOYED` `STUDENT` `RETIRED` `ON_LEAVE` `OTHER` |
| `isSingleParent` | 한부모 여부 | `true` / `false` |
| `childrenAges` | 자녀 나이 목록 | 숫자 배열 |
| `hasDisability` | 장애 여부 | `true` / `false` |
| `disabilityLevel` | 장애 정도 | `SEVERE` `MILD` |
| `isPregnant` | 임신·출산 여부 | `true` / `false` |
| `basicLivelihoodType` | 기초생활보장 급여 구분 | `LIVELIHOOD` `MEDICAL` `HOUSING` `EDUCATION` `NONE` |
| `receivingPrograms` | 현재 받고 있는 제도 목록 | 문자열 배열 |
| `region` | 거주 지역 (시도) | 문자열 |

> **"중위소득 60% 이하" 는 `householdIncomePct` 를 쓰세요.**
> `incomeMonthly` 에 직접 금액을 적으면 가구원수를 반영하지 못합니다.
> 비율은 시스템이 기준중위소득 표를 보고 계산해 줍니다.

### `op` — 어떻게 비교할 것인가

| op | 뜻 | value 예시 |
| --- | --- | --- |
| `between` | 범위 안 (**양 끝 포함**) | `[19, 34]` — 숫자 **2개** |
| `lte` | 이하 (≤) | `60` |
| `gte` | 이상 (≥) | `19` |
| `eq` | 같다 | `true`, `"SEVERE"` |
| `in` | 목록에 있다 | `["MONTHLY_RENT", "JEONSE"]` |
| `contains` | 배열이 이 값을 품고 있다 | `"HOUSING_BENEFIT"` |
| `exists` | 값이 입력되어 있기만 하면 통과 | (`value` 불필요) |

`in` 과 `contains` 를 헷갈리기 쉽습니다.

- `in` — **내 값 하나**가 여러 후보 중 하나인가 → `housingType` 이 월세 **또는** 전세인가
- `contains` — **내 목록**이 특정 값을 품고 있는가 → `receivingPrograms` 안에 주거급여가 있는가

---

## 4. 급여 — `benefit`

| `type` | 뜻 | 필요한 값 |
| --- | --- | --- |
| `MONTHLY` | 매달 지급 | `amount`(월 지급액), `months`(지급 개월) |
| `ONCE` | 1회성 | `amount` |
| `YEARLY` | 연 정액 | `amount` |
| `RATE` | 요금 감면율 | `ratePct` (0 초과 100 이하) |
| `IN_KIND` | 현물·서비스 | `note` (무엇을 받는지 설명) |

- 금액은 **원 단위 숫자**입니다. `200000` (O) / `"20만원"` (X)
- `months` 를 안 적으면 계속 지급되는 것으로 보고 12개월로 계산합니다
- 지급 기간이 1년을 넘어도 **연간 예상액은 첫 1년치만** 셉니다
- `RATE` 와 `IN_KIND` 는 금액으로 환산할 수 없어 **총액 합계에 들어가지 않습니다** (정상입니다)

---

## 5. 출처 — `source` ★ 반드시 채우세요

```json
"source": {
  "url": "https://www.bokjiro.go.kr/...",
  "revisedAt": "2026-01-01",
  "agency": "보건복지부",
  "note": "확인이 필요한 사항을 여기에 남겨주세요"
}
```

- `url` — **공식 안내 페이지**. 블로그·뉴스 말고 정부 사이트
- `revisedAt` — **`YYYY-MM-DD` 형식**. 개정일 또는 확인한 날짜
- `note` — 대조가 필요한 사항, 반영하지 못한 요건 등

> 심사위원이 **반드시** 묻습니다. "이 정보 어디서 가져왔나요?", "언제 기준인가요?"

---

## 6. 자주 하는 실수

| 실수 | 검사기가 알려주는 말 |
| --- | --- |
| 필드 이름 오타 (`incomeMontly`) | `field "incomeMontly" 는 없는 항목입니다. 혹시 incomeMonthly 인가요?` |
| `between` 에 숫자 하나만 | `between 의 value 는 원소가 2개여야 합니다 (현재 1개)` |
| `revisedAt` 빠뜨림 | `source.revisedAt 이 없습니다 (개정일. 심사에서 물어봅니다)` |
| `id` 가 다른 제도와 겹침 | `id "..." 가 ....json 와 중복입니다` |
| 없는 연산자 (`morethan`) | `op "morethan" 를 모릅니다. 쓸 수 있는 값: ...` |
| 쉼표 빠짐·괄호 안 닫힘 | `JSON 형식이 잘못됐습니다: ...` |

그 밖에 자주 나오는 판단 실수:

- **애매한 요건을 억지로 조건으로 만들지 마세요.** 표현할 수 없으면 `note` 에 적고 넘어갑니다.
  잘못된 조건은 빠진 조건보다 나쁩니다 — 받을 수 있는 사람을 탈락시킵니다
- **금액에 `,` 나 `원` 을 넣지 마세요.** `2400000` 만 씁니다
- **`amount` 는 월 지급액입니다.** 총액이 아닙니다

---

## 7. 지금 표현할 수 없는 요건

DSL 에 아직 없는 것들입니다. 만나면 `note` 에 남겨주세요.

- **부모(원가구) 소득** — 청년 제도에 자주 나오지만 입력 항목이 없습니다
- **자녀 나이 전체에 대한 조건** — "모든 자녀가 18세 미만" 같은 비교를 못 합니다. `exists` 로 두고 메모하세요
- **연령에 따라 달라지는 상한** — "청년은 5억, 그 외 4억" 은 한 조건으로 못 씁니다. 낮은 쪽으로 적고 메모하세요
- **취업 경험·가입 기간** — 해당 항목이 없습니다

이런 요건이 자주 나오면 백엔드에 알려주세요. 항목을 추가할 수 있습니다.

---

## 8. 마지막 확인

```bash
cd api
go run ./cmd/validate
go test ./...
```

둘 다 통과하면 커밋하세요.
