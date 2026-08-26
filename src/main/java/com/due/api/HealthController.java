package com.due.api;

import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

import java.util.Map;

/** FE 연결 확인용. Phase 1 부터 실제 판정 API 가 이 패키지에 붙는다. */
@RestController
@RequestMapping("/api")
public class HealthController {

    @GetMapping("/health")
    public Map<String, Object> health() {
        return Map.of(
                "status", "ok",
                "service", "due-backend",
                "storesUserData", false // 설계 원칙 2 — 아무것도 저장하지 않는다
        );
    }
}
