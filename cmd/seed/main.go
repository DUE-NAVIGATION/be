// cmd/seed 는 data/programs/*.json 을 SQLite 로 옮겨 담는다.
//
//	go run ./cmd/seed                       # data → data/due.db
//	go run ./cmd/seed -data data -db /x.db
//
// ★ 방향은 언제나 JSON → DB 한 쪽이다. DB 를 고치고 JSON 으로 되돌리는 길은
// 만들지 않는다. 제도 데이터는 git 에서 리뷰되어야 하기 때문이다 —
// 누가 언제 어떤 근거로 금액을 바꿨는지가 남지 않으면 심사에서 답할 말이 없다.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/DUE-NAVIGATION/be/internal/loader"
	"github.com/DUE-NAVIGATION/be/internal/store"
)

func main() {
	dataDir := flag.String("data", "data", "제도 JSON 이 있는 디렉터리")
	dbPath := flag.String("db", "", "만들 DB 파일 (기본: <data>/due.db)")
	flag.Parse()

	if *dbPath == "" {
		*dbPath = filepath.Join(*dataDir, "due.db")
	}

	// 1) JSON 을 읽는다. 여기서 걸러진 것은 DB 에도 들어가지 않는다
	src, err := loader.New(filepath.Join(*dataDir, "programs"))
	if err != nil {
		fail("제도 디렉터리를 읽지 못했습니다: %v", err)
	}

	problems := src.Problems()
	for _, p := range problems {
		fmt.Fprintf(os.Stderr, "  건너뜀  %s — %s\n", p.File, p.Reason)
	}

	programs := src.Programs()
	if len(programs) == 0 {
		// 빈 DB 를 만들어 두면 서버가 조용히 "제도 0건" 으로 뜬다. 그게 더 나쁘다
		fail("읽힌 제도가 없습니다. data/programs/README.md 를 볼 것")
	}

	// 2) 기존 파일을 지우고 새로 만든다.
	//
	// ★ 통째로 다시 쓸 것이므로 남겨둘 이유가 없고, 무엇보다 journal_mode 같은
	//   파일 영구 설정이 이전 스키마에서 그대로 따라오는 것을 막는다.
	//   (WAL 로 만들어진 DB 는 읽기 전용으로 열 수 없어 배포에서 서버가 죽는다)
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if err := os.Remove(*dbPath + suffix); err != nil && !os.IsNotExist(err) {
			fail("기존 DB 를 지우지 못했습니다: %v", err)
		}
	}

	db, err := store.Open(*dbPath)
	if err != nil {
		fail("%v", err)
	}
	defer db.Close()

	relations := src.Relations()
	if err := store.Seed(db, programs, relations); err != nil {
		fail("%v", err)
	}

	fmt.Printf("완료  %s\n", *dbPath)
	fmt.Printf("      제도 %d건 · 관계 %d건", len(programs), len(relations))
	if len(problems) > 0 {
		fmt.Printf(" · 건너뜀 %d건", len(problems))
	}
	fmt.Println()

	if len(relations) == 0 {
		fmt.Println("      ※ 중복수급 관계가 0건입니다 (data/relations.json)")
	}
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "실패: "+format+"\n", args...)
	os.Exit(1)
}
