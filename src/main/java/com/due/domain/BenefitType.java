package com.due.domain;

public enum BenefitType {
    MONTHLY, // 월 정액 × months
    ONCE,    // 1회성
    YEARLY,  // 연 정액
    RATE,    // 요금 감면율 등 — 금액 산정 불가, 합산에서 제외
    IN_KIND  // 현물·서비스 — 금액 없음
}
