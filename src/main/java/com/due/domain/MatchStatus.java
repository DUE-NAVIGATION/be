package com.due.domain;

/**
 * 제도별 판정 결과.
 * ELIGIBLE   — 확인된 조건이 전부 충족
 * INELIGIBLE — 명시적으로 탈락
 * NEEDS_INFO — 탈락은 아니지만 확인이 더 필요
 */
public enum MatchStatus { ELIGIBLE, INELIGIBLE, NEEDS_INFO }
