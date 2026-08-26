package com.due.domain;

/**
 * 조건 단위 판정.
 * UNKNOWN = 입력값이 없어 판정 불가. ★ 값이 없다고 FAIL 로 떨어뜨리지 않는다.
 */
public enum ConditionStatus { PASS, FAIL, UNKNOWN }
