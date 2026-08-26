package com.due.domain;

import java.util.List;

/**
 * 자격요건.
 *  all  — 전부 PASS 여야 통과 (AND)
 *  any  — 하나라도 PASS 면 통과 (OR)
 *  none — 하나라도 PASS 면 탈락 (배제 조건)
 */
public record Eligibility(
        List<Condition> all,
        List<Condition> any,
        List<Condition> none
) {
    public List<Condition> allOrEmpty()  { return all  == null ? List.of() : all;  }
    public List<Condition> anyOrEmpty()  { return any  == null ? List.of() : any;  }
    public List<Condition> noneOrEmpty() { return none == null ? List.of() : none; }
}
