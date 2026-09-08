// Package store 는 제도 데이터를 SQLite 에서 읽는 저장소다.
//
// ★ 진실의 원본은 여전히 data/programs/*.json 이다. 이 패키지는 cmd/seed 가
// 옮겨 담은 것을 읽을 뿐이다. loader.Store 와 같은 메서드를 제공하므로
// 서버는 둘 중 무엇을 쓰는지 몰라도 된다 (PROGRAM_SOURCE 환경변수).
//
// ★★ 사용자 데이터는 이 DB 에 들어가지 않는다. schema.sql 을 볼 것.
package store

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/DUE-NAVIGATION/be/internal/loader"
	"github.com/DUE-NAVIGATION/be/internal/model"

	// 순수 Go SQLite 드라이버. cgo 가 필요 없어 distroless 이미지에 그대로 들어간다
	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

// Schema 는 스키마 DDL 이다. cmd/seed 가 쓴다.
func Schema() string { return schemaSQL }

// Open 은 쓰기 가능하게 열고 스키마를 보장한다. 파일이 없으면 만든다.
//
// ★ cmd/seed 전용이다. 서버는 이걸 쓰지 않는다 — OpenReadOnly 를 쓴다.
func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite",
		"file:"+path+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("DB 를 열지 못했습니다: %w", err)
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("스키마를 적용하지 못했습니다: %w", err)
	}
	return db, nil
}

// OpenReadOnly 는 읽기 전용으로 연다. 스키마를 건드리지 않는다.
//
// ★ 서버는 제도 데이터를 읽기만 한다. 그 사실을 주석이 아니라 연결 모드로 강제한다 —
// 실수로 INSERT 를 쓰는 코드가 들어와도 SQLite 가 거부한다.
//
// 실무적인 이유도 있다. 배포 이미지(distroless, nonroot)에서 /app/data 는
// 쓸 수 없다. 쓰기 모드로 열면 서버가 뜨자마자 죽는다 — 실제로 겪었다.
func OpenReadOnly(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite",
		"file:"+path+"?mode=ro&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("DB 를 열지 못했습니다: %w", err)
	}
	// sql.Open 은 실제로 연결하지 않는다. 파일이 없는 것을 여기서 알아야
	// "제도 0건" 으로 조용히 뜨는 일이 없다
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf(
			"제도 DB 를 열지 못했습니다 (%s). `go run ./cmd/seed` 를 먼저 실행하세요: %w",
			path, err)
	}
	return db, nil
}

// Store 는 DB 에서 읽은 제도를 메모리에 들고 있다.
//
// 요청마다 DB 를 때리지 않는 이유: 판정은 제도 전체를 훑는다. 제도 수는
// 수십 건이고 거의 바뀌지 않는다. 메모리에 올려두면 loader.Store 와
// 동작이 완전히 같아져서, 데모 중 저장소를 바꿔도 결과가 달라지지 않는다.
type Store struct {
	db *sql.DB

	mu        sync.RWMutex
	programs  []model.Program
	relations []model.Relation
	problems  []loader.Problem
}

// New 는 DB 를 읽기 전용으로 열고 전부 읽어 들인다.
func New(path string) (*Store, error) {
	db, err := OpenReadOnly(path)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.Reload(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

// Reload 는 DB 를 다시 읽는다.
//
// 한 행이 깨져도 전체를 버리지 않는다 — 나머지 제도로 서비스는 계속 돌아야 한다.
// 건너뛴 행은 Problems() 로 드러난다. 조용히 빠지면 아무도 모른다.
func (s *Store) Reload() error {
	programs, problems, err := readPrograms(s.db)
	if err != nil {
		return err
	}
	relations, relProblems, err := readRelations(s.db)
	if err != nil {
		return err
	}
	problems = append(problems, relProblems...)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.programs, s.relations, s.problems = programs, relations, problems
	return nil
}

// 아래 넷은 loader.Store 와 시그니처가 같다. handler 는 둘을 구분하지 않는다.

func (s *Store) Programs() []model.Program {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.Program, len(s.programs))
	copy(out, s.programs)
	return out
}

func (s *Store) Relations() []model.Relation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.Relation, len(s.relations))
	copy(out, s.relations)
	return out
}

func (s *Store) Problems() []loader.Problem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]loader.Problem, len(s.problems))
	copy(out, s.problems)
	return out
}

func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.programs)
}

