import { useLocale } from "../../i18n/LocaleContext";

export function LoadingState() {
  const { t } = useLocale();
  return (
    <div className="loading-state" role="status" aria-live="polite">
      <div className="results-intro">
        <div>
          <p className="eyebrow">{t("loading.eyebrow")}</p>
          <h2>{t("loading.title")}</h2>
        </div>
        <span className="loading-dot" aria-hidden="true" />
      </div>
      <p className="loading-copy">{t("loading.copy")}</p>
      <div className="skeleton-list" aria-hidden="true">
        {[0, 1, 2].map((item) => (
          <div className="skeleton-card" key={item}>
            <div className="skeleton-line short" />
            <div className="skeleton-line medium" />
            <div className="skeleton-line" />
            <div className="skeleton-line" />
          </div>
        ))}
      </div>
    </div>
  );
}
