import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it } from "vitest";
import type { AlternativesResponse, SearchRequest, SearchResponse } from "../../api/contracts";
import { CatalogUnavailableError, ValidationApiError } from "../../api/errors";
import { MockRecommendationGateway, mockFixtures } from "../../api/mockRecommendationGateway";
import type { RecommendationGateway } from "../../api/recommendationGateway";
import { LocaleProvider } from "../../i18n/LocaleContext";
import { RecommendationPage } from "./RecommendationPage";

class FixedGateway implements RecommendationGateway {
  readonly recommendRequests: SearchRequest[] = [];
  readonly alternativesRequests: SearchRequest[] = [];

  constructor(
    private readonly outcome: SearchResponse | Error | ((request: SearchRequest) => SearchResponse | Promise<SearchResponse>),
    private readonly alternativesOutcome: AlternativesResponse | Error | ((request: SearchRequest) => AlternativesResponse | Promise<AlternativesResponse>) = { alternatives: [] },
  ) {}

  async recommend(request: SearchRequest): Promise<SearchResponse> {
    this.recommendRequests.push(request);
    if (typeof this.outcome === "function") return this.outcome(request);
    if (this.outcome instanceof Error) throw this.outcome;
    return this.outcome;
  }

  async getAlternatives(request: SearchRequest): Promise<AlternativesResponse> {
    this.alternativesRequests.push(request);
    if (typeof this.alternativesOutcome === "function") return this.alternativesOutcome(request);
    if (this.alternativesOutcome instanceof Error) throw this.alternativesOutcome;
    return this.alternativesOutcome;
  }
}

const budgetAlternative: AlternativesResponse = {
  alternatives: [{
    type: "BUDGET",
    current_budget_kzt: 1_000_000,
    suggested_budget_kzt: 1_250_000,
    eligible_count: 4,
  }],
};

const dateAlternative: AlternativesResponse = {
  alternatives: [{
    type: "DATE",
    current_date: "2026-11-14",
    suggested_date: "2026-12-21",
    distance_days: 1,
    eligible_count: 2,
  }],
};

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((resolver) => { resolve = resolver; });
  return { promise, resolve };
}

async function submit(gateway: RecommendationGateway) {
  const user = userEvent.setup();
  render(<RecommendationPage gateway={gateway} />);
  await user.click(screen.getByRole("button", { name: "Найти подрядчиков" }));
  return user;
}

describe("RecommendationPage", () => {
  beforeEach(() => window.localStorage.clear());

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

  it("loads alternatives after NO_MATCH", async () => {
    const pending = deferred<AlternativesResponse>();
    const gateway = new FixedGateway(mockFixtures.noMatch, () => pending.promise);
    await submit(gateway);
    expect(await screen.findByText("Ищем, что можно изменить…")).toBeInTheDocument();
    expect(gateway.alternativesRequests).toHaveLength(1);
    pending.resolve({ alternatives: [] });
  });

  it("renders a BUDGET alternative", async () => {
    await submit(new FixedGateway(mockFixtures.noMatch, budgetAlternative));
    expect(await screen.findByText("Увеличить бюджет")).toBeInTheDocument();
    expect(screen.getByText("до 1 250 000 ₸")).toBeInTheDocument();
    expect(screen.getByText("Появятся 4 подходящих варианта")).toBeInTheDocument();
  });

  it("renders a DATE alternative", async () => {
    await submit(new FixedGateway(mockFixtures.noMatch, dateAlternative));
    expect(await screen.findByText("Изменить дату")).toBeInTheDocument();
    expect(screen.getByText("21 декабря")).toBeInTheDocument();
    expect(screen.getByText("±1 день")).toBeInTheDocument();
  });

  it("keeps NO_MATCH when alternatives are empty", async () => {
    await submit(new FixedGateway(mockFixtures.noMatch));
    expect(await screen.findByText("Подрядчики есть, но никто не подходит под выбранные условия")).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByText("Что можно изменить?")).not.toBeInTheDocument());
  });

  it("keeps NO_MATCH when alternatives fail", async () => {
    await submit(new FixedGateway(mockFixtures.noMatch, new Error("endpoint unavailable")));
    expect(await screen.findByText("Подрядчики есть, но никто не подходит под выбранные условия")).toBeInTheDocument();
    expect(screen.queryByText("Сервис временно недоступен")).not.toBeInTheDocument();
  });

  it("applies only BUDGET and runs recommend again", async () => {
    let calls = 0;
    const gateway = new FixedGateway(
      () => (++calls === 1 ? mockFixtures.noMatch : mockFixtures.popular),
      budgetAlternative,
    );
    const user = await submit(gateway);
    await user.click(await screen.findByRole("button", { name: "Применить и найти" }));

    await waitFor(() => expect(gateway.recommendRequests).toHaveLength(2));
    expect(gateway.recommendRequests[1]).toEqual({
      ...gateway.recommendRequests[0],
      budget_kzt: 1_250_000,
    });
    expect(screen.getByLabelText(/Бюджет на подрядчика/)).toHaveValue(1_250_000);
  });

  it("applies only DATE and runs recommend again", async () => {
    let calls = 0;
    const gateway = new FixedGateway(
      () => (++calls === 1 ? mockFixtures.noMatch : mockFixtures.popular),
      dateAlternative,
    );
    const user = await submit(gateway);
    await user.click(await screen.findByRole("button", { name: "Применить и найти" }));

    await waitFor(() => expect(gateway.recommendRequests).toHaveLength(2));
    expect(gateway.recommendRequests[1]).toEqual({
      ...gateway.recommendRequests[0],
      date: "2026-12-21",
    });
    expect(screen.getByLabelText(/Дата/)).toHaveValue("2026-12-21");
  });

  it("does not let an old alternatives request overwrite a newer one", async () => {
    const first = deferred<AlternativesResponse>();
    const second = deferred<AlternativesResponse>();
    let alternativesCall = 0;
    const gateway = new FixedGateway(
      mockFixtures.noMatch,
      () => (++alternativesCall === 1 ? first.promise : second.promise),
    );
    const user = await submit(gateway);
    await waitFor(() => expect(gateway.alternativesRequests).toHaveLength(1));

    const budgetInput = screen.getByLabelText(/Бюджет на подрядчика/);
    await user.clear(budgetInput);
    await user.type(budgetInput, "200000");
    await user.click(screen.getByRole("button", { name: "Найти подрядчиков" }));
    await waitFor(() => expect(gateway.alternativesRequests).toHaveLength(2));

    await act(async () => { second.resolve(dateAlternative); });
    expect(await screen.findByText("21 декабря")).toBeInTheDocument();
    await act(async () => { first.resolve(budgetAlternative); });
    expect(screen.queryByText("до 1 250 000 ₸")).not.toBeInTheDocument();
    expect(screen.getByText("21 декабря")).toBeInTheDocument();
  });

  it("does not translate backend explanations in the frontend", async () => {
    window.localStorage.setItem("apex-match-locale", "en");
    const user = userEvent.setup();
    render(
      <LocaleProvider>
        <RecommendationPage gateway={new FixedGateway(mockFixtures.popular)} />
      </LocaleProvider>,
    );
    await user.click(screen.getByRole("button", { name: "Find contractors" }));
    expect(await screen.findByText(mockFixtures.popular.results[0]!.explanation)).toBeInTheDocument();
  });
});
