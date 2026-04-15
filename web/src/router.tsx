import { lazy, Suspense } from 'react'
import { Routes, Route } from 'react-router-dom'
import { RequireAuth } from '@/auth/RequireAuth'
import HomePage from '@/pages/home'
import ExplorePage from '@/pages/explore'
import GettingStartedPage from '@/pages/getting-started'
import ChangelogPage from '@/pages/changelog'
import PublisherDetailPage from '@/pages/publishers/detail'
import MCPListPage from '@/pages/mcp/list'
import MCPDetailPage from '@/pages/mcp/detail'
import AgentListPage from '@/pages/agents/list'
import AgentDetailPage from '@/pages/agents/detail'
import { AuthCallback } from '@/auth/AuthCallback'
import NotFoundPage from '@/pages/not-found'

// Admin pages are code-split: the admin bundle is only fetched when an
// authenticated user navigates to /admin/*. First-time public visitors never
// pay the cost of the admin surface (forms, editors, bulk actions).
const AdminLayout = lazy(() => import('@/pages/admin/layout'))
const AdminDashboard = lazy(() => import('@/pages/admin/dashboard'))
const AdminMCPList = lazy(() => import('@/pages/admin/mcp/list'))
const AdminMCPDetail = lazy(() => import('@/pages/admin/mcp/detail'))
const AdminMCPNew = lazy(() => import('@/pages/admin/mcp/new'))
const AdminAgentList = lazy(() => import('@/pages/admin/agents/list'))
const AdminAgentDetail = lazy(() => import('@/pages/admin/agents/detail'))
const AdminAgentNew = lazy(() => import('@/pages/admin/agents/new'))
const AdminPublisherList = lazy(() => import('@/pages/admin/publishers/list'))
const AdminPublisherDetail = lazy(() => import('@/pages/admin/publishers/detail'))
const AdminPublisherNew = lazy(() => import('@/pages/admin/publishers/new'))
const AdminApiKeys = lazy(() => import('@/pages/admin/api-keys'))
const AdminReports = lazy(() => import('@/pages/admin/reports'))

// Minimal fallback shown while an admin chunk is loading. Intentionally tiny —
// the admin surface is gated behind auth and the chunks are small, so a full
// skeleton screen would flash unnecessarily.
function AdminLoading() {
  return (
    <div className="flex min-h-[40vh] items-center justify-center text-sm text-muted-foreground">
      Loading…
    </div>
  )
}

export function AppRoutes() {
  return (
    <Routes>
      <Route path="/" element={<HomePage />} />
      <Route path="/explore" element={<ExplorePage />} />
      <Route path="/getting-started" element={<GettingStartedPage />} />
      <Route path="/changelog" element={<ChangelogPage />} />
      <Route path="/publishers/:slug" element={<PublisherDetailPage />} />
      <Route path="/mcp" element={<MCPListPage />} />
      <Route path="/mcp/:ns/:slug" element={<MCPDetailPage />} />
      <Route path="/agents" element={<AgentListPage />} />
      <Route path="/agents/:ns/:slug" element={<AgentDetailPage />} />
      <Route path="/auth/callback" element={<AuthCallback />} />
      <Route
        path="/admin"
        element={
          <RequireAuth>
            <Suspense fallback={<AdminLoading />}>
              <AdminLayout />
            </Suspense>
          </RequireAuth>
        }
      >
        <Route index element={<AdminDashboard />} />
        <Route path="mcp" element={<AdminMCPList />} />
        <Route path="mcp/new" element={<AdminMCPNew />} />
        <Route path="mcp/:ns/:slug" element={<AdminMCPDetail />} />
        <Route path="agents" element={<AdminAgentList />} />
        <Route path="agents/new" element={<AdminAgentNew />} />
        <Route path="agents/:ns/:slug" element={<AdminAgentDetail />} />
        <Route path="publishers" element={<AdminPublisherList />} />
        <Route path="publishers/new" element={<AdminPublisherNew />} />
        <Route path="publishers/:slug" element={<AdminPublisherDetail />} />
        <Route path="api-keys" element={<AdminApiKeys />} />
        <Route path="reports" element={<AdminReports />} />
      </Route>
      <Route path="*" element={<NotFoundPage />} />
    </Routes>
  )
}
