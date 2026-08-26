package com.due.domain;

import java.util.List;

/**
 * 제도별 판정 결과.
 *
 * @param conditions      조건 단위 근거. 항상 채운다
 * @param estimatedAmount 연간 예상 수령액 (원). 산정 불가면 null
 * @param missingFields   판정에 더 필요한 필드 목록
 */
public record MatchResult(
        Program program,
        MatchStatus status,
        List<ConditionResult> conditions,
        Long estimatedAmount,
        List<String> missingFields
) {}
