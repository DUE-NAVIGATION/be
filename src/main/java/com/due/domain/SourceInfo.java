package com.due.domain;

import com.fasterxml.jackson.annotation.JsonProperty;

/**
 * 출처.
 * ★ revised_at 은 반드시 채운다. 심사에서 물어본다.
 *
 * <p>제도 JSON 의 표기(snake_case)를 그대로 쓴다 — 비개발자 팀원이 채우는 파일이라
 * 기획서에 적힌 이름과 어긋나지 않게 한다.
 */
public record SourceInfo(
        String url,
        /** 개정일 (YYYY-MM-DD) */
        @JsonProperty("revised_at") String revisedAt,
        /** 출처 기관명 */
        String agency
) {}