// ── 읽기 ────────────────────────────────────────────────────

const selectPrograms = `
SELECT id, name, category, summary,
       benefit_type, benefit_amount, benefit_months, benefit_rate_pct, benefit_note,
       apply_channel, apply_documents, apply_period,
       source_url, source_revised_at, source_agency, source_note
FROM programs
ORDER BY id`

func readPrograms(db *sql.DB) ([]model.Program, []loader.Problem, error) {
	rows, err := db.Query(selectPrograms)
	if err != nil {
		return nil, nil, fmt.Errorf("제도를 읽지 못했습니다: %w", err)
	}
	defer rows.Close()

	programs := []model.Program{}
	problems := []loader.Problem{}

	for rows.Next() {
		var p model.Program
		var channelJSON, documentsJSON string

		err := rows.Scan(
			&p.ID, &p.Name, &p.Category, &p.Summary,
			&p.Benefit.Type, &p.Benefit.Amount, &p.Benefit.Months,
			&p.Benefit.RatePct, &p.Benefit.Note,
			&channelJSON, &documentsJSON, &p.Apply.Period,
			&p.Source.URL, &p.Source.RevisedAt, &p.Source.Agency, &p.Source.Note,
		)
		if err != nil {
			return nil, nil, fmt.Errorf("제도 행을 읽지 못했습니다: %w", err)
		}

		if err := json.Unmarshal([]byte(channelJSON), &p.Apply.Channel); err != nil {
			problems = append(problems, loader.Problem{
				File: "programs:" + p.ID, Reason: "apply_channel 이 JSON 배열이 아닙니다",
			})
			continue
		}
		if err := json.Unmarshal([]byte(documentsJSON), &p.Apply.Documents); err != nil {
			problems = append(problems, loader.Problem{
				File: "programs:" + p.ID, Reason: "apply_documents 가 JSON 배열이 아닙니다",
			})
			continue
		}
		programs = append(programs, p)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	// 조건은 제도마다 한 번씩 질의하지 않고 한 번에 읽어 붙인다.
	// 제도 수가 늘어도 질의 수는 2회로 고정된다
	byID, condProblems, err := readConditions(db)
	if err != nil {
		return nil, nil, err
	}
	problems = append(problems, condProblems...)

	for i := range programs {
		programs[i].Eligibility = byID[programs[i].ID]
	}
	return programs, problems, nil
}

const selectConditions = `
SELECT program_id, grp, field, op, value_json, label, note
FROM conditions
ORDER BY program_id, grp, ord`

func readConditions(db *sql.DB) (map[string]model.Eligibility, []loader.Problem, error) {
	rows, err := db.Query(selectConditions)
	if err != nil {
		return nil, nil, fmt.Errorf("자격요건을 읽지 못했습니다: %w", err)
	}
	defer rows.Close()

	out := map[string]model.Eligibility{}
	problems := []loader.Problem{}

	for rows.Next() {
		var programID, grp string
		var c model.Condition
		var value sql.NullString

		if err := rows.Scan(&programID, &grp, &c.Field, &c.Op, &value,
			&c.Label, &c.Note); err != nil {
			return nil, nil, fmt.Errorf("조건 행을 읽지 못했습니다: %w", err)
		}

		if value.Valid && value.String != "" {
			if err := json.Unmarshal([]byte(value.String), &c.Value); err != nil {
				// ★ 이 조건 하나만 버린다. 제도 전체를 버리면 나머지 조건의
				//   근거까지 사라져 화면이 설명력을 잃는다
				problems = append(problems, loader.Problem{
					File:   "conditions:" + programID,
					Reason: c.Field + " 조건의 value 가 JSON 이 아닙니다",
				})
				continue
			}
		}

		e := out[programID]
		switch grp {
		case "all":
			e.All = append(e.All, c)
		case "any":
			e.Any = append(e.Any, c)
		case "none":
			e.None = append(e.None, c)
		}
		out[programID] = e
	}
	return out, problems, rows.Err()
}

const selectRelations = `
SELECT from_id, to_id, type, reduce_pct, reason
FROM relations
ORDER BY from_id, to_id`

func readRelations(db *sql.DB) ([]model.Relation, []loader.Problem, error) {
	rows, err := db.Query(selectRelations)
	if err != nil {
		return nil, nil, fmt.Errorf("제도 관계를 읽지 못했습니다: %w", err)
	}
	defer rows.Close()

	out := []model.Relation{}
	for rows.Next() {
		var r model.Relation
		if err := rows.Scan(&r.From, &r.To, &r.Type, &r.ReducePct, &r.Reason); err != nil {
			return nil, nil, fmt.Errorf("관계 행을 읽지 못했습니다: %w", err)
		}
		out = append(out, r)
	}
	return out, []loader.Problem{}, rows.Err()
}

// ── 쓰기 (cmd/seed 전용) ────────────────────────────────────

// Seed 는 제도와 관계를 DB 에 밀어 넣는다.
//
// 통째로 지우고 다시 쓴다. JSON 이 원본이므로 DB 는 언제나 그 사본이면 된다 —
// 부분 갱신을 지원하면 "JSON 에서 지운 제도가 DB 에 남아 있는" 상태가 생긴다.
//
// 트랜잭션 하나로 처리한다. 중간에 실패하면 이전 상태 그대로다.
func Seed(db *sql.DB, programs []model.Program, relations []model.Relation) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint — 성공 시 Commit 뒤의 Rollback 은 무시된다

	for _, stmt := range []string{
		"DELETE FROM conditions", "DELETE FROM programs", "DELETE FROM relations",
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("기존 데이터를 비우지 못했습니다: %w", err)
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)

	insProgram, err := tx.Prepare(`
INSERT INTO programs (id, name, category, summary,
    benefit_type, benefit_amount, benefit_months, benefit_rate_pct, benefit_note,
    apply_channel, apply_documents, apply_period,
    source_url, source_revised_at, source_agency, source_note, seeded_at)
VALUES (?,?,?,?, ?,?,?,?,?, ?,?,?, ?,?,?,?, ?)`)
	if err != nil {
		return err
	}
	defer insProgram.Close()

	insCondition, err := tx.Prepare(`
INSERT INTO conditions (program_id, grp, ord, field, op, value_json, label, note)
VALUES (?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer insCondition.Close()

	for _, p := range programs {
		channel, err := jsonArray(p.Apply.Channel)
		if err != nil {
			return fmt.Errorf("%s 의 신청 채널: %w", p.ID, err)
		}
		documents, err := jsonArray(p.Apply.Documents)
		if err != nil {
			return fmt.Errorf("%s 의 필요 서류: %w", p.ID, err)
		}

		if _, err := insProgram.Exec(p.ID, p.Name, string(p.Category), p.Summary,
			string(p.Benefit.Type), p.Benefit.Amount, p.Benefit.Months,
			p.Benefit.RatePct, p.Benefit.Note,
			channel, documents, p.Apply.Period,
			p.Source.URL, p.Source.RevisedAt, p.Source.Agency, p.Source.Note,
			now); err != nil {
			return fmt.Errorf("%s 를 넣지 못했습니다: %w", p.ID, err)
		}

		groups := []struct {
			name  string
			conds []model.Condition
		}{
			{"all", p.Eligibility.All},
			{"any", p.Eligibility.Any},
			{"none", p.Eligibility.None},
		}
		for _, g := range groups {
			for i, c := range g.conds {
				var value any
				if c.Value != nil {
					b, err := json.Marshal(c.Value)
					if err != nil {
						return fmt.Errorf("%s 의 %s 조건 value: %w", p.ID, c.Field, err)
					}
					value = string(b)
				}
				if _, err := insCondition.Exec(p.ID, g.name, i, c.Field,
					string(c.Op), value, c.Label, c.Note); err != nil {
					return fmt.Errorf("%s 의 %s 조건을 넣지 못했습니다: %w",
						p.ID, c.Field, err)
				}
			}
		}
	}

	insRelation, err := tx.Prepare(`
INSERT INTO relations (from_id, to_id, type, reduce_pct, reason)
VALUES (?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer insRelation.Close()

	for _, r := range relations {
		if _, err := insRelation.Exec(r.From, r.To, string(r.Type),
			r.ReducePct, r.Reason); err != nil {
			return fmt.Errorf("관계 %s→%s 를 넣지 못했습니다: %w", r.From, r.To, err)
		}
	}

	return tx.Commit()
}

// jsonArray 는 nil 슬라이스도 "[]" 로 만든다. NULL 을 넣지 않는다.
func jsonArray(v []string) (string, error) {
	if v == nil {
		v = []string{}
	}
	b, err := json.Marshal(v)
	return string(b), err
}
