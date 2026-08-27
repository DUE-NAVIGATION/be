package ai

import (
	"strings"
	"testing"
)

// 반드시 걸러야 하는 것들.
// ★ 하나라도 새면 되돌릴 수 없다. 미탐이 오탐보다 훨씬 나쁘다.
func TestSanitizeMasksSensitive(t *testing.T) {
	tests := []struct {
		name string
		in   string
		kind Kind
		// 원문에서 사라져야 하는 조각
		gone string
	}{
		{
			name: "주민등록번호 — 하이픈 있음",
			in:   "제 주민번호는 900101-1234567 입니다",
			kind: KindResidentID,
			gone: "900101-1234567",
		},
		{
			name: "주민등록번호 — 하이픈 없음",
			in:   "9001011234567 로 조회해 주세요",
			kind: KindResidentID,
			gone: "9001011234567",
		},
		{
			name: "외국인등록번호 — 성별자리 5~8",
			in:   "등록번호 950320-5678901",
			kind: KindResidentID,
			gone: "950320-5678901",
		},
		{
			name: "카드번호 — 4자리씩 끊어 쓴 형태",
			in:   "카드 5555-4444-3333-2222 로 결제했어요",
			kind: KindCardNumber,
			gone: "5555-4444-3333-2222",
		},
		{
			name: "카드번호 — 붙여 쓴 형태 (Luhn 통과)",
			in:   "4111111111111111 이 제 카드입니다",
			kind: KindCardNumber,
			gone: "4111111111111111",
		},
		{
			name: "계좌번호 — 하이픈 형태",
			in:   "110-123-456789 으로 보내주세요",
			kind: KindAccountNumber,
			gone: "110-123-456789",
		},
		{
			name: "계좌번호 — 은행 이름이 앞에 옴",
			in:   "국민은행 1002123456789 입니다",
			kind: KindAccountNumber,
			gone: "1002123456789",
		},
		{
			name: "휴대전화",
			in:   "연락처는 010-1234-5678 이에요",
			kind: KindPhone,
			gone: "010-1234-5678",
		},
		{
			name: "휴대전화 — 붙여 쓴 형태",
			in:   "01012345678 로 연락 주세요",
			kind: KindPhone,
			gone: "01012345678",
		},
		{
			name: "유선전화",
			in:   "주민센터 02-123-4567 로 문의했습니다",
			kind: KindPhone,
			gone: "02-123-4567",
		},
		{
			name: "이메일",
			in:   "메일은 hong.gildong@example.com 입니다",
			kind: KindEmail,
			gone: "hong.gildong@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Sanitize(tt.in)

			if strings.Contains(got.Text, tt.gone) {
				t.Errorf("★ 민감정보가 그대로 남았다: %q", got.Text)
			}
			if got.Found[tt.kind] == 0 {
				t.Errorf("%s 로 잡히지 않았다. 실제: %v / 결과: %q",
					tt.kind, got.Found, got.Text)
			}
			if !got.HasSensitive() {
				t.Error("HasSensitive 가 false 다")
			}
		})
	}
}

// 오탐이면 안 되는 것들.
// 이 서비스는 숫자를 많이 다룬다. 정상 값이 가려지면 판정이 불가능해진다.
func TestSanitizeKeepsNormalNumbers(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{"월 소득", "월 소득은 800000원 입니다"},
		{"보증금", "보증금 20000000원에 월세 400000원 살아요"},
		{"기준중위소득 4인", "4인 가구 기준중위소득은 6494738원입니다"},
		{"쉼표 있는 금액", "연 2,400,000원을 받을 수 있습니다"},
		{"나이·가구원수", "저는 33살이고 가구원은 2명입니다"},
		{"연도와 날짜", "2026년 1월 1일부터 시행, 개정일 2026-01-01"},
		{"제도 금액", "월 230000원씩 12개월"},
		{"자녀 나이", "아이는 7살이고 둘째는 12살이에요"},
		{"큰 재산 금액", "재산이 500000000원 정도 됩니다"},
		{"퍼센트", "중위소득 대비 45.2% 수준입니다"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Sanitize(tt.in)
			if got.Text != tt.in {
				t.Errorf("정상 문장이 가려졌다\n 입력: %q\n 결과: %q\n 잡힌 것: %v",
					tt.in, got.Text, got.Found)
			}
			if got.HasSensitive() {
				t.Errorf("민감정보가 없는데 %v 로 잡혔다", got.Found)
			}
		})
	}
}

