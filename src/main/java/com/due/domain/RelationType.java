package com.due.domain;

public enum RelationType {
    EXCLUSIVE,   // 동시 수급 불가
    REDUCING,    // 동시 수급 시 감액
    PREREQUISITE // 선행 조건
}
