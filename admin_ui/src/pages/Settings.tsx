import React, { useState } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { 
  Server, 
  Database, 
  Shield, 
  Bell,
  Save,
  RefreshCw
} from 'lucide-react'
import { Textarea } from '@/components/ui/textarea'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api'

interface ServerConfig {
  host: string
  port: number
  maxConnections: number
  timeout: number
  enableCORS: boolean
  enableMetrics: boolean
}

interface DatabaseConfig {
  host: string
  port: number
  database: string
  username: string
  maxConnections: number
  connectionTimeout: number
}

interface SecurityConfig {
  enableAuth: boolean
  tokenExpiry: number
  maxLoginAttempts: number
  enableSSL: boolean
}

interface NotificationConfig {
  enableEmailAlerts: boolean
  emailRecipients: string
  enableSlackNotifications: boolean
  slackWebhook: string
  alertThresholds: {
    cpu: number
    memory: number
    queueSize: number
  }
}

interface SettingsData {
  serverConfig: ServerConfig
  dbConfig: DatabaseConfig
  securityConfig: SecurityConfig
  notificationConfig: NotificationConfig
}

const defaultSettings: SettingsData = {
  serverConfig: {
    host: '0.0.0.0',
    port: 8080,
    maxConnections: 1000,
    timeout: 30,
    enableCORS: true,
    enableMetrics: true
  },
  dbConfig: {
    host: 'localhost',
    port: 5432,
    database: 'portask',
    username: 'portask_user',
    maxConnections: 50,
    connectionTimeout: 10
  },
  securityConfig: {
    enableAuth: true,
    tokenExpiry: 3600,
    maxLoginAttempts: 5,
    enableSSL: false
  },
  notificationConfig: {
    enableEmailAlerts: true,
    emailRecipients: 'admin@example.com',
    enableSlackNotifications: false,
    slackWebhook: '',
    alertThresholds: {
      cpu: 80,
      memory: 85,
      queueSize: 10000
    }
  }
}

