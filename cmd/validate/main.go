// Command validate 는 제도 JSON 을 검사한다.
//
//	go run ./cmd/validate
//
// 제도 데이터 작성이 이 프로젝트 최대 병목이고, 비개발자 팀원도 함께 채운다.
// 그래서 이 도구의 목표는 "틀린 곳을 사람 말로, 한 번에 다 알려주기" 다.
// 문제가 하나라도 있으면 exit 1 로 끝난다 — 나중에 CI 에 그대로 걸 수 있다.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/DUE-NAVIGATION/be/internal/loader"
)

func main() {
	dir := flag.String("dir", filepath.Join("data", "programs"), "제도 JSON 디렉터리")
	flag.Parse()

	store, err := loader.New(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✖ %v\n", err)
		os.Exit(1)
	}

	problems := store.Problems()
	count := store.Count()

	// 파일별로 묶어서 보여준다. 한 파일을 열어놓고 다 고칠 수 있게
	byFile := map[string][]string{}
	var order []string
	for _, p := range problems {
		if _, seen := byFile[p.File]; !seen {
			order = append(order, p.File)
		}
		byFile[p.File] = append(byFile[p.File], p.Reason)
	}

	for _, file := range order {
		fmt.Printf("\n✖ %s\n", file)
		for _, reason := range byFile[file] {
			fmt.Printf("    - %s\n", reason)
		}
	}

	fmt.Printf("\n─────────────────────────────────────────\n")
	fmt.Printf("정상 %d건", count)
	if len(problems) > 0 {
		fmt.Printf(" · 문제 %d건 (%d개 파일)", len(problems), len(order))
	}
	fmt.Println()

	if len(problems) > 0 {
		fmt.Println("\n작성 가이드: data/programs/README.md")
		os.Exit(1)
	}

	if count == 0 {
		fmt.Println("\n⚠ 제도가 하나도 없습니다. data/programs/ 에 JSON 을 추가하세요.")
		fmt.Println("  작성 가이드: data/programs/README.md")
		os.Exit(1)
	}

	fmt.Println("\n✔ 전부 통과했습니다.")
}
