import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { createBrowserRouter, RouterProvider } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Toaster } from 'sonner'
import { AuthProvider } from '@/auth/AuthContext'
import { ThemeProvider } from '@/components/providers'
import { AppRoutes } from '@/router'
import './app/globals.css'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { staleTime: 30_000, retry: 1 },
  },
})

// A data router (vs <BrowserRouter>) so navigation can be blocked while a
// form has unsaved changes (useBlocker). Routing stays declarative in
// AppRoutes via descendant <Routes>; the providers that use router hooks
// live inside the splat route's element.
const router = createBrowserRouter([
  {
    path: '*',
    element: (
      <AuthProvider>
        <ThemeProvider>
          <AppRoutes />
          {/* Toast notifications: top-right, theme-aware. */}
          <Toaster position="top-right" richColors closeButton />
        </ThemeProvider>
      </AuthProvider>
    ),
  },
])

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  </StrictMode>
)
