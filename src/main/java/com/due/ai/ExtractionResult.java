package com.due.ai;

import com.due.domain.Confidence;
import com.due.domain.UserContext;

import java.util.List;
import java.util.Map;

/**
 * 자연어 → UserContext 추출 결과 (Phase 4).
 *
 * ★ AI 는 여기까지만 한다. 판정은 절대 시키지 않는다.
 *   프롬프트에 "판정하지 말 것, 제도명을 언급하지 말 것" 을 명시한다.
 *
 * @param confidence        필드명 → 확신도. 애매하면 LOW
 * @param followUpQuestions 부족한 필드를 되묻는 질문
 */
public record ExtractionResult(
        UserContext extracted,
        Map<String, Confidence> confidence,
        List<String> followUpQuestions
) {}
