package rules

import (
	"fmt"
	"sort"

	"github.com/DUE-NAVIGATION/be/internal/model"
)

// Resolution 은 중복수급 정리 결과다.
type Resolution struct {
	Results []model.MatchResult
	// 배타 관계로 제외된 제도 id. 화면에서 "A 와 동시에 받을 수 없습니다" 로 쓴다
	Excluded []string
}

// 배타 조합을 완전 탐색할 최대 크기.
// 실제 제도 관계는 2~4개짜리 뭉치라 여기 걸릴 일이 거의 없다.
// 넘어가면 탐욕법으로 내려가고, 그 사실을 결과에 남긴다.
const maxExactCluster = 22

// ResolveConflicts 는 제도 간 관계를 반영해 실제로 함께 받을 수 있는 조합을 남긴다.
//
// 순서가 중요하다.
//  1. PREREQUISITE — 선행 제도가 안 되면 후행 제도도 안 된다
//  2. EXCLUSIVE    — 동시 수급 불가. 남길 조합은 총액이 가장 큰 쪽으로 고른다
//  3. REDUCING     — 함께 받되 금액이 깎인다
//
// ★ EstimatedAmount 가 채워진 뒤에 호출해야 한다 (WithEstimates 먼저).
// 금액을 모르면 "어느 조합이 최대인가" 를 판단할 수 없다.
//
// 순수 함수다. 관계 데이터는 호출자가 읽어서 넘긴다 — 이 패키지는 파일을 읽지 않는다.
func ResolveConflicts(results []model.MatchResult, relations []model.Relation) Resolution {
	out := make([]model.MatchResult, len(results))
	copy(out, results)

	index := map[string]int{}
	for i, r := range out {
		index[r.Program.ID] = i
	}

	applyPrerequisites(out, index, relations)
	excluded := applyExclusives(out, index, relations)
	applyReducing(out, index, relations)

	return Resolution{Results: out, Excluded: excluded}
}

// applyPrerequisites 는 선행 제도가 성립하지 않는 제도를 끌어내린다.
//
// 연쇄(A→B→C)가 있을 수 있으므로 변화가 없을 때까지 반복한다.
func applyPrerequisites(results []model.MatchResult, index map[string]int, relations []model.Relation) {
	// 관계 수만큼 돌면 아무리 긴 연쇄라도 안정된다
	for pass := 0; pass <= len(relations); pass++ {
		changed := false

		for _, rel := range relations {
			if rel.Type != model.RelationPrerequisite {
				continue
			}
			fromIdx, ok := index[rel.From]
			if !ok {
				continue
			}
			if results[fromIdx].Status == model.MatchIneligible {
				continue // 이미 탈락. 더 내릴 곳이 없다
			}

			toIdx, ok := index[rel.To]
			if !ok {
				// 선행 제도가 결과에 없다. 판정할 수 없으므로 단정하지 않는다
				if demote(&results[fromIdx], model.MatchNeedsInfo,
					fmt.Sprintf("선행 조건: %s", rel.To),
					model.StatusUnknown,
					fmt.Sprintf("확인 필요 — 선행 제도(%s)를 판정하지 못했습니다", rel.To)) {
					changed = true
				}
				continue
			}

			switch results[toIdx].Status {
			case model.MatchIneligible:
				if demote(&results[fromIdx], model.MatchIneligible,
					fmt.Sprintf("선행 조건: %s", results[toIdx].Program.Name),
					model.StatusFail,
					fmt.Sprintf("선행 제도 '%s' 에 해당하지 않습니다", results[toIdx].Program.Name)) {
					changed = true
				}
			case model.MatchNeedsInfo:
				if demote(&results[fromIdx], model.MatchNeedsInfo,
					fmt.Sprintf("선행 조건: %s", results[toIdx].Program.Name),
					model.StatusUnknown,
					fmt.Sprintf("확인 필요 — 선행 제도 '%s' 부터 확인해야 합니다", results[toIdx].Program.Name)) {
					changed = true
				}
			}
		}

		if !changed {
			return
		}
	}
}

