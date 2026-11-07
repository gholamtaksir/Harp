import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "./App";
import { FeedbackProvider } from "./components/ui";
import "./index.css";
import { StoreProvider } from "./lib/store";

// Disable the browser context menu outside text fields; the app is not a web page.
document.addEventListener("contextmenu", (event) => {
  const target = event.target as HTMLElement;
  if (!target.closest("input, textarea, .selectable")) event.preventDefault();
});

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <StoreProvider>
      <FeedbackProvider>
        <App />
      </FeedbackProvider>
    </StoreProvider>
  </StrictMode>,
);
