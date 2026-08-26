package com.due.config;

import org.springframework.boot.context.properties.ConfigurationProperties;

/**
 * DUE 런타임 설정.
 *
 * @param demoMode true 면 AI 호출 없이 캐시된 응답을 쓴다 (Phase 7).
 *                 현장 와이파이가 죽어도 데모가 돌아가야 한다.
 */
@ConfigurationProperties(prefix = "due")
public record DueProperties(
        boolean demoMode,
        Anthropic anthropic
) {
    /** ★ apiKey 는 절대 로그에 찍지 않는다 */
    public record Anthropic(String apiKey, String model, int timeoutSeconds) {}
}
