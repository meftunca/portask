import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { apiBase } from '@/lib/api'
import { Archive, Edit, GitBranch, MessageSquare, Plus, RefreshCw, Trash2 } from 'lucide-react'
import { useEffect, useState } from 'react'

interface Topic {
  name: string
  partitions: number
  replication_factor: number
  message_count: number
  total_bytes: number
  created_at: string
  config: {
    retention_ms: number
    compression_type: string
  }
}

export default function TopicsNative() {
  const [topics, setTopics] = useState<Topic[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  // Modal states
  const [selectedTopic, setSelectedTopic] = useState<Topic | null>(null)
  const [showDetailModal, setShowDetailModal] = useState(false)
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [showDeleteModal, setShowDeleteModal] = useState(false)

  // Form state for create/edit
  const [formData, setFormData] = useState({
    name: '',
    partitions: 1,
    replication_factor: 1,
    retention_ms: 604800000, // 7 days default
    compression_type: 'snappy'
  })

  const fetchTopics = async () => {
    setLoading(true)
    setError(null)
    try {
      const response = await apiBase.get('/api/v1/topics')

      if (response.data?.success) {
        setTopics(response.data.topics || [])
      } else {
        setError(response.data?.error || 'Failed to fetch topics')
        setTopics([])
      }
    } catch (err: any) {
      console.error('[TopicsNative] Error:', err)
      setError(err.message || 'Failed to connect to API')
      setTopics([])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchTopics()
    const interval = setInterval(fetchTopics, 10000)
    return () => clearInterval(interval)
  }, [])

  const formatBytes = (bytes: number) => {
    if (bytes < 1024) return `${bytes} B`
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(2)} KB`
    if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(2)} MB`
    return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`
  }

  const formatRetention = (ms: number) => {
    const hours = ms / (1000 * 60 * 60)
    if (hours < 24) return `${hours}h`
    const days = hours / 24
    return `${Math.round(days)}d`
  }

  return (
    <div className="flex-1 space-y-4 p-4 md:p-8 pt-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-3xl font-bold tracking-tight">Topics</h2>
          <p className="text-muted-foreground">
            Unified topic management for all protocols
          </p>
        </div>
        <div className="flex items-center space-x-2">
          <Button variant="outline" size="sm" onClick={fetchTopics} disabled={loading}>
            <RefreshCw className={loading ? "mr-2 h-4 w-4 animate-spin" : "mr-2 h-4 w-4"} />
            Refresh
          </Button>
          <Button size="sm" onClick={() => setShowCreateModal(true)}>
            <Plus className="mr-2 h-4 w-4" />
            Create Topic
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
            <CardTitle className="text-sm font-medium">Total Topics</CardTitle>
            <GitBranch className="h-4 w-4 text-blue-600" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-blue-600">{topics.length}</div>
            <p className="text-xs text-muted-foreground mt-1">Active topics</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Messages</CardTitle>
            <MessageSquare className="h-4 w-4 text-purple-600" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-purple-600">
              {topics.reduce((sum, t) => sum + t.message_count, 0).toLocaleString()}
            </div>
            <p className="text-xs text-muted-foreground mt-1">Across all topics</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Storage</CardTitle>
            <Archive className="h-4 w-4 text-orange-600" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-orange-600">
              {formatBytes(topics.reduce((sum, t) => sum + t.total_bytes, 0))}
            </div>
            <p className="text-xs text-muted-foreground mt-1">Storage used</p>
          </CardContent>
        </Card>
      </div>

      {/* Topics Table */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center">
            <GitBranch className="mr-2 h-5 w-5" /> Topics List
          </CardTitle>
          <CardDescription>
            All topics managed by Portask Native API
          </CardDescription>
        </CardHeader>
        <CardContent>
          {loading ? (
            <div className="flex items-center justify-center py-8">
              <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
            </div>
          ) : topics.length === 0 ? (
            <div className="text-center py-8">
              <GitBranch className="mx-auto h-12 w-12 text-muted-foreground opacity-50" />
              <p className="mt-4 text-muted-foreground">No topics found. Create your first topic to get started.</p>
              <Button className="mt-4" onClick={() => setShowCreateModal(true)}>
                <Plus className="mr-2 h-4 w-4" />
                Create Topic
              </Button>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Partitions</TableHead>
                  <TableHead>Messages</TableHead>
                  <TableHead>Size</TableHead>
                  <TableHead>Retention</TableHead>
                  <TableHead>Compression</TableHead>
                  <TableHead>Created</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {topics.map((topic) => (
                  <TableRow
                    key={topic.name}
                    className="cursor-pointer hover:bg-muted/50"
                    onClick={() => {
                      setSelectedTopic(topic)
                      setShowDetailModal(true)
                    }}
                  >
                    <TableCell className="font-medium">{topic.name}</TableCell>
                    <TableCell>
                      <Badge variant="outline">{topic.partitions}</Badge>
                    </TableCell>
                    <TableCell>{topic.message_count.toLocaleString()}</TableCell>
                    <TableCell>{formatBytes(topic.total_bytes)}</TableCell>
                    <TableCell>{formatRetention(topic.config.retention_ms)}</TableCell>
                    <TableCell>
                      <Badge variant="secondary">{topic.config.compression_type}</Badge>
                    </TableCell>
                    <TableCell>{new Date(topic.created_at).toLocaleDateString()}</TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end space-x-2" onClick={(e) => e.stopPropagation()}>
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => {
                            setSelectedTopic(topic)
                            setShowDetailModal(true)
                          }}
                        >
                          <Edit className="h-4 w-4" />
                        </Button>
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => {
                            setSelectedTopic(topic)
                            setShowDeleteModal(true)
                          }}
                        >
                          <Trash2 className="h-4 w-4 text-red-600" />
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

      {/* Detail Modal */}
      <Dialog open={showDetailModal} onOpenChange={setShowDetailModal}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>Topic Details</DialogTitle>
            <DialogDescription>
              View and manage topic configuration
            </DialogDescription>
          </DialogHeader>
          {selectedTopic && (
            <div className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <Label>Topic Name</Label>
                  <p className="font-semibold">{selectedTopic.name}</p>
                </div>
                <div>
                  <Label>Partitions</Label>
                  <p className="font-semibold">{selectedTopic.partitions}</p>
                </div>
                <div>
                  <Label>Replication Factor</Label>
                  <p className="font-semibold">{selectedTopic.replication_factor}</p>
                </div>
                <div>
                  <Label>Message Count</Label>
                  <p className="font-semibold">{selectedTopic.message_count.toLocaleString()}</p>
                </div>
                <div>
                  <Label>Total Size</Label>
                  <p className="font-semibold">{formatBytes(selectedTopic.total_bytes)}</p>
                </div>
                <div>
                  <Label>Retention</Label>
                  <p className="font-semibold">{formatRetention(selectedTopic.config.retention_ms)}</p>
                </div>
                <div>
                  <Label>Compression</Label>
                  <Badge variant="secondary">{selectedTopic.config.compression_type}</Badge>
                </div>
                <div>
                  <Label>Created At</Label>
                  <p className="font-semibold">{new Date(selectedTopic.created_at).toLocaleString()}</p>
                </div>
              </div>
            </div>
          )}
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowDetailModal(false)}>
              Close
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Create Topic Modal */}
      <Dialog open={showCreateModal} onOpenChange={setShowCreateModal}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Create New Topic</DialogTitle>
            <DialogDescription>
              Configure your new topic
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div>
              <Label htmlFor="name">Topic Name</Label>
              <Input
                id="name"
                placeholder="my-topic"
                value={formData.name}
                onChange={(e) => setFormData({ ...formData, name: e.target.value })}
              />
            </div>
            <div className="grid grid-cols-2 gap-4">
              <div>
                <Label htmlFor="partitions">Partitions</Label>
                <Input
                  id="partitions"
                  type="number"
                  min="1"
                  value={formData.partitions}
                  onChange={(e) => setFormData({ ...formData, partitions: parseInt(e.target.value) })}
                />
              </div>
              <div>
                <Label htmlFor="replication">Replication Factor</Label>
                <Input
                  id="replication"
                  type="number"
                  min="1"
                  value={formData.replication_factor}
                  onChange={(e) => setFormData({ ...formData, replication_factor: parseInt(e.target.value) })}
                />
              </div>
            </div>
            
            <div className="grid grid-cols-2 gap-4">
              <div>
                <Label htmlFor="compression">Compression Type</Label>
                <select
                  id="compression"
                  className="w-full px-3 py-2 border border-gray-300 rounded-md text-sm"
                  value={formData.compression_type}
                  onChange={(e) => setFormData({ ...formData, compression_type: e.target.value })}
                >
                  <option value="none">None</option>
                  <option value="gzip">GZip</option>
                  <option value="snappy">Snappy</option>
                  <option value="lz4">LZ4</option>
                  <option value="zstd">Zstandard</option>
                </select>
              </div>
              <div>
                <Label htmlFor="retention">Retention (days)</Label>
                <Input
                  id="retention"
                  type="number"
                  min="1"
                  value={formData.retention_ms / (1000 * 60 * 60 * 24)}
                  onChange={(e) => setFormData({ ...formData, retention_ms: parseInt(e.target.value) * 1000 * 60 * 60 * 24 })}
                />
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowCreateModal(false)}>
              Cancel
            </Button>
            <Button onClick={async () => {
              try {
                // Format payload correctly for backend
                const payload = {
                  name: formData.name,
                  partitions: formData.partitions,
                  replication_factor: formData.replication_factor,
                  config: {
                    retention_ms: formData.retention_ms,
                    compression_type: formData.compression_type,
                    max_message_bytes: 1048576, // 1MB default
                    min_insync_replicas: 1
                  }
                }
                await apiBase.post('/api/v1/topics', payload)
                setShowCreateModal(false)
                fetchTopics()
                // Reset form
                setFormData({
                  name: '',
                  partitions: 1,
                  replication_factor: 1,
                  retention_ms: 604800000,
                  compression_type: 'snappy'
                })
              } catch (err: any) {
                console.error('Failed to create topic:', err)
                alert(err.response?.data?.error || 'Failed to create topic')
              }
            }}>
              Create Topic
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete Confirmation Modal */}
      <Dialog open={showDeleteModal} onOpenChange={setShowDeleteModal}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete Topic</DialogTitle>
            <DialogDescription>
              Are you sure you want to delete this topic? This action cannot be undone.
            </DialogDescription>
          </DialogHeader>
          {selectedTopic && (
            <div className="py-4">
              <p className="font-semibold">Topic: {selectedTopic.name}</p>
              <p className="text-sm text-muted-foreground">
                {selectedTopic.message_count} messages will be permanently deleted
              </p>
            </div>
          )}
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowDeleteModal(false)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={async () => {
                if (selectedTopic) {
                  try {
                    await apiBase.delete(`/api/v1/topics/${selectedTopic.name}`)
                    setShowDeleteModal(false)
                    fetchTopics()
                  } catch (err) {
                    console.error('Failed to delete topic:', err)
                  }
                }
              }}
            >
              Delete Topic
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

