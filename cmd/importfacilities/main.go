// Command importfacilities 는 공공데이터 CSV 를 시설 JSON 으로 옮긴다.
//
//	go run ./cmd/importfacilities -in 원본.csv -out data/facilities/seoul-child-center.json \
//	  -type CHILD_CENTER -sido 서울특별시
//
// 대상은 전국사회복지시설표준데이터(사회보장정보원) 형식이지만, 지자체마다
// 컬럼명이 조금씩 다르므로 **헤더 이름을 여러 후보로 찾는다.** 못 찾은 컬럼은
// 비워 두고 끝에 알려준다 — 조용히 빈 값으로 채우면 아무도 모른다.
//
// ★ 연락처를 지어내지 않는다. 전화·홈페이지가 모두 없는 행은 건너뛴다.
// 연락할 수 없는 시설은 목록만 늘리고 이용자를 막다른 길로 보낸다.
//
// ★ 사람이 채워야 하는 것 (이 도구가 만들 수 없다)
//   - summary   "이 시설이 나에게 뭘 해주나" 를 이용자 말로
//   - services  제공 서비스
//   - fee       이용료
//   - documents 방문 시 준비물
//   - eligibility 이용 대상 조건 (비워 두면 "관할 주민 누구나" 라는 뜻)
package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/DUE-NAVIGATION/be/internal/loader"
	"github.com/DUE-NAVIGATION/be/internal/model"
)

func main() {
	in := flag.String("in", "", "원본 CSV 경로 (필수)")
	out := flag.String("out", "", "만들 JSON 경로 (필수)")
	typ := flag.String("type", "", "시설 종류. 비우면 시설종류명 컬럼에서 유추한다")
	sido := flag.String("sido", "", "시도명. 주소에서 못 읽었을 때 쓴다")
	sector := flag.String("sector", "", "PUBLIC / PRIVATE. 비우면 설립주체 컬럼에서 유추한다")
	idPrefix := flag.String("prefix", "", "id 앞에 붙일 말 (기본: sido 로마자 없이 'fac')")
	flag.Parse()

	if *in == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "사용법: -in 원본.csv -out data/facilities/파일.json [-type CHILD_CENTER] [-sido 서울특별시]")
		os.Exit(2)
	}
	if *typ != "" && !model.FacilityTypeIsKnown(model.FacilityType(*typ)) {
		fmt.Fprintf(os.Stderr, "모르는 시설 종류입니다: %q\n쓸 수 있는 값: %v\n",
			*typ, model.KnownFacilityTypes())
		os.Exit(2)
	}
	if *sector != "" && !model.SectorIsKnown(model.Sector(*sector)) {
		fmt.Fprintf(os.Stderr, "sector 는 PUBLIC 또는 PRIVATE 입니다: %q\n", *sector)
		os.Exit(2)
	}
	if *idPrefix == "" {
		*idPrefix = "fac"
	}

	rows, header, err := readCSV(*in)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✖ %v\n", err)
		os.Exit(1)
	}

	col := newColumns(header)
	facilities, skipped := convert(rows, col, model.FacilityType(*typ), model.Sector(*sector), *sido, *idPrefix)

	// 검증까지 여기서 돌린다. 깨진 채로 파일을 만들면 서버가 조용히 건너뛴다
	valid := make([]model.Facility, 0, len(facilities))
	invalid := 0
	for _, f := range facilities {
		if errs := loader.ValidateFacility(f); len(errs) > 0 {
			invalid++
			fmt.Fprintf(os.Stderr, "✖ %s (%s)\n", f.Name, f.ID)
			for _, e := range errs {
				fmt.Fprintf(os.Stderr, "    - %s\n", e)
			}
			continue
		}
		valid = append(valid, f)
	}

	sort.SliceStable(valid, func(i, j int) bool { return valid[i].ID < valid[j].ID })

	body, err := json.MarshalIndent(valid, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "✖ JSON 을 만들지 못했습니다: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*out, append(body, '\n'), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "✖ 파일을 쓰지 못했습니다: %v\n", err)
		os.Exit(1)
	}

	// ── 보고 ────────────────────────────────────────────────
	fmt.Printf("\n완료  %s\n", *out)
	fmt.Printf("      %d건 저장", len(valid))
	if len(skipped) > 0 {
		fmt.Printf(" · %d건 건너뜀", len(skipped))
	}
	if invalid > 0 {
		fmt.Printf(" · %d건 검증 실패", invalid)
	}
	fmt.Println()

	if missing := col.missing(); len(missing) > 0 {
		fmt.Printf("\n⚠ 찾지 못한 컬럼 (해당 값은 비어 있습니다)\n")
		for _, m := range missing {
			fmt.Printf("    - %s\n", m)
		}
		fmt.Println("  원본 CSV 의 헤더 이름을 확인하고 columns 후보에 추가하세요.")
	}

	if len(skipped) > 0 {
		fmt.Printf("\n건너뛴 행\n")
		for _, s := range skipped[:min(len(skipped), 10)] {
			fmt.Printf("    - %s\n", s)
		}
		if len(skipped) > 10 {
			fmt.Printf("    … 외 %d건\n", len(skipped)-10)
		}
	}

	fmt.Println("\n★ 다음은 사람이 채워야 합니다 (이 도구가 만들 수 없습니다)")
	fmt.Println("    summary    이 시설이 나에게 뭘 해주는지, 이용자 말로")
	fmt.Println("    services   제공 서비스")
	fmt.Println("    fee        이용료")
	fmt.Println("    documents  방문 시 준비물")
	fmt.Println("    eligibility 이용 대상 조건 (비우면 '관할 주민 누구나')")
	fmt.Println("\n    작성 가이드: data/facilities/README.md")
}

