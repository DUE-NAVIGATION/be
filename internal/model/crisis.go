package model

// CrisisSignal 은 지금 안전이 걱정되는 상황의 신호다.
//
// ★ 이 값으로 판정하지 않는다. 받을 수 있는 것은 그대로 두고, 지금 바로
// 이야기할 수 있는 곳(24시간 상담 전화)을 화면 맨 위로 올리는 데만 쓴다.
//
// ★ 어떤 기관이 어떤 신호에 응답하는지는 시설 JSON 의 crisis 에 적는다.
// 전화번호를 코드에 박지 않는다 (시설 데이터는 코드에 하드코딩하지 않는다).
//
// 가장 민감한 입력이다. 저장하지 않고, 로그에 남기지 않고, 기관에 보내는
// 문의 문구에도 넣지 않는다.
type CrisisSignal string

const (
	// 스스로를 해치고 싶은 마음 · 죽고 싶은 마음
	CrisisSelfHarm CrisisSignal = "SELF_HARM"
	// 누군가에게 폭력·위협을 당하고 있다
	CrisisViolence CrisisSignal = "VIOLENCE"
)

func KnownCrisisSignals() []CrisisSignal {
	return []CrisisSignal{CrisisSelfHarm, CrisisViolence}
}

func CrisisIsKnown(c CrisisSignal) bool {
	for _, k := range KnownCrisisSignals() {
		if k == c {
			return true
		}
	}
	return false
}

// RespondsTo 는 기관이 응답하는 신호(offers) 중 사용자의 신호가 하나라도 있는지 본다.
func RespondsTo(offers, signals []CrisisSignal) bool {
	for _, o := range offers {
		for _, s := range signals {
			if o == s {
				return true
			}
		}
	}
	return false
}
