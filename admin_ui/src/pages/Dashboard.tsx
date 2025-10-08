import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { useMetricsWebSocket } from '@/hooks/useWebSocket'
import { apiBase } from '@/lib/api'
import { Activity, Database, MessageSquare, Network, Server, Users, Wifi, WifiOff } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Area, AreaChart, CartesianGrid, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'

interface SystemMetrics {
  uptime: string
  connections: number
  messages_total: number
  memory_usage: string
  cpu_usage: string
  status: string
  goroutines?: number
  gc_count?: number
  messages_rate?: number
}

interface NativeStats {
  topics_count: number
  consumer_groups_count: number
  transactions_active: number
  messages_per_sec: number
}

export default function Dashboard() {
  const [metrics, setMetrics] = useState<SystemMetrics>({
    uptime: '0s',
    connections: 0,
    messages_total: 0,
    memory_usage: '0 MB',
    cpu_usage: '0%',
    status: 'connecting'
  })

  const [nativeStats, setNativeStats] = useState<NativeStats>({
    topics_count: 0,
    consumer_groups_count: 0,
    transactions_active: 0,
    messages_per_sec: 0
  })

  const [isConnected, setIsConnected] = useState(false)
  const [throughputData, setThroughputData] = useState<Array<{ time: string, messages: number, latency: number }>>([])
  const [memoryData, setMemoryData] = useState<Array<{ time: string, alloc: number, sys: number }>>([])

  // WebSocket for real-time updates
  const { data: wsData, isConnected: wsConnected } = useMetricsWebSocket()

  // Update metrics from WebSocket if available
  useEffect(() => {
    if (wsData && wsConnected) {
      console.log('[Dashboard] WebSocket data received:', wsData)

      // Process WebSocket metrics update
      const data = wsData
      const newMetrics = {
        uptime: data.core?.uptime_seconds ? `${Math.round(data.core.uptime_seconds)}s` : '0s',
        connections: data.network?.connections_active || 0,
        messages_total: data.storage?.total_messages || 0,
        memory_usage: `${data.system?.alloc_mb || 0} MB`,
        cpu_usage: '0%',
        status: 'healthy',
        goroutines: data.system?.num_goroutines || 0,
        gc_count: data.system?.num_gc || 0,
        messages_rate: data.network?.messages_received || 0
      }
      setMetrics(newMetrics)
      setIsConnected(true)

      // Update chart data
      const now = new Date().toLocaleTimeString()

      setThroughputData(prev => {
        const newData = [...prev, {
          time: now,
          messages: data.storage?.total_messages || 0,
          latency: data.core?.avg_latency_ms || 0
        }]
        return newData.slice(-20)
      })

      setMemoryData(prev => {
        const newData = [...prev, {
          time: now,
          alloc: data.system?.alloc_mb || 0,
          sys: data.system?.sys_mb || 0
        }]
        return newData.slice(-20)
      })
    }
  }, [wsData, wsConnected])

  // Fallback to HTTP polling if WebSocket is not connected
  useEffect(() => {
    // Only use polling if WebSocket is not connected
    if (wsConnected) {
      console.log('[Dashboard] Using WebSocket, skipping HTTP polling')
      return
    }

    console.log('[Dashboard] WebSocket not available, using HTTP polling')

    const fetchMetrics = async () => {
      try {
        const response = await apiBase.get('/metrics',)
        const data = response.data

        const newMetrics = {
          uptime: data.core?.uptime_seconds ? `${Math.round(data.core.uptime_seconds)}s` : '0s',
          connections: data.network?.connections_active || 0,
          messages_total: data.storage?.total_messages || 0,
          memory_usage: `${data.system?.alloc_mb || 0} MB`,
          cpu_usage: '0%',
          status: 'healthy',
          goroutines: data.system?.num_goroutines || 0,
          gc_count: data.system?.num_gc || 0,
          messages_rate: data.network?.messages_received || 0
        }
        setMetrics(newMetrics)
        setIsConnected(true)

        // Update chart data
        const now = new Date().toLocaleTimeString()

        setThroughputData(prev => {
          const newData = [...prev, {
            time: now,
            messages: data.storage?.total_messages || 0,
            latency: data.core?.avg_latency_ms || 0
          }]
          return newData.slice(-20)
        })

        setMemoryData(prev => {
          const newData = [...prev, {
            time: now,
            alloc: data.system?.alloc_mb || 0,
            sys: data.system?.sys_mb || 0
          }]
          return newData.slice(-20)
        })

      } catch (error) {
        setIsConnected(false)
        setMetrics((m) => ({ ...m, status: 'disconnected' }))
      }
    }

    fetchMetrics()
    const interval = setInterval(fetchMetrics, 5000)
    return () => clearInterval(interval)
  }, [wsConnected])

  const cards = [
    {
      title: 'System Status',
      value: metrics.status,
      description: 'Overall system health',
      icon: Server,
      color: metrics.status === 'healthy' ? 'text-green-600' : 'text-red-600'
    },
    {
      title: 'Active Connections',
      value: metrics.connections.toString(),
      description: 'Currently connected clients',
      icon: Users,
      color: 'text-blue-600'
    },
    {
      title: 'Total Messages',
      value: metrics.messages_total.toString(),
      description: 'Messages processed',
      icon: MessageSquare,
      color: 'text-purple-600'
    },
    {
      title: 'Memory Usage',
      value: metrics.memory_usage,
      description: 'Current memory consumption',
      icon: Database,
      color: 'text-orange-600'
    },
    {
      title: 'Uptime',
      value: metrics.uptime,
      description: 'System uptime',
      icon: Activity,
      color: 'text-green-600'
    },
    {
      title: 'Network Status',
      value: isConnected ? 'Connected' : 'Disconnected',
      description: 'API connection status',
      icon: Network,
      color: isConnected ? 'text-green-600' : 'text-red-600'
    },
    {
      title: 'Goroutines',
      value: metrics.goroutines?.toString() || '0',
      description: 'Active goroutines',
      icon: Activity,
      color: 'text-cyan-600'
    },
    {
      title: 'GC Cycles',
      value: metrics.gc_count?.toString() || '0',
      description: 'Garbage collection count',
      icon: Database,
      color: 'text-amber-600'
    },
    {
      title: 'Messages/Sec',
      value: metrics.messages_rate?.toString() || '0',
      description: 'Message throughput',
      icon: MessageSquare,
      color: 'text-indigo-600'
    }
  ]

  return (
    <div className="flex-1 space-y-4 p-4 md:p-8 pt-6">
      <div className="flex items-center justify-between space-y-2">
        <h2 className="text-3xl font-bold tracking-tight">Dashboard</h2>
        <div className="flex items-center space-x-2">
          {wsConnected ? (
            <div className="flex items-center space-x-2 px-3 py-1 rounded-md bg-green-500/10 border border-green-500/20">
              <Wifi className="h-4 w-4 text-green-600" />
              <span className="text-xs font-medium text-green-600">Real-time</span>
            </div>
          ) : (
            <div className="flex items-center space-x-2 px-3 py-1 rounded-md bg-orange-500/10 border border-orange-500/20">
              <WifiOff className="h-4 w-4 text-orange-600" />
              <span className="text-xs font-medium text-orange-600">Polling (5s)</span>
            </div>
          )}
          <Button variant="outline" size="sm">
            Refresh
          </Button>
        </div>
      </div>

      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
        {cards.map((card, index) => {
          const Icon = card.icon
          return (
            <Card key={index}>
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
                <p className="text-xs text-muted-foreground">
                  {card.description}
                </p>
              </CardContent>
            </Card>
          )
        })}
      </div>

      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-7">
        <Card className="col-span-4">
          <CardHeader>
            <CardTitle>System Overview</CardTitle>
            <CardDescription>
              Real-time monitoring of Portask message queue system
            </CardDescription>
          </CardHeader>
          <CardContent className="pl-2">
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <span className="text-sm font-medium">Message Throughput</span>
                <span className="text-sm text-muted-foreground">Last 20 updates</span>
              </div>
              <ResponsiveContainer width="100%" height={200}>
                <AreaChart data={throughputData}>
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
            </div>
          </CardContent>
        </Card>

        <Card className="col-span-3">
          <CardHeader>
            <CardTitle>Recent Activity</CardTitle>
            <CardDescription>
              Latest system events and messages
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              <div className="flex items-center space-x-4">
                <div className="w-2 h-2 bg-green-500 rounded-full"></div>
                <div className="space-y-1">
                  <p className="text-sm font-medium">System Started</p>
                  <p className="text-xs text-muted-foreground">2 minutes ago</p>
                </div>
              </div>
              <div className="flex items-center space-x-4">
                <div className="w-2 h-2 bg-blue-500 rounded-full"></div>
                <div className="space-y-1">
                  <p className="text-sm font-medium">New Connection</p>
                  <p className="text-xs text-muted-foreground">5 minutes ago</p>
                </div>
              </div>
              <div className="flex items-center space-x-4">
                <div className="w-2 h-2 bg-purple-500 rounded-full"></div>
                <div className="space-y-1">
                  <p className="text-sm font-medium">Message Published</p>
                  <p className="text-xs text-muted-foreground">10 minutes ago</p>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Memory & Performance Charts */}
      <div className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Memory Usage</CardTitle>
            <CardDescription>
              Alloc vs System memory over time
            </CardDescription>
          </CardHeader>
          <CardContent>
            <ResponsiveContainer width="100%" height={250}>
              <LineChart data={memoryData}>
                <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
                <XAxis
                  dataKey="time"
                  className="text-xs"
                  tick={{ fill: 'currentColor' }}
                />
                <YAxis
                  className="text-xs"
                  tick={{ fill: 'currentColor' }}
                  label={{ value: 'MB', angle: -90, position: 'insideLeft' }}
                />
                <Tooltip
                  contentStyle={{
                    backgroundColor: 'hsl(var(--card))',
                    border: '1px solid hsl(var(--border))'
                  }}
                />
                <Line
                  type="monotone"
                  dataKey="alloc"
                  stroke="#8884d8"
                  strokeWidth={2}
                  name="Allocated"
                  dot={false}
                />
                <Line
                  type="monotone"
                  dataKey="sys"
                  stroke="#82ca9d"
                  strokeWidth={2}
                  name="System"
                  dot={false}
                />
              </LineChart>
            </ResponsiveContainer>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Average Latency</CardTitle>
            <CardDescription>
              Response time in milliseconds
            </CardDescription>
          </CardHeader>
          <CardContent>
            <ResponsiveContainer width="100%" height={250}>
              <AreaChart data={throughputData}>
                <defs>
                  <linearGradient id="colorLatency" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#fbbf24" stopOpacity={0.8} />
                    <stop offset="95%" stopColor="#fbbf24" stopOpacity={0} />
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
                  label={{ value: 'ms', angle: -90, position: 'insideLeft' }}
                />
                <Tooltip
                  contentStyle={{
                    backgroundColor: 'hsl(var(--card))',
                    border: '1px solid hsl(var(--border))'
                  }}
                />
                <Area
                  type="monotone"
                  dataKey="latency"
                  stroke="#fbbf24"
                  fillOpacity={1}
                  fill="url(#colorLatency)"
                  name="Latency (ms)"
                />
              </AreaChart>
            </ResponsiveContainer>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
