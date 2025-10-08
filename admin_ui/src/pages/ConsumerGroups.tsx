import { useEffect, useState } from 'react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { 
  Users, 
  Activity, 
  AlertCircle, 
  CheckCircle, 
  RefreshCw,
  TrendingUp,
  Clock
} from 'lucide-react'
import { apiBase } from '@/lib/api'

interface ConsumerGroup {
  id: string
  name: string
  state: 'Stable' | 'Rebalancing' | 'Dead' | 'Empty'
  protocol: string
  protocolType: string
  members: Member[]
}

interface Member {
  id: string
  clientId: string
  clientHost: string
  metadata: string
  assignment: PartitionAssignment[]
}

interface PartitionAssignment {
  topic: string
  partitions: number[]
}

interface GroupLag {
  group: string
  topic: string
  partition: number
  currentOffset: number
  logEndOffset: number
  lag: number
}

export default function ConsumerGroups() {
  const [groups, setGroups] = useState<ConsumerGroup[]>([])
  const [selectedGroup, setSelectedGroup] = useState<string>('')
  const [groupDetails, setGroupDetails] = useState<ConsumerGroup | null>(null)
  const [groupLag, setGroupLag] = useState<GroupLag[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // Fetch all consumer groups
  const fetchGroups = async () => {
    setLoading(true)
    setError(null)
    try {
      // TODO: Replace with actual API endpoint when backend implements it
      // For now, show sample data
      const sampleGroups: ConsumerGroup[] = [
        {
          id: 'consumer-group-1',
          name: 'consumer-group-1',
          state: 'Stable',
          protocol: 'range',
          protocolType: 'consumer',
          members: [
            {
              id: 'consumer-1',
              clientId: 'consumer-1',
              clientHost: '/127.0.0.1',
              metadata: 'consumer-1-metadata',
              assignment: [
                { topic: 'orders', partitions: [0, 1] },
                { topic: 'payments', partitions: [0] }
              ]
            }
          ]
        }
      ]
      
      setGroups(sampleGroups)
      
      if (sampleGroups.length > 0 && !selectedGroup) {
        setSelectedGroup(sampleGroups[0].name)
      }
    } catch (err) {
      setError('Failed to fetch consumer groups')
      console.error(err)
    } finally {
      setLoading(false)
    }
  }

  // Fetch group details
  const fetchGroupDetails = async (groupName: string) => {
    try {
      // TODO: Replace with actual API call
      // GET /api/v1/kafka/consumer-groups/{groupName}
      const group = groups.find(g => g.name === groupName)
      setGroupDetails(group || null)
      
      // Fetch lag info
      if (group) {
        const sampleLag: GroupLag[] = [
          {
            group: groupName,
            topic: 'orders',
            partition: 0,
            currentOffset: 1500,
            logEndOffset: 1502,
            lag: 2
          },
          {
            group: groupName,
            topic: 'orders',
            partition: 1,
            currentOffset: 3200,
            logEndOffset: 3200,
            lag: 0
          },
          {
            group: groupName,
            topic: 'payments',
            partition: 0,
            currentOffset: 890,
            logEndOffset: 895,
            lag: 5
          }
        ]
        setGroupLag(sampleLag)
      }
    } catch (err) {
      console.error('Failed to fetch group details:', err)
    }
  }

  useEffect(() => {
    fetchGroups()
    // Refresh every 10 seconds
    const interval = setInterval(fetchGroups, 10000)
    return () => clearInterval(interval)
  }, [])

  useEffect(() => {
    if (selectedGroup) {
      fetchGroupDetails(selectedGroup)
    }
  }, [selectedGroup, groups])

  const getStateColor = (state: string) => {
    switch (state) {
      case 'Stable':
        return 'bg-green-500/10 text-green-600 border-green-500/20'
      case 'Rebalancing':
        return 'bg-yellow-500/10 text-yellow-600 border-yellow-500/20'
      case 'Dead':
        return 'bg-red-500/10 text-red-600 border-red-500/20'
      case 'Empty':
        return 'bg-gray-500/10 text-gray-600 border-gray-500/20'
      default:
        return 'bg-gray-500/10 text-gray-600 border-gray-500/20'
    }
  }

  const getStateIcon = (state: string) => {
    switch (state) {
      case 'Stable':
        return <CheckCircle className="h-4 w-4" />
      case 'Rebalancing':
        return <Activity className="h-4 w-4 animate-pulse" />
      case 'Dead':
        return <AlertCircle className="h-4 w-4" />
      case 'Empty':
        return <Users className="h-4 w-4" />
      default:
        return null
    }
  }

  const totalLag = groupLag.reduce((sum, item) => sum + item.lag, 0)
  const maxLag = groupLag.length > 0 ? Math.max(...groupLag.map(l => l.lag)) : 0

  return (
    <div className="flex-1 space-y-4 p-4 md:p-8 pt-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-3xl font-bold tracking-tight">Consumer Groups</h2>
          <p className="text-muted-foreground">
            Manage and monitor Kafka consumer groups
          </p>
        </div>
        <Button onClick={fetchGroups} disabled={loading}>
          <RefreshCw className={`h-4 w-4 mr-2 ${loading ? 'animate-spin' : ''}`} />
          Refresh
        </Button>
      </div>

      {error && (
        <Card className="border-red-500/20 bg-red-500/5">
          <CardContent className="pt-6">
            <div className="flex items-center space-x-2 text-red-600">
              <AlertCircle className="h-5 w-5" />
              <p>{error}</p>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Summary Cards */}
      <div className="grid gap-4 md:grid-cols-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Groups</CardTitle>
            <Users className="h-4 w-4 text-blue-600" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-blue-600">{groups.length}</div>
            <p className="text-xs text-muted-foreground">
              Active consumer groups
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Members</CardTitle>
            <Activity className="h-4 w-4 text-green-600" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-green-600">
              {groupDetails?.members.length || 0}
            </div>
            <p className="text-xs text-muted-foreground">
              Active consumers
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Lag</CardTitle>
            <TrendingUp className="h-4 w-4 text-orange-600" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-orange-600">{totalLag}</div>
            <p className="text-xs text-muted-foreground">
              Messages behind
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Max Lag</CardTitle>
            <Clock className="h-4 w-4 text-purple-600" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-purple-600">{maxLag}</div>
            <p className="text-xs text-muted-foreground">
              Slowest partition
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Group Selector */}
      <Card>
        <CardHeader>
          <CardTitle>Select Consumer Group</CardTitle>
          <CardDescription>
            Choose a consumer group to view details
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Select value={selectedGroup} onValueChange={setSelectedGroup}>
            <SelectTrigger className="w-full">
              <SelectValue placeholder="Select a group..." />
            </SelectTrigger>
            <SelectContent>
              {groups.map((group) => (
                <SelectItem key={group.name} value={group.name}>
                  <div className="flex items-center space-x-2">
                    <span>{group.name}</span>
                    <Badge className={getStateColor(group.state)}>
                      {group.state}
                    </Badge>
                  </div>
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </CardContent>
      </Card>

      {/* Group Details */}
      {groupDetails && (
        <>
          <Card>
            <CardHeader>
              <div className="flex items-center justify-between">
                <CardTitle>Group Details</CardTitle>
                <Badge className={getStateColor(groupDetails.state)}>
                  <div className="flex items-center space-x-1">
                    {getStateIcon(groupDetails.state)}
                    <span>{groupDetails.state}</span>
                  </div>
                </Badge>
              </div>
            </CardHeader>
            <CardContent>
              <div className="grid gap-4 md:grid-cols-3">
                <div>
                  <p className="text-sm font-medium text-muted-foreground">
                    Group ID
                  </p>
                  <p className="text-lg font-mono">{groupDetails.name}</p>
                </div>
                <div>
                  <p className="text-sm font-medium text-muted-foreground">
                    Protocol
                  </p>
                  <p className="text-lg">{groupDetails.protocol}</p>
                </div>
                <div>
                  <p className="text-sm font-medium text-muted-foreground">
                    Protocol Type
                  </p>
                  <p className="text-lg">{groupDetails.protocolType}</p>
                </div>
              </div>
            </CardContent>
          </Card>

          {/* Members */}
          <Card>
            <CardHeader>
              <CardTitle>Group Members</CardTitle>
              <CardDescription>
                {groupDetails.members.length} active consumer(s)
              </CardDescription>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Member ID</TableHead>
                    <TableHead>Client ID</TableHead>
                    <TableHead>Host</TableHead>
                    <TableHead>Assigned Partitions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {groupDetails.members.map((member) => (
                    <TableRow key={member.id}>
                      <TableCell className="font-mono text-xs">
                        {member.id}
                      </TableCell>
                      <TableCell>{member.clientId}</TableCell>
                      <TableCell className="font-mono text-xs">
                        {member.clientHost}
                      </TableCell>
                      <TableCell>
                        <div className="flex flex-wrap gap-1">
                          {member.assignment.map((assign, idx) => (
                            <Badge key={idx} variant="outline">
                              {assign.topic}: {assign.partitions.join(', ')}
                            </Badge>
                          ))}
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </CardContent>
          </Card>

          {/* Consumer Lag */}
          <Card>
            <CardHeader>
              <CardTitle>Consumer Lag</CardTitle>
              <CardDescription>
                Offset lag per topic partition
              </CardDescription>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Topic</TableHead>
                    <TableHead>Partition</TableHead>
                    <TableHead>Current Offset</TableHead>
                    <TableHead>Log End Offset</TableHead>
                    <TableHead>Lag</TableHead>
                    <TableHead>Status</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {groupLag.map((lag, idx) => {
                    const lagStatus = lag.lag === 0 ? 'current' : lag.lag < 10 ? 'minor' : 'major'
                    const statusColor = 
                      lagStatus === 'current' ? 'text-green-600' :
                      lagStatus === 'minor' ? 'text-yellow-600' :
                      'text-red-600'
                    
                    return (
                      <TableRow key={idx}>
                        <TableCell className="font-medium">{lag.topic}</TableCell>
                        <TableCell>{lag.partition}</TableCell>
                        <TableCell className="font-mono">
                          {lag.currentOffset.toLocaleString()}
                        </TableCell>
                        <TableCell className="font-mono">
                          {lag.logEndOffset.toLocaleString()}
                        </TableCell>
                        <TableCell className={`font-bold ${statusColor}`}>
                          {lag.lag}
                        </TableCell>
                        <TableCell>
                          <Badge 
                            variant="outline" 
                            className={
                              lagStatus === 'current' ? 'border-green-500/20 bg-green-500/10 text-green-600' :
                              lagStatus === 'minor' ? 'border-yellow-500/20 bg-yellow-500/10 text-yellow-600' :
                              'border-red-500/20 bg-red-500/10 text-red-600'
                            }
                          >
                            {lagStatus === 'current' ? 'Up to date' : lagStatus === 'minor' ? 'Minor lag' : 'Behind'}
                          </Badge>
                        </TableCell>
                      </TableRow>
                    )
                  })}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </>
      )}

      {groups.length === 0 && !loading && (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-12">
            <Users className="h-12 w-12 text-muted-foreground mb-4" />
            <h3 className="text-lg font-medium mb-2">No Consumer Groups Found</h3>
            <p className="text-sm text-muted-foreground text-center max-w-md">
              There are no active consumer groups. Consumer groups will appear here once
              clients start consuming from Kafka topics.
            </p>
          </CardContent>
        </Card>
      )}
    </div>
  )
}

