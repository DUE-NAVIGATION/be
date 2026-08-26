package com.due.domain;

import java.util.Map;

/**
 * 기준중위소득 표.
 * ★ 이 값 하나로 대부분의 제도 자격이 갈린다. 반드시 코드로 계산한다.
 */
public record MedianIncomeTable(
        int year,
        SourceInfo source,
        /** 가구원수 → 월 기준중위소득 (원) */
        Map<String, Long> byHouseholdSize
) {}
