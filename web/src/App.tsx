import { BrowserRouter, Route, Routes } from "react-router";

import { AppLayout } from "@/components/app-layout";
import { AuthProvider } from "@/components/auth-provider";
import { Toaster } from "@/components/ui/sonner";
import { DLPPage } from "@/pages/dlp";
import { KeysPage } from "@/pages/keys";
import { LoginPage } from "@/pages/login";
import { LogsPage } from "@/pages/logs";
import { OverviewPage } from "@/pages/overview";
import { ProvidersPage } from "@/pages/providers";
import { RegexModelsPage } from "@/pages/regex-models";
import { RoutesPage } from "@/pages/routes";
import { SettingsPage } from "@/pages/settings";
import { VirtualModelsPage } from "@/pages/virtual-models";

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          {/* Everything under AppLayout requires a session; the layout
              redirects to /login when there is not one. */}
          <Route element={<AppLayout />}>
            <Route index element={<OverviewPage />} />
            <Route path="providers" element={<ProvidersPage />} />
            <Route path="routes" element={<RoutesPage />} />
            <Route path="virtual-models" element={<VirtualModelsPage />} />
            <Route path="regex-models" element={<RegexModelsPage />} />
            <Route path="keys" element={<KeysPage />} />
            <Route path="logs" element={<LogsPage />} />
            <Route path="dlp" element={<DLPPage />} />
            <Route path="settings" element={<SettingsPage />} />
          </Route>
          {/* Unknown deep links land on the overview rather than a blank
              screen; the Go handler already serves index.html for them. */}
          <Route path="*" element={<OverviewPage />} />
        </Routes>
        <Toaster />
      </AuthProvider>
    </BrowserRouter>
  );
}
