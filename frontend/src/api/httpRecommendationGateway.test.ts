import { afterEach, describe, expect, it, vi } from "vitest";
import type { SearchRequest } from "./contracts";
import { HttpRecommendationGateway } from "./httpRecommendationGateway";
import { mockFixtures } from "./mockRecommendationGateway";

const request: SearchRequest = {
  city: "Алматы",
  category: "Ведущий",
  event_format: "корпоратив",
  date: "2026-11-14",
  budget_kzt: 1_000_000,
  language: "русский",
  duration_hours: 6,
};

describe("HttpRecommendationGateway", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("sends Accept-Language to recommendations and alternatives", async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify(mockFixtures.popular), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ alternatives: [] }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    const gateway = new HttpRecommendationGateway("/api");

    await gateway.recommend(request, { locale: "kk" });
    await gateway.getAlternatives(request, { locale: "en" });

    expect(fetchMock.mock.calls[0]?.[0]).toBe("/api/recommendations");
    expect(fetchMock.mock.calls[0]?.[1]?.headers).toMatchObject({ "Accept-Language": "kk" });
    expect(fetchMock.mock.calls[1]?.[0]).toBe("/api/recommendations/alternatives");
    expect(fetchMock.mock.calls[1]?.[1]?.headers).toMatchObject({ "Accept-Language": "en" });
  });
});
