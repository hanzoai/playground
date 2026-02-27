import { Route, BrowserRouter as Router, Routes } from "react-router-dom";
import { SidebarNew } from "./components/Navigation/SidebarNew";
import { TopNavigation } from "./components/Navigation/TopNavigation";
import { RootRedirect } from "./components/RootRedirect";
import { navigationSections } from "./config/navigation";
import { ModeProvider } from "./contexts/ModeContext";
import { ThemeProvider } from "./components/theme-provider";
import { useFocusManagement } from "./hooks/useFocusManagement";
import { SidebarProvider, SidebarInset } from "./components/ui/sidebar";
import { ControlPlanePage } from "./pages/ControlPlanePage";
import { EnhancedDashboardPage } from "./pages/EnhancedDashboardPage";
import { ExecutionsPage } from "./pages/ExecutionsPage";
import { EnhancedExecutionDetailPage } from "./pages/EnhancedExecutionDetailPage";
import { EnhancedWorkflowDetailPage } from "./pages/EnhancedWorkflowDetailPage";
import { NodeDetailPage } from "./pages/NodeDetailPage";
import { NodesPage } from "./pages/NodesPage";
import { PackagesPage } from "./pages/PackagesPage";
import { BotDetailPage } from "./pages/BotDetailPage.tsx";
import { WorkflowsPage } from "./pages/WorkflowsPage.tsx";
import { WorkflowDeckGLTestPage } from "./pages/WorkflowDeckGLTestPage";
import { DIDExplorerPage } from "./pages/DIDExplorerPage";
import { CredentialsPage } from "./pages/CredentialsPage";
import { ObservabilityWebhookSettingsPage } from "./pages/ObservabilityWebhookSettingsPage";
import { CanvasPage } from "./pages/CanvasPage";
import { LaunchPage } from "./pages/LaunchPage";
import { TeamPage } from "./pages/TeamPage";
import { SpacesPage } from "./pages/SpacesPage";
import { SpaceSettingsPage } from "./pages/SpaceSettingsPage";
import { AuthProvider } from "./contexts/AuthContext";
import { AuthGuard } from "./components/AuthGuard";
import { AuthCallbackPage } from "./components/AuthCallbackPage";
import { GatewaySettings } from "./components/settings/GatewaySettings";
import { PreferencesSettings } from "./components/settings/PreferencesSettings";
import { NetworkSettings } from "./components/settings/NetworkSettings";
import { NetworkPage } from "./pages/NetworkPage";
import { MarketplacePage } from "./pages/MarketplacePage";
import { ListingDetailPage } from "./pages/ListingDetailPage";
import { CreateListingPage } from "./pages/CreateListingPage";
import { SellerDashboardPage } from "./pages/SellerDashboardPage";
import { GlobalCommandPalette } from "./components/GlobalCommandPalette";
import { PreferencesOnboarding } from "./components/onboarding/PreferencesOnboarding";
import { usePreferencesStore } from "./stores/preferencesStore";
import { useNotificationSound } from "./hooks/useNotificationSound";

function SettingsPage() {
  return <GatewaySettings />;
}

function PreferencesGate({ children }: { children: React.ReactNode }) {
  const onboardingComplete = usePreferencesStore((s) => s.onboardingComplete);

  if (!onboardingComplete) {
    return (
      <PreferencesOnboarding
        onComplete={() => usePreferencesStore.getState().setOnboardingComplete(true)}
      />
    );
  }

  return <>{children}</>;
}

function NotificationSoundProvider({ children }: { children: React.ReactNode }) {
  useNotificationSound();
  return <>{children}</>;
}

