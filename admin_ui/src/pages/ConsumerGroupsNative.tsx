import { ConsumerGroupModal } from '@/components/modals/ConsumerGroupModal'
import { CreateConsumerGroupModal } from '@/components/modals/CreateConsumerGroupModal'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { apiBase } from '@/lib/api'
import { Activity, Eye, Plus, RefreshCw, TrendingDown, Users } from 'lucide-react'
import { useEffect, useState } from 'react'

interface ConsumerGroup {
  id: string
  name: string
  state: string
  protocol: string
  members: any[]
  subscriptions: string[]
  created_at: string
  generation: number
}

export default function ConsumerGroupsNative() {
  const [groups, setGroups] = useState<ConsumerGroup[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  // Modal state
  const [selectedGroup, setSelectedGroup] = useState<ConsumerGroup | null>(null)
  const [showDetailModal, setShowDetailModal] = useState(false)
  const [showCreateModal, setShowCreateModal] = useState(false)

  const fetchGroups = async () => {
    setLoading(true)
    setError(null)
    try {
      const response = await apiBase.get('/api/v1/consumer-groups')

      if (response.data?.success) {
        setGroups(response.data.groups || [])
      } else {
        setError(response.data?.error || 'Failed to fetch consumer groups')
        setGroups([])
      }
    } catch (err: any) {
      console.error('[ConsumerGroupsNative] Error:', err)
      setError(err.message || 'Failed to connect to API')
      setGroups([])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchGroups()
    const interval = setInterval(fetchGroups, 10000)
    return () => clearInterval(interval)
  }, [])

  const getStateBadgeVariant = (state: string) => {
    switch (state.toLowerCase()) {
      case 'stable':
        return 'default'
      case 'rebalancing':
        return 'secondary'
      case 'empty':
        return 'outline'
      case 'dead':
        return 'destructive'
      default:
        return 'outline'
    }
  }

  return (
    <div className="flex-1 space-y-4 p-4 md:p-8 pt-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-3xl font-bold tracking-tight">Consumer Groups</h2>
          <p className="text-muted-foreground">
            Unified consumer group management
          </p>
        </div>
        <div className="flex items-center space-x-2">
          <Button variant="outline" size="sm" onClick={fetchGroups} disabled={loading}>
            <RefreshCw className={loading ? "mr-2 h-4 w-4 animate-spin" : "mr-2 h-4 w-4"} />
            Refresh
          </Button>
          <Button size="sm" onClick={() => setShowCreateModal(true)}>
            <Plus className="mr-2 h-4 w-4" />
            Create Group
          </Button>
        </div>
      </div>

      {error && (
        <div className="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded relative" role="alert">
          <strong className="font-bold">Error:</strong>
          <span className="block sm:inline"> {error}</span>
        </div>
      )}

      {/* Summary Cards */}
      <div className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Groups</CardTitle>
            <Users className="h-4 w-4 text-blue-600" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-blue-600">{groups.length}</div>
            <p className="text-xs text-muted-foreground mt-1">Active consumer groups</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Active Members</CardTitle>
            <Activity className="h-4 w-4 text-green-600" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-green-600">
              {groups.reduce((sum, g) => sum + (g.members?.length || 0), 0)}
            </div>
            <p className="text-xs text-muted-foreground mt-1">Connected consumers</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Stable Groups</CardTitle>
            <TrendingDown className="h-4 w-4 text-purple-600" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-purple-600">
              {groups.filter(g => g.state === 'Stable').length}
            </div>
            <p className="text-xs text-muted-foreground mt-1">No rebalancing needed</p>
          </CardContent>
        </Card>
      </div>

      {/* Groups Table */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center">
            <Users className="mr-2 h-5 w-5" /> Consumer Groups
          </CardTitle>
          <CardDescription>
            All consumer groups managed by Portask Native API
          </CardDescription>
        </CardHeader>
        <CardContent>
          {loading ? (
            <div className="flex items-center justify-center py-8">
              <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
            </div>
          ) : groups.length === 0 ? (
            <div className="text-center py-8">
              <Users className="mx-auto h-12 w-12 text-muted-foreground opacity-50" />
              <p className="mt-4 text-muted-foreground">No consumer groups found. Create your first group to get started.</p>
              <Button className="mt-4" onClick={() => setShowCreateModal(true)}>
                <Plus className="mr-2 h-4 w-4" />
                Create Group
              </Button>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Group ID</TableHead>
                  <TableHead>State</TableHead>
                  <TableHead>Protocol</TableHead>
                  <TableHead>Members</TableHead>
                  <TableHead>Topics</TableHead>
                  <TableHead>Generation</TableHead>
                  <TableHead>Created</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {groups.map((group) => (
                  <TableRow
                    key={group.id}
                    className="cursor-pointer hover:bg-muted/50"
                    onClick={() => {
                      setSelectedGroup(group)
                      setShowDetailModal(true)
                    }}
                  >
                    <TableCell className="font-medium">{group.name}</TableCell>
                    <TableCell>
                      <Badge variant={getStateBadgeVariant(group.state)}>
                        {group.state}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline">{group.protocol}</Badge>
                    </TableCell>
                    <TableCell>{group.members?.length || 0}</TableCell>
                    <TableCell>{group.subscriptions?.join(', ') || '-'}</TableCell>
                    <TableCell>{group.generation}</TableCell>
                    <TableCell>{new Date(group.created_at).toLocaleDateString()}</TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end space-x-2" onClick={(e) => e.stopPropagation()}>
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => {
                            setSelectedGroup(group)
                            setShowDetailModal(true)
                          }}
                        >
                          <Eye className="h-4 w-4" />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      {/* Consumer Group Detail Modal */}
      <ConsumerGroupModal
        group={selectedGroup}
        open={showDetailModal}
        onOpenChange={setShowDetailModal}
      />

      {/* Create Consumer Group Modal */}
      <CreateConsumerGroupModal
        open={showCreateModal}
        onOpenChange={setShowCreateModal}
        onSuccess={fetchGroups}
      />
    </div>
  )
}

