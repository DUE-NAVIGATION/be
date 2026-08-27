package ai

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Kind 는 걸러낸 민감정보의 종류다.
type Kind string

const (
	KindResidentID    Kind = "RESIDENT_ID"    // 주민등록번호 · 외국인등록번호
	KindCardNumber    Kind = "CARD_NUMBER"    // 카드번호
	KindAccountNumber Kind = "ACCOUNT_NUMBER" // 계좌번호
	KindPhone         Kind = "PHONE"          // 전화번호
	KindEmail         Kind = "EMAIL"          // 이메일
)

// 마스킹 문구. 지우지 않고 자리를 남기는 이유:
// "무엇이 있었는지" 는 알려주되 "값" 은 주지 않기 위해서다.
// 통째로 지우면 문장이 끊겨 구조화 품질이 떨어진다.
var label = map[Kind]string{
	KindResidentID:    "[주민등록번호]",
	KindCardNumber:    "[카드번호]",
	KindAccountNumber: "[계좌번호]",
	KindPhone:         "[전화번호]",
	KindEmail:         "[이메일]",
}

// Result 는 필터를 통과시킨 결과다.
//
// ★ 걸러낸 원문 값을 절대 담지 않는다.
// 여기에 값을 담으면 로그나 에러 메시지를 통해 그대로 새어 나간다.
// 종류와 건수만 남긴다.
type Result struct {
	Text  string
	Found map[Kind]int
}

func (r Result) HasSensitive() bool { return len(r.Found) > 0 }

// Summary 는 로그에 남길 수 있는 한 줄이다. 값이 들어 있지 않다.
func (r Result) Summary() string {
	if len(r.Found) == 0 {
		return "없음"
	}
	kinds := make([]string, 0, len(r.Found))
	for k := range r.Found {
		kinds = append(kinds, string(k))
	}
	sort.Strings(kinds) // 로그가 실행마다 달라지지 않게

	parts := make([]string, 0, len(kinds))
	for _, k := range kinds {
		parts = append(parts, k+" "+strconv.Itoa(r.Found[Kind(k)])+"건")
	}
	return strings.Join(parts, ", ")
}

// ── 패턴 ────────────────────────────────────────────────────
//
// ★ 오탐(정상 값을 가림)보다 미탐(민감정보를 놓침)이 훨씬 나쁘다.
// 다만 이 서비스는 숫자를 많이 다룬다 — 소득 800000, 보증금 20000000,
// 기준중위소득 6494738. 이런 값이 가려지면 판정 자체가 불가능해진다.
// 그래서 "숫자가 길다" 만으로 가리지 않고 모양을 함께 본다.

var (
	// 주민등록번호·외국인등록번호: 앞 6자리가 실제 날짜 모양이고 뒤가 7자리.
	// 성별자리는 1~8 (내국인 1~4, 외국인 5~8).
	reResidentID = regexp.MustCompile(
		`\b\d{2}(?:0[1-9]|1[0-2])(?:0[1-9]|[12]\d|3[01])[-\s]?[1-8]\d{6}\b`)

	// 카드번호 — 4자리씩 끊어 쓴 형태. 이 모양이면 Luhn 을 보지 않고 가린다.
	reCardGrouped = regexp.MustCompile(`\b\d{4}[-\s]\d{4}[-\s]\d{4}[-\s]\d{4}\b`)

	// 카드번호 — 붙여 쓴 13~19자리. 오탐을 줄이려고 Luhn 검사를 통과할 때만 가린다.
	reCardPlain = regexp.MustCompile(`\b\d{13,19}\b`)

	// 계좌번호 — 하이픈으로 끊은 형태 (110-123-456789, 1002-123-456789).
	// 총 자릿수 10~16 인 것만 인정한다.
	reAccountHyphen = regexp.MustCompile(`\b\d{2,6}-\d{2,7}-\d{2,7}(?:-\d{1,6})?\b`)

	// 계좌번호 — 은행 이름이 앞에 오는 형태. 이때는 붙여 써도 계좌로 본다.
	reAccountBank = regexp.MustCompile(
		`(?:국민|신한|우리|하나|농협|기업|씨티|SC제일|카카오뱅크|토스뱅크|케이뱅크|` +
			`새마을|신협|우체국|산업|수협|대구|부산|경남|광주|전북|제주)\s*(?:은행)?\s*` +
			`(?:계좌)?\s*[:：]?\s*\d[\d-]{8,19}`)

	// 휴대전화 — 01X 로 시작하는 10~11자리
	rePhoneMobile = regexp.MustCompile(`\b(?:\+?82[-\s]?)?01[016789][-\s]?\d{3,4}[-\s]?\d{4}\b`)

	// 유선전화 — 지역번호가 있고 구분자가 있는 형태.
	// 구분자를 요구한다. 안 그러면 8~10자리 금액이 전부 걸린다.
	rePhoneLocal = regexp.MustCompile(`\b0(?:2|[3-6][1-5])[-\s]\d{3,4}[-\s]\d{4}\b`)

	reEmail = regexp.MustCompile(`\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b`)
)