func readCSV(path string) ([][]string, []string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("CSV 를 열지 못했습니다: %w", err)
	}
	defer f.Close()

	// 공공데이터 CSV 는 EUC-KR 인 경우가 많다. UTF-8 로 변환해서 넣으라고 안내한다
	// (표준 라이브러리만 쓰기 위해 인코딩 변환은 하지 않는다)
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1 // 행마다 열 수가 다른 파일이 실제로 있다
	r.LazyQuotes = true

	header, err := r.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("헤더를 읽지 못했습니다: %w", err)
	}
	for i := range header {
		// BOM 과 공백 제거
		header[i] = strings.TrimSpace(strings.TrimPrefix(header[i], "\ufeff"))
	}

	var rows [][]string
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("본문을 읽지 못했습니다: %w", err)
		}
		rows = append(rows, rec)
	}
	return rows, header, nil
}

// columns 는 헤더 이름을 열 번호로 옮긴다.
//
// 지자체마다 컬럼명이 달라서 후보를 여럿 둔다. 예: "시설명" / "기관명" / "센터명"
type columns struct {
	idx      map[string]int
	notFound []string
}

var candidates = map[string][]string{
	"name":      {"시설명", "기관명", "센터명", "업체명", "사업장명"},
	"type":      {"시설종류명", "시설종류", "시설구분", "시설유형"},
	"road":      {"소재지도로명주소", "도로명주소", "주소", "소재지"},
	"lot":       {"소재지지번주소", "지번주소"},
	"phone":     {"전화번호", "시설전화번호", "연락처", "대표전화"},
	"fax":       {"팩스번호", "팩스"},
	"website":   {"홈페이지주소", "홈페이지", "url", "URL"},
	"lat":       {"위도", "latitude"},
	"lng":       {"경도", "longitude"},
	"capacity":  {"입소정원수", "입소정원", "정원"},
	"authority": {"관할행정기관", "관리기관명", "관할기관"},
	"revised":   {"데이터기준일자", "기준일자", "데이터기준일"},
	"sector":    {"설립주체", "설치주체", "운영주체", "설립주체구분", "운영주체구분"},
}

func newColumns(header []string) *columns {
	pos := map[string]int{}
	for i, h := range header {
		pos[strings.TrimSpace(h)] = i
	}

	c := &columns{idx: map[string]int{}}
	keys := make([]string, 0, len(candidates))
	for k := range candidates {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		found := false
		for _, cand := range candidates[key] {
			if i, ok := pos[cand]; ok {
				c.idx[key] = i
				found = true
				break
			}
		}
		if !found {
			c.notFound = append(c.notFound, fmt.Sprintf("%s (후보: %s)",
				key, strings.Join(candidates[key], " · ")))
		}
	}
	return c
}

