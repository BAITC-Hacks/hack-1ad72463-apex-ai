import { useLocale } from "../../i18n/LocaleContext";
import type { TranslationKey } from "../../i18n/translations";

export type TechnicalErrorKind = "catalog" | "network" | "invalid" | "unexpected";

const copyKeys: Record<TechnicalErrorKind, { title: TranslationKey; message: TranslationKey }> = {
  catalog: { title: "errors.catalogTitle", message: "errors.catalogCopy" },
  network: { title: "errors.networkTitle", message: "errors.networkCopy" },
  invalid: { title: "errors.invalidTitle", message: "errors.invalidCopy" },
  unexpected: { title: "errors.unexpectedTitle", message: "errors.unexpectedCopy" },
};

interface ErrorStateProps {
  kind: TechnicalErrorKind;
  onRetry: () => void;
}

export function ErrorState({ kind, onRetry }: ErrorStateProps) {
  const { t } = useLocale();
  return (
    <div className="error-state" role="alert">
      <div className="empty-icon danger" aria-hidden="true">!</div>
      <p className="eyebrow">{t("errors.eyebrow")}</p>
      <h2>{t(copyKeys[kind].title)}</h2>
      <p>{t(copyKeys[kind].message)}</p>
      <button className="secondary-button" type="button" onClick={onRetry}>{t("errors.retry")}</button>
    </div>
  );
}
