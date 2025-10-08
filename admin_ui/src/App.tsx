import { BrowserRouter as Router, Routes, Route } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from 'react-query'
import { Toaster } from '@/components/ui/toaster'
import { ThemeProvider } from '@/components/theme-provider'
import Layout from '@/components/layout/Layout'
// Portask Native Pages (v2.0)
import PortaskDashboard from '@/pages/PortaskDashboard'
import TopicsNative from '@/pages/TopicsNative'
import ConsumerGroupsNative from '@/pages/ConsumerGroupsNative'
import TransactionsNative from '@/pages/TransactionsNative'
// Legacy/Protocol-specific pages
import Dashboard from '@/pages/Dashboard'
import Messages from '@/pages/Messages'
import Topics from '@/pages/Topics'
import ConsumerGroups from '@/pages/ConsumerGroups'
import KafkaDashboard from '@/pages/KafkaDashboard'
import AMQPDashboard from '@/pages/AMQPDashboard'
import Connections from '@/pages/Connections'
import Monitoring from '@/pages/Monitoring'
import Settings from '@/pages/Settings'
import './index.css'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: 1,
    },
  },
})

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ThemeProvider defaultTheme="system" storageKey="portask-ui-theme">
        <Router>
          <Layout>
            <Routes>
              {/* Portask Native v2.0 Routes */}
              <Route path="/" element={<PortaskDashboard />} />
              <Route path="/dashboard" element={<PortaskDashboard />} />
              <Route path="/topics" element={<TopicsNative />} />
              <Route path="/consumer-groups" element={<ConsumerGroupsNative />} />
              <Route path="/transactions" element={<TransactionsNative />} />
              
              {/* Legacy/Protocol-specific Routes */}
              <Route path="/system" element={<Dashboard />} />
              <Route path="/messages" element={<Messages />} />
              <Route path="/kafka" element={<KafkaDashboard />} />
              <Route path="/amqp" element={<AMQPDashboard />} />
              <Route path="/connections" element={<Connections />} />
              <Route path="/monitoring" element={<Monitoring />} />
              <Route path="/settings" element={<Settings />} />
            </Routes>
          </Layout>
        </Router>
        <Toaster />
      </ThemeProvider>
    </QueryClientProvider>
  )
}

export default App
