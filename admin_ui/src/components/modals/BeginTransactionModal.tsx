import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { apiBase } from '@/lib/api'
import { X } from 'lucide-react'
import { useState } from 'react'

interface BeginTransactionModalProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSuccess: () => void
}

export function BeginTransactionModal({ open, onOpenChange, onSuccess }: BeginTransactionModalProps) {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [formData, setFormData] = useState({
    transaction_id: '',
    topics: [] as string[],
    timeout_ms: 60000 // 60 seconds default
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
      await apiBase.post('/api/v1/transactions/begin', {
        transaction_id: formData.transaction_id,
        topics: formData.topics,
        timeout_ms: formData.timeout_ms
      })

      onSuccess()
      onOpenChange(false)

      // Reset form
      setFormData({
        transaction_id: '',
        topics: [],
        timeout_ms: 60000
      })
    } catch (err: any) {
      console.error('Failed to begin transaction:', err)
      setError(err.response?.data?.error || err.message || 'Failed to begin transaction')
    } finally {
      setLoading(false)
    }
  }

  const isValid = formData.transaction_id && formData.topics.length > 0

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-xl">
        <DialogHeader>
          <DialogTitle>Begin Transaction</DialogTitle>
          <DialogDescription>
            Start a new transaction for atomic message operations
          </DialogDescription>
        </DialogHeader>

        {error && (
          <div className="bg-red-50 border border-red-200 rounded-lg p-3 text-sm text-red-800">
            <strong>Error:</strong> {error}
          </div>
        )}

        <div className="space-y-4">
          <div>
            <Label htmlFor="transaction_id">Transaction ID *</Label>
            <Input
              id="transaction_id"
              placeholder="tx-order-123"
              value={formData.transaction_id}
              onChange={(e) => setFormData({ ...formData, transaction_id: e.target.value })}
              disabled={loading}
            />
          </div>

          <div>
            <Label htmlFor="topics">Affected Topics *</Label>
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

          <div>
            <Label htmlFor="timeout">Timeout (milliseconds)</Label>
            <Input
              id="timeout"
              type="number"
              min="1000"
              step="1000"
              value={formData.timeout_ms}
              onChange={(e) => setFormData({ ...formData, timeout_ms: parseInt(e.target.value) })}
              disabled={loading}
            />
            <p className="text-xs text-muted-foreground mt-1">
              Transaction will automatically abort after this timeout
            </p>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={loading}>
            Cancel
          </Button>
          <Button onClick={handleSubmit} disabled={loading || !isValid}>
            {loading ? 'Starting...' : 'Begin Transaction'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