// demote 는 결과의 상태를 낮추고 그 근거를 남긴다.
// 이미 그만큼 낮거나 같은 근거가 있으면 아무것도 하지 않고 false 를 돌려준다.
//
// ★ 상태만 바꾸고 끝내지 않는다. 왜 내려갔는지를 조건 목록에 남겨야
// 사용자가 "이건 왜 안 되지" 를 화면에서 확인할 수 있다.
func demote(r *model.MatchResult, to model.MatchStatus, label string, condStatus model.ConditionStatus, reason string) bool {
	if !isLower(to, r.Status) {
		return false
	}
	r.Status = to
	r.Conditions = append(r.Conditions, model.ConditionResult{
		// Field 를 비워 둔다 — 사용자 입력으로 채울 수 있는 값이 아니므로
		// MissingFields 에 들어가면 안 된다
		Condition: model.Condition{Label: label},
		Status:    condStatus,
		Reason:    reason,
	})
	return true
}

// isLower 는 a 가 b 보다 낮은(나쁜) 상태인지 본다.
// ELIGIBLE > NEEDS_INFO > INELIGIBLE
func isLower(a, b model.MatchStatus) bool {
	return rank(a) < rank(b)
}

func rank(s model.MatchStatus) int {
	switch s {
	case model.MatchEligible:
		return 2
	case model.MatchNeedsInfo:
		return 1
	}
	return 0
}

// applyExclusives 는 동시에 받을 수 없는 제도 중 총액이 큰 조합만 남긴다.
func applyExclusives(results []model.MatchResult, index map[string]int, relations []model.Relation) []string {
	// 해당 가능한 제도만 대상이다. 어차피 못 받는 제도끼리는 다툴 일이 없다
	var ids []string
	for _, r := range results {
		if r.Status == model.MatchEligible {
			ids = append(ids, r.Program.ID)
		}
	}
	sort.Strings(ids) // 결과를 결정론적으로 만든다
	if len(ids) == 0 {
		return nil
	}

	pos := map[string]int{}
	for i, id := range ids {
		pos[id] = i
	}

	// 인접 관계를 비트마스크로 만든다
	adj := make([]uint32, len(ids))
	hasConflict := false
	for _, rel := range relations {
		if rel.Type != model.RelationExclusive {
			continue
		}
		a, okA := pos[rel.From]
		b, okB := pos[rel.To]
		if !okA || !okB || a == b {
			continue
		}
		adj[a] |= 1 << uint(b)
		adj[b] |= 1 << uint(a)
		hasConflict = true
	}
	if !hasConflict {
		return nil
	}

	weight := make([]int64, len(ids))
	for i, id := range ids {
		weight[i] = results[index[id]].EstimatedAmount
	}

	keep := selectBest(ids, adj, weight)

	var excluded []string
	for i, id := range ids {
		if keep&(1<<uint(i)) != 0 {
			continue
		}
		if adj[i] == 0 {
			continue // 다툰 적이 없는데 빠졌을 리 없다
		}
		excluded = append(excluded, id)

		r := &results[index[id]]
		winners := conflictWinners(i, adj, keep, ids, results, index)
		r.Status = model.MatchIneligible
		r.Conditions = append(r.Conditions, model.ConditionResult{
			Condition: model.Condition{Label: "중복수급 제한"},
			Status:    model.StatusFail,
			Reason: fmt.Sprintf("%s 와(과) 동시에 받을 수 없습니다. 금액이 더 큰 쪽을 남겼습니다",
				winners),
		})
	}
	return excluded
}

