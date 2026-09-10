# 시설 데이터

이용자가 **실제로 전화하거나 찾아갈 수 있는 곳**의 목록이다.
제도(`data/programs/`)가 "무엇을 받을 수 있는가" 라면, 여기는 "어디로 가면 되는가" 다.

## ★ 절대 규칙 — 연락처를 지어내지 마라

전화번호·주소는 **확인된 것만** 적는다. 확인되지 않으면 **비워 둔다.**

틀린 번호로 전화하게 만드는 것은 안내하지 않는 것보다 나쁘다.
사각지대에 있는 사람은 한 번 헛걸음하면 다시 시도하지 않는다.
이 서비스가 하려는 일의 정반대가 된다.

`ValidateFacility` 가 "연락할 방법이 하나도 없는 시설" 을 걸러낸다.
연락할 수 없는 시설은 목록만 늘리고 이용자를 막다른 길로 보낸다.

## 파일 형식

제도와 달리 **파일 하나에 여러 시설을 배열로** 담는다.
지역아동센터만 한 구에 수십 곳이라 파일을 하나씩 만들 수 없다.

```
data/facilities/
  national-hotlines.json     전국 상담 창구 (관할 NATIONWIDE)
  seoul-gwanak.json          예: 서울 관악구 시설
  _example.json              _ 로 시작하면 읽지 않는다
```

## 필수 항목

| 항목 | 설명 |
|---|---|
| `id` | 파일 전체에서 유일. `시도-시군구-이름` 꼴을 권장 |
| `name` | 시설명 (원본 그대로) |
| `type` | 아래 시설 종류 중 하나 |
| `coverage` | **관할 범위. 판정의 1순위 조건이다** |
| `contact` | 연락 수단 최소 하나 |
| `location` | HOTLINE 이 아니면 주소 필수 |
| `source.revisedAt` | 데이터 기준일자. 심사에서 물어본다 |

### 시설 종류 (`type`)

```
CHILD_CENTER       지역아동센터
COMMUNITY_WELFARE  종합사회복지관
ELDERLY            노인복지관·경로당
DISABILITY         장애인복지관
MENTAL_HEALTH      정신건강복지센터
SELF_SUFFICIENCY   지역자활센터
SHELTER            쉼터·보호시설
SINGLE_PARENT      한부모가족복지시설
FAMILY_CENTER      가족센터·건강가정지원센터
JOB_CENTER         고용복지플러스센터
HOTLINE            전화 상담 창구
COMMUNITY_CENTER   행정복지센터(주민센터)
OTHER
```

### 관할 (`coverage`) — 가장 중요한 항목

```json
{ "scope": "NATIONWIDE" }
{ "scope": "SIDO",    "sido": "서울특별시" }
{ "scope": "SIGUNGU", "sido": "서울특별시", "sigungu": "관악구" }
```

관할은 판정에서 **조건 한 줄**로 다뤄져 근거표에 그대로 나온다.
코드 안에서 조용히 걸러내지 않는 이유: "왜 우리 동네 센터가 안 나오지" 를
화면이 스스로 답할 수 있어야 하기 때문이다.

```
관악구 거주자 | district | 강남구 | = 관악구 | FAIL | 관할 밖입니다 (입력: 강남구)
```

★ **거주지를 모르면 FAIL 이 아니라 UNKNOWN 이다.**
지역을 안 적었다는 이유로 갈 수 있는 곳이 사라지면 이 서비스는 실패한다.

### 이용 대상 (`eligibility`)

제도와 **완전히 같은 구조**다 (`all` / `any` / `none`). 규칙 엔진도 같은 것을 쓴다.

★ 비워 두면 "그 지역 살면 누구나" 라는 뜻이고, 그건 정상이다.
제도는 조건이 비면 확인필요로 두지만 시설은 이용 가능으로 본다 —
지역아동센터처럼 조건이 관할뿐인 곳이 실제로 많다.

## 데이터 출처

**전국사회복지시설표준데이터** (사회보장정보원 / 보건복지부)
<https://www.data.go.kr/data/15096296/standard.do>

제공 항목이 우리 모델과 이렇게 대응한다.

| 표준데이터 | 우리 필드 |
|---|---|
| 시설명 | `name` |
| 시설종류명 | `type` (매핑 필요) |
| 소재지도로명주소 | `location.roadAddress` |
| 소재지지번주소 | `location.lotAddress` |
| 위도 · 경도 | `location.lat` · `location.lng` |
| 전화번호 | `contact.phone` |
| 홈페이지주소 | `contact.website` |
| 관할행정기관 | `authority` |
| 데이터기준일자 | `source.revisedAt` |

**표준데이터에 없어서 사람이 채워야 하는 것**

- `coverage` — 관할 범위. 주소의 시군구에서 유추하되, 시도 단위 시설인지 확인할 것
- `eligibility` — 이용 대상 조건. 시설 홈페이지·전화 확인이 필요하다
- `services` · `fee` · `documents` — 무엇을 해주고 얼마이고 뭘 챙겨가야 하는지
- `summary` — 이용자 말로 쓴 한 줄 설명. **"이 시설이 나에게 뭘 해주나" 에 답할 것**

## 검증

```bash
go run ./cmd/validate
```

깨진 시설은 **건너뛰고 나머지로 서버가 뜬다.** 건너뛴 것은 로그와
`GET /api/facilities` 의 `problems` 에 남는다. 조용히 사라지지 않는다.

## 지금 들어 있는 것

- `national-hotlines.json` — 129 · 109 · 1366 · 1388 · 1577-0199 (전국, 확인 완료)

★ **109 주의** — 2024-01-01 부로 자살예방 상담전화가 109 로 통합되었다.
옛 번호 `1393` 을 안내하지 마라.
