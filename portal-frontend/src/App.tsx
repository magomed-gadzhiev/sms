import { lazy, Suspense } from 'react';
import { Routes, Route, Navigate, Outlet, useLocation, useParams } from 'react-router-dom';
import { useAuth } from './contexts/AuthContext';
import { RequireRole } from './components/RequireRole';
import { RequireReseller } from './components/RequireReseller';
import { UserLayout } from './components/layout/UserLayout';
import { PublicLayout } from './components/layout/PublicLayout';
import { LandingPage } from './pages/public/LandingPage';
import { PricingPage } from './pages/public/PricingPage';
import { FeaturesPage } from './pages/public/FeaturesPage';
import { DocsPage } from './pages/public/DocsPage';
import { DocArticlePage } from './pages/public/DocArticlePage';
import { BlogPage } from './pages/public/BlogPage';
import { BlogPostPage } from './pages/public/BlogPostPage';
import { AboutPage } from './pages/public/AboutPage';
import { ContactPage } from './pages/public/ContactPage';
import { LoginPage } from './pages/auth/LoginPage';
import { RegisterPage } from './pages/auth/RegisterPage';
import { PasswordResetRequestPage } from './pages/auth/PasswordResetRequestPage';
import { PasswordResetPage } from './pages/auth/PasswordResetPage';
import { ProfilePage } from './pages/profile/ProfilePage';
const CommandCenter = lazy(() => import('./pages/CommandCenter').then((m) => ({ default: m.CommandCenter })));
const NotificationsPage = lazy(() => import('./pages/notifications/NotificationsPage').then((m) => ({ default: m.NotificationsPage })));
import { MessagesPage } from './pages/messages/MessagesPage';
import { APIKeysPage } from './pages/api-keys/APIKeysPage';
import { AnalyticsPage } from './pages/analytics/AnalyticsPage';
import { WebhooksPage } from './pages/webhooks/WebhooksPage';
import { SubAccountsListPage } from './pages/sub-accounts/SubAccountsListPage';
import { SubAccountDetailPage } from './pages/sub-accounts/SubAccountDetailPage';
import { AuditLogPage } from './pages/audit/AuditLogPage';
import { ProvidersPage } from './pages/providers/ProvidersPage';
import { ProviderWizardPage } from './pages/providers/ProviderWizardPage';
import { ContactListsPage } from './pages/contacts/ContactListsPage';
import { ContactListDetailPage } from './pages/contacts/ContactListDetailPage';
import { ImportWizardPage } from './pages/contacts/ImportWizardPage';
import { OptOutListPage } from './pages/contacts/OptOutListPage';
import { CampaignsPage } from './pages/campaigns/CampaignsPage';
import { CampaignWizardPage } from './pages/campaigns/CampaignWizardPage';
import { CampaignDetailPage } from './pages/campaigns/CampaignDetailPage';
import { CampaignSchedulesPage } from './pages/campaigns/CampaignSchedulesPage';
import { TemplatesPage } from './pages/templates/TemplatesPage';
import { CompaniesPage } from './pages/companies/CompaniesPage';
import { CompanyDetailPage } from './pages/companies/CompanyDetailPage';
import { SenderNamesPage } from './pages/sender-names/SenderNamesPage';
import { SenderNameDetailPage } from './pages/sender-names/SenderNameDetailPage';
import { SenderNameOperatorsPage } from './pages/sender-names/SenderNameOperatorsPage';
import { SenderNameBillingHistory } from './pages/sender-names/SenderNameBillingHistory';
import { BillingPage } from './pages/billing/BillingPage';
import { MessageDetailPage } from './pages/messages/MessageDetailPage';
import { TariffsPage } from './pages/tariffs/TariffsPage';
import { LookupPage } from './pages/lookup/LookupPage';
import { DomainsPage } from './pages/settings/DomainsPage';
import { NotificationSettingsPage } from './pages/settings/NotificationSettingsPage';
import { SmppSettingsPage } from './pages/settings/SmppSettingsPage';
import { DefaultSendersPage } from './pages/settings/DefaultSendersPage';
import { SegmentsPage } from './pages/segments/SegmentsPage';
import { SegmentDetailPage } from './pages/segments/SegmentDetailPage';
import { CascadeHistoryPage } from './pages/cascade-history/CascadeHistoryPage';
import { CascadeDeliveryDetail } from './pages/cascade-history/CascadeDeliveryDetail';
import { RoutingPage } from './pages/routing/RoutingPage';
import { QuickSendPage } from './pages/quick-send/QuickSendPage';
import { ModerationPage } from './pages/network/ModerationPage';
import { NetworkDashboardPage } from './pages/network/NetworkDashboardPage';
import { NetworkRoutingPage } from './pages/network/NetworkRoutingPage';
import { NetworkTariffsPage } from './pages/network/NetworkTariffsPage';
import { NetworkAnalyticsPage } from './pages/network/NetworkAnalyticsPage';
const NetworkStatisticsPage = lazy(() => import('./pages/network/NetworkStatisticsPage'));

