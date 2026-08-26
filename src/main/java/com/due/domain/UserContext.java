package com.due.domain;

import java.util.List;

/**
 * 사용자 상황 — 판정 입력값.
 *
 * ※ 모든 필드는 null 가능하다. 모르는 값이 있는 게 정상이다.
 *   비어 있는 필드는 그 조건을 UNKNOWN 으로 만들고, 결과는 NEEDS_INFO 가 된다.
 * ※ 이름·주소·주민번호 등 식별정보는 이 타입에 절대 넣지 않는다 (설계 원칙 2).
 * ※ 이 객체는 요청 처리 중에만 메모리에 존재한다. 저장하지 않는다.
 */
public record UserContext(
        /** 가구원 수 (본인 포함) */
        Integer householdSize,
        /** 만 나이 */
        Integer age,
        /** 월 소득 (원). 소득평가액 기준 */
        Long incomeMonthly,
        /** 재산 총액 (원). 소득환산 대상 */
        Long assets,
        HousingType housingType,
        /** 보증금 (원) */
        Long deposit,
        /** 월세 (원) */
        Long monthlyRent,
        EmploymentStatus employmentStatus,
        /** 한부모 가구 여부 */
        Boolean isSingleParent,
        /** 자녀 나이 목록 (만 나이) */
        List<Integer> childrenAges,
        Boolean hasDisability,
        DisabilityLevel disabilityLevel,
        Boolean isPregnant,
        /** 현재 수급 중인 제도 id 또는 급여 코드 (중복수급·배제 판정용) */
        List<String> receivingPrograms,
        /** 거주 지역 (시도 단위) */
        String region,
        BasicLivelihoodType basicLivelihoodType,
        /** ★ 계산 엔진이 채우는 파생값 — 중위소득 대비 비율(%) */
        Double householdIncomePct
) {
    /** 빈 상황. 아무 것도 모르는 상태 */
    public static UserContext empty() {
        return new UserContext(null, null, null, null, null, null, null, null,
                null, null, null, null, null, null, null, null, null);
    }

    /** 계산된 중위소득 비율을 채운 새 인스턴스 (원본은 불변) */
    public UserContext withHouseholdIncomePct(Double pct) {
        return new UserContext(householdSize, age, incomeMonthly, assets, housingType,
                deposit, monthlyRent, employmentStatus, isSingleParent, childrenAges,
                hasDisability, disabilityLevel, isPregnant, receivingPrograms, region,
                basicLivelihoodType, pct);
    }
}
