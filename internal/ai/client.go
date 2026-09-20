package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// 기본값. 데모에서 8초 넘게 멈춰 있으면 안 된다 (기획서 Phase 7).
const (
	defaultBaseURL   = "https://api.anthropic.com"
	defaultModel     = "claude-sonnet-5"
	defaultTimeout   = 8 * time.Second
	anthropicVersion = "2023-06-01"
	// ★ 생각 토큰이 이 한도 안에 들어간다. 생각을 켠 뒤 1024 로 두면
	// 생각하다가 한도에 걸려 도구 호출이 아예 나오지 않는다.
	// 실제 답(JSON)은 300 토큰 남짓이라 나머지는 생각 몫이다.
	// 한도일 뿐 여기까지 쓰지 않는다 — effort=low 가 실제 사용량을 정한다.
	defaultMaxTokens  = 4096
	maxResponseBytes  = 1 << 20 // 1MB. 응답이 이보다 클 이유가 없다
	followUpQuestions = 3       // 한 번에 되묻는 질문 수 상한
)

var (
	// ErrNoAPIKey 는 키가 없어 호출조차 못 하는 경우다.
	// 프론트가 "수동 입력" 으로 폴백할 수 있게 구분한다.
	ErrNoAPIKey = errors.New("ANTHROPIC_API_KEY 가 설정되지 않았습니다")
	// ErrUnavailable 은 호출했지만 쓸 수 있는 답을 못 받은 경우다.
	ErrUnavailable = errors.New("AI 응답을 받지 못했습니다")
)

// Cache 는 데모용 캐시다 (Phase 7).
//
// 현장 와이파이가 죽어도 데모가 돌아가야 한다. 지금은 자리만 만들어 두고,
// 파일 기반 구현은 Phase 7 에서 붙인다. nil 이면 캐시를 쓰지 않는다.
type Cache interface {
	// Lookup 은 미리 뽑아둔 응답을 돌려준다. 두 번째 값이 false 면 없는 것이다.
	Lookup(op, key string) (json.RawMessage, bool)
}

// Config 는 AI 게이트웨이 설정이다.
type Config struct {
	// ★ 코드에 넣지 않는다. 환경변수로만 받는다
	APIKey  string
	Model   string
	Timeout time.Duration

	// DemoMode 가 켜지면 Cache 를 먼저 본다
	DemoMode bool
	Cache    Cache

	// DailyLimit 은 하루 AI 호출 상한이다. 0 이면 제한하지 않는다.
	// 넘으면 Cache 가 있으면 캐시로 답하고, 없으면 ErrBudgetExceeded (budget.go)
	DailyLimit int

	// 테스트에서 바꿔 끼운다. 비어 있으면 실제 Anthropic 주소를 쓴다
	BaseURL string
	HTTP    *http.Client
}

// Client 는 Claude API 게이트웨이다.
//
// ★ 이 패키지는 판정에 관여하지 않는다. 자연어를 구조로 옮기고,
// 이미 나온 판정 결과를 사람 말로 풀 뿐이다.
type Client struct {
	cfg    Config
	http   *http.Client
	budget *budget
}

func New(cfg Config) *Client {
	if cfg.Model == "" {
		cfg.Model = defaultModel
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultTimeout
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	hc := cfg.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: cfg.Timeout}
	}
	return &Client{cfg: cfg, http: hc, budget: newBudget(cfg.DailyLimit)}
}

// Enabled 는 호출할 준비가 되었는지 알려준다.
// 키가 없으면 핸들러가 503 으로 답하고, 프론트는 수동 입력으로 넘어간다.
func (c *Client) Enabled() bool {
	return c != nil && (c.cfg.APIKey != "" || (c.cfg.DemoMode && c.cfg.Cache != nil))
}

// ── Anthropic Messages API ──────────────────────────────────

type toolSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type messagesRequest struct {
	Model     string `json:"model"`
	MaxTokens int    `json:"max_tokens"`
	// ★ 문자열이 아니라 블록 배열이다. 캐시 표시를 달 자리가 필요하다
	System     []systemBlock `json:"system"`
	Messages   []message     `json:"messages"`
	Tools      []toolSpec    `json:"tools"`
	ToolChoice toolChoice    `json:"tool_choice"`

	// ★ temperature 를 보내지 않는다. Sonnet 5 는 sampling 파라미터
	// (temperature · top_p · top_k)를 받지 않고 400 으로 거절한다.
	// 구조화의 일관성은 도구 스키마 강제(tool_choice)가 만든다.
	//
	// ★ 생각(thinking)은 켠다. 2026-09-20 에 껐다가 되돌렸다 —
	// 이유는 아래 thinkingConfig 주석에 적었다.
	Thinking     thinkingConfig `json:"thinking"`
	OutputConfig outputConfig   `json:"output_config"`
}

// thinkingConfig 는 확장 사고 설정이다.
//
// ★ 2026-09-20 — 껐다가 되돌렸다.
//
// 처음에는 껐다. 8초 타임아웃과 max_tokens 1024 안에서 생각 토큰이 자리를
// 먹는 것이 걱정이었고, "정해진 스키마를 채우는 단순 작업" 이라고 봤다.
// 단순 작업이 아니었다. 한 문장에서 항목 예닐곱 개를 동시에 골라내
// 각각 enum 에 맞추는 일이다. 생각 없이 한 번 훑으면 가장 눈에 띄는 항목
// 하나만 채우고 끝낸다.
//
// 실제 증상 — "서른둘이고 관악구 원룸 월세 살아요. 보증금 천만원에 월
// 오십오만원 냅니다. 다니던 회사가 지난달 문을 닫았어요." 에서 뽑힌 것이
// employmentStatus 하나뿐이었고, 되묻기에서 "주거 형태가 어떻게 되나요" 를
// 물었다. 방금 월세라고 말한 것을. 도구 스키마에 필드 설명을 붙여도
// 달라지지 않았다 — 지시의 문제가 아니라 읽는 깊이의 문제였다.
//
// ※ budget_tokens 는 Sonnet 5 에서 제거됐다. 넣으면 400 이다.
// ※ Sonnet 5 에서 켜는 방법은 adaptive 하나뿐이다. 깊이는 effort 로 조절한다.
type thinkingConfig struct {
	Type string `json:"type"`
}

// outputConfig 는 생각의 깊이를 정한다.
//
// low 로 둔다. 추출은 어려운 추론이 아니라 빠뜨리지 않는 것이 관건이고,
// 8초 안에 끝나야 한다. low 로 부족하면 medium 까지만 올린다 —
// 그 위는 이 작업에 쓸 이유가 없고 출력 토큰만 늘린다($10/1M).
type outputConfig struct {
	Effort string `json:"effort"`
}

// systemBlock 은 시스템 문구 한 덩이다. 마지막 블록에 캐시 표시를 단다.
type systemBlock struct {
	Type         string        `json:"type"`
	Text         string        `json:"text"`
	CacheControl *cacheControl `json:"cache_control,omitempty"`
}

type cacheControl struct {
	Type string `json:"type"`
}

