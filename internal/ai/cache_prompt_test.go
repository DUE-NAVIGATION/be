package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ★ 이 테스트가 이 파일의 존재 이유다.
//
// 시스템 문구와 도구 스키마는 매 호출마다 똑같다. 캐시 표시가 빠지면 조용히
// 매번 전체 입력 비용을 낸다 — 오류도 경고도 나지 않는다. 그래서 요청 본문을 본다.
func TestSystemPromptIsCacheable(t *testing.T) {
	var sent struct {
		System []struct {
			Type         string `json:"type"`
			Text         string `json:"text"`
			CacheControl *struct {
				Type string `json:"type"`
			} `json:"cache_control"`
		} `json:"system"`
		Tools []json.RawMessage `json:"tools"`
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &sent); err != nil {
			t.Errorf("요청 본문을 읽지 못했다: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"tool_use","name":"record_context",` +
			`"input":{"extracted":{},"confidence":{},"followUpQuestions":[]}}],` +
			`"stop_reason":"tool_use",` +
			`"usage":{"input_tokens":120,"output_tokens":30,"cache_read_input_tokens":1800}}`))
	}))
	defer srv.Close()

	c := New(Config{APIKey: "test-key", BaseURL: srv.URL})
	if _, err := c.Extract(context.Background(), "혼자 아이를 키우고 있습니다"); err != nil {
		t.Fatalf("Extract: %v", err)
	}

	if len(sent.System) != 1 {
		t.Fatalf("시스템 블록 = %d개, want 1", len(sent.System))
	}
	if sent.System[0].Text == "" {
		t.Error("시스템 문구가 비어서 나갔다")
	}
	if cc := sent.System[0].CacheControl; cc == nil || cc.Type != "ephemeral" {
		t.Error("시스템 블록에 캐시 표시가 없다 — 매 호출마다 전체 입력 비용을 낸다")
	}
	// tools → system 순서로 조립되므로, 도구가 함께 실려야 캐시에 같이 담긴다
	if len(sent.Tools) == 0 {
		t.Error("도구 스키마가 요청에서 빠졌다")
	}
}
