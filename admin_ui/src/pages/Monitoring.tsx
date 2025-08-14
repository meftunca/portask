import React, { useState, useEffect } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { 
  LineChart, 
  Line, 
  AreaChart, 
  Area, 
  BarChart, 
  Bar, 
  XAxis, 
  YAxis, 
  CartesianGrid, 
  Tooltip, 
  ResponsiveContainer,
  PieChart,
  Pie,
  Cell
} from 'recharts'
import { 
  Activity, 
  AlertTriangle, 
  Clock, 
  Cpu, 
  Database, 
  MemoryStick,
  RefreshCw
} from 'lucide-react'
import { api, apiBase } from '@/lib/api'

interface SystemMetrics {
  timestamp: string
  cpu: number
  memory: number
  disk: number
  messagesPerSecond: number
  activeConnections: number
  queueSize: number
}

interface AlertMetric {
  id: string
  type: 'warning' | 'error' | 'info'
  message: string
  timestamp: Date
  component: string
}

const Monitoring: React.FC = () => {
  const [metrics, setMetrics] = useState<SystemMetrics[]>([])
  const [alerts, setAlerts] = useState<AlertMetric[]>([])
  const [loading, setLoading] = useState(true)
  const [realTimeMode, setRealTimeMode] = useState(false)

  useEffect(() => {
    // Gerçek API'den veri çek
    const fetchMonitoring = async () => {
      setLoading(true);
      try {
        const res = await api.get('/metrics');
        const data = res.data;
        
        // Backend'den gelen metrics data yapısını UI formatına çevir
        const currentTime = new Date().toISOString();
        const formattedMetrics: SystemMetrics[] = [{
          timestamp: currentTime,
          cpu: 0, // Backend'de CPU usage yok, 0 olarak ayarla
          memory: data.system?.alloc_mb || 0,
          disk: 0, // Backend'de disk usage yok
          messagesPerSecond: 0, // Bu hesaplanabilir ama şimdilik 0
          activeConnections: data.network?.connections_active || 0,
          queueSize: 0 // Backend'de queue size yok
        }];

        // Alert'leri de backend data'sından oluştur
        const formattedAlerts: AlertMetric[] = [];
        
        // Yüksek memory kullanımı kontrolü
        if (data.system?.alloc_mb > 1000) {
          formattedAlerts.push({
            id: '1',
            type: 'warning',
            message: `High memory usage: ${data.system.alloc_mb}MB`,
            timestamp: new Date(),
            component: 'memory'
          });
        }

        // Bağlantı sayısı kontrolü
        if (data.network?.connections_active > 100) {
          formattedAlerts.push({
            id: '2',
            type: 'info',
            message: `High connection count: ${data.network.connections_active}`,
            timestamp: new Date(),
            component: 'network'
          });
        }

        setMetrics(formattedMetrics);
        setAlerts(formattedAlerts);
      } catch (e) {
        console.error('Monitoring fetch error:', e);
        setMetrics([]);
        setAlerts([]);
      }
      setLoading(false);
    };
    fetchMonitoring();
  }, [])

  useEffect(() => {
    let interval: NodeJS.Timeout
    
    if (realTimeMode) {
      interval = setInterval(() => {
        setMetrics(prev => {
          const newData = [...prev.slice(1)]
          const lastMetric = prev[prev.length - 1]
          const now = new Date()
          
          newData.push({
            timestamp: now.toLocaleTimeString(),
            cpu: Math.max(0, Math.min(100, lastMetric.cpu + (Math.random() - 0.5) * 20)),
            memory: Math.max(0, Math.min(100, lastMetric.memory + (Math.random() - 0.5) * 10)),
            disk: Math.max(0, Math.min(100, lastMetric.disk + (Math.random() - 0.5) * 5)),
            messagesPerSecond: Math.max(0, lastMetric.messagesPerSecond + Math.floor((Math.random() - 0.5) * 200)),
            activeConnections: Math.max(0, lastMetric.activeConnections + Math.floor((Math.random() - 0.5) * 4)),
            queueSize: Math.max(0, lastMetric.queueSize + Math.floor((Math.random() - 0.5) * 1000))
          })
          
          return newData
        })
      }, 2000)
    }
    
    return () => {
      if (interval) clearInterval(interval)
    }
  }, [realTimeMode])

  const currentMetrics = metrics.length > 0 ? metrics[metrics.length - 1] : null

  const getAlertColor = (type: AlertMetric['type']) => {
    switch (type) {
      case 'error': return 'bg-red-100 text-red-800'
      case 'warning': return 'bg-yellow-100 text-yellow-800'
      case 'info': return 'bg-blue-100 text-blue-800'
      default: return 'bg-gray-100 text-gray-800'
    }
  }

  const getAlertIcon = (type: AlertMetric['type']) => {
    switch (type) {
      case 'error': return <AlertTriangle className="h-4 w-4" />
      case 'warning': return <AlertTriangle className="h-4 w-4" />
      case 'info': return <Activity className="h-4 w-4" />
      default: return <Activity className="h-4 w-4" />
    }
  }

  const pieData = currentMetrics ? [
    { name: 'Used', value: currentMetrics.memory, color: '#3b82f6' },
    { name: 'Free', value: 100 - currentMetrics.memory, color: '#e5e7eb' }
  ] : []

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">System Monitoring</h1>
          <p className="text-muted-foreground">
            Real-time system metrics and performance monitoring
          </p>
        </div>
        <div className="flex items-center space-x-2">
          <Button 
            variant={realTimeMode ? "default" : "outline"}
            onClick={() => setRealTimeMode(!realTimeMode)}
          >
            <RefreshCw className={`mr-2 h-4 w-4 ${realTimeMode ? 'animate-spin' : ''}`} />
            {realTimeMode ? 'Real-time ON' : 'Real-time OFF'}
          </Button>
        </div>
      </div>

      {/* Current Metrics */}
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">CPU Usage</CardTitle>
            <Cpu className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {currentMetrics ? `${currentMetrics.cpu.toFixed(1)}%` : 'N/A'}
            </div>
            <p className="text-xs text-muted-foreground">
              Current utilization
            </p>
          </CardContent>
        </Card>
        
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Memory Usage</CardTitle>
            <MemoryStick className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {currentMetrics ? `${currentMetrics.memory.toFixed(1)}%` : 'N/A'}
            </div>
            <p className="text-xs text-muted-foreground">
              RAM utilization
            </p>
          </CardContent>
        </Card>
        
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Messages/sec</CardTitle>
            <Activity className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {currentMetrics ? currentMetrics.messagesPerSecond.toLocaleString() : 'N/A'}
            </div>
            <p className="text-xs text-muted-foreground">
              Current throughput
            </p>
          </CardContent>
        </Card>
        
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Queue Size</CardTitle>
            <Database className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {currentMetrics ? currentMetrics.queueSize.toLocaleString() : 'N/A'}
            </div>
            <p className="text-xs text-muted-foreground">
              Pending messages
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Charts */}
      <div className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>System Performance</CardTitle>
          </CardHeader>
          <CardContent>
            <ResponsiveContainer width="100%" height={300}>
              <LineChart data={metrics}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="timestamp" />
                <YAxis />
                <Tooltip />
                <Line 
                  type="monotone" 
                  dataKey="cpu" 
                  stroke="#3b82f6" 
                  strokeWidth={2}
                  name="CPU (%)"
                />
                <Line 
                  type="monotone" 
                  dataKey="memory" 
                  stroke="#10b981" 
                  strokeWidth={2}
                  name="Memory (%)"
                />
              </LineChart>
            </ResponsiveContainer>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Message Throughput</CardTitle>
          </CardHeader>
          <CardContent>
            <ResponsiveContainer width="100%" height={300}>
              <AreaChart data={metrics}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="timestamp" />
                <YAxis />
                <Tooltip />
                <Area 
                  type="monotone" 
                  dataKey="messagesPerSecond" 
                  stroke="#8b5cf6" 
                  fill="#8b5cf6" 
                  fillOpacity={0.3}
                  name="Messages/sec"
                />
              </AreaChart>
            </ResponsiveContainer>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Active Connections</CardTitle>
          </CardHeader>
          <CardContent>
            <ResponsiveContainer width="100%" height={300}>
              <BarChart data={metrics.slice(-12)}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="timestamp" />
                <YAxis />
                <Tooltip />
                <Bar 
                  dataKey="activeConnections" 
                  fill="#f59e0b" 
                  name="Connections"
                />
              </BarChart>
            </ResponsiveContainer>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Memory Distribution</CardTitle>
          </CardHeader>
          <CardContent>
            <ResponsiveContainer width="100%" height={300}>
              <PieChart>
                <Pie
                  data={pieData}
                  cx="50%"
                  cy="50%"
                  outerRadius={80}
                  fill="#8884d8"
                  dataKey="value"
                  label={({ name, value }) => `${name}: ${value.toFixed(1)}%`}
                >
                  {pieData.map((entry, index) => (
                    <Cell key={`cell-${index}`} fill={entry.color} />
                  ))}
                </Pie>
                <Tooltip />
              </PieChart>
            </ResponsiveContainer>
          </CardContent>
        </Card>
      </div>

      {/* Alerts */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center">
            <AlertTriangle className="mr-2 h-5 w-5" />
            Recent Alerts
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            {alerts.map((alert) => (
              <div 
                key={alert.id}
                className="flex items-center space-x-4 p-4 border rounded-lg"
              >
                <div className="flex-shrink-0">
                  <Badge className={getAlertColor(alert.type)}>
                    {getAlertIcon(alert.type)}
                  </Badge>
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium text-gray-900">
                    {alert.message}
                  </p>
                  <p className="text-sm text-gray-500">
                    {alert.component} • {alert.timestamp.toLocaleString()}
                  </p>
                </div>
                <div className="flex-shrink-0">
                  <Button variant="ghost" size="sm">
                    Dismiss
                  </Button>
                </div>
              </div>
            ))}
          </div>
        </CardContent>
      </Card>
    </div>
  )
}

export default Monitoring