const AdminLayout = lazy(() => import('./pages/admin/AdminLayout').then((m) => ({ default: m.AdminLayout })));
const AdminClientsPage = lazy(() => import('./pages/admin/ClientsPage').then((m) => ({ default: m.ClientsPage })));
const AdminProvidersPage = lazy(() => import('./pages/admin/ProvidersPage').then((m) => ({ default: m.ProvidersPage })));
const AdminRoutesPage = lazy(() => import('./pages/admin/RoutesPage').then((m) => ({ default: m.RoutesPage })));
const AdminBillingPage = lazy(() => import('./pages/admin/billing/BillingPage').then((m) => ({ default: m.BillingPage })));
const AdminMonitoringPage = lazy(() => import('./pages/admin/MonitoringPage').then((m) => ({ default: m.MonitoringPage })));
const AdminAnalyticsPage = lazy(() => import('./pages/admin/AnalyticsPage').then((m) => ({ default: m.AnalyticsPage })));
const AdminTemplatesPage = lazy(() => import('./pages/admin/templates/TemplatesPage').then((m) => ({ default: m.TemplatesPage })));
const AdminSenderNamesPage = lazy(() => import('./pages/admin/SenderNamesAdminPage').then((m) => ({ default: m.SenderNamesAdminPage })));
const AdminSenderNameDetailPage = lazy(() => import('./pages/admin/sender-names/SenderNameDetailPage').then((m) => ({ default: m.SenderNameDetailPage })));
const AdminDetalizationPage = lazy(() => import('./pages/admin/detalization/DetalizationPage').then((m) => ({ default: m.DetalizationPage })));
const AdminWebhooksPage = lazy(() => import('./pages/admin/WebhooksPage').then((m) => ({ default: m.WebhooksPage })));
const AdminHLRPage = lazy(() => import('./pages/admin/HLRPage').then((m) => ({ default: m.HLRPage })));
const AdminCountriesPage = lazy(() => import('./pages/admin/CountriesPage').then((m) => ({ default: m.CountriesPage })));
const AdminAuditLogPage = lazy(() => import('./pages/admin/AuditLogPage').then((m) => ({ default: m.AuditLogPage })));
const AdminTarificationPage = lazy(() => import('./pages/admin/tarification/TarificationPage').then((m) => ({ default: m.TarificationPage })));
const AdminDashboardPage = lazy(() => import('./pages/admin/dashboard/DashboardPage').then((m) => ({ default: m.DashboardPage })));
const AdminUsersPage = lazy(() => import('./pages/admin/users/UsersPage').then((m) => ({ default: m.UsersPage })));
const AdminRolesPage = lazy(() => import('./pages/admin/users/RolesPage').then((m) => ({ default: m.RolesPage })));
const AdminChannelsPage = lazy(() => import('./pages/channels/ChannelsPage').then((m) => ({ default: m.ChannelsPage })));
const AdminDeliveryStrategiesPage = lazy(() => import('./pages/delivery-strategies/DeliveryStrategiesPage').then((m) => ({ default: m.DeliveryStrategiesPage })));
const AdminClientRoutesPage = lazy(() => import('./pages/admin/client-routes/ClientRoutesPage').then((m) => ({ default: m.ClientRoutesPage })));
const AdminIndividualTariffsPage = lazy(() => import('./pages/admin/tarification/IndividualTariffsPage').then((m) => ({ default: m.IndividualTariffsPage })));
const AdminSettingsPage = lazy(() => import('./pages/admin/settings/SettingsPage').then((m) => ({ default: m.SettingsPage })));
const AdminLegalEntitiesPage = lazy(() => import('./pages/admin/legal-entities/LegalEntitiesPage').then((m) => ({ default: m.LegalEntitiesPage })));
const AdminContractsPage = lazy(() => import('./pages/admin/contracts/ContractsPage').then((m) => ({ default: m.ContractsPage })));
const AdminOperatorTemplatesPage = lazy(() => import('./pages/admin/operator-templates/OperatorTemplatesPage').then((m) => ({ default: m.OperatorTemplatesPage })));
const AdminConnectionsPage = lazy(() => import('./pages/admin/connections/ConnectionsPage').then((m) => ({ default: m.ConnectionsPage })));

