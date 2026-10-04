import type { BudgetSummary } from "./components/AppFrame";

// Invented people on reserved domains. Budget figures are micro-USD as the
// API sends them (Budget schema); the frame formats them.
export const seller = { role: "seller" as const, email: "ravi.seller@example.in" };
export const reviewer = { role: "reviewer" as const, email: "asha.reviewer@example.in" };

export const budgetOk: BudgetSummary = {
  spentMicroUsd: 4_210_000,
  limitMicroUsd: 8_000_000,
  blockedAt: null,
};

export const budgetBlocked: BudgetSummary = {
  spentMicroUsd: 7_996_400,
  limitMicroUsd: 8_000_000,
  blockedAt: "2026-10-04T10:40:00Z",
};