// 데모 시나리오 문장이 온전히 통과해야 한다. 여기서 뭔가 가려지면 발표가 깨진다.
func TestSanitizeDemoSentence(t *testing.T) {
	in := "혼자 애 키우는데 일이 끊겼어요. 아이는 7살이고 월세 살아요. 월 소득은 80만원 정도예요."

	got := Sanitize(in)
	if got.Text != in {
		t.Errorf("데모 문장이 변형됐다\n 입력: %q\n 결과: %q", in, got.Text)
	}
	if got.HasSensitive() {
		t.Errorf("데모 문장에서 %v 가 잡혔다", got.Found)
	}
}

// 한 문장에 여러 종류가 섞여 있어도 전부 걸러야 한다.
func TestSanitizeMultiple(t *testing.T) {
	in := "저는 900101-1234567 이고 010-1234-5678, me@test.com, 신한은행 110-123-456789 입니다"

	got := Sanitize(in)

	for _, kind := range []Kind{KindResidentID, KindPhone, KindEmail, KindAccountNumber} {
		if got.Found[kind] == 0 {
			t.Errorf("%s 가 잡히지 않았다. 결과: %q / %v", kind, got.Text, got.Found)
		}
	}
	for _, gone := range []string{"900101", "1234567", "010-1234-5678", "me@test.com", "456789"} {
		if strings.Contains(got.Text, gone) {
			t.Errorf("★ %q 가 남았다: %q", gone, got.Text)
		}
	}
}

// ★ Result 는 걸러낸 값을 절대 담지 않는다.
// 여기에 값이 담기면 로그·에러 메시지를 통해 그대로 새어 나간다.
func TestResultNeverCarriesTheValue(t *testing.T) {
	const secret = "900101-1234567"
	got := Sanitize("주민번호 " + secret)

	if strings.Contains(got.Summary(), secret) {
		t.Errorf("★ Summary 에 원문이 들어 있다: %q", got.Summary())
	}
	for k := range got.Found {
		if strings.Contains(string(k), secret) {
			t.Error("★ Found 의 키에 원문이 들어 있다")
		}
	}
	if got.Summary() == "" {
		t.Error("Summary 가 비었다")
	}
}

func TestSummary(t *testing.T) {
	if got := Sanitize("아무것도 없는 문장입니다").Summary(); got != "없음" {
		t.Errorf("Summary = %q, want 없음", got)
	}

	got := Sanitize("010-1234-5678 과 010-9999-8888 로 연락 주세요").Summary()
	if got != "PHONE 2건" {
		t.Errorf("Summary = %q, want PHONE 2건", got)
	}
}

func TestLuhn(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"4111111111111111", true},  // 널리 쓰이는 테스트 번호
		{"5555555555554444", true},  // 테스트 번호
		{"1234567890123456", false}, // 아무 숫자
		{"6494738649473864", false}, // 기준중위소득을 이어붙인 값
	}
	for _, tt := range tests {
		if got := luhnValid(tt.in); got != tt.want {
			t.Errorf("luhnValid(%s) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

// 빈 문자열·공백에서 죽지 않아야 한다.
func TestSanitizeEdgeCases(t *testing.T) {
	for _, in := range []string{"", " ", "\n", "한글만 있는 문장", "1", "----"} {
		got := Sanitize(in)
		if got.Text != in {
			t.Errorf("입력 %q 가 %q 로 바뀌었다", in, got.Text)
		}
	}
}