// systemPrompt 는 시스템 문구를 캐시 가능한 블록으로 만든다.
//
// ★ 우리 프롬프트는 시스템 문구와 도구 스키마가 매 호출마다 똑같다(고정 약 1,900 토큰).
// 바뀌는 건 사용자가 쓴 문장뿐이다. 요청은 tools → system → messages 순서로 조립되므로
// 마지막 시스템 블록에 표시를 달면 **도구 스키마까지 함께** 캐시된다.
//
// 두 번째 호출부터 그 부분의 입력 비용이 1/10 로 떨어진다(처음 한 번만 1.25배).
//
// ★ 캐시 최소 길이는 모델마다 다르다 — Sonnet 5 는 1,024 토큰이다.
// 이보다 짧은 앞부분은 표시를 달아도 조용히 캐시되지 않는다. 오류가 나지 않으므로
// 로그의 "캐시읽음" 이 계속 0 이면 그게 유일한 신호다.
// Haiku 4.5 는 4,096 토큰이라 우리 프롬프트로는 걸리지 않는다. 그래서 Sonnet 5 를 쓴다.
//
// extract 의 고정부는 약 1,900 토큰이라 걸린다. explain 은 더 짧아
// 최소 길이에 못 미칠 수 있다 — 실제 호출의 usage 로 확인할 것.
func systemPrompt(text string) []systemBlock {
	return []systemBlock{{
		Type:         "text",
		Text:         text,
		CacheControl: &cacheControl{Type: "ephemeral"},
	}}
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type toolChoice struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type messagesResponse struct {
	Content []struct {
		Type  string          `json:"type"`
		Name  string          `json:"name"`
		Input json.RawMessage `json:"input"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	// 캐시가 먹고 있는지 확인하는 유일한 근거. 숫자만 로그에 남긴다
	Usage struct {
		InputTokens              int `json:"input_tokens"`
		OutputTokens             int `json:"output_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// callTool 은 도구 사용을 강제해 정해진 모양의 JSON 만 받아온다.
//
// ★ "JSON 으로 답해줘" 라고 부탁하지 않는다. 도구 스키마로 강제한다.
// 부탁하면 앞뒤에 설명이 붙거나 형식이 흔들린다.
//
// 실패하면 한 번 다시 시도한다. 그래도 안 되면 에러를 올려
// 프론트가 수동 입력으로 폴백하게 한다.
func (c *Client) callTool(ctx context.Context, system, user, toolName, toolDesc string, schema json.RawMessage) (json.RawMessage, error) {
	if c.cfg.APIKey == "" {
		return nil, ErrNoAPIKey
	}

	body, err := json.Marshal(messagesRequest{
		Model:        c.cfg.Model,
		MaxTokens:    defaultMaxTokens,
		System:       systemPrompt(system),
		Messages:     []message{{Role: "user", Content: user}},
		Tools:        []toolSpec{{Name: toolName, Description: toolDesc, InputSchema: schema}},
		ToolChoice:   toolChoice{Type: "tool", Name: toolName},
		Thinking:     thinkingConfig{Type: "adaptive"},
		OutputConfig: outputConfig{Effort: "low"},
	})
	if err != nil {
		return nil, fmt.Errorf("요청을 만들지 못했습니다: %w", err)
	}

	var lastErr error
	for attempt := 1; attempt <= 2; attempt++ {
		out, err := c.once(ctx, body, toolName)
		if err == nil {
			return out, nil
		}
		lastErr = err

		// 취소·마감이면 다시 시도하지 않는다. 이미 시간이 없다
		if ctx.Err() != nil {
			break
		}
		// ★ 실패 사유만 남긴다. 요청 본문에는 사용자 입력이 들어 있다
		slog.Warn("AI 호출 실패", "attempt", attempt, "err", err)
	}
	return nil, fmt.Errorf("%w: %v", ErrUnavailable, lastErr)
}

func (c *Client) once(ctx context.Context, body []byte, toolName string) (json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.cfg.BaseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.cfg.APIKey)
	req.Header.Set("anthropic-version", anthropicVersion)

	start := time.Now()
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, err
	}
	// ★ 남기는 것: 상태와 소요시간. 본문은 남기지 않는다
	slog.Debug("AI 호출", "status", resp.StatusCode, "ms", time.Since(start).Milliseconds())

	var out messagesResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("응답을 읽지 못했습니다 (status %d)", resp.StatusCode)
	}
	if out.Error != nil {
		return nil, fmt.Errorf("API 오류 (%s): %s", out.Error.Type, out.Error.Message)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API 가 %d 를 돌려줬습니다", resp.StatusCode)
	}

	// ★ 남기는 것은 숫자뿐이다. 캐시읽음이 0 으로만 계속 찍히면 캐싱이 깨진 것이다
	slog.Info("AI 사용량",
		"입력", out.Usage.InputTokens,
		"출력", out.Usage.OutputTokens,
		"캐시읽음", out.Usage.CacheReadInputTokens,
		"캐시씀", out.Usage.CacheCreationInputTokens)

	for _, blk := range out.Content {
		if blk.Type == "tool_use" && blk.Name == toolName {
			return blk.Input, nil
		}
	}
	return nil, errors.New("도구 호출 결과가 응답에 없습니다")
}

// cached 는 데모 모드에서 미리 뽑아둔 응답을 찾는다.
func (c *Client) cached(op, key string) (json.RawMessage, bool) {
	if !c.cfg.DemoMode || c.cfg.Cache == nil {
		return nil, false
	}
	return c.cfg.Cache.Lookup(op, key)
}