func (c *columns) get(row []string, key string) string {
	i, ok := c.idx[key]
	if !ok || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

func (c *columns) missing() []string { return c.notFound }

func convert(
	rows [][]string, col *columns,
	forced model.FacilityType, forcedSector model.Sector, sidoFlag, prefix string,
) ([]model.Facility, []string) {
	var (
		out     []model.Facility
		skipped []string
		seen    = map[string]int{}
	)

	for n, row := range rows {
		name := col.get(row, "name")
		if name == "" {
			skipped = append(skipped, fmt.Sprintf("%d행: 시설명이 비었습니다", n+2))
			continue
		}

		phone := normalizePhone(col.get(row, "phone"))
		website := normalizeURL(col.get(row, "website"))

		// ★ 연락할 수 없는 시설은 넣지 않는다
		if phone == "" && website == "" {
			skipped = append(skipped,
				fmt.Sprintf("%d행 %s: 전화·홈페이지가 모두 없습니다", n+2, name))
			continue
		}

		road := col.get(row, "road")
		lot := col.get(row, "lot")
		sido, sigungu := splitRegion(road, lot, sidoFlag)
		if sigungu == "" {
			skipped = append(skipped,
				fmt.Sprintf("%d행 %s: 주소에서 시군구를 읽지 못했습니다 (%q)", n+2, name, road))
			continue
		}

		ftype := forced
		if ftype == "" {
			ftype = guessType(col.get(row, "type"))
		}

		// 모르면 비워 둔다 — 검증에서 걸려 사람이 확인하게 된다
		fsector := forcedSector
		if fsector == "" {
			fsector = guessSector(col.get(row, "sector"))
		}

		revised := col.get(row, "revised")
		if revised == "" {
			// 기준일자가 없으면 검증에서 걸린다. 사람이 채우도록 비워 두지 않고
			// 명시적으로 표시해 눈에 띄게 한다
			revised = "UNKNOWN-원본에-기준일자-없음"
		}

		id := makeID(prefix, sigungu, name)
		seen[id]++
		if seen[id] > 1 {
			id = fmt.Sprintf("%s-%d", id, seen[id])
		}

		f := model.Facility{
			ID:     id,
			Name:   name,
			Type:   ftype,
			Sector: fsector,
			Coverage: model.Coverage{
				Scope: model.CoverageSigungu, Sido: sido, Sigungu: sigungu,
			},
			Location: model.Location{
				Sido: sido, Sigungu: sigungu,
				RoadAddress: road, LotAddress: lot,
				Lat: parseFloat(col.get(row, "lat")),
				Lng: parseFloat(col.get(row, "lng")),
			},
			Contact: model.Contact{
				Phone: phone, Fax: col.get(row, "fax"), Website: website,
			},
			Authority: col.get(row, "authority"),
			Source: model.Source{
				RevisedAt: revised,
				Note:      "공공데이터 CSV 에서 자동 변환. summary·services·fee·documents·eligibility 는 사람이 채워야 한다",
			},
		}
		out = append(out, f)
	}
	return out, skipped
}

// splitRegion 은 주소에서 시도·시군구를 뽑는다.
//
// 도로명주소는 "서울특별시 관악구 관악로 145" 처럼 시작하므로 앞 두 토큰이면 된다.
// 세종특별자치시처럼 시군구가 없는 곳이 있어 시군구가 비어도 오류로 보지 않는다.
func splitRegion(road, lot, fallbackSido string) (string, string) {
	addr := road
	if addr == "" {
		addr = lot
	}
	fields := strings.Fields(addr)

	sido, sigungu := "", ""
	if len(fields) > 0 && isSido(fields[0]) {
		sido = fields[0]
		if len(fields) > 1 && isSigungu(fields[1]) {
			sigungu = fields[1]
		}
	} else if fallbackSido != "" {
		sido = fallbackSido
		if len(fields) > 0 && isSigungu(fields[0]) {
			sigungu = fields[0]
		}
	}
	return sido, sigungu
}

var sidoSuffix = regexp.MustCompile(`(특별시|광역시|특별자치시|특별자치도|도)$`)

func isSido(s string) bool { return sidoSuffix.MatchString(s) }

func isSigungu(s string) bool {
	return strings.HasSuffix(s, "구") || strings.HasSuffix(s, "시") ||
		strings.HasSuffix(s, "군")
}

// guessType 은 원본의 시설종류명을 우리 종류로 옮긴다.
// 못 알아보면 OTHER 다 — 틀린 분류로 넣는 것보다 낫다.
func guessType(raw string) model.FacilityType {
	s := strings.ReplaceAll(raw, " ", "")
	switch {
	case strings.Contains(s, "지역아동센터"):
		return model.FacilityChildCenter
	case strings.Contains(s, "종합사회복지관"), strings.Contains(s, "사회복지관"):
		return model.FacilityCommunityWelfare
	case strings.Contains(s, "노인복지관"), strings.Contains(s, "경로당"):
		return model.FacilityElderly
	case strings.Contains(s, "장애인"):
		return model.FacilityDisability
	case strings.Contains(s, "정신건강"):
		return model.FacilityMentalHealth
	case strings.Contains(s, "자활"):
		return model.FacilitySelfSufficiency
	case strings.Contains(s, "쉼터"), strings.Contains(s, "보호시설"):
		return model.FacilityShelter
	case strings.Contains(s, "한부모"), strings.Contains(s, "모자"), strings.Contains(s, "부자"):
		return model.FacilitySingleParent
	case strings.Contains(s, "가족센터"), strings.Contains(s, "건강가정"):
		return model.FacilityFamilyCenter
	case strings.Contains(s, "고용복지"):
		return model.FacilityJobCenter
	case strings.Contains(s, "행정복지센터"), strings.Contains(s, "주민센터"):
		return model.FacilityCommunityCenter
	}
	return model.FacilityOther
}

// guessSector 는 설립주체 문자열을 공공/민간으로 옮긴다.
//
// 공공 표지를 먼저 본다. "지자체(법인위탁)" 처럼 둘 다 적힌 값은 설치 주체가
// 지자체이므로 PUBLIC 이다. 어느 쪽인지 모르면 빈 값 — 추측해서 넣지 않는다.
func guessSector(raw string) model.Sector {
	s := strings.ReplaceAll(raw, " ", "")
	if s == "" {
		return ""
	}
	for _, k := range []string{"국가", "국립", "지자체", "지방자치단체", "시립", "구립", "군립", "도립", "공립", "공공"} {
		if strings.Contains(s, k) {
			return model.SectorPublic
		}
	}
	for _, k := range []string{"법인", "개인", "민간", "단체", "재단", "종교"} {
		if strings.Contains(s, k) {
			return model.SectorPrivate
		}
	}
	return ""
}

// makeID 는 사람이 읽을 수 있는 id 를 만든다.
// 한글을 그대로 쓰면 URL·파일명에서 다루기 번거로우므로 이름은 해시 대신
// 순번으로 구분하고, 시군구는 그대로 둔다 (id 는 내부 식별자다).
var idUnsafe = regexp.MustCompile(`[^0-9A-Za-z가-힣]+`)

func makeID(prefix, sigungu, name string) string {
	clean := func(s string) string {
		return strings.Trim(idUnsafe.ReplaceAllString(s, "-"), "-")
	}
	return strings.ToLower(prefix) + "-" + clean(sigungu) + "-" + clean(name)
}

// normalizePhone 은 전화번호에서 군더더기를 뗀다.
// ★ 없는 번호를 만들어내지 않는다. 숫자가 없으면 빈 문자열이다.
func normalizePhone(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// "02-880-1234 (내선 3)" 같은 값에서 앞의 번호만 쓴다
	if i := strings.IndexAny(s, "(,/"); i > 0 {
		s = strings.TrimSpace(s[:i])
	}
	hasDigit := strings.ContainsFunc(s, func(r rune) bool { return r >= '0' && r <= '9' })
	if !hasDigit {
		return ""
	}
	return s
}

func normalizeURL(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return ""
	}
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		s = "https://" + s
	}
	return s
}

func parseFloat(s string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return v
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
