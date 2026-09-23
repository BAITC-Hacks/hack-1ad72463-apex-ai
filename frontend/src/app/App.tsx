import { createRecommendationGateway, appConfig } from "./config";
import { RecommendationPage } from "../features/recommendations/RecommendationPage";

const gateway = createRecommendationGateway(appConfig);

export function App() {
  return (
    <div className="app-shell">
      <header className="topbar">
        <a className="brand" href="#main" aria-label="Apex Match — к форме поиска">
          <span className="brand-mark" aria-hidden="true">A</span>
          <span>Apex Match</span>
        </a>
        <div className="topbar-badges">
          <span className="badge badge-neutral">HackAlem AI</span>
          {appConfig.apiMode === "mock" && <span className="badge badge-mock">DEMO / MOCK API</span>}
        </div>
      </header>
      <RecommendationPage gateway={gateway} />
    </div>
  );
}
