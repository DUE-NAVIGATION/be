# BE — DUE 백엔드

복지 사각지대 내비게이터의 백엔드. 판정·계산·AI 게이트웨이·제도 저장소를 담당한다.

- 프론트엔드: https://github.com/DUE-NAVIGATION/FE
- 프로젝트 전체 설명·설계: https://github.com/DUE-NAVIGATION/.github

## 이 모듈의 책임

자격 판정, 금액 계산, AI 호출, 제도 데이터 로딩. **FE는 결과를 그리기만 한다.**

## ★ 최상위 설계 원칙 (절대 어기지 말 것)

### 1. 판정은 AI가 하지 않는다

- 자격 판정과 금액 계산은 **결정론적 규칙 엔진**(`com.due.rules`)이 한다
- AI의 역할은 딱 둘: ①자연어 → 구조화된 입력값 ②판정 결과 → 사람 말 설명
- LLM에게 "이 사람이 이 제도에 해당하나요?"를 묻는 코드를 절대 작성하지 마라
- 이유: 환각, 비일관성, 근거 부재. 심사에서 가장 먼저 공격당하는 지점이다

### 2. 아무것도 저장하지 않는다

- 로그인 없음. DB에 사용자 입력을 저장하지 않는다
- `UserContext`는 요청 처리 중 메모리에만 존재한다. 필드·DB·**로그** 어디에도 남기지 않는다
- LLM에 보낼 때도 최소 항목만. 이름·주소·주민번호는 전송하지 않는다
- 전송 전 시크릿 필터로 주민번호·계좌번호 패턴을 제거한다
- 문서 이미지는 OCR 직후 폐기
- 이것이 발표의 핵심 장면이다. 절대 편의를 위해 저장 기능을 추가하지 마라

### 3. 단정하지 않는다

- 결과는 `ELIGIBLE` / `INELIGIBLE` / `NEEDS_INFO` 세 가지. 애매하면 반드시 `NEEDS_INFO`
- 입력값이 없는 조건은 `FAIL`이 아니라 `UNKNOWN`이다. **값이 없다고 탈락시키지 마라**
- 조건 단위 근거(`ConditionResult`)를 항상 함께 내려준다 (설명 가능성)

## 기술 스택

Spring Boot 3.5.3 · Java 21 · Maven (`./mvnw`, 로컬 mvn 설치 불필요)

JPA/PostgreSQL은 일부러 넣지 않았다. 제도 데이터는 JSON 파일에서 읽고, 사용자 입력은
저장하지 않는다. DataSource가 없으면 앱이 부팅에 실패해 데모 리스크가 된다.

## 패키지 구조

```
com.due
├─ domain/    공용 모델 (UserContext, Program, MatchResult …)
├─ rules/     ★ 규칙 엔진 — 순수 함수, 스프링 의존 없음, 테스트 우선
├─ income/    소득 계산 엔진
├─ program/   제도 저장소 (resources/programs/*.json 로더)
├─ ai/        AI 게이트웨이 (구조화 · 설명 · OCR)
├─ api/       REST 컨트롤러
└─ config/    CORS, 설정 프로퍼티

src/main/resources/
├─ programs/*.json      제도 데이터
├─ median-income.json   기준중위소득 표 (Phase 2)
└─ application.yml
```

## 규칙

- `com.due.rules`, `com.due.income`은 **스프링에 의존하지 않는 순수 함수**로 짠다.
  `@Service`를 붙이지 말고 일반 클래스로 둔다. 테스트를 먼저 쓴다
- `com.due.ai`는 구조화와 설명만 한다. **판정을 시키는 프롬프트를 작성하지 마라**
- 제도 데이터는 `resources/programs/*.json`. 코드에 하드코딩 금지
- `domain`의 record를 고치면 FE의 `types/index.ts`도 같이 고친다. 한쪽만 고치면 런타임에 깨진다
- 사용자 입력값을 로그에 찍지 않는다

## 작업 방식

1. 코드 변경 시 **파일 전체 출력**. 부분 스니펫 금지
2. 규칙 엔진과 계산 엔진은 **테스트를 먼저 쓴다**
3. LLM 프롬프트도 코드 리뷰 대상 — 초안을 보여주고 진행
4. 해커톤이다. 스코프를 넓히지 마라. 요청 범위만

## 하지 말 것

- LLM에게 자격 판정을 맡기는 코드
- 사용자 입력을 DB·로그에 저장
- 로그인·인증·관리자 API
- 실제 신청 API 연동 (존재하지 않음)
- 제도 데이터를 코드에 하드코딩

## 명령어

```bash
./mvnw spring-boot:run   # http://localhost:8080
./mvnw test
./mvnw package
```

`JAVA_HOME`이 JDK 21 이상을 가리켜야 한다.

## 진행 현황

- [x] Phase 0 — 셋업 + 도메인 타입
- [ ] Phase 1 — 규칙 엔진 `com.due.rules` ★ 최우선
- [ ] Phase 2 — 소득 계산 + 중복수급
- [ ] Phase 3 — 제도 데이터 30~50건 ★ 최대 병목, Phase 1과 병렬
- [ ] Phase 4 — AI 구조화 `com.due.ai`
- [ ] Phase 6 — 문서 번역 (여유 시)
- [ ] Phase 7 — 데모 안정화 ★ 반드시
