import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { apiBase } from '@/lib/api'
import { useMetricsSSE } from '@/hooks/useSSE'
import {
  Blocks,
  Clock,
  Database,
  GitBranch,
  MessageSquare,
  Server,
  TrendingUp,
  Users,
  Wifi,
  WifiOff
} from 'lucide-react'
import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { Area, AreaChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'

interface PortaskMetrics {
  health: {
    status: string
    version: string
    uptime: number
  }
  topics: {
    count: number
    total_messages: number
  }
  consumer_groups: {
    count: number
    active_members: number
  }
  transactions: {
    active: number
    committed: number
    aborted: number
  }
  performance: {
    messages_per_sec: number
    avg_latency_ms: number
    memory_mb: number
  }
}

export default function PortaskDashboard() {
  const [metrics, setMetrics] = useState<PortaskMetrics>({
    health: { status: 'loading', version: '2.0.0', uptime: 0 },
    topics: { count: 0, total_messages: 0 },
    consumer_groups: { count: 0, active_members: 0 },
    transactions: { active: 0, committed: 0, aborted: 0 },
    performance: { messages_per_sec: 0, avg_latency_ms: 0, memory_mb: 0 }
  })

  const [chartData, setChartData] = useState<Array<{ time: string; messages: number; latency: number }>>([])

  // Use SSE for real-time metrics
  const { data: sseData, isConnected: sseConnected } = useMetricsSSE()

  // Update metrics from SSE if available
  useEffect(() => {
    if (sseData && sseConnected) {
      console.log('[PortaskDashboard] SSE data received:', sseData)
      
      // Update performance metrics from SSE
      setMetrics(prev => ({
        ...prev,
        performance: {
          messages_per_sec: (sseData as any).network?.messages_received || 0,
          avg_latency_ms: (sseData as any).core?.avg_latency_ms || 0,
          memory_mb: (sseData as any).system?.alloc_mb || 0
        }
      }))

      // Update chart data
      const now = new Date().toLocaleTimeString()
      setChartData(prev => {
        const newData = [...prev, {
          time: now,
          messages: (sseData as any).storage?.total_messages || 0,
          latency: (sseData as any).core?.avg_latency_ms || 0
        }]
        return newData.slice(-20)
      })
    }
  }, [sseData, sseConnected])

  // Fallback: HTTP polling for initial data and non-real-time metrics
  useEffect(() => {
    const fetchMetrics = async () => {
      try {
        // Fetch all data in parallel
        const [healthRes, topicsRes, groupsRes, transactionsRes, metricsRes] = await Promise.all([
          apiBase.get('/health').catch(err => {
            console.error('[PortaskDashboard] Health fetch failed:', err.message)
            return { data: { status: 'unknown', version: '2.0.0', uptime: 0 } }
          }),
          apiBase.get('/api/v1/topics').catch(err => {
            console.error('[PortaskDashboard] Topics fetch failed:', err.message)
            return { data: { count: 0, topics: [] } }
          }),
          apiBase.get('/api/v1/consumer-groups').catch(err => {
            console.error('[PortaskDashboard] Consumer groups fetch failed:', err.message)
            return { data: { count: 0, groups: [] } }
          }),
          apiBase.get('/api/v1/transactions').catch(err => {
            console.error('[PortaskDashboard] Transactions fetch failed:', err.message)
            return { data: { count: 0, transactions: [] } }
          }),
          apiBase.get('/metrics').catch(err => {
            console.error('[PortaskDashboard] Metrics fetch failed:', err.message)
            return { data: { core: {}, system: {}, network: {} } }
          })
        ])

        setMetrics({
          health: {
            status: healthRes.data?.status || 'unknown',
            version: healthRes.data?.version || '2.0.0',
            uptime: healthRes.data?.uptime || 0
          },
          topics: {
            count: topicsRes.data?.count || 0,
            total_messages: topicsRes.data?.topics?.reduce((sum: number, t: any) => sum + (t.message_count || 0), 0) || 0
          },
          consumer_groups: {
            count: groupsRes.data?.count || 0,
            active_members: groupsRes.data?.groups?.reduce((sum: number, g: any) => sum + (g.members?.length || 0), 0) || 0
          },
          transactions: {
            active: transactionsRes.data?.transactions?.filter((t: any) => t.state === 'ACTIVE').length || 0,
            committed: transactionsRes.data?.transactions?.filter((t: any) => t.state === 'COMMITTED').length || 0,
            aborted: transactionsRes.data?.transactions?.filter((t: any) => t.state === 'ABORTED').length || 0
          },
          performance: {
            messages_per_sec: metricsRes.data?.network?.messages_received || 0,
            avg_latency_ms: metricsRes.data?.core?.avg_latency_ms || 0,
            memory_mb: metricsRes.data?.system?.alloc_mb || 0
          }
        })

        // Update chart data
        const now = new Date().toLocaleTimeString()
        setChartData(prev => {
          const newData = [...prev, {
            time: now,
            messages: topicsRes.data?.topics?.reduce((sum: number, t: any) => sum + (t.message_count || 0), 0) || 0,
            latency: metricsRes.data?.core?.avg_latency_ms || 0
          }]
          return newData.slice(-20) // Keep last 20 points
        })
      } catch (error: any) {
        console.error('[PortaskDashboard] Unexpected error fetching metrics:', error?.message || error)
      }
    }

    fetchMetrics()
    // Only poll if SSE is not connected
    const interval = sseConnected ? null : setInterval(fetchMetrics, 10000) // Refresh every 10 seconds
    return () => {
      if (interval) clearInterval(interval)
    }
  }, [sseConnected])

  const formatUptime = (seconds: number) => {
    if (seconds < 60) return `${Math.round(seconds)}s`
    if (seconds < 3600) return `${Math.round(seconds / 60)}m`
    if (seconds < 86400) return `${Math.round(seconds / 3600)}h`
    return `${Math.round(seconds / 86400)}d`
  }

  const cards = [
    {
      title: 'System Status',
      value: metrics.health.status.toUpperCase(),
      description: `v${metrics.health.version} • Uptime: ${formatUptime(metrics.health.uptime)}`,
      icon: Server,
      color: metrics.health.status === 'healthy' ? 'text-green-600' : 'text-red-600',
      link: '/settings'
    },
    {
      title: 'Topics',
      value: metrics.topics.count.toString(),
      description: `${metrics.topics.total_messages.toLocaleString()} messages`,
      icon: GitBranch,
      color: 'text-blue-600',
      link: '/topics'
    },
    {
      title: 'Consumer Groups',
      value: metrics.consumer_groups.count.toString(),
      description: `${metrics.consumer_groups.active_members} active members`,
      icon: Users,
      color: 'text-purple-600',
      link: '/consumer-groups'
    },
    {
      title: 'Active Transactions',
      value: metrics.transactions.active.toString(),
      description: `${metrics.transactions.committed} committed, ${metrics.transactions.aborted} aborted`,
      icon: Blocks,
      color: 'text-orange-600',
      link: '/transactions'
    },
    {
      title: 'Throughput',
      value: `${metrics.performance.messages_per_sec.toLocaleString()}/s`,
      description: 'Messages per second',
      icon: TrendingUp,
      color: 'text-cyan-600',
      link: '/messages'
    },
    {
      title: 'Latency',
      value: `${metrics.performance.avg_latency_ms}ms`,
      description: 'Average response time',
      icon: Clock,
      color: 'text-amber-600',
      link: '/messages'
    },
    {
      title: 'Memory',
      value: `${metrics.performance.memory_mb}MB`,
      description: 'Current memory usage',
      icon: Database,
      color: 'text-indigo-600',
      link: '/settings'
    },
    {
      title: 'Messages',
      value: metrics.topics.total_messages.toLocaleString(),
      description: 'Total messages stored',
      icon: MessageSquare,
      color: 'text-pink-600',
      link: '/messages'
    }
  ]

  return (
    <div className="flex-1 space-y-4 p-4 md:p-8 pt-6">
      <div className="flex items-center justify-between space-y-2">
        <div>
          <h2 className="text-3xl font-bold tracking-tight">Portask Dashboard</h2>
          <p className="text-muted-foreground flex items-center gap-2">
            Unified message queue system overview
            {sseConnected ? (
              <Badge variant="default" className="gap-1 ml-2">
                <Wifi className="h-3 w-3" />
                Live Updates
              </Badge>
            ) : (
              <Badge variant="secondary" className="gap-1 ml-2">
                <WifiOff className="h-3 w-3" />
                Polling Mode
              </Badge>
            )}
          </p>
        </div>
        <div className="flex items-center space-x-2">
          <Badge variant={metrics.health.status === 'healthy' ? 'default' : 'destructive'}>
            {metrics.health.status.toUpperCase()}
          </Badge>
          <Button variant="outline" size="sm">
            Refresh
          </Button>
        </div>
      </div>

      {/* Native API Stats Cards */}
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
        {cards.map((card, index) => {
          const Icon = card.icon
          return (
            <Link to={card.link} key={index}>
              <Card className="hover:border-primary transition-colors cursor-pointer">
                <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                  <CardTitle className="text-sm font-medium">
                    {card.title}
                  </CardTitle>
                  <Icon className={`h-4 w-4 ${card.color}`} />
                </CardHeader>
                <CardContent>
                  <div className={`text-2xl font-bold ${card.color}`}>
                    {card.value}
                  </div>
                  <p className="text-xs text-muted-foreground mt-1">
                    {card.description}
                  </p>
                </CardContent>
              </Card>
            </Link>
          )
        })}
      </div>

      {/* Performance Chart */}
      <Card className="col-span-4">
        <CardHeader>
          <CardTitle>Performance Overview</CardTitle>
          <CardDescription>
            Real-time message throughput and latency monitoring
          </CardDescription>
        </CardHeader>
        <CardContent>
          <ResponsiveContainer width="100%" height={300}>
            <AreaChart data={chartData}>
              <defs>
                <linearGradient id="colorMessages" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#8884d8" stopOpacity={0.8} />
                  <stop offset="95%" stopColor="#8884d8" stopOpacity={0} />
                </linearGradient>
              </defs>
              <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
              <XAxis
                dataKey="time"
                className="text-xs"
                tick={{ fill: 'currentColor' }}
              />
              <YAxis
                className="text-xs"
                tick={{ fill: 'currentColor' }}
              />
              <Tooltip
                contentStyle={{
                  backgroundColor: 'hsl(var(--card))',
                  border: '1px solid hsl(var(--border))'
                }}
              />
              <Area
                type="monotone"
                dataKey="messages"
                stroke="#8884d8"
                fillOpacity={1}
                fill="url(#colorMessages)"
                name="Total Messages"
              />
            </AreaChart>
          </ResponsiveContainer>
        </CardContent>
      </Card>

      {/* Quick Actions */}
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center">
              <GitBranch className="mr-2 h-5 w-5" /> Topics
            </CardTitle>
            <CardDescription>Manage message topics</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-2">
              <p className="text-2xl font-bold">{metrics.topics.count}</p>
              <Link to="/topics">
                <Button variant="outline" className="w-full">View Topics</Button>
              </Link>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center">
              <Users className="mr-2 h-5 w-5" /> Consumer Groups
            </CardTitle>
            <CardDescription>Manage consumer groups</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-2">
              <p className="text-2xl font-bold">{metrics.consumer_groups.count}</p>
              <Link to="/consumer-groups">
                <Button variant="outline" className="w-full">View Groups</Button>
              </Link>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center">
              <Blocks className="mr-2 h-5 w-5" /> Transactions
            </CardTitle>
            <CardDescription>Monitor transactions</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-2">
              <p className="text-2xl font-bold">{metrics.transactions.active}</p>
              <Link to="/transactions">
                <Button variant="outline" className="w-full">View Transactions</Button>
              </Link>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}

