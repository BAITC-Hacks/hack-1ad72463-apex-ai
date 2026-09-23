import { createRecommendationGateway, appConfig } from "./config";
import { RecommendationPage } from "../features/recommendations/RecommendationPage";
import { LocaleProvider, useLocale } from "../i18n/LocaleContext";
import type { Locale } from "../i18n/translations";

const gateway = createRecommendationGateway(appConfig);
const localeOptions: Array<{ locale: Locale; label: string }> = [
  { locale: "ru", label: "RU" },
  { locale: "kk", label: "KZ" },
  { locale: "en", label: "EN" },
];

function AppContent() {
  const { locale, setLocale, t } = useLocale();
  return (
    <div className="app-shell">
      <header className="topbar">
        <a className="brand" href="#main" aria-label={t("app.brandAria")}>
          <span className="brand-mark" aria-hidden="true">A</span>
          <span>Apex Match</span>
        </a>
        <div className="topbar-actions">
          <div className="locale-switcher" role="group" aria-label={t("app.localeSwitcher")}>
            {localeOptions.map((option) => (
              <button
                aria-pressed={locale === option.locale}
                className={locale === option.locale ? "locale-button active" : "locale-button"}
                key={option.locale}
                onClick={() => setLocale(option.locale)}
                type="button"
              >
                {option.label}
              </button>
            ))}
          </div>
          <div className="topbar-badges">
            <span className="badge badge-neutral">HackAlem AI</span>
            {appConfig.apiMode === "mock" && <span className="badge badge-mock">{t("app.mockBadge")}</span>}
          </div>
        </div>
      </header>
      <RecommendationPage gateway={gateway} />
    </div>
  );
}

export function App() {
  return <LocaleProvider><AppContent /></LocaleProvider>;
}
