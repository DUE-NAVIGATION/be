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
	defaultBaseURL    = "https://api.anthropic.com"
	defaultModel      = "claude-sonnet-5"
	defaultTimeout    = 8 * time.Second
	anthropicVersion  = "2023-06-01"
	defaultMaxTokens  = 1024
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

	// 테스트에서 바꿔 끼운다. 비어 있으면 실제 Anthropic 주소를 쓴다
	BaseURL string
	HTTP    *http.Client
}

// Client 는 Claude API 게이트웨이다.
//
// ★ 이 패키지는 판정에 관여하지 않는다. 자연어를 구조로 옮기고,
// 이미 나온 판정 결과를 사람 말로 풀 뿐이다.
type Client struct {
	cfg  Config
	http *http.Client
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
	return &Client{cfg: cfg, http: hc}
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
	Model      string     `json:"model"`
	MaxTokens  int        `json:"max_tokens"`
	System     string     `json:"system"`
	Messages   []message  `json:"messages"`
	Tools      []toolSpec `json:"tools"`
	ToolChoice toolChoice `json:"tool_choice"`
	// 같은 입력에 같은 답이 나오도록 낮춘다. 구조화는 창의성이 필요 없다
	Temperature float64 `json:"temperature"`
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
	Error      *struct {
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
		Model:       c.cfg.Model,
		MaxTokens:   defaultMaxTokens,
		System:      system,
		Messages:    []message{{Role: "user", Content: user}},
		Tools:       []toolSpec{{Name: toolName, Description: toolDesc, InputSchema: schema}},
		ToolChoice:  toolChoice{Type: "tool", Name: toolName},
		Temperature: 0,
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
