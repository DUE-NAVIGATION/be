package com.due.domain;

import java.util.List;

/**
 * 전체 판정 결과 — 화면 상단 "확인된 것 6건 · 연 480만원 · 추가 확인 3건" 의 원본.
 */
public record MatchSummary(
        List<MatchResult> eligible,
        List<MatchResult> needsInfo,
        List<MatchResult> ineligible,
        /** eligible 합산 연간 예상액 (원) */
        long totalYearlyAmount,
        /** 중복수급 배제로 제거된 제도 id */
        List<String> excludedByConflict
) {}
