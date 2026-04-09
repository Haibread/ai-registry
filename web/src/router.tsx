import { Routes, Route } from 'react-router-dom'
import { RequireAuth } from '@/auth/RequireAuth'
import HomePage from '@/pages/home'
import MCPListPage from '@/pages/mcp/list'
import MCPDetailPage from '@/pages/mcp/detail'
import AgentListPage from '@/pages/agents/list'
import AgentDetailPage from '@/pages/agents/detail'
import { AuthCallback } from '@/auth/AuthCallback'
import AdminLayout from '@/pages/admin/layout'
import AdminDashboard from '@/pages/admin/dashboard'
import AdminMCPList from '@/pages/admin/mcp/list'
import AdminMCPDetail from '@/pages/admin/mcp/detail'
import AdminMCPNew from '@/pages/admin/mcp/new'
import AdminAgentList from '@/pages/admin/agents/list'
import AdminAgentDetail from '@/pages/admin/agents/detail'
import AdminAgentNew from '@/pages/admin/agents/new'
import AdminPublisherList from '@/pages/admin/publishers/list'
import AdminPublisherDetail from '@/pages/admin/publishers/detail'
import AdminPublisherNew from '@/pages/admin/publishers/new'
import AdminApiKeys from '@/pages/admin/api-keys'
import NotFoundPage from '@/pages/not-found'

export function AppRoutes() {
  return (
    <Routes>
      <Route path="/" element={<HomePage />} />
      <Route path="/mcp" element={<MCPListPage />} />
      <Route path="/mcp/:ns/:slug" element={<MCPDetailPage />} />
      <Route path="/agents" element={<AgentListPage />} />
      <Route path="/agents/:ns/:slug" element={<AgentDetailPage />} />
      <Route path="/auth/callback" element={<AuthCallback />} />
      <Route
        path="/admin"
        element={
          <RequireAuth>
            <AdminLayout />
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
      </Route>
      <Route path="*" element={<NotFoundPage />} />
    </Routes>
  )
}
