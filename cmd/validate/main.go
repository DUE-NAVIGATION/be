// Command validate 는 제도·시설 JSON 을 검사한다.
//
//	go run ./cmd/validate
//	go run ./cmd/validate -data data
//
// 데이터 작성이 이 프로젝트 최대 병목이고, 비개발자 팀원도 함께 채운다.
// 그래서 이 도구의 목표는 "틀린 곳을 사람 말로, 한 번에 다 알려주기" 다.
// 문제가 하나라도 있으면 exit 1 로 끝난다 — Docker 빌드와 CI 에 그대로 걸린다.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/DUE-NAVIGATION/be/internal/loader"
)

func main() {
	dataDir := flag.String("data", "data", "데이터 디렉터리 (programs · facilities 의 부모)")
	dir := flag.String("dir", "", "제도 JSON 디렉터리 (지정하면 -data 를 무시하고 제도만 검사)")
	flag.Parse()

	programDir := filepath.Join(*dataDir, "programs")
	onlyPrograms := *dir != ""
	if onlyPrograms {
		programDir = *dir
	}

	// ── 제도 ────────────────────────────────────────────────
	programs, err := loader.New(programDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✖ %v\n", err)
		os.Exit(1)
	}
	programProblems := report("제도", programs.Problems())

	// ── 시설 ────────────────────────────────────────────────
	// 디렉터리가 없으면 0건으로 조용히 넘어간다. 시설 데이터를 아직 안 넣은
	// 상태에서 이 도구가 실패하면 제도 작성까지 막힌다.
	facilityCount, facilityProblems := 0, 0
	if !onlyPrograms {
		facilities, err := loader.NewFacilities(filepath.Join(*dataDir, "facilities"))
		if err != nil {
			fmt.Fprintf(os.Stderr, "✖ %v\n", err)
			os.Exit(1)
		}
		facilityCount = facilities.Count()
		facilityProblems = report("시설", facilities.Problems())
	}

	// ── 요약 ────────────────────────────────────────────────
	fmt.Printf("\n─────────────────────────────────────────\n")
	fmt.Printf("제도 %d건", programs.Count())
	if !onlyPrograms {
		fmt.Printf(" · 시설 %d건", facilityCount)
	}
	if total := programProblems + facilityProblems; total > 0 {
		fmt.Printf(" · 문제 %d건", total)
	}
	fmt.Println()

	if programProblems+facilityProblems > 0 {
		fmt.Println("\n작성 가이드")
		if programProblems > 0 {
			fmt.Println("  제도: data/programs/README.md")
		}
		if facilityProblems > 0 {
			fmt.Println("  시설: data/facilities/README.md")
		}
		os.Exit(1)
	}

	if programs.Count() == 0 {
		fmt.Println("\n⚠ 제도가 하나도 없습니다. data/programs/ 에 JSON 을 추가하세요.")
		fmt.Println("  작성 가이드: data/programs/README.md")
		os.Exit(1)
	}

	// ★ 시설 0건은 실패로 보지 않는다. 제도 판정은 시설 없이도 완전히 동작한다.
	// 다만 이 서비스의 결말이 빠진 상태이므로 눈에 띄게 알린다.
	if !onlyPrograms && facilityCount == 0 {
		fmt.Println("\n⚠ 시설이 하나도 없습니다. 판정은 되지만 \"어디로 연락하면 되는지\" 를")
		fmt.Println("  보여줄 수 없습니다. 작성 가이드: data/facilities/README.md")
	}

	fmt.Println("\n✔ 전부 통과했습니다.")
}

// report 는 문제를 파일별로 묶어 보여주고 건수를 돌려준다.
// 한 파일을 열어놓고 다 고칠 수 있게 묶는다.
func report(what string, problems []loader.Problem) int {
	byFile := map[string][]string{}
	var order []string
	for _, p := range problems {
		if _, seen := byFile[p.File]; !seen {
			order = append(order, p.File)
		}
		byFile[p.File] = append(byFile[p.File], p.Reason)
	}

	for _, file := range order {
		fmt.Printf("\n✖ [%s] %s\n", what, file)
		for _, reason := range byFile[file] {
			fmt.Printf("    - %s\n", reason)
		}
	}
	return len(problems)
}
