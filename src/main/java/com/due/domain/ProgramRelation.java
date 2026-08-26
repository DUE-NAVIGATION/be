package com.due.domain;

/** 제도 간 관계 — 중복수급 판정용 (Phase 2) */
public record ProgramRelation(
        String from,
        String to,
        RelationType type,
        /** REDUCING 일 때 감액률(%) */
        Double reducePct,
        String reason
) {}
