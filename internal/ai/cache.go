package ai

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sort"
)

// DefaultKey 는 "어떤 입력이든" 을 뜻하는 키다.
//
// ★ 발표 중 오타 한 글자에 데모가 깨지지 않게 하는 마지막 그물이다.
// 정확히 일치하는 항목이 없으면 이 항목을 쓴다.
const DefaultKey = "*"

// FileCache 는 미리 뽑아둔 AI 응답을 담은 파일이다 (data/demo-cache.json).
//
// ★ 이게 있으면 API 키도 네트워크도 없이 데모가 완주된다.
// 현장 와이파이가 죽어도, 키가 만료돼도 발표는 돌아가야 한다.
//
// 읽기 전용이다. 서버가 뜰 때 한 번 읽고 그 뒤로는 바뀌지 않는다.
type FileCache struct {
	// op("extract"/"explain") → 키 → 응답
	entries map[string]map[string]json.RawMessage
	path    string
}

// cacheFile 은 data/demo-cache.json 의 모양이다.
type cacheFile struct {
	// 사람이 읽을 메모. 코드는 쓰지 않는다
	Note string `json:"note,omitempty"`
	// op → 키 → 응답 본문 (각 op 의 도구 출력과 같은 모양)
	Entries map[string]map[string]json.RawMessage `json:"entries"`
}

// LoadCache 는 데모 캐시 파일을 읽는다.
//
// 파일이 없으면 에러다 — DEMO_MODE 를 켰다는 것은 캐시를 쓰겠다는 뜻이므로,
// 없는데 조용히 넘어가면 발표 당일에야 알게 된다.
func LoadCache(path string) (*FileCache, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("데모 캐시를 읽지 못했습니다 (%s): %w", path, err)
	}

	var f cacheFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("데모 캐시 형식이 잘못됐습니다 (%s): %w", path, err)
	}
	if len(f.Entries) == 0 {
		return nil, fmt.Errorf("데모 캐시가 비어 있습니다 (%s)", path)
	}

	// 키를 정규화해 둔다. 파일에 적힌 공백과 실제 입력의 공백이 달라
	// 캐시가 빗나가는 일을 막는다
	entries := make(map[string]map[string]json.RawMessage, len(f.Entries))
	for op, byKey := range f.Entries {
		norm := make(map[string]json.RawMessage, len(byKey))
		for k, v := range byKey {
			if k == DefaultKey {
				norm[DefaultKey] = v
				continue
			}
			norm[cacheKey(Sanitize(k).Text)] = v
		}
		entries[op] = norm
	}

	return &FileCache{entries: entries, path: path}, nil
}

// Lookup 은 미리 뽑아둔 응답을 찾는다.
//
// 정확히 일치하는 것을 먼저 보고, 없으면 기본 항목("*")으로 내려간다.
// 기본 항목을 쓸 때는 경고를 남긴다 — 조용히 넘어가면 리허설에서
// "왜 항상 같은 답이 나오지" 를 눈치채지 못한다.
func (c *FileCache) Lookup(op, key string) (json.RawMessage, bool) {
	byKey, ok := c.entries[op]
	if !ok {
		return nil, false
	}
	if v, ok := byKey[key]; ok {
		return v, true
	}
	if v, ok := byKey[DefaultKey]; ok {
		// ★ 입력 원문을 남기지 않는다. 무슨 일이 있었는지만 남긴다
		slog.Warn("데모 캐시: 정확히 맞는 항목이 없어 기본 항목을 씁니다", "op", op)
		return v, true
	}
	return nil, false
}

// Ops 는 캐시에 들어 있는 기능 목록이다. 시작 로그에 쓴다.
func (c *FileCache) Ops() []string {
	out := make([]string, 0, len(c.entries))
	for op := range c.entries {
		out = append(out, op)
	}
	sort.Strings(out)
	return out
}

// Count 는 op 에 들어 있는 항목 수다 (기본 항목 포함).
func (c *FileCache) Count(op string) int { return len(c.entries[op]) }

// HasDefault 는 그물이 깔려 있는지 알려준다.
// 없으면 리허설과 다른 입력이 들어왔을 때 데모가 실패한다.
func (c *FileCache) HasDefault(op string) bool {
	_, ok := c.entries[op][DefaultKey]
	return ok
}
