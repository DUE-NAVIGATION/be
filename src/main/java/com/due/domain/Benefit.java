package com.due.domain;

/**
 * 급여 정의.
 * RATE / IN_KIND 는 금액 산정이 불가하므로 총액 합산에서 제외된다
 * — 데모의 "연 480만원" 이 오염되지 않게 한다.
 */
public record Benefit(
        BenefitType type,
        /** 원 단위. RATE / IN_KIND 면 null */
        Long amount,
        /** MONTHLY 일 때 지급 개월 수 */
        Integer months,
        /** RATE 일 때 감면율(%) */
        Double ratePct,
        /** 금액이 가구원수·소득에 따라 달라져 단순 산정이 불가할 때의 설명 */
        String note
) {}