function AppContent() {
  // Use focus management hook to ensure trackpad navigation works
  useFocusManagement();

  return (
    <SidebarProvider defaultOpen={true}>
      <div className="flex h-screen w-full bg-background text-foreground transition-colors">
        {/* Sidebar */}
        <SidebarNew sections={navigationSections} />

        {/* Main Content */}
        <SidebarInset>
          {/* Top Navigation */}
          <TopNavigation />

          {/* Main Content Area */}
          <main className="flex flex-1 min-w-0 flex-col overflow-y-auto overflow-x-hidden">
            <Routes>
              <Route path="/" element={<div className="p-4 md:p-6 lg:p-8 min-h-full"><RootRedirect /></div>} />
              <Route path="/launch" element={<div className="p-4 md:p-6 lg:p-8 min-h-full"><LaunchPage /></div>} />
              <Route path="/metrics" element={<div className="p-4 md:p-6 lg:p-8 min-h-full"><EnhancedDashboardPage /></div>} />
              <Route path="/dashboard" element={<div className="p-4 md:p-6 lg:p-8 min-h-full"><EnhancedDashboardPage /></div>} />
              <Route path="/nodes" element={<div className="p-4 md:p-6 lg:p-8 min-h-full"><NodesPage /></div>} />
              <Route path="/nodes/:nodeId" element={<div className="p-4 md:p-6 lg:p-8 min-h-full"><NodeDetailPage /></div>} />
              <Route path="/bots/all" element={<div className="p-4 md:p-6 lg:p-8 min-h-full"><ControlPlanePage /></div>} />
              <Route
                path="/bots/:fullBotId"
                element={<div className="p-4 md:p-6 lg:p-8 min-h-full"><BotDetailPage /></div>}
              />
              <Route path="/executions" element={<div className="p-4 md:p-6 lg:p-8 min-h-full"><ExecutionsPage /></div>} />
              <Route
                path="/executions/:executionId"
                element={<div className="p-4 md:p-6 lg:p-8 min-h-full"><EnhancedExecutionDetailPage /></div>}
              />
              <Route path="/workflows" element={<div className="p-4 md:p-6 lg:p-8 min-h-full"><WorkflowsPage /></div>} />
              <Route
                path="/workflows/:workflowId"
                element={<EnhancedWorkflowDetailPage />}
              />
              <Route path="/packages" element={<div className="p-4 md:p-6 lg:p-8 min-h-full"><PackagesPage /></div>} />
              <Route path="/settings" element={<div className="p-4 md:p-6 lg:p-8 min-h-full"><SettingsPage /></div>} />
              <Route path="/settings/preferences" element={<div className="p-4 md:p-6 lg:p-8 min-h-full"><PreferencesSettings /></div>} />
              <Route path="/settings/network" element={<div className="p-4 md:p-6 lg:p-8 min-h-full"><NetworkSettings /></div>} />
              <Route path="/network" element={<div className="p-4 md:p-6 lg:p-8 min-h-full"><NetworkPage /></div>} />
              <Route path="/marketplace" element={<div className="p-4 md:p-6 lg:p-8 min-h-full"><MarketplacePage /></div>} />
              <Route path="/marketplace/listing/:listingId" element={<div className="p-4 md:p-6 lg:p-8 min-h-full"><ListingDetailPage /></div>} />
              <Route path="/marketplace/create" element={<div className="p-4 md:p-6 lg:p-8 min-h-full"><CreateListingPage /></div>} />
              <Route path="/marketplace/seller" element={<div className="p-4 md:p-6 lg:p-8 min-h-full"><SellerDashboardPage /></div>} />
              <Route path="/agents" element={<div className="p-4 md:p-6 lg:p-8 min-h-full"><LaunchPage /></div>} />
              <Route path="/identity/dids" element={<div className="p-4 md:p-6 lg:p-8 min-h-full"><DIDExplorerPage /></div>} />
              <Route path="/identity/credentials" element={<div className="p-4 md:p-6 lg:p-8 min-h-full"><CredentialsPage /></div>} />
              <Route path="/settings/observability-webhook" element={<div className="p-4 md:p-6 lg:p-8 min-h-full"><ObservabilityWebhookSettingsPage /></div>} />
              <Route path="/playground" element={<div className="flex-1 min-h-0 relative"><CanvasPage /></div>} />
              <Route path="/spaces" element={<div className="p-4 md:p-6 lg:p-8 min-h-full"><SpacesPage /></div>} />
              <Route path="/spaces/settings" element={<div className="p-4 md:p-6 lg:p-8 min-h-full"><SpaceSettingsPage /></div>} />
              <Route path="/teams" element={<div className="p-4 md:p-6 lg:p-8 min-h-full"><TeamPage /></div>} />
              <Route path="/test/deckgl" element={<div className="p-4 md:p-6 lg:p-8 min-h-full"><WorkflowDeckGLTestPage /></div>} />
            </Routes>
          </main>
        </SidebarInset>
      </div>
      {/* Global command palette (Cmd+K) — available on all pages except playground */}
      <GlobalCommandPalette />
    </SidebarProvider>
  );
}

function AppRoutes() {
  return (
    <Routes>
      {/* OAuth callback must be outside AuthGuard */}
      <Route path="/auth/callback" element={<AuthCallbackPage />} />
      <Route
        path="/*"
        element={
          <AuthGuard>
            <PreferencesGate>
              <NotificationSoundProvider>
                <AppContent />
              </NotificationSoundProvider>
            </PreferencesGate>
          </AuthGuard>
        }
      />
    </Routes>
  );
}

function App() {
  return (
    <ThemeProvider
      attribute="class"
      defaultTheme="system"
      enableSystem
      disableTransitionOnChange
    >
      <ModeProvider>
        <AuthProvider>
          <Router basename={import.meta.env.VITE_BASE_PATH || "/"}>
            <AppRoutes />
          </Router>
        </AuthProvider>
      </ModeProvider>
    </ThemeProvider>
  );
}

export default App;
