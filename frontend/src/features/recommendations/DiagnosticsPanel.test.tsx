import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";
import type { SearchDiagnostics } from "../../api/contracts";
import { LocaleProvider } from "../../i18n/LocaleContext";
import { mockFixtures } from "../../api/mockRecommendationGateway";
import { buildFunnel, DiagnosticsPanel } from "./DiagnosticsPanel";

describe("DiagnosticsPanel", () => {
  beforeEach(() => window.localStorage.clear());

  it("computes sequential funnel counts", () => {
    expect(buildFunnel(mockFixtures.popular.diagnostics).map((step) => step.remaining)).toEqual([10, 9, 7, 7, 7, 4, 3]);
  });

  it("marks an omitted language check without changing the count", () => {
    const diagnostics: SearchDiagnostics = {
      ...mockFixtures.popular.diagnostics,
      omitted_optional_checks: ["language"],
    };
    render(<LocaleProvider><DiagnosticsPanel diagnostics={diagnostics} mode="matches" /></LocaleProvider>);
    const step = screen.getByText("Язык — не учитывался").closest("li");
    expect(step).toHaveTextContent("7");
    expect(step).not.toHaveTextContent("−");
  });

  it("shows a human-readable sorting rule", () => {
    render(<LocaleProvider><DiagnosticsPanel diagnostics={mockFixtures.popular.diagnostics} mode="matches" /></LocaleProvider>);
    expect(screen.getByText("Сначала по стартовой цене, затем по ID")).toBeInTheDocument();
    expect(screen.queryByText("price_from_kzt:asc")).not.toBeInTheDocument();
  });
});
