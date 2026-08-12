import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import App from "@/App";
import { ThemeProvider } from "@/components/theme-provider";
import { I18nProvider } from "@/lib/i18n";
import "@/index.css";

const container = document.getElementById("root");
if (!container) {
  throw new Error("index.html is missing #root");
}

createRoot(container).render(
  <StrictMode>
    <I18nProvider>
      <ThemeProvider defaultTheme="dark">
        <App />
      </ThemeProvider>
    </I18nProvider>
  </StrictMode>,
);
