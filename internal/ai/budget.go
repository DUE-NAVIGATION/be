package ai

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// ErrBudgetExceeded 는 하루 호출 상한을 넘었다는 뜻이다.
//
// ★ ErrNoAPIKey 를 감싼다. 핸들러가 이미 "AI 를 쓸 수 없음(503)" 으로 답하고
// 프론트는 수동 입력으로 넘어가도록 되어 있어서, 이용자에게는 키가 없을 때와
// 똑같이 보이는 것이 맞다 — 어느 쪽이든 판정은 그대로 동작한다.
var ErrBudgetExceeded = fmt.Errorf("하루 AI 호출 상한을 넘었습니다: %w", ErrNoAPIKey)

// budget 은 "오늘 몇 번 불렀나" 를 센다.
//
// ★ 공개 주소에 붙은 AI 엔드포인트는 남이 반복 호출하면 그대로 요금이 된다.
// IP 당 분당 제한(handler/ratelimit.go)이 한 겹, 이 하루 상한이 또 한 겹이다.
//
// 메모리에만 둔다. 누가 불렀는지는 세지 않으므로 사용자에 관한 기록이 아니고
// (설계 원칙 2), 서버가 재시작되면 0 부터 다시 세도 무해하다.
type budget struct {
	mu    sync.Mutex
	limit int
	now   func() time.Time
	day   string
	used  int
}

func newBudget(limit int) *budget {
	return &budget{limit: limit, now: time.Now}
}

// spend 는 한 번 더 호출할 수 있으면 세고 true 를 돌려준다.
// limit 이 0 이하면 제한하지 않는다.
func (b *budget) spend() bool {
	if b == nil || b.limit <= 0 {
		return true
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// 날짜가 바뀌면 다시 0 부터. 서버 시각 기준이다
	if today := b.now().Format(time.DateOnly); today != b.day {
		b.day, b.used = today, 0
	}
	if b.used >= b.limit {
		return false
	}
	b.used++
	return true
}

// used 는 오늘 쓴 횟수와 상한이다. 로그에만 쓴다.
func (b *budget) used0() (int, int) {
	if b == nil {
		return 0, 0
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.used, b.limit
}

// allowCall 은 하루 상한 안에서 호출할 수 있는지 본다.
func (c *Client) allowCall(op string) bool {
	if c.budget.spend() {
		return true
	}
	used, limit := c.budget.used0()
	// ★ 입력 원문도, 누가 불렀는지도 남기지 않는다. 숫자만 남긴다
	slog.Warn("AI 하루 호출 상한에 걸렸습니다. 캐시가 있으면 캐시로 답합니다",
		"op", op, "오늘", used, "상한", limit)
	return false
}

// cacheFallback 은 데모 모드가 아니어도 캐시를 뒤진다.
//
// ★ 상한을 넘었을 때만 쓴다. 평소에는 실제 호출이 나가야 하고,
// 데모 모드의 캐시 우선 경로는 그대로다 (client.cached).
func (c *Client) cacheFallback(op, key string) (json.RawMessage, bool) {
	if c == nil || c.cfg.Cache == nil {
		return nil, false
	}
	return c.cfg.Cache.Lookup(op, key)
}
