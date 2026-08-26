package com.due.domain;

/**
 * 조건 연산자.
 * 제도 JSON 의 eligibility.*.op 값과 1:1 로 대응한다.
 */
public enum ConditionOp {
    BETWEEN,  // value: [min, max] — 양 끝 포함
    LTE,      // value: number
    GTE,      // value: number
    EQ,       // value: primitive
    IN,       // value: primitive[] — 대상 값이 목록에 포함
    CONTAINS, // value: primitive — 대상 배열이 값을 포함
    EXISTS    // value 무시 — 값이 존재하기만 하면 PASS
}
