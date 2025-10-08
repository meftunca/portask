import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { apiBase } from '@/lib/api'
import { Activity, Blocks, CheckCircle, Plus, RefreshCw, XCircle } from 'lucide-react'
import { useEffect, useState } from 'react'

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

export default function TransactionsNative() {
  const [transactions, setTransactions] = useState<Transaction[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const fetchTransactions = async () => {
    setLoading(true)
    setError(null)
    try {
      const response = await apiBase.get('/api/v1/transactions')
      
      if (response.data?.success) {
        setTransactions(response.data.transactions || [])
      } else {
        setError(response.data?.error || 'Failed to fetch transactions')
        setTransactions([])
      }
    } catch (err: any) {
      console.error('[TransactionsNative] Error:', err)
      setError(err.message || 'Failed to connect to API')
      setTransactions([])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchTransactions()
    const interval = setInterval(fetchTransactions, 5000)
    return () => clearInterval(interval)
  }, [])

  const getStateBadgeVariant = (state: string) => {
    switch (state.toUpperCase()) {
      case 'ACTIVE':
        return 'default'
      case 'COMMITTED':
        return 'default'
      case 'ABORTED':
        return 'destructive'
      case 'EXPIRED':
        return 'secondary'
      default:
        return 'outline'
    }
  }

  const getStateColor = (state: string) => {
    switch (state.toUpperCase()) {
      case 'ACTIVE':
        return 'text-blue-600'
      case 'COMMITTED':
        return 'text-green-600'
      case 'ABORTED':
        return 'text-red-600'
      case 'EXPIRED':
        return 'text-orange-600'
      default:
        return 'text-gray-600'
    }
  }

  const activeCount = transactions.filter(t => t.state === 'ACTIVE').length
  const committedCount = transactions.filter(t => t.state === 'COMMITTED').length
  const abortedCount = transactions.filter(t => t.state === 'ABORTED').length

  return (
    <div className="flex-1 space-y-4 p-4 md:p-8 pt-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-3xl font-bold tracking-tight">Transactions</h2>
          <p className="text-muted-foreground">
            Distributed transaction monitoring
          </p>
        </div>
        <div className="flex items-center space-x-2">
          <Button variant="outline" size="sm" onClick={fetchTransactions} disabled={loading}>
            <RefreshCw className={loading ? "mr-2 h-4 w-4 animate-spin" : "mr-2 h-4 w-4"} />
            Refresh
          </Button>
          <Button size="sm">
            <Plus className="mr-2 h-4 w-4" />
            Begin Transaction
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
      <div className="grid gap-4 md:grid-cols-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Active</CardTitle>
            <Activity className="h-4 w-4 text-blue-600" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-blue-600">{activeCount}</div>
            <p className="text-xs text-muted-foreground mt-1">In progress</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Committed</CardTitle>
            <CheckCircle className="h-4 w-4 text-green-600" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-green-600">{committedCount}</div>
            <p className="text-xs text-muted-foreground mt-1">Successfully completed</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Aborted</CardTitle>
            <XCircle className="h-4 w-4 text-red-600" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-red-600">{abortedCount}</div>
            <p className="text-xs text-muted-foreground mt-1">Rolled back</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total</CardTitle>
            <Blocks className="h-4 w-4 text-purple-600" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-purple-600">{transactions.length}</div>
            <p className="text-xs text-muted-foreground mt-1">All transactions</p>
          </CardContent>
        </Card>
      </div>

      {/* Transactions Table */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center">
            <Blocks className="mr-2 h-5 w-5" /> Transactions List
          </CardTitle>
          <CardDescription>
            All transactions managed by Portask Native API
          </CardDescription>
        </CardHeader>
        <CardContent>
          {loading ? (
            <div className="flex items-center justify-center py-8">
              <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
            </div>
          ) : transactions.length === 0 ? (
            <div className="text-center py-8">
              <Blocks className="mx-auto h-12 w-12 text-muted-foreground opacity-50" />
              <p className="mt-4 text-muted-foreground">No transactions found. Begin a transaction to get started.</p>
              <Button className="mt-4">
                <Plus className="mr-2 h-4 w-4" />
                Begin Transaction
              </Button>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Transaction ID</TableHead>
                  <TableHead>State</TableHead>
                  <TableHead>Topics</TableHead>
                  <TableHead>Messages</TableHead>
                  <TableHead>Timeout</TableHead>
                  <TableHead>Created</TableHead>
                  <TableHead>Expires</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {transactions.map((txn) => (
                  <TableRow key={txn.id}>
                    <TableCell className="font-mono text-xs">{txn.id.substring(0, 12)}...</TableCell>
                    <TableCell>
                      <Badge variant={getStateBadgeVariant(txn.state)} className={getStateColor(txn.state)}>
                        {txn.state}
                      </Badge>
                    </TableCell>
                    <TableCell>{txn.topics?.join(', ') || '-'}</TableCell>
                    <TableCell>{txn.messages_count}</TableCell>
                    <TableCell>{txn.timeout_ms}ms</TableCell>
                    <TableCell>{new Date(txn.created_at).toLocaleTimeString()}</TableCell>
                    <TableCell>{new Date(txn.expires_at).toLocaleTimeString()}</TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end space-x-2">
                        {txn.state === 'ACTIVE' && (
                          <>
                            <Button variant="outline" size="sm" className="text-green-600">
                              Commit
                            </Button>
                            <Button variant="outline" size="sm" className="text-red-600">
                              Abort
                            </Button>
                          </>
                        )}
                        {txn.state !== 'ACTIVE' && (
                          <Button variant="outline" size="sm">
                            View
                          </Button>
                        )}
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