function RequireAuth() {
  const { isAuthenticated, loading } = useAuth();
  const location = useLocation();

  if (loading) return (
    <div className="min-h-screen flex items-center justify-center bg-gray-50" role="status" aria-live="polite">
      <div className="text-center">
        <div className="w-8 h-8 border-2 border-primary border-t-transparent rounded-full animate-spin mx-auto mb-3" />
        <p className="text-sm text-gray-500">Загрузка...</p>
      </div>
    </div>
  );
  if (!isAuthenticated) return <Navigate to="/login" state={{ from: location }} replace />;
  return <UserLayout />;
}

function SubAccountRedirect() {
  const { id } = useParams();
  return <Navigate to={`/network/sub-accounts/${id}`} replace />;
}

export function App() {
  return (
    <Routes>
      <Route element={<PublicLayout />}>
        <Route path="/" element={<LandingPage />} />
        <Route path="/pricing" element={<PricingPage />} />
        <Route path="/features" element={<FeaturesPage />} />
        <Route path="/docs" element={<DocsPage />} />
        <Route path="/docs/:slug" element={<DocArticlePage />} />
        <Route path="/blog" element={<BlogPage />} />
        <Route path="/blog/:slug" element={<BlogPostPage />} />
        <Route path="/about" element={<AboutPage />} />
        <Route path="/contact" element={<ContactPage />} />
        <Route path="/en" element={<LandingPage />} />
        <Route path="/en/pricing" element={<PricingPage />} />
        <Route path="/en/features" element={<FeaturesPage />} />
        <Route path="/en/docs" element={<DocsPage />} />
        <Route path="/en/docs/:slug" element={<DocArticlePage />} />
        <Route path="/en/blog" element={<BlogPage />} />
        <Route path="/en/blog/:slug" element={<BlogPostPage />} />
        <Route path="/en/about" element={<AboutPage />} />
        <Route path="/en/contact" element={<ContactPage />} />
      </Route>

      <Route path="/login" element={<LoginPage />} />
      <Route path="/register" element={<RegisterPage />} />
      <Route path="/reset-password-request" element={<PasswordResetRequestPage />} />
      <Route path="/reset-password" element={<PasswordResetPage />} />

      <Route element={<RequireAuth />}>
        <Route path="/dashboard" element={<Navigate to="/command-center" replace />} />
        <Route path="/command-center" element={<Suspense fallback={null}><CommandCenter /></Suspense>} />
        <Route path="/notifications" element={<Suspense fallback={null}><NotificationsPage /></Suspense>} />
        <Route path="/messages" element={<MessagesPage />} />
        <Route path="/messages/:id" element={<MessageDetailPage />} />
        <Route path="/contact-lists" element={<ContactListsPage />} />
        <Route path="/contact-lists/:id" element={<ContactListDetailPage />} />
        <Route path="/contact-lists/:id/import" element={<ImportWizardPage />} />
        <Route path="/opt-out" element={<OptOutListPage />} />
        <Route path="/segments" element={<SegmentsPage />} />
        <Route path="/segments/new" element={<SegmentDetailPage />} />
        <Route path="/segments/:id" element={<SegmentDetailPage />} />
        <Route path="/quick-send" element={<QuickSendPage />} />
        <Route path="/campaigns" element={<CampaignsPage />} />
        <Route path="/campaigns/new" element={<CampaignWizardPage />} />
        <Route path="/campaigns/:id" element={<CampaignDetailPage />} />
        <Route path="/campaign-schedules" element={<CampaignSchedulesPage />} />
        <Route path="/api-keys" element={<APIKeysPage />} />
        <Route path="/webhooks" element={<WebhooksPage />} />
        <Route path="/analytics" element={<AnalyticsPage />} />
        <Route path="/providers" element={<ProvidersPage />} />
        <Route path="/providers/new" element={<ProviderWizardPage />} />
        <Route path="/routing" element={<RoutingPage />} />
        <Route path="/templates" element={<TemplatesPage />} />
        <Route path="/companies" element={<CompaniesPage />} />
        <Route path="/companies/:id" element={<CompanyDetailPage />} />
        <Route path="/sender-names" element={<SenderNamesPage />} />
        <Route path="/sender-names/:id" element={<SenderNameDetailPage />} />
        <Route path="/sender-names/:id/operators" element={<SenderNameOperatorsPage />} />
        <Route path="/sender-registrations/:id/billing" element={<SenderNameBillingHistory />} />
        <Route path="/billing" element={<BillingPage />} />
        <Route path="/tariffs" element={<TariffsPage />} />
        <Route path="/lookup" element={<LookupPage />} />
        <Route path="/profile" element={<ProfilePage />} />
        <Route path="/audit-log" element={<AuditLogPage />} />
        <Route path="/settings/domains" element={<DomainsPage />} />
        <Route path="/settings/notifications" element={<NotificationSettingsPage />} />
        <Route path="/settings/smpp" element={<SmppSettingsPage />} />
        <Route path="/settings/default-senders" element={<DefaultSendersPage />} />
        <Route path="/cascade/history" element={<CascadeHistoryPage />} />
        <Route path="/cascade/history/:id" element={<CascadeDeliveryDetail />} />

        {/* Network mode (reseller) */}
        <Route path="/network" element={<RequireReseller><Outlet /></RequireReseller>}>
          <Route index element={<Navigate to="/network/dashboard" replace />} />
          <Route path="dashboard" element={<NetworkDashboardPage />} />
          <Route path="sub-accounts" element={<SubAccountsListPage />} />
          <Route path="sub-accounts/:id" element={<SubAccountDetailPage />} />
          <Route path="moderation" element={<ModerationPage />} />
          <Route path="routing" element={<NetworkRoutingPage />} />
          <Route path="tariffs" element={<NetworkTariffsPage />} />
          <Route path="statistics" element={<NetworkStatisticsPage />} />
        </Route>

        {/* Backward compat redirects */}
        <Route path="/sub-accounts" element={<Navigate to="/network/sub-accounts" replace />} />
        <Route path="/sub-accounts/:id" element={<SubAccountRedirect />} />
      </Route>

      <Route
        path="/admin/*"
        element={
          <RequireRole role="admin">
            <Suspense fallback={<div className="p-8 text-center text-gray-400">Loading admin...</div>}>
              <AdminLayout />
            </Suspense>
          </RequireRole>
        }
      >
        <Route index element={<Navigate to="/admin/dashboard" replace />} />
        <Route path="dashboard" element={<Suspense fallback={null}><AdminDashboardPage /></Suspense>} />
        <Route path="clients" element={<Suspense fallback={null}><AdminClientsPage /></Suspense>} />
        <Route path="providers" element={<Suspense fallback={null}><AdminProvidersPage /></Suspense>} />
        <Route path="routes" element={<Suspense fallback={null}><AdminRoutesPage /></Suspense>} />
        <Route path="connections" element={<Suspense fallback={null}><AdminConnectionsPage /></Suspense>} />
        <Route path="billing" element={<Suspense fallback={null}><AdminBillingPage /></Suspense>} />
        <Route path="monitoring" element={<Suspense fallback={null}><AdminMonitoringPage /></Suspense>} />
        <Route path="analytics" element={<Suspense fallback={null}><AdminAnalyticsPage /></Suspense>} />
        <Route path="templates" element={<Suspense fallback={null}><AdminTemplatesPage /></Suspense>} />
        <Route path="sender-names" element={<Suspense fallback={null}><AdminSenderNamesPage /></Suspense>} />
        <Route path="sender-names/:id" element={<Suspense fallback={null}><AdminSenderNameDetailPage /></Suspense>} />
        <Route path="webhooks" element={<Suspense fallback={null}><AdminWebhooksPage /></Suspense>} />
        <Route path="hlr" element={<Suspense fallback={null}><AdminHLRPage /></Suspense>} />
        <Route path="countries" element={<Suspense fallback={null}><AdminCountriesPage /></Suspense>} />
        <Route path="tarification" element={<Suspense fallback={null}><AdminTarificationPage /></Suspense>} />
        <Route path="audit" element={<Suspense fallback={null}><AdminAuditLogPage /></Suspense>} />
        <Route path="detalization" element={<Suspense fallback={null}><AdminDetalizationPage /></Suspense>} />
        <Route path="users" element={<Suspense fallback={null}><AdminUsersPage /></Suspense>} />
        <Route path="users/roles" element={<Suspense fallback={null}><AdminRolesPage /></Suspense>} />
        <Route path="channels" element={<Suspense fallback={null}><AdminChannelsPage /></Suspense>} />
        <Route path="delivery-strategies" element={<Suspense fallback={null}><AdminDeliveryStrategiesPage /></Suspense>} />
        <Route path="individual-routes" element={<Suspense fallback={null}><AdminClientRoutesPage /></Suspense>} />
        <Route path="individual-tariffs" element={<Suspense fallback={null}><AdminIndividualTariffsPage /></Suspense>} />
        <Route path="settings" element={<Suspense fallback={null}><AdminSettingsPage /></Suspense>} />
        <Route path="legal-entities" element={<Suspense fallback={null}><AdminLegalEntitiesPage /></Suspense>} />
        <Route path="contracts" element={<Suspense fallback={null}><AdminContractsPage /></Suspense>} />
        <Route path="operator-templates" element={<Suspense fallback={null}><AdminOperatorTemplatesPage /></Suspense>} />
      </Route>

      <Route path="*" element={<Navigate to="/command-center" replace />} />
    </Routes>
  );
}
