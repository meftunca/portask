import { useEffect, useState } from 'react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Activity, Database, MessageSquare, Network, Server, Users } from 'lucide-react'
import { api, apiBase } from '@/lib/api'

interface SystemMetrics {
  uptime: string
  connections: number
  messages_total: number
  memory_usage: string
  cpu_usage: string
  status: string
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

  const [isConnected, setIsConnected] = useState(false)

  useEffect(() => {
    // Gerçek API'den veri çek
    const fetchMetrics = async () => {
      try {
        const response = await apiBase.get('/metrics',)
        const data = response.data
        setMetrics({
          uptime: data.core?.uptime_seconds ? `${Math.round(data.core.uptime_seconds)}s` : '0s',
          connections: data.network?.connections_active || 0,
          messages_total: data.storage?.total_messages || 0,
          memory_usage: `${data.system?.alloc_mb || 0} MB`,
          cpu_usage: '0%', // CPU usage would come from system metrics
          status: 'healthy'
        })
        setIsConnected(true)
      } catch (error) {
        setIsConnected(false)
        setMetrics((m) => ({ ...m, status: 'disconnected' }))
      }
    }
    fetchMetrics()
    const interval = setInterval(fetchMetrics, 5000)
    return () => clearInterval(interval)
  }, [])

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
    }
  ]

  return (
    <div className="flex-1 space-y-4 p-4 md:p-8 pt-6">
      <div className="flex items-center justify-between space-y-2">
        <h2 className="text-3xl font-bold tracking-tight">Dashboard</h2>
        <div className="flex items-center space-x-2">
          <Button variant="outline" size="sm">
            Refresh
          </Button>
        </div>
      </div>

      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
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
                <span className="text-sm text-muted-foreground">Real-time</span>
              </div>
              <div className="h-32 bg-muted rounded-md flex items-center justify-center">
                <span className="text-muted-foreground text-sm">
                  📊 Chart Component Here
                </span>
              </div>
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
    </div>
  )
}
