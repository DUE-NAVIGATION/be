package com.due.domain;

import java.util.List;

public record ApplyInfo(
        /** 신청 채널. 예: BOKJIRO, COMMUNITY_CENTER */
        List<String> channel,
        /** 필요 서류 */
        List<String> documents,
        /** 신청 기간 안내 */
        String period
) {}
