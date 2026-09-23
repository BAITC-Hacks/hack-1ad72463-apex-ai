export function LoadingState() {
  return (
    <div className="loading-state" role="status" aria-live="polite">
      <div className="results-intro">
        <div>
          <p className="eyebrow">Идёт проверка условий</p>
          <h2>Подбираем варианты…</h2>
        </div>
        <span className="loading-dot" aria-hidden="true" />
      </div>
      <p className="loading-copy">Сверяем параметры с каталогом и календарём доступности.</p>
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