// Sanitize 는 LLM 으로 나가기 전에 민감정보를 가린다.
//
// ★ Phase 4 의 첫 번째 작업이다. 이 함수 없이 LLM 호출 코드를 먼저 만들지 마라.
// 한 번 나간 정보는 되돌릴 수 없다.
//
// 적용 순서가 결과를 바꾼다. 자세한 이유는 함수 본문의 주석에 적었다.
func Sanitize(s string) Result {
	found := map[Kind]int{}

	mask := func(re *regexp.Regexp, kind Kind, accept func(string) bool) {
		s = re.ReplaceAllStringFunc(s, func(m string) string {
			if accept != nil && !accept(m) {
				return m
			}
			found[kind]++
			return label[kind]
		})
	}

	// ★ 순서가 결과를 바꾼다. 아래 순서를 임의로 바꾸지 마라.
	//
	// 같은 글자가 여러 패턴에 걸린다.
	//   010-1234-5678  → 전화번호이자 "계좌번호 하이픈 형태"(11자리)
	//   1002123456789  → 계좌번호이자 "주민등록번호 모양"(10/02/12 + 3456789)
	// 더 강한 단서를 가진 쪽을 먼저 처리한다.

	// 1) 은행 이름이 앞에 붙은 계좌 — 문맥이 가장 확실하다.
	//    은행명은 남기고 숫자만 가린다. 문장이 끊기면 구조화 품질이 떨어진다
	s = reAccountBank.ReplaceAllStringFunc(s, func(m string) string {
		i := strings.IndexFunc(m, func(r rune) bool { return r >= '0' && r <= '9' })
		if i < 0 {
			return m
		}
		found[KindAccountNumber]++
		return m[:i] + label[KindAccountNumber]
	})

	// 2) 주민등록번호 — 날짜 모양 + 성별자리라 단서가 강하다
	mask(reResidentID, KindResidentID, nil)

	// 3) 카드번호
	mask(reCardGrouped, KindCardNumber, nil)
	mask(reCardPlain, KindCardNumber, func(m string) bool {
		// 붙여 쓴 긴 숫자는 카드일 수도, 그냥 큰 수일 수도 있다.
		// Luhn 을 통과할 때만 카드로 본다 — 오탐을 크게 줄인다
		return luhnValid(m)
	})

	// 4) 전화번호 — ★ 계좌(하이픈)보다 먼저. 010-1234-5678 이 계좌로 잡힌다
	mask(rePhoneMobile, KindPhone, nil)
	mask(rePhoneLocal, KindPhone, nil)

	// 5) 계좌번호 — 하이픈 형태. 여기까지 남은 것만 계좌로 본다
	mask(reAccountHyphen, KindAccountNumber, func(m string) bool {
		n := countDigits(m)
		return n >= 10 && n <= 16
	})

	// 6) 이메일
	mask(reEmail, KindEmail, nil)

	if len(found) == 0 {
		found = nil
	}
	return Result{Text: s, Found: found}
}

// luhnValid 는 카드번호 검사식(Luhn)을 확인한다.
//
// 오른쪽부터 한 자리 걸러 2배 하고, 10 이상이면 9를 뺀다.
// 전부 더해 10으로 나누어떨어지면 유효한 번호 형식이다.
func luhnValid(s string) bool {
	sum, alt := 0, false
	for i := len(s) - 1; i >= 0; i-- {
		c := s[i]
		if c < '0' || c > '9' {
			continue
		}
		d := int(c - '0')
		if alt {
			if d *= 2; d > 9 {
				d -= 9
			}
		}
		sum += d
		alt = !alt
	}
	return sum%10 == 0
}

func countDigits(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			n++
		}
	}
	return n
}
