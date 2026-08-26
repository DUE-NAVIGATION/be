package com.due.domain;

/**
 * 단일 조건 — 제도 자격요건의 최소 단위.
 *
 * @param field 판정 대상 필드명. UserContext 의 필드명과 일치해야 한다
 * @param op    연산자
 * @param value 비교값. op 에 따라 숫자 / 문자열 / 배열
 * @param label 화면에 보여줄 사람 말. 예: "만 19~34세"
 * @param note  작성자 메모. 판정에는 쓰이지 않는다
 */
public record Condition(
        String field,
        ConditionOp op,
        Object value,
        String label,
        String note
) {}
