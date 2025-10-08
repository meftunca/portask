import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { apiBase } from '@/lib/api'
import { useState } from 'react'

interface Transaction {
  id: string
  state: string
  topics: string[]
  messages_count: number
  created_at: string
  updated_at: string
  expires_at: string
  timeout_ms: number
}

interface TransactionModalProps {
  transaction: Transaction | null
  open: boolean
  onOpenChange: (open: boolean) => void
  onUpdate: () => void
}

export function TransactionModal({ transaction, open, onOpenChange, onUpdate }: TransactionModalProps) {
  const [loading, setLoading] = useState(false)

  if (!transaction) return null

  const handleCommit = async () => {
    setLoading(true)
    try {
      await apiBase.post('/api/v1/transactions/commit', { transaction_id: transaction.id })
      onUpdate()
      onOpenChange(false)
    } catch (err) {
      console.error('Failed to commit transaction:', err)
    } finally {
      setLoading(false)
    }
  }

  const handleAbort = async () => {
    setLoading(true)
    try {
      await apiBase.post('/api/v1/transactions/abort', { transaction_id: transaction.id })
      onUpdate()
      onOpenChange(false)
    } catch (err) {
      console.error('Failed to abort transaction:', err)
    } finally {
      setLoading(false)
    }
  }

  const getStateBadge = (state: string) => {
    switch (state.toUpperCase()) {
      case 'ACTIVE':
        return <Badge variant="default">Active</Badge>
      case 'COMMITTED':
        return <Badge variant="default" className="bg-green-500">Committed</Badge>
      case 'ABORTED':
        return <Badge variant="destructive">Aborted</Badge>
      default:
        return <Badge variant="secondary">{state}</Badge>
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>Transaction Details</DialogTitle>
          <DialogDescription>
            View and manage transaction state
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <div>
              <Label>Transaction ID</Label>
              <p className="font-mono text-sm break-all">{transaction.id}</p>
            </div>
            <div>
              <Label>State</Label>
              <div className="mt-1">{getStateBadge(transaction.state)}</div>
            </div>
            <div>
              <Label>Messages Count</Label>
              <p className="font-semibold">{transaction.messages_count}</p>
            </div>
            <div>
              <Label>Timeout</Label>
              <p className="font-semibold">{transaction.timeout_ms}ms</p>
            </div>
            <div>
              <Label>Created At</Label>
              <p className="text-sm">{new Date(transaction.created_at).toLocaleString()}</p>
            </div>
            <div>
              <Label>Expires At</Label>
              <p className="text-sm">{new Date(transaction.expires_at).toLocaleString()}</p>
            </div>
          </div>

          <div>
            <Label>Affected Topics</Label>
            <div className="flex flex-wrap gap-2 mt-2">
              {transaction.topics.map((topic) => (
                <Badge key={topic} variant="outline">{topic}</Badge>
              ))}
            </div>
          </div>

          {transaction.state === 'ACTIVE' && (
            <div className="bg-yellow-50 border border-yellow-200 rounded-lg p-4">
              <p className="text-sm text-yellow-800">
                <strong>Note:</strong> This transaction is active. You can commit or abort it.
              </p>
            </div>
          )}
        </div>

        <DialogFooter>
          {transaction.state === 'ACTIVE' ? (
            <>
              <Button
                variant="outline"
                onClick={handleAbort}
                disabled={loading}
              >
                Abort Transaction
              </Button>
              <Button
                onClick={handleCommit}
                disabled={loading}
              >
                Commit Transaction
              </Button>
            </>
          ) : (
            <Button variant="outline" onClick={() => onOpenChange(false)}>
              Close
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

