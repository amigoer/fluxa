import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom"
import { Toaster } from "sonner"
import { AuthProvider, useAuth } from "@/lib/auth"
import { RequireAdmin, RequireEmployee } from "@/components/shared/require-auth"
import { ConsoleLayout } from "@/layouts/console-layout"
import { adminHasAnyPermission } from "@/layouts/nav-config"

import { SetupPage } from "@/pages/login/setup-page"
import { LoginPage } from "@/pages/login/login-page"

import { OverviewPage } from "@/pages/admin/overview-page"
import { QuickstartPage } from "@/pages/admin/quickstart-page"
import { ProvidersPage } from "@/pages/admin/providers-page"
import { ModelsRoutingPage } from "@/pages/admin/models-routing-page"
import { PlaygroundPage } from "@/pages/admin/playground-page"
import { ProcurementPage } from "@/pages/admin/procurement-page"
import { MembersPage } from "@/pages/admin/members-page"
import { RolesPage } from "@/pages/admin/roles-page"
import { KeysPage } from "@/pages/admin/keys-page"
import { IdentitySourcesPage } from "@/pages/admin/identity-sources-page"
import { NotifyChannelsPage } from "@/pages/admin/notify-channels-page"
import { DlpRulesPage } from "@/pages/admin/dlp-rules-page"
import { SecurityEventsPage } from "@/pages/admin/security-events-page"
import { CallLogsPage } from "@/pages/admin/call-logs-page"
import { OperationLogsPage } from "@/pages/admin/operation-logs-page"

import { UsagePage } from "@/pages/employee/usage-page"
import { PricingPage } from "@/pages/employee/pricing-page"
import { MyRoutingPage } from "@/pages/employee/my-routing-page"
import { QuotaRequestsPage } from "@/pages/employee/quota-requests-page"

function Landing() {
  const { loading, member, permissions } = useAuth()
  if (loading) return null
  if (!member) return <Navigate to="/login" replace />
  return <Navigate to={adminHasAnyPermission(permissions) ? "/admin/overview" : "/app/usage"} replace />
}

function AppRoutes() {
  return (
    <Routes>
      <Route path="/setup" element={<SetupPage />} />
      <Route path="/login" element={<LoginPage />} />

      <Route path="/" element={<Landing />} />

      <Route
        path="/admin"
        element={
          <RequireAdmin>
            <ConsoleLayout persona="admin" />
          </RequireAdmin>
        }
      >
        <Route path="overview" element={<OverviewPage />} />
        <Route path="quickstart" element={<QuickstartPage />} />
        <Route path="providers" element={<ProvidersPage />} />
        <Route path="models-routing" element={<ModelsRoutingPage />} />
        <Route path="playground" element={<PlaygroundPage />} />
        <Route path="procurement" element={<ProcurementPage />} />
        <Route path="members" element={<MembersPage />} />
        <Route path="roles" element={<RolesPage />} />
        <Route path="keys" element={<KeysPage />} />
        <Route path="identity-sources" element={<IdentitySourcesPage />} />
        <Route path="notify-channels" element={<NotifyChannelsPage />} />
        <Route path="dlp-rules" element={<DlpRulesPage />} />
        <Route path="security-events" element={<SecurityEventsPage />} />
        <Route path="call-logs" element={<CallLogsPage />} />
        <Route path="operation-logs" element={<OperationLogsPage />} />
      </Route>

      <Route
        path="/app"
        element={
          <RequireEmployee>
            <ConsoleLayout persona="employee" />
          </RequireEmployee>
        }
      >
        <Route path="usage" element={<UsagePage />} />
        <Route path="pricing" element={<PricingPage />} />
        <Route path="routing" element={<MyRoutingPage />} />
        <Route path="quota-requests" element={<QuotaRequestsPage />} />
      </Route>

      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <AppRoutes />
        {/* Top-center: bottom-right sat on top of the footer on a phone,
            and on the primary action of whatever form had just been
            submitted. Both offsets clear the 56px top bar so a toast never
            covers the breadcrumb or the sidebar button -- sonner keeps a
            separate mobile offset, and setting only `offset` left narrow
            screens on its 16px default. */}
        <Toaster position="top-center" offset={{ top: "68px" }} mobileOffset={{ top: "68px" }} />
      </AuthProvider>
    </BrowserRouter>
  )
}
