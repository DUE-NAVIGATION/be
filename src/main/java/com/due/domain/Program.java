package com.due.domain;

/**
 * 제도 정의. resources/programs/*.json 과 1:1 로 대응한다.
 * ★ 제도 데이터를 코드에 하드코딩하지 않는다. 반드시 JSON.
 */
public record Program(
        String id,
        String name,
        ProgramCategory category,
        /** 한 줄 설명 */
        String summary,
        Eligibility eligibility,
        Benefit benefit,
        ApplyInfo apply,
        SourceInfo source
) {}
