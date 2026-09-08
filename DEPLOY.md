# 배포

FE 는 Vercel, BE 는 Fly.io 에 올린다. 둘은 서로의 주소만 알면 된다.

```
브라우저 ──▶ Vercel (Next.js)  ──▶ Fly.io (Go)  ──▶ Anthropic API
             정적 화면            판정·계산          구조화·설명만
                                  제도 DB(읽기 전용)
```

**DB 는 여기 없다.** 사용자 데이터를 담는 저장소가 없기 때문이다 —
제도 데이터만 이미지 안의 SQLite 파일에 들어 있고, 그마저 읽기 전용이다.

---

## 1. 백엔드 — Fly.io

### 최초 1회

```bash
cd api
fly launch --no-deploy          # app 이름을 정한다. fly.toml 이 이미 있다
fly secrets set ANTHROPIC_API_KEY=sk-ant-...
```

키를 넣지 않아도 된다. **키가 없어도 서버는 뜨고 판정은 정상 동작한다** —
대화형 입력만 막히고 화면이 직접 입력으로 넘어간다.

### 배포

```bash
fly deploy
fly logs                        # "제도 저장소 source=..." 가 보이면 정상
```

### 배포 후 확인

```bash
curl https://due-api.fly.dev/healthz
```

```json
{ "status": "ok", "programCount": 3, "medianIncomeYear": 2026,
  "storesUserData": false, "aiEnabled": true }
```

- `programCount` 가 **0 이면 안 된다.** 제도를 못 읽은 것이다
- `storesUserData` 는 **항상 false** 다. 서버가 스스로 밝히는 값이다
- `aiEnabled` 가 false 면 시크릿이 안 들어간 것이다 (판정에는 지장 없다)

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
NEXT_PUBLIC_API_BASE_URL = https://due-api.fly.dev
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

```bash
# 프로덕션 + 로컬 둘 다 허용
fly secrets set CORS_ALLOWED_ORIGINS="https://due-navigator.vercel.app,http://localhost:3000,http://localhost:3001"
```

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

- [ ] `curl <BE>/healthz` → `programCount` 가 0 이 아니다
- [ ] Vercel 프로덕션 도메인으로 열어 첫 화면이 "규칙 엔진 정상"
- [ ] 직접 입력 → 결과 화면까지 완주
- [ ] **새로고침** 하면 결과가 사라진다 (버그 아님, 이게 요점)
- [ ] 조건별 근거표가 펼쳐진다
- [ ] 휴대폰에서 한 번 — 채점자가 폰으로 열 수 있다
- [ ] 하단 고지 문구가 모든 화면에 보인다
- [ ] 브라우저 콘솔에 CORS 오류가 없다
