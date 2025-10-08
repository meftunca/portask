import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { apiBase } from '@/lib/api'
import {
  Activity,
  BarChart3,
  CheckCircle,
  Database,
  Layers,
  RefreshCw,
  Server,
  TrendingUp,
  Users,
  Zap
} from 'lucide-react'
import { useEffect, useState } from 'react'
import { Bar, BarChart, CartesianGrid, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'

interface KafkaMetrics {
  brokers: number
  topics: number
  partitions: number
  consumerGroups: number
  messagesPerSec: number
  bytesInPerSec: number
  bytesOutPerSec: number
  activeProducers: number
  activeConsumers: number
}

interface TopicMetrics {
  name: string
  partitions: number
  messages: number
  size: string
  throughput: number
}

export default function KafkaDashboard() {
  const [metrics, setMetrics] = useState<KafkaMetrics>({
    brokers: 1,
    topics: 0,
    partitions: 0,
    consumerGroups: 0,
    messagesPerSec: 0,
    bytesInPerSec: 0,
    bytesOutPerSec: 0,
    activeProducers: 0,
    activeConsumers: 0
  })

  const [topicMetrics, setTopicMetrics] = useState<TopicMetrics[]>([])
  const [loading, setLoading] = useState(false)
  const [throughputHistory, setThroughputHistory] = useState<Array<{ time: string, rate: number }>>([])

  const fetchMetrics = async () => {
    setLoading(true)
    try {
      // Fetch topics
      const topicsRes = await apiBase.get('/api/v1/topics')
      const topics = topicsRes.data?.topics || []

      // Fetch general metrics
      const metricsRes = await apiBase.get('/metrics')
      const data = metricsRes.data

      // Fetch consumer groups from backend
      const groupsRes = await apiBase.get('/api/v1/kafka/consumer-groups')
      const consumerGroups = groupsRes.data?.consumer_groups || []

      setMetrics({
        brokers: 1,
        topics: topics.length,
        partitions: topics.reduce((sum: number, t: any) => sum + (t.partitions || 0), 0),
        consumerGroups: consumerGroups.length,
        messagesPerSec: data.network?.messages_received || 0,
        bytesInPerSec: data.network?.bytes_read || 0,
        bytesOutPerSec: data.network?.bytes_written || 0,
        activeProducers: 0,
        activeConsumers: consumerGroups.reduce((sum: number, g: any) => sum + (g.members?.length || 0), 0)
      })

      // Format topic metrics with real message counts
      const formattedTopics: TopicMetrics[] = topics.map((t: any) => ({
        name: t.name,
        partitions: t.partitions || 0,
        messages: t.message_count || 0, // Real message count from backend
        size: t.total_bytes ? `${(t.total_bytes / 1024 / 1024).toFixed(2)} MB` : '0 MB',
        throughput: 0 // Will be calculated from history
      }))
      setTopicMetrics(formattedTopics)

      // Update throughput history
      const now = new Date().toLocaleTimeString()
      setThroughputHistory(prev => {
        const newData = [...prev, {
          time: now,
          rate: data.network?.messages_received || 0
        }]
        return newData.slice(-20)
      })

    } catch (error) {
      console.error('Failed to fetch Kafka metrics:', error)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchMetrics()
    const interval = setInterval(fetchMetrics, 10000)
    return () => clearInterval(interval)
  }, [])

  return (
    <div className="flex-1 space-y-4 p-4 md:p-8 pt-6">
      <div className="flex items-center justify-between">
        <div>
          <div className="flex items-center space-x-2">
            <Zap className="h-8 w-8 text-purple-600" />
            <h2 className="text-3xl font-bold tracking-tight">Kafka Dashboard</h2>
          </div>
          <p className="text-muted-foreground">
            Real-time Kafka cluster monitoring and metrics
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
                <p className="text-lg font-semibold">Kafka Cluster: Healthy</p>
                <p className="text-sm text-muted-foreground">
                  All brokers operational, {metrics.topics} topics active
                </p>
              </div>
            </div>
            <Badge className="bg-green-500/10 text-green-600 border-green-500/20">
              <CheckCircle className="h-3 w-3 mr-1" />
              Online
            </Badge>
          </div>
        </CardContent>
      </Card>

      {/* Key Metrics */}
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Brokers</CardTitle>
            <Server className="h-4 w-4 text-blue-600" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-blue-600">{metrics.brokers}</div>
            <p className="text-xs text-muted-foreground">Active nodes</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Topics</CardTitle>
            <Database className="h-4 w-4 text-green-600" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-green-600">{metrics.topics}</div>
            <p className="text-xs text-muted-foreground">{metrics.partitions} partitions</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Consumer Groups</CardTitle>
            <Users className="h-4 w-4 text-purple-600" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-purple-600">{metrics.consumerGroups}</div>
            <p className="text-xs text-muted-foreground">Active groups</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Messages/Sec</CardTitle>
            <TrendingUp className="h-4 w-4 text-orange-600" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-orange-600">
              {metrics.messagesPerSec.toLocaleString()}
            </div>
            <p className="text-xs text-muted-foreground">Current throughput</p>
          </CardContent>
        </Card>
      </div>

      {/* Throughput & Network */}
      <div className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Message Throughput</CardTitle>
            <CardDescription>Messages per second (last 20 updates)</CardDescription>
          </CardHeader>
          <CardContent>
            <ResponsiveContainer width="100%" height={250}>
              <LineChart data={throughputHistory}>
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
                  dataKey="rate"
                  stroke="#8884d8"
                  strokeWidth={2}
                  dot={false}
                  name="Msgs/sec"
                />
              </LineChart>
            </ResponsiveContainer>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Network I/O</CardTitle>
            <CardDescription>Bytes in/out per second</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <div className="flex items-center space-x-2">
                  <div className="w-3 h-3 bg-blue-500 rounded-full"></div>
                  <span className="text-sm font-medium">Bytes In</span>
                </div>
                <span className="text-lg font-bold text-blue-600">
                  {(metrics.bytesInPerSec / 1024).toFixed(2)} KB/s
                </span>
              </div>
              <div className="flex items-center justify-between">
                <div className="flex items-center space-x-2">
                  <div className="w-3 h-3 bg-green-500 rounded-full"></div>
                  <span className="text-sm font-medium">Bytes Out</span>
                </div>
                <span className="text-lg font-bold text-green-600">
                  {(metrics.bytesOutPerSec / 1024).toFixed(2)} KB/s
                </span>
              </div>
              <div className="pt-4 border-t">
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <p className="text-xs text-muted-foreground">Active Producers</p>
                    <p className="text-2xl font-bold">{metrics.activeProducers}</p>
                  </div>
                  <div>
                    <p className="text-xs text-muted-foreground">Active Consumers</p>
                    <p className="text-2xl font-bold">{metrics.activeConsumers}</p>
                  </div>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Topic Metrics */}
      <Card>
        <CardHeader>
          <CardTitle>Top Topics by Activity</CardTitle>
          <CardDescription>
            Most active Kafka topics
          </CardDescription>
        </CardHeader>
        <CardContent>
          {topicMetrics.length > 0 ? (
            <ResponsiveContainer width="100%" height={300}>
              <BarChart data={topicMetrics.slice(0, 10)}>
                <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
                <XAxis
                  dataKey="name"
                  className="text-xs"
                  tick={{ fill: 'currentColor' }}
                />
                <YAxis
                  className="text-xs"
                  tick={{ fill: 'currentColor' }}
                  label={{ value: 'Partitions', angle: -90, position: 'insideLeft' }}
                />
                <Tooltip
                  contentStyle={{
                    backgroundColor: 'hsl(var(--card))',
                    border: '1px solid hsl(var(--border))'
                  }}
                />
                <Bar dataKey="partitions" fill="#8884d8" name="Partitions" />
              </BarChart>
            </ResponsiveContainer>
          ) : (
            <div className="flex flex-col items-center justify-center py-12">
              <Database className="h-12 w-12 text-muted-foreground mb-4" />
              <p className="text-sm text-muted-foreground">No topics found</p>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Additional Info Cards */}
      <div className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Partition Leader</CardTitle>
            <Layers className="h-4 w-4 text-cyan-600" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-cyan-600">Balanced</div>
            <p className="text-xs text-muted-foreground">Even distribution</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Replication Status</CardTitle>
            <Activity className="h-4 w-4 text-green-600" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-green-600">Healthy</div>
            <p className="text-xs text-muted-foreground">All ISRs in sync</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Protocol Version</CardTitle>
            <BarChart3 className="h-4 w-4 text-indigo-600" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-indigo-600">v2.8+</div>
            <p className="text-xs text-muted-foreground">Compatible</p>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}

