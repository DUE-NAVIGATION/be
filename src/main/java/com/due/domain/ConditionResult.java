package com.due.domain;

/**
 * 조건 단위 판정 결과.
 * ★ 설명 가능성의 핵심 — "왜 해당/미해당인지"를 조건 단위로 보여주기 위해 항상 남긴다.
 *
 * @param actual 사용자 입력의 실제 값. 화면의 "입력: 29세"
 * @param reason 사람 말 사유
 */
public record ConditionResult(
        Condition condition,
        ConditionStatus status,
        Object actual,
        String reason
) {}
