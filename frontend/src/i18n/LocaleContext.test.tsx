import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it } from "vitest";
import { App } from "../app/App";

describe("public UI localization", () => {
  beforeEach(() => {
    cleanup();
    window.localStorage.clear();
    document.documentElement.lang = "";
  });

  it("uses Russian by default", async () => {
    render(<App />);
    expect(screen.getByRole("heading", { name: "Найдите подходящего подрядчика" })).toBeInTheDocument();
    await waitFor(() => expect(document.documentElement.lang).toBe("ru"));
  });

  it("switches from Russian to Kazakh", async () => {
    const user = userEvent.setup();
    render(<App />);
    await user.click(screen.getByRole("button", { name: "KZ" }));
    expect(screen.getByRole("heading", { name: "Сәйкес мердігерді табыңыз" })).toBeInTheDocument();
    expect(document.documentElement.lang).toBe("kk");
  });

  it("restores the selected locale from localStorage", async () => {
    const user = userEvent.setup();
    const first = render(<App />);
    await user.click(screen.getByRole("button", { name: "EN" }));
    await waitFor(() => expect(window.localStorage.getItem("apex-match-locale")).toBe("en"));
    first.unmount();

    render(<App />);
    expect(screen.getByRole("heading", { name: "Find the right contractor" })).toBeInTheDocument();
  });

  it("keeps canonical select values when labels are translated", async () => {
    const user = userEvent.setup();
    render(<App />);
    await user.click(screen.getByRole("button", { name: "EN" }));
    const category = screen.getByRole("combobox", { name: /Category/ });
    expect(category).toHaveValue("Ведущий");
    expect(within(category).getByRole("option", { name: "Host" })).toHaveAttribute("value", "Ведущий");
  });

  it("renders English static UI", () => {
    window.localStorage.setItem("apex-match-locale", "en");
    render(<App />);
    expect(screen.getByText("Event contractor matching")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Find contractors" })).toBeInTheDocument();
  });

  it("renders Kazakh static UI", () => {
    window.localStorage.setItem("apex-match-locale", "kk");
    render(<App />);
    expect(screen.getByText("Іс-шара мердігерлерін таңдау")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Мердігерлерді табу" })).toBeInTheDocument();
  });
});
