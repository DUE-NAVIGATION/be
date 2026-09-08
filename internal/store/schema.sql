-- DUE 제도 저장소 스키마
--
-- ★★ 이 데이터베이스에 사용자 테이블은 없다. 앞으로도 만들지 않는다.
--    담기는 것은 "제도가 무엇인가" 뿐이고, "누가 무엇을 물었는가" 는 담기지 않는다.
--    설계 원칙 2(아무것도 저장하지 않는다)는 이 파일에서도 지켜진다.
--    사용자 상황은 요청이 살아 있는 동안 메모리에만 존재한다.
--
-- 진실의 원본은 여전히 data/programs/*.json 이다 (git 에서 리뷰·diff 가 된다).
-- 이 DB 는 cmd/seed 가 그 JSON 을 옮겨 담은 것이다. 한쪽만 고치지 마라.

-- ★ WAL 을 쓰지 않는다. 이 DB 는 seed 가 한 번 쓰고 그 뒤로는 읽기만 한다.
--   WAL 은 -wal/-shm 파일에 쓰기를 요구하는데, 배포 이미지(distroless·nonroot)의
--   /app/data 는 쓸 수 없어서 서버가 뜨자마자 죽는다. 실제로 겪었다.
--
--   journal_mode 는 파일에 영구 저장된다 — 한 번 WAL 로 만든 DB 는 스키마에서
--   빼도 그대로 남는다. 그래서 명시적으로 되돌린다.
PRAGMA journal_mode = DELETE;
PRAGMA foreign_keys = ON;

-- ── 제도 ────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS programs (
    id                TEXT PRIMARY KEY,
    name              TEXT NOT NULL,
    category          TEXT NOT NULL,
    summary           TEXT NOT NULL DEFAULT '',

    -- 급여. 금액은 전부 정수(원)다. 부동소수점을 쓰지 않는다
    benefit_type      TEXT    NOT NULL,
    benefit_amount    INTEGER NOT NULL DEFAULT 0,
    benefit_months    INTEGER NOT NULL DEFAULT 0,
    benefit_rate_pct  REAL    NOT NULL DEFAULT 0,
    benefit_note      TEXT    NOT NULL DEFAULT '',

    -- 신청. 채널·서류는 길이가 정해지지 않은 목록이라 JSON 배열로 둔다.
    -- 판정에 쓰이지 않고 화면에 그대로 나열되기만 하므로 쪼갤 이유가 없다
    apply_channel     TEXT NOT NULL DEFAULT '[]',
    apply_documents   TEXT NOT NULL DEFAULT '[]',
    apply_period      TEXT NOT NULL DEFAULT '',

    -- 출처. revised_at 은 심사에서 물어본다 — 비워두지 마라
    source_url        TEXT NOT NULL DEFAULT '',
    source_revised_at TEXT NOT NULL DEFAULT '',
    source_agency     TEXT NOT NULL DEFAULT '',
    source_note       TEXT NOT NULL DEFAULT '',

    seeded_at         TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_programs_category ON programs (category);

-- ── 자격요건 ────────────────────────────────────────────────
-- 조건은 제도의 부속이 아니라 판정의 최소 단위다. 따로 둔다.
--
--   grp = all   전부 PASS 여야 통과 (AND)
--   grp = any   하나라도 PASS 면 통과 (OR)
--   grp = none  하나라도 PASS 면 탈락 (배제 조건)
--
-- ord 는 JSON 배열에서의 순서다. 화면의 근거표가 이 순서로 그려지므로
-- 순서가 흔들리면 같은 입력에 다른 화면이 나온다.
CREATE TABLE IF NOT EXISTS conditions (
    program_id TEXT    NOT NULL REFERENCES programs (id) ON DELETE CASCADE,
    grp        TEXT    NOT NULL CHECK (grp IN ('all', 'any', 'none')),
    ord        INTEGER NOT NULL,

    field      TEXT NOT NULL,
    op         TEXT NOT NULL CHECK (
                   op IN ('between','lte','gte','eq','in','contains','exists')
               ),
    -- 비교값. op 에 따라 숫자·문자열·불리언·배열이 온다.
    -- 형이 하나가 아니므로 JSON 으로 담는다. exists 면 NULL
    value_json TEXT,
    label      TEXT NOT NULL DEFAULT '',
    note       TEXT NOT NULL DEFAULT '',

    PRIMARY KEY (program_id, grp, ord)
);

CREATE INDEX IF NOT EXISTS idx_conditions_field ON conditions (field);

-- ── 제도 간 관계 (중복수급) ─────────────────────────────────
CREATE TABLE IF NOT EXISTS relations (
    from_id    TEXT NOT NULL,
    to_id      TEXT NOT NULL,
    type       TEXT NOT NULL CHECK (
                   type IN ('EXCLUSIVE', 'REDUCING', 'PREREQUISITE')
               ),
    reduce_pct REAL NOT NULL DEFAULT 0,
    reason     TEXT NOT NULL DEFAULT '',

    PRIMARY KEY (from_id, to_id, type)
);
