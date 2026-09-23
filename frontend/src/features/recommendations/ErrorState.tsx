export type TechnicalErrorKind = "catalog" | "network" | "invalid" | "unexpected";

const copy: Record<TechnicalErrorKind, { title: string; message: string }> = {
  catalog: { title: "Сервис временно недоступен", message: "Каталог пока не готов к поиску. Попробуйте ещё раз через несколько секунд." },
  network: { title: "Не удалось связаться с сервисом", message: "Проверьте подключение и повторите запрос. Введённые параметры сохранены." },
  invalid: { title: "Получен некорректный ответ сервиса", message: "Мы не показываем частичные или непроверенные данные. Повторите запрос после проверки API." },
  unexpected: { title: "Не удалось выполнить запрос", message: "Возникла техническая ошибка. Параметры формы сохранены." },
};

interface ErrorStateProps {
  kind: TechnicalErrorKind;
  onRetry: () => void;
}

export function ErrorState({ kind, onRetry }: ErrorStateProps) {
  return (
    <div className="error-state" role="alert">
      <div className="empty-icon danger" aria-hidden="true">!</div>
      <p className="eyebrow">Техническая ошибка</p>
      <h2>{copy[kind].title}</h2>
      <p>{copy[kind].message}</p>
      <button className="secondary-button" type="button" onClick={onRetry}>Повторить</button>
    </div>
  );
}
