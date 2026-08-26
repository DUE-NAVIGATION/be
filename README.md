# DUE — BE

복지 사각지대 내비게이터 백엔드. Spring Boot 3.5.3 + Java 21.

> due = "마땅히 지급되어야 할". **이건 원래 당신 것입니다.**

- 프론트엔드: https://github.com/DUE-NAVIGATION/FE
- 프로젝트 전체 설명·설계: https://github.com/DUE-NAVIGATION/.github

## 시작하기

```bash
cp .env.example .env       # 값을 채워 환경변수로 주입
./mvnw spring-boot:run     # http://localhost:8080
```

`JAVA_HOME`이 JDK 21 이상을 가리켜야 한다. Maven은 래퍼(`./mvnw`)를 쓰므로 별도 설치가 필요 없다.

```bash
curl http://localhost:8080/api/health
# {"service":"due-backend","storesUserData":false,"status":"ok"}
```

| 명령어                   | 설명            |
| ------------------------ | --------------- |
| `./mvnw spring-boot:run` | 개발 서버       |
| `./mvnw test`            | 테스트          |
| `./mvnw package`         | 실행 가능 JAR   |

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
```

## 이 저장소의 규칙 (요약)

1. **판정은 AI가 하지 않는다.** 자격 판정·금액 계산은 `com.due.rules`의 결정론적 규칙 엔진.
   AI는 ①자연어 → 구조화 ②판정 결과 → 사람 말 설명, 딱 둘만 한다
2. **아무것도 저장하지 않는다.** `UserContext`는 요청 처리 중 메모리에만 존재한다.
   DB·필드·**로그** 어디에도 남기지 않는다
3. **단정하지 않는다.** 값이 없는 조건은 `FAIL`이 아니라 `UNKNOWN`,
   결과는 `NEEDS_INFO`. 조건 단위 근거를 항상 함께 내려준다

### DB를 쓰지 않는 이유

JPA/PostgreSQL을 일부러 넣지 않았다. 제도 데이터는 JSON 파일에서 읽고, 사용자 입력은
저장하지 않는다. DataSource가 없으면 앱이 부팅에 실패해 데모 리스크가 된다.

전체 규칙은 [CLAUDE.md](CLAUDE.md) 참조.

## 현재 상태

Phase 0 완료 — 셋업, 도메인 타입, health 엔드포인트.
다음은 **Phase 1 규칙 엔진**이고, **Phase 3 제도 데이터**와 병렬로 진행해야 한다.