// conflictWinners 는 이 제도를 밀어낸 상대 제도의 이름을 사람 말로 만든다.
func conflictWinners(i int, adj []uint32, keep uint32, ids []string, results []model.MatchResult, index map[string]int) string {
	var names []string
	for j := range ids {
		if adj[i]&(1<<uint(j)) == 0 {
			continue
		}
		if keep&(1<<uint(j)) == 0 {
			continue
		}
		names = append(names, results[index[ids[j]]].Program.Name)
	}
	if len(names) == 0 {
		return "다른 제도"
	}
	out := names[0]
	for _, n := range names[1:] {
		out += ", " + n
	}
	return out
}

// selectBest 는 서로 배타인 제도들 중 총액이 최대가 되는 조합을 고른다.
//
// 뭉치가 작으면 완전 탐색해서 진짜 최대를 찾는다.
// 너무 크면 금액 큰 순서로 고르는 탐욕법으로 내려간다 — 이때는 최대라고 단정할 수 없다.
func selectBest(ids []string, adj []uint32, weight []int64) uint32 {
	n := len(ids)
	if n > maxExactCluster {
		return selectGreedy(n, adj, weight)
	}

	var best uint32
	var bestWeight int64 = -1

	// allowed: 아직 고를 수 있는 제도. picked: 고른 제도
	var walk func(i int, allowed, picked uint32, total int64)
	walk = func(i int, allowed, picked uint32, total int64) {
		if i == n {
			if total > bestWeight {
				bestWeight, best = total, picked
			}
			return
		}
		bit := uint32(1) << uint(i)

		// 고를 수 있으면 고른 경우를 먼저 본다 (금액이 큰 답을 일찍 찾는다)
		if allowed&bit != 0 {
			walk(i+1, allowed&^(adj[i]|bit), picked|bit, total+weight[i])
		}
		walk(i+1, allowed&^bit, picked, total)
	}

	all := uint32(0)
	for i := 0; i < n; i++ {
		all |= 1 << uint(i)
	}
	walk(0, all, 0, 0)

	return best
}

// selectGreedy 는 금액이 큰 제도부터 담는다. 최적을 보장하지 않는다.
func selectGreedy(n int, adj []uint32, weight []int64) uint32 {
	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		return weight[order[a]] > weight[order[b]]
	})

	var picked, blocked uint32
	for _, i := range order {
		bit := uint32(1) << uint(i)
		if blocked&bit != 0 {
			continue
		}
		picked |= bit
		blocked |= adj[i] | bit
	}
	return picked
}

// applyReducing 은 함께 받을 수 있지만 금액이 깎이는 관계를 반영한다.
func applyReducing(results []model.MatchResult, index map[string]int, relations []model.Relation) {
	for _, rel := range relations {
		if rel.Type != model.RelationReducing || rel.ReducePct <= 0 {
			continue
		}
		fromIdx, okA := index[rel.From]
		toIdx, okB := index[rel.To]
		if !okA || !okB {
			continue
		}
		// 둘 다 실제로 받을 때만 깎인다
		if results[fromIdx].Status != model.MatchEligible ||
			results[toIdx].Status != model.MatchEligible {
			continue
		}

		before := results[fromIdx].EstimatedAmount
		if before <= 0 {
			continue
		}
		pct := rel.ReducePct
		if pct > 100 {
			pct = 100
		}
		// 깎이는 금액을 먼저 구해서 뺀다. 사용자에게 불리하게 반올림되지 않게 한다
		cut := int64(float64(before) * pct / 100)
		results[fromIdx].EstimatedAmount = before - cut

		reason := rel.Reason
		if reason == "" {
			reason = fmt.Sprintf("%s 와(과) 함께 받으면 %.0f%% 감액됩니다",
				results[toIdx].Program.Name, pct)
		}
		results[fromIdx].Conditions = append(results[fromIdx].Conditions, model.ConditionResult{
			Condition: model.Condition{Label: "중복수급 감액"},
			Status:    model.StatusPass,
			Actual:    results[fromIdx].EstimatedAmount,
			Reason:    reason,
		})
	}
}
