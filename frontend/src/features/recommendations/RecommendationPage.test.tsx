import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import type { SearchRequest, SearchResponse } from "../../api/contracts";
import { CatalogUnavailableError, ValidationApiError } from "../../api/errors";
import { MockRecommendationGateway, mockFixtures } from "../../api/mockRecommendationGateway";
import type { RecommendationGateway } from "../../api/recommendationGateway";
import { RecommendationPage } from "./RecommendationPage";

class FixedGateway implements RecommendationGateway {
  constructor(private readonly outcome: SearchResponse | Error) {}

  async recommend(_request: SearchRequest): Promise<SearchResponse> {
    if (this.outcome instanceof Error) throw this.outcome;
    return this.outcome;
  }
}

async function submit(gateway: RecommendationGateway) {
  const user = userEvent.setup();
  render(<RecommendationPage gateway={gateway} />);
  await user.click(screen.getByRole("button", { name: "Найти подрядчиков" }));
  return user;
}

describe("RecommendationPage", () => {
  it("renders the complete recommendation form", () => {
    render(<RecommendationPage gateway={new MockRecommendationGateway()} />);
    expect(screen.getByRole("combobox", { name: /Город/ })).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: /Категория/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Найти подрядчиков" })).toBeInTheDocument();
  });

  it("renders contractor cards for MATCHES_FOUND", async () => {
    await submit(new FixedGateway(mockFixtures.popular));
    expect(await screen.findByText("Алекс Меридиан")).toBeInTheDocument();
    expect(screen.getAllByRole("article")).toHaveLength(3);
  });

  it("keeps NO_CATALOG and NO_MATCH visually distinct", async () => {
    const { unmount } = render(<RecommendationPage gateway={new FixedGateway(mockFixtures.noCatalog)} />);
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Найти подрядчиков" }));
    expect(await screen.findByText("В этом городе пока нет подрядчиков этой категории")).toBeInTheDocument();
    unmount();

    await submit(new FixedGateway(mockFixtures.noMatch));
    expect(await screen.findByText("Подрядчики есть, но никто не подходит под выбранные условия")).toBeInTheDocument();
  });

  it("shows the synthetic profile badge", async () => {
    await submit(new FixedGateway(mockFixtures.rare));
    expect(await screen.findByText("Синтетический профиль")).toBeInTheDocument();
  });

  it("always labels a contractor price as starting from", async () => {
    await submit(new FixedGateway(mockFixtures.popular));
    const prices = await screen.findAllByText(/^от .* ₸$/);
    expect(prices).toHaveLength(3);
  });

  it("does not render null max_hours as zero", async () => {
    await submit(new FixedGateway(mockFixtures.rare));
    expect(await screen.findByText("Студия Айша")).toBeInTheDocument();
    expect(screen.queryByText("0 ч")).not.toBeInTheDocument();
  });

  it("maps a 422 validation detail to its field", async () => {
    await submit(new FixedGateway(new ValidationApiError([
      { field: "date", code: "DATE_OUT_OF_RANGE", message: "Дата вне календарного окна." },
    ])));
    expect(await screen.findByText("Дата вне календарного окна.")).toBeInTheDocument();
    expect(screen.getByLabelText(/Дата/)).toHaveAttribute("aria-invalid", "true");
  });

  it("renders 503 as a technical error instead of NO_MATCH", async () => {
    await submit(new FixedGateway(new CatalogUnavailableError()));
    expect(await screen.findByText("Сервис временно недоступен")).toBeInTheDocument();
    expect(screen.queryByText("Подрядчики есть, но никто не подходит под выбранные условия")).not.toBeInTheDocument();
  });
});
