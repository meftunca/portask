import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { apiBase } from '@/lib/api'
import { X } from 'lucide-react'
import { useState } from 'react'

interface CreateConsumerGroupModalProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSuccess: () => void
}

export function CreateConsumerGroupModal({ open, onOpenChange, onSuccess }: CreateConsumerGroupModalProps) {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [formData, setFormData] = useState({
    group_id: '',
    member_id: '',
    topics: [] as string[],
    protocol: 'range',
    protocol_type: 'consumer'
  })
  const [topicInput, setTopicInput] = useState('')

  const handleAddTopic = () => {
    if (topicInput.trim() && !formData.topics.includes(topicInput.trim())) {
      setFormData({
        ...formData,
        topics: [...formData.topics, topicInput.trim()]
      })
      setTopicInput('')
    }
  }

  const handleRemoveTopic = (topic: string) => {
    setFormData({
      ...formData,
      topics: formData.topics.filter(t => t !== topic)
    })
  }

  const handleSubmit = async () => {
    setLoading(true)
    setError(null)

    try {
      // Create consumer group
      await apiBase.post('/api/v1/consumer-groups', {
        group_id: formData.group_id,
        protocol: formData.protocol,
        protocol_type: formData.protocol_type
      })

      // Join the group with a member
      await apiBase.post(`/api/v1/consumer-groups/${formData.group_id}/join`, {
        member_id: formData.member_id,
        topics: formData.topics,
        metadata: {}
      })

      onSuccess()
      onOpenChange(false)

      // Reset form
      setFormData({
        group_id: '',
        member_id: '',
        topics: [],
        protocol: 'range',
        protocol_type: 'consumer'
      })
    } catch (err: any) {
      console.error('Failed to create consumer group:', err)
      setError(err.response?.data?.error || err.message || 'Failed to create consumer group')
    } finally {
      setLoading(false)
    }
  }

  const isValid = formData.group_id && formData.member_id && formData.topics.length > 0

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-xl">
        <DialogHeader>
          <DialogTitle>Create Consumer Group</DialogTitle>
          <DialogDescription>
            Create a new consumer group with initial member
          </DialogDescription>
        </DialogHeader>

        {error && (
          <div className="bg-red-50 border border-red-200 rounded-lg p-3 text-sm text-red-800">
            <strong>Error:</strong> {error}
          </div>
        )}

        <div className="space-y-4">
          <div>
            <Label htmlFor="group_id">Group ID *</Label>
            <Input
              id="group_id"
              placeholder="my-consumer-group"
              value={formData.group_id}
              onChange={(e) => setFormData({ ...formData, group_id: e.target.value })}
              disabled={loading}
            />
          </div>

          <div>
            <Label htmlFor="member_id">Initial Member ID *</Label>
            <Input
              id="member_id"
              placeholder="consumer-1"
              value={formData.member_id}
              onChange={(e) => setFormData({ ...formData, member_id: e.target.value })}
              disabled={loading}
            />
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <Label htmlFor="protocol">Protocol</Label>
              <select
                id="protocol"
                className="w-full px-3 py-2 border border-gray-300 rounded-md text-sm"
                value={formData.protocol}
                onChange={(e) => setFormData({ ...formData, protocol: e.target.value })}
                disabled={loading}
              >
                <option value="range">Range</option>
                <option value="roundrobin">Round Robin</option>
                <option value="sticky">Sticky</option>
              </select>
            </div>
            <div>
              <Label htmlFor="protocol_type">Protocol Type</Label>
              <select
                id="protocol_type"
                className="w-full px-3 py-2 border border-gray-300 rounded-md text-sm"
                value={formData.protocol_type}
                onChange={(e) => setFormData({ ...formData, protocol_type: e.target.value })}
                disabled={loading}
              >
                <option value="consumer">Consumer</option>
              </select>
            </div>
          </div>

          <div>
            <Label htmlFor="topics">Subscribed Topics *</Label>
            <div className="flex gap-2 mt-1">
              <Input
                id="topics"
                placeholder="orders"
                value={topicInput}
                onChange={(e) => setTopicInput(e.target.value)}
                onKeyPress={(e) => {
                  if (e.key === 'Enter') {
                    e.preventDefault()
                    handleAddTopic()
                  }
                }}
                disabled={loading}
              />
              <Button
                type="button"
                variant="outline"
                onClick={handleAddTopic}
                disabled={loading || !topicInput.trim()}
              >
                Add
              </Button>
            </div>
            {formData.topics.length > 0 && (
              <div className="flex flex-wrap gap-2 mt-2">
                {formData.topics.map((topic) => (
                  <Badge key={topic} variant="secondary" className="gap-1">
                    {topic}
                    <X
                      className="h-3 w-3 cursor-pointer"
                      onClick={() => handleRemoveTopic(topic)}
                    />
                  </Badge>
                ))}
              </div>
            )}
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={loading}>
            Cancel
          </Button>
          <Button onClick={handleSubmit} disabled={loading || !isValid}>
            {loading ? 'Creating...' : 'Create Consumer Group'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

