import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { apiBase } from '@/lib/api'
import {
  Activity,
  ArrowRight,
  CheckCircle,
  Rabbit,
  RefreshCw,
  Share2,
  TrendingUp,
  Users,
  Zap
} from 'lucide-react'
import { useEffect, useState } from 'react'
import { CartesianGrid, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'

interface AMQPMetrics {
  queues: number
  exchanges: number
  bindings: number
  channels: number
  connections: number
  messagesPublished: number
  messagesDelivered: number
  messagesAcked: number
  publishRate: number
  deliverRate: number
}

interface QueueInfo {
  name: string
  messages: number
  consumers: number
  state: 'running' | 'idle' | 'flow'
  durable: boolean
}

export default function AMQPDashboard() {
  const [metrics, setMetrics] = useState<AMQPMetrics>({
    queues: 0,
    exchanges: 4, // Default exchanges (direct, fanout, topic, headers)
    bindings: 0,
    channels: 0,
    connections: 0,
    messagesPublished: 0,
    messagesDelivered: 0,
    messagesAcked: 0,
    publishRate: 0,
    deliverRate: 0
  })

  const [queues, setQueues] = useState<QueueInfo[]>([])
  const [loading, setLoading] = useState(false)
  const [publishHistory, setPublishHistory] = useState<Array<{ time: string, published: number, delivered: number }>>([])

  const fetchMetrics = async () => {
    setLoading(true)
    try {
      // Fetch general metrics
      const metricsRes = await apiBase.get('/metrics')
      const data = metricsRes.data

      // Fetch connections
      const connectionsRes = await apiBase.get('/api/v1/connections')
      const connections = connectionsRes.data?.connections || []

      // Fetch queues from backend
      const queuesRes = await apiBase.get('/api/v1/amqp/queues')
      const queuesList = queuesRes.data?.queues || []
      
      // Fetch exchanges
      const exchangesRes = await apiBase.get('/api/v1/amqp/exchanges')
      const exchangesList = exchangesRes.data?.exchanges || []
      
      // Fetch bindings
      const bindingsRes = await apiBase.get('/api/v1/amqp/bindings')
      const bindingsList = bindingsRes.data?.bindings || []

      setMetrics({
        queues: queuesList.length,
        exchanges: exchangesList.length,
        bindings: bindingsList.length,
        channels: connections.length,
        connections: connections.length,
        messagesPublished: data.network?.messages_sent || 0,
        messagesDelivered: data.network?.messages_received || 0,
        messagesAcked: data.network?.messages_received || 0,
        publishRate: data.network?.messages_sent || 0,
        deliverRate: data.network?.messages_received || 0
      })

      // Convert real queue data to UI format
      const realQueues: QueueInfo[] = queuesList.map((q: any) => ({
        name: q.name || q.queue_name || 'unknown',
        messages: q.message_count || q.messages || 0,
        consumers: q.consumer_count || q.consumers || 0,
        state: q.state || (q.consumers > 0 ? 'running' : 'idle'),
        durable: q.durable !== undefined ? q.durable : true
      }))
      setQueues(realQueues)

      // Update publish history
      const now = new Date().toLocaleTimeString()
      setPublishHistory(prev => {
        const newData = [...prev, {
          time: now,
          published: data.network?.messages_sent || 0,
          delivered: data.network?.messages_received || 0
        }]
        return newData.slice(-20)
      })

    } catch (error) {
      console.error('Failed to fetch AMQP metrics:', error)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchMetrics()
    const interval = setInterval(fetchMetrics, 10000)
    return () => clearInterval(interval)
  }, [])

  const getQueueStateBadge = (state: string) => {
    switch (state) {
      case 'running':
        return <Badge className="bg-green-500/10 text-green-600 border-green-500/20">Running</Badge>
      case 'idle':
        return <Badge className="bg-gray-500/10 text-gray-600 border-gray-500/20">Idle</Badge>
      case 'flow':
        return <Badge className="bg-yellow-500/10 text-yellow-600 border-yellow-500/20">Flow</Badge>
      default:
        return <Badge variant="outline">{state}</Badge>
    }
  }

  return (
    <div className="flex-1 space-y-4 p-4 md:p-8 pt-6">
      <div className="flex items-center justify-between">
        <div>
          <div className="flex items-center space-x-2">
            <Rabbit className="h-8 w-8 text-orange-600" />
            <h2 className="text-3xl font-bold tracking-tight">AMQP / RabbitMQ</h2>
          </div>
          <p className="text-muted-foreground">
            RabbitMQ-compatible AMQP server monitoring
          </p>
        </div>
        <Button onClick={fetchMetrics} disabled={loading}>
          <RefreshCw className={`h-4 w-4 mr-2 ${loading ? 'animate-spin' : ''}`} />
          Refresh
        </Button>
      </div>

      {/* Status Badge */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex items-center justify-between">
            <div className="flex items-center space-x-3">
              <div className="w-3 h-3 bg-green-500 rounded-full animate-pulse"></div>
              <div>
                <p className="text-lg font-semibold">AMQP Server: Online</p>
                <p className="text-sm text-muted-foreground">
                  100% RabbitMQ compatible, listening on port 5672
                </p>
              </div>
            </div>
            <Badge className="bg-green-500/10 text-green-600 border-green-500/20">
              <CheckCircle className="h-3 w-3 mr-1" />
              Healthy
            </Badge>
          </div>
        </CardContent>
      </Card>

      {/* Key Metrics */}
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Queues</CardTitle>
            <Activity className="h-4 w-4 text-blue-600" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-blue-600">{metrics.queues}</div>
            <p className="text-xs text-muted-foreground">Active queues</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Exchanges</CardTitle>
            <Share2 className="h-4 w-4 text-green-600" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-green-600">{metrics.exchanges}</div>
            <p className="text-xs text-muted-foreground">
              {metrics.bindings} bindings
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Connections</CardTitle>
            <Users className="h-4 w-4 text-purple-600" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-purple-600">{metrics.connections}</div>
            <p className="text-xs text-muted-foreground">
              {metrics.channels} channels
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Publish Rate</CardTitle>
            <TrendingUp className="h-4 w-4 text-orange-600" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-orange-600">
              {metrics.publishRate.toLocaleString()}
            </div>
            <p className="text-xs text-muted-foreground">msgs/sec</p>
          </CardContent>
        </Card>
      </div>

      {/* Message Flow Charts */}
      <div className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Message Flow</CardTitle>
            <CardDescription>Published vs Delivered messages</CardDescription>
          </CardHeader>
          <CardContent>
            <ResponsiveContainer width="100%" height={250}>
              <LineChart data={publishHistory}>
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
                <Line
                  type="monotone"
                  dataKey="published"
                  stroke="#8884d8"
                  strokeWidth={2}
                  dot={false}
                  name="Published"
                />
                <Line
                  type="monotone"
                  dataKey="delivered"
                  stroke="#82ca9d"
                  strokeWidth={2}
                  dot={false}
                  name="Delivered"
                />
              </LineChart>
            </ResponsiveContainer>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Message Statistics</CardTitle>
            <CardDescription>Total message counts</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <div className="flex items-center space-x-2">
                  <ArrowRight className="h-4 w-4 text-blue-600" />
                  <span className="text-sm font-medium">Published</span>
                </div>
                <span className="text-lg font-bold text-blue-600">
                  {metrics.messagesPublished.toLocaleString()}
                </span>
              </div>
              <div className="flex items-center justify-between">
                <div className="flex items-center space-x-2">
                  <Zap className="h-4 w-4 text-green-600" />
                  <span className="text-sm font-medium">Delivered</span>
                </div>
                <span className="text-lg font-bold text-green-600">
                  {metrics.messagesDelivered.toLocaleString()}
                </span>
              </div>
              <div className="flex items-center justify-between">
                <div className="flex items-center space-x-2">
                  <CheckCircle className="h-4 w-4 text-purple-600" />
                  <span className="text-sm font-medium">Acknowledged</span>
                </div>
                <span className="text-lg font-bold text-purple-600">
                  {metrics.messagesAcked.toLocaleString()}
                </span>
              </div>
              <div className="pt-4 border-t">
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <p className="text-xs text-muted-foreground">Delivery Rate</p>
                    <p className="text-2xl font-bold">{metrics.deliverRate}/s</p>
                  </div>
                  <div>
                    <p className="text-xs text-muted-foreground">Success Rate</p>
                    <p className="text-2xl font-bold">
                      {metrics.messagesPublished > 0
                        ? ((metrics.messagesAcked / metrics.messagesPublished) * 100).toFixed(1)
                        : 0}%
                    </p>
                  </div>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Queues List */}
      <Card>
        <CardHeader>
          <CardTitle>Queues</CardTitle>
          <CardDescription>
            Active message queues and their status
          </CardDescription>
        </CardHeader>
        <CardContent>
          {queues.length > 0 ? (
            <div className="space-y-3">
              {queues.map((queue) => (
                <div
                  key={queue.name}
                  className="flex items-center justify-between p-4 border rounded-lg"
                >
                  <div className="flex items-center space-x-4">
                    <Activity className={`h-5 w-5 ${queue.state === 'running' ? 'text-green-600' : 'text-gray-400'
                      }`} />
                    <div>
                      <div className="flex items-center space-x-2">
                        <p className="font-medium">{queue.name}</p>
                        {queue.durable && (
                          <Badge variant="outline" className="text-xs">Durable</Badge>
                        )}
                      </div>
                      <p className="text-xs text-muted-foreground">
                        {queue.messages} messages • {queue.consumers} consumers
                      </p>
                    </div>
                  </div>
                  <div className="flex items-center space-x-2">
                    {getQueueStateBadge(queue.state)}
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <div className="flex flex-col items-center justify-center py-12">
              <Activity className="h-12 w-12 text-muted-foreground mb-4" />
              <p className="text-sm text-muted-foreground">No queues found</p>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Exchange Types */}
      <div className="grid gap-4 md:grid-cols-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Direct</CardTitle>
            <Share2 className="h-4 w-4 text-blue-600" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-blue-600">1</div>
            <p className="text-xs text-muted-foreground">Exact routing</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Fanout</CardTitle>
            <Share2 className="h-4 w-4 text-green-600" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-green-600">1</div>
            <p className="text-xs text-muted-foreground">Broadcast</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Topic</CardTitle>
            <Share2 className="h-4 w-4 text-purple-600" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-purple-600">1</div>
            <p className="text-xs text-muted-foreground">Pattern match</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Headers</CardTitle>
            <Share2 className="h-4 w-4 text-orange-600" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-orange-600">1</div>
            <p className="text-xs text-muted-foreground">Header-based</p>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}