const Settings: React.FC = () => {
  const queryClient = useQueryClient()
  // Local state fallback (ilk yükleme için)
  const [localSettings, setLocalSettings] = useState<SettingsData>(defaultSettings)
  const [saving, setSaving] = useState(false)

  // Ayarları backend'den çek
  const { data, isLoading, isError } = useQuery<SettingsData>({
    queryKey: ['settings'],
    queryFn: async () => {
      const res = await api.get('/admin/config')
      return res.data as SettingsData
    }
  })
  console.log("data",data)

  // Ayarları backend'e kaydet
  const mutation = useMutation({
    mutationFn: async (settings: SettingsData) => {
      await api.put('/admin/config', settings)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['settings'] })
    }
  })

  // Form state'leri
  const [serverConfig, setServerConfig] = useState<ServerConfig>(localSettings.serverConfig)
  const [dbConfig, setDbConfig] = useState<DatabaseConfig>(localSettings.dbConfig)
  const [securityConfig, setSecurityConfig] = useState<SecurityConfig>(localSettings.securityConfig)
  const [notificationConfig, setNotificationConfig] = useState<NotificationConfig>(localSettings.notificationConfig)

  // Ayarlar yüklendiğinde form state'lerini güncelle
  React.useEffect(() => {
    if (data) {
      console.log("data",data)
      setServerConfig(data.serverConfig)
      setDbConfig(data.dbConfig)
      setSecurityConfig(data.securityConfig)
      setNotificationConfig(data.notificationConfig)
    }
  }, [data])

  const handleSave = async () => {
    setSaving(true)
    await mutation.mutateAsync({
      serverConfig,
      dbConfig,
      securityConfig,
      notificationConfig
    })
    setSaving(false)
  }

  const handleTest = async () => {
    // Test configuration
    console.log('Testing configuration...')
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-8">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
      </div>
    )
  }
  if (isError) {
    return <div className="text-red-500">Ayarlar yüklenemedi.</div>
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">Settings</h1>
          <p className="text-muted-foreground">
            Configure system settings and preferences
          </p>
        </div>
        <div className="flex items-center space-x-2">
          <Button variant="outline" onClick={handleTest}>
            <RefreshCw className="mr-2 h-4 w-4" />
            Test Connection
          </Button>
          <Button onClick={handleSave} disabled={saving}>
            {saving ? (
              <RefreshCw className="mr-2 h-4 w-4 animate-spin" />
            ) : (
              <Save className="mr-2 h-4 w-4" />
            )}
            Save Changes
          </Button>
        </div>
      </div>

      <Tabs defaultValue="server" className="space-y-4">
        <TabsList>
          <TabsTrigger value="server">
            <Server className="mr-2 h-4 w-4" />
            Server
          </TabsTrigger>
          <TabsTrigger value="database">
            <Database className="mr-2 h-4 w-4" />
            Database
          </TabsTrigger>
          <TabsTrigger value="security">
            <Shield className="mr-2 h-4 w-4" />
            Security
          </TabsTrigger>
          <TabsTrigger value="notifications">
            <Bell className="mr-2 h-4 w-4" />
            Notifications
          </TabsTrigger>
        </TabsList>

        <TabsContent value="server">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center">
                <Server className="mr-2 h-5 w-5" />
                Server Configuration
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-6">
              {isLoading ? (
                <div className="flex items-center justify-center py-8">
                  <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
                </div>
              ) : isError ? (
                <div className="text-red-500">Ayarlar yüklenemedi.</div>
              ) : (
                <>
                  <div className="grid gap-4 md:grid-cols-2">
                    <div className="space-y-2">
                      <Label htmlFor="host">Host</Label>
                      <Input
                        id="host"
                        value={serverConfig.host}
                        onChange={(e) => setServerConfig(prev => ({ ...prev, host: e.target.value }))}
                      />
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="port">Port</Label>
                      <Input
                        id="port"
                        type="number"
                        value={serverConfig.port}
                        onChange={(e) => setServerConfig(prev => ({ ...prev, port: parseInt(e.target.value) }))}
                      />
                    </div>
                  </div>

                  <div className="grid gap-4 md:grid-cols-2">
                    <div className="space-y-2">
                      <Label htmlFor="maxConnections">Max Connections</Label>
                      <Input
                        id="maxConnections"
                        type="number"
                        value={serverConfig.maxConnections}
                        onChange={(e) => setServerConfig(prev => ({ ...prev, maxConnections: parseInt(e.target.value) }))}
                      />
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="timeout">Timeout (seconds)</Label>
                      <Input
                        id="timeout"
                        type="number"
                        value={serverConfig.timeout}
                        onChange={(e) => setServerConfig(prev => ({ ...prev, timeout: parseInt(e.target.value) }))}
                      />
                    </div>
                  </div>

                  <div className="space-y-4">
                    <div className="flex items-center justify-between">
                      <div className="space-y-0.5">
                        <Label>Enable CORS</Label>
                        <p className="text-sm text-muted-foreground">
                          Allow cross-origin requests
                        </p>
                      </div>
                      <Switch
                        checked={serverConfig.enableCORS}
                        onCheckedChange={(checked: boolean) => setServerConfig(prev => ({ ...prev, enableCORS: checked }))}
                      />
                    </div>

                    <div className="flex items-center justify-between">
                      <div className="space-y-0.5">
                        <Label>Enable Metrics</Label>
                        <p className="text-sm text-muted-foreground">
                          Collect performance metrics
                        </p>
                      </div>
                      <Switch
                        checked={serverConfig.enableMetrics}
                        onCheckedChange={(checked: boolean) => setServerConfig(prev => ({ ...prev, enableMetrics: checked }))}
                      />
                    </div>
                  </div>
                </>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="database">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center">
                <Database className="mr-2 h-5 w-5" />
                Database Configuration
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-6">
              <div className="grid gap-4 md:grid-cols-2">
                <div className="space-y-2">
                  <Label htmlFor="dbHost">Host</Label>
                  <Input
                    id="dbHost"
                    value={dbConfig.host}
                    onChange={(e) => setDbConfig(prev => ({ ...prev, host: e.target.value }))}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="dbPort">Port</Label>
                  <Input
                    id="dbPort"
                    type="number"
                    value={dbConfig.port}
                    onChange={(e) => setDbConfig(prev => ({ ...prev, port: parseInt(e.target.value) }))}
                  />
                </div>
              </div>

              <div className="grid gap-4 md:grid-cols-2">
                <div className="space-y-2">
                  <Label htmlFor="database">Database</Label>
                  <Input
                    id="database"
                    value={dbConfig.database}
                    onChange={(e) => setDbConfig(prev => ({ ...prev, database: e.target.value }))}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="username">Username</Label>
                  <Input
                    id="username"
                    value={dbConfig.username}
                    onChange={(e) => setDbConfig(prev => ({ ...prev, username: e.target.value }))}
                  />
                </div>
              </div>

              <div className="grid gap-4 md:grid-cols-2">
                <div className="space-y-2">
                  <Label htmlFor="dbMaxConnections">Max Connections</Label>
                  <Input
                    id="dbMaxConnections"
                    type="number"
                    value={dbConfig.maxConnections}
                    onChange={(e) => setDbConfig(prev => ({ ...prev, maxConnections: parseInt(e.target.value) }))}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="connectionTimeout">Connection Timeout (seconds)</Label>
                  <Input
                    id="connectionTimeout"
                    type="number"
                    value={dbConfig.connectionTimeout}
                    onChange={(e) => setDbConfig(prev => ({ ...prev, connectionTimeout: parseInt(e.target.value) }))}
                  />
                </div>
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="security">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center">
                <Shield className="mr-2 h-5 w-5" />
                Security Configuration
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-6">
              <div className="space-y-4">
                <div className="flex items-center justify-between">
                  <div className="space-y-0.5">
                    <Label>Enable Authentication</Label>
                    <p className="text-sm text-muted-foreground">
                      Require authentication for API access
                    </p>
                  </div>
                  <Switch
                    checked={securityConfig.enableAuth}
                    onCheckedChange={(checked: boolean) => setSecurityConfig(prev => ({ ...prev, enableAuth: checked }))}
                  />
                </div>

                <div className="flex items-center justify-between">
                  <div className="space-y-0.5">
                    <Label>Enable SSL</Label>
                    <p className="text-sm text-muted-foreground">
                      Use HTTPS for secure connections
                    </p>
                  </div>
                  <Switch
                    checked={securityConfig.enableSSL}
                    onCheckedChange={(checked: boolean) => setSecurityConfig(prev => ({ ...prev, enableSSL: checked }))}
                  />
                </div>
              </div>

              <div className="grid gap-4 md:grid-cols-2">
                <div className="space-y-2">
                  <Label htmlFor="tokenExpiry">Token Expiry (seconds)</Label>
                  <Input
                    id="tokenExpiry"
                    type="number"
                    value={securityConfig.tokenExpiry}
                    onChange={(e) => setSecurityConfig(prev => ({ ...prev, tokenExpiry: parseInt(e.target.value) }))}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="maxLoginAttempts">Max Login Attempts</Label>
                  <Input
                    id="maxLoginAttempts"
                    type="number"
                    value={securityConfig.maxLoginAttempts}
                    onChange={(e) => setSecurityConfig(prev => ({ ...prev, maxLoginAttempts: parseInt(e.target.value) }))}
                  />
                </div>
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="notifications">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center">
                <Bell className="mr-2 h-5 w-5" />
                Notification Configuration
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-6">
              <div className="space-y-4">
                <div className="flex items-center justify-between">
                  <div className="space-y-0.5">
                    <Label>Email Alerts</Label>
                    <p className="text-sm text-muted-foreground">
                      Send alerts via email
                    </p>
                  </div>
                  <Switch
                    checked={notificationConfig.enableEmailAlerts}
                    onCheckedChange={(checked: boolean) => setNotificationConfig(prev => ({ ...prev, enableEmailAlerts: checked }))}
                  />
                </div>

                {notificationConfig.enableEmailAlerts && (
                  <div className="space-y-2">
                    <Label htmlFor="emailRecipients">Email Recipients</Label>
                    <Textarea
                      id="emailRecipients"
                      placeholder="admin@example.com, user@example.com"
                      value={notificationConfig.emailRecipients}
                      onChange={(e) => setNotificationConfig(prev => ({ ...prev, emailRecipients: e.target.value }))}
                    />
                  </div>
                )}

                <div className="flex items-center justify-between">
                  <div className="space-y-0.5">
                    <Label>Slack Notifications</Label>
                    <p className="text-sm text-muted-foreground">
                      Send alerts to Slack channel
                    </p>
                  </div>
                  <Switch
                    checked={notificationConfig.enableSlackNotifications}
                    onCheckedChange={(checked: boolean) => setNotificationConfig(prev => ({ ...prev, enableSlackNotifications: checked }))}
                  />
                </div>

                {notificationConfig.enableSlackNotifications && (
                  <div className="space-y-2">
                    <Label htmlFor="slackWebhook">Slack Webhook URL</Label>
                    <Input
                      id="slackWebhook"
                      placeholder="https://hooks.slack.com/services/..."
                      value={notificationConfig.slackWebhook}
                      onChange={(e) => setNotificationConfig(prev => ({ ...prev, slackWebhook: e.target.value }))}
                    />
                  </div>
                )}
              </div>

              <div className="space-y-4">
                <h3 className="text-lg font-medium">Alert Thresholds</h3>
                <div className="grid gap-4 md:grid-cols-3">
                  <div className="space-y-2">
                    <Label htmlFor="cpuThreshold">CPU Usage (%)</Label>
                    <Input
                      id="cpuThreshold"
                      type="number"
                      value={notificationConfig.alertThresholds.cpu}
                      onChange={(e) => setNotificationConfig(prev => ({
                        ...prev,
                        alertThresholds: { ...prev.alertThresholds, cpu: parseInt(e.target.value) }
                      }))}
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="memoryThreshold">Memory Usage (%)</Label>
                    <Input
                      id="memoryThreshold"
                      type="number"
                      value={notificationConfig.alertThresholds.memory}
                      onChange={(e) => setNotificationConfig(prev => ({
                        ...prev,
                        alertThresholds: { ...prev.alertThresholds, memory: parseInt(e.target.value) }
                      }))}
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="queueThreshold">Queue Size</Label>
                    <Input
                      id="queueThreshold"
                      type="number"
                      value={notificationConfig.alertThresholds.queueSize}
                      onChange={(e) => setNotificationConfig(prev => ({
                        ...prev,
                        alertThresholds: { ...prev.alertThresholds, queueSize: parseInt(e.target.value) }
                      }))}
                    />
                  </div>
                </div>
              </div>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  )
}

export default Settings
