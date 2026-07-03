import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter, Route, Routes } from "react-router-dom";

import AppLayout from "./layouts/AppLayout";
import AuditDetailPage from "./pages/AuditDetailPage";
import AuditsPage from "./pages/AuditsPage";
import DashboardPage from "./pages/DashboardPage";
import NewAuditPage from "./pages/NewAuditPage";
import SettingsPage from "./pages/SettingsPage";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      refetchOnWindowFocus: false,
    },
  },
});

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Routes>
          <Route element={<AppLayout />}>
            <Route path="/" element={<DashboardPage />} />
            <Route path="/audits" element={<AuditsPage />} />
            <Route path="/audits/new" element={<NewAuditPage />} />
            <Route path="/audits/:id" element={<AuditDetailPage />} />
            <Route path="/settings" element={<SettingsPage />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  );
}
