# 배포

FE 는 **Vercel**, BE 는 **Render** 에 올린다. 둘은 서로의 주소만 알면 된다.

```
브라우저 ──▶ Vercel (Next.js)  ──▶ Render (Go, Docker)
             정적 화면            판정·계산 · 시설·제도 데이터(읽기 전용)
```

**DB 는 여기 없다.** 사용자 데이터를 담는 저장소가 없기 때문이다 —
시설·제도 데이터는 이미지 안에 들어 있고, 읽기 전용이다. 그래서 디스크·볼륨 설정이 필요 없다.

---

## 1. 백엔드 — Render

### 최초 1회

1. `be` 저장소를 GitHub 에 푸시한다 (Render 는 GitHub 에서 가져간다)
2. Render 대시보드 → **New → Blueprint** → `DUE-NAVIGATION/be` 선택
   → 저장소의 `render.yaml` 을 읽어 `due-api` 서비스를 만든다 (Docker · 무료 · 싱가포르)
3. 대시보드에서 값을 넣으라고 묻는 두 개
   - `CORS_ALLOWED_ORIGINS` — Vercel 프로덕션 도메인. **아직 모르면 일단 비워 두고 3단계 후에 넣는다**
   - `ANTHROPIC_API_KEY` — 비워 둬도 된다. `DEMO_MODE=true` 라 캐시 응답으로 시연된다

빌드 중에 **시설·제도 데이터 검증이 자동으로 돈다.** 깨진 데이터가 있으면 배포가 멈춘다.

### 배포 후 확인

```bash
curl https://due-api.onrender.com/healthz
```

```json
{ "status": "ok", "programCount": 3, "facilityCount": 183,
  "medianIncomeYear": 2026, "storesUserData": false, "aiEnabled": false }
```

- `programCount` · `facilityCount` 가 **0 이면 안 된다.** 데이터를 못 읽은 것이다
- `storesUserData` 는 **항상 false** 다. 서버가 스스로 밝히는 값이다
- 주소의 `due-api` 부분은 서비스 이름이 겹치면 Render 가 바꿔 준다. 실제 주소를 대시보드에서 확인할 것

### ★ 무료 플랜은 잠든다

15분간 요청이 없으면 잠들고, 첫 요청이 **30초~1분** 걸린다. 프론트는 8초에 타임아웃이 나서
"백엔드 없음" 이 뜬다. 고장이 아니다.

- **발표 5분 전에 `/healthz` 를 브라우저로 한 번 연다** — 깨어나면 이후는 즉시 응답한다
- 심사 기간 내내 깨워 두려면 UptimeRobot 같은 무료 모니터링으로 10분마다 `/healthz` 를 부른다

### 환경변수

| 이름 | 기본값 | 설명 |
|---|---|---|
| `PORT` | `8080` | |
| `PROGRAM_SOURCE` | `json` | `json` \| `sqlite` |
| `PROGRAM_DB` | `data/due.db` | `sqlite` 일 때만 |
| `DATA_DIR` | `data` | |
| `CORS_ALLOWED_ORIGINS` | `http://localhost:3000` | **쉼표 구분. 정확 일치** |
| `ANTHROPIC_API_KEY` | (없음) | 시크릿으로만 |
| `DEMO_MODE` | `false` | 캐시된 응답으로 API 호출 없이 시연 |

---

## 2. 프론트엔드 — Vercel

### 최초 1회

Vercel 에서 `DUE-NAVIGATION/FE` 저장소를 가져온다. 프레임워크는 자동으로 Next.js 로 잡힌다.

**환경변수를 반드시 넣는다** (Settings → Environment Variables):

```
NEXT_PUBLIC_API_BASE_URL = https://due-api.onrender.com
```

`NEXT_PUBLIC_` 접두어가 붙은 값은 **브라우저까지 나간다.** 그래서 여기 넣어도 되는 것은
백엔드 주소뿐이다. **`ANTHROPIC_API_KEY` 를 여기 넣지 마라** — 키가 그대로 공개된다.

### 배포 후 확인

첫 화면 상단에 이렇게 떠야 한다.

```
● 규칙 엔진 정상   제도 3건 · 2026년 기준중위소득
```

`백엔드 없음` 이 뜨면 십중팔구 아래 둘 중 하나다.

1. `NEXT_PUBLIC_API_BASE_URL` 이 안 들어갔거나 오타
2. **백엔드의 `CORS_ALLOWED_ORIGINS` 에 Vercel 도메인이 없다**

---

## 3. ★ 두 번 겪은 함정 — CORS 오리진

백엔드는 허용 오리진을 **정확히 일치**로만 받는다. 와일드카드가 없다.

- Vercel 은 배포마다 **프리뷰 도메인**을 새로 만든다
  (`due-navigator-git-main-....vercel.app`). 이 주소로 열면 막힌다.
  **채점자에게는 프로덕션 도메인을 준다.**
- 로컬에서 포트 3000 이 잡히면 Next 가 3001 로 뜨는데, 그 순간 전 요청이 막힌다

Render 대시보드 → `due-api` → Environment 에서 이렇게 넣는다 (쉼표 구분, 공백 없이).

```
CORS_ALLOWED_ORIGINS = https://due-navigator.vercel.app,http://localhost:3000
```

값을 바꾸면 Render 가 서버를 다시 띄운다. 1분쯤 뒤에 다시 확인한다.

증상은 언제나 같다 — 화면에 **"백엔드 없음"** 만 뜬다. 브라우저 콘솔을 열면
`blocked by CORS policy` 가 보인다. 여기부터 확인할 것.

---

## 4. 제도 DB

```bash
go run ./cmd/seed                  # data/programs/*.json → data/due.db
PROGRAM_SOURCE=sqlite go run ./cmd/server
```

- **원본은 언제나 `data/programs/*.json`** 이다. DB 는 그 사본이다
- 방향은 JSON → DB 한 쪽뿐이다. DB 를 고쳐도 JSON 으로 돌아오지 않는다
- `data/*.db` 는 커밋하지 않는다 (생성물). Docker 빌드 중에 만들어진다
- 저장소를 바꿔도 **판정 결과는 동일하다.** 테스트로 고정해 두었다
  (`internal/store/sqlite_test.go`)

문제가 생기면 `PROGRAM_SOURCE=json` 으로 되돌린다. 그게 기본값인 이유다.

---

## 5. 제출 전 점검

- [ ] ★ **발표 5분 전 `/healthz` 로 Render 를 깨웠다**

- [ ] `curl <BE>/healthz` → `programCount` 가 0 이 아니다
- [ ] Vercel 프로덕션 도메인으로 열어 첫 화면이 "규칙 엔진 정상"
- [ ] 직접 입력 → 결과 화면까지 완주
- [ ] **새로고침** 하면 결과가 사라진다 (버그 아님, 이게 요점)
- [ ] 조건별 근거표가 펼쳐진다
- [ ] 휴대폰에서 한 번 — 채점자가 폰으로 열 수 있다
- [ ] 하단 고지 문구가 모든 화면에 보인다
- [ ] 브라우저 콘솔에 CORS 오류가 없다

---

## 부록 — Fly.io 로 옮길 때

`fly.toml` 이 남아 있다. 잠들지 않게 하려면(최소 1대 상시) Fly 가 낫지만, 신규 계정은 카드 등록이 필요하다.

```bash
fly launch --no-deploy
fly secrets set CORS_ALLOWED_ORIGINS="https://due-navigator.vercel.app"
fly deploy
```

그 경우 Vercel 의 `NEXT_PUBLIC_API_BASE_URL` 을 `https://<앱이름>.fly.dev` 로 바꾼다.
