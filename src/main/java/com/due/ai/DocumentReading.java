package com.due.ai;

import com.due.domain.UserContext;

import java.util.List;

/**
 * 행정문서 → 쉬운 말 변환 결과 (Phase 6).
 *
 * ★ "원문의 법적 효력이 우선합니다" 고지 필수.
 * ★ 원본 이미지는 OCR 직후 폐기한다. 저장 금지.
 */
public record DocumentReading(
        /** 중학생 수준 요약 */
        String summary,
        /** 이게 무슨 문서인지 */
        String whatIsIt,
        /** 내가 해야 할 일 */
        List<String> whatYouMustDo,
        /** 기한 */
        String deadline,
        /** 안 하면 생기는 일 */
        String consequenceIfIgnored,
        /** 문서에서 드러난 상황 — 제도 매칭으로 연결 */
        UserContext inferredContext
) {}
