import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

interface ConsumerGroup {
  id: string
  name: string
  state: string
  protocol: string
  members: any[]
  subscriptions: string[]
  created_at: string
  generation: number
  leader?: string
  protocol_type?: string
}

interface ConsumerGroupModalProps {
  group: ConsumerGroup | null
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function ConsumerGroupModal({ group, open, onOpenChange }: ConsumerGroupModalProps) {
  if (!group) return null

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-3xl max-h-[80vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Consumer Group Details</DialogTitle>
          <DialogDescription>
            View consumer group configuration and members
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-6">
          {/* Basic Info */}
          <div className="grid grid-cols-2 gap-4">
            <div>
              <Label>Group ID</Label>
              <p className="font-semibold">{group.id}</p>
            </div>
            <div>
              <Label>State</Label>
              <Badge variant={group.state === 'Stable' ? 'default' : 'secondary'}>
                {group.state}
              </Badge>
            </div>
            <div>
              <Label>Protocol</Label>
              <p className="font-semibold">{group.protocol}</p>
            </div>
            <div>
              <Label>Generation</Label>
              <p className="font-semibold">{group.generation}</p>
            </div>
            {group.leader && (
              <div>
                <Label>Leader</Label>
                <p className="font-semibold">{group.leader}</p>
              </div>
            )}
            <div>
              <Label>Created At</Label>
              <p className="font-semibold">{new Date(group.created_at).toLocaleString()}</p>
            </div>
          </div>

          {/* Subscriptions */}
          <div>
            <Label>Subscribed Topics</Label>
            <div className="flex flex-wrap gap-2 mt-2">
              {group.subscriptions.map((topic) => (
                <Badge key={topic} variant="outline">{topic}</Badge>
              ))}
            </div>
          </div>

          {/* Members */}
          <div>
            <Label>Members ({group.members?.length || 0})</Label>
            {group.members && group.members.length > 0 ? (
              <Table className="mt-2">
                <TableHeader>
                  <TableRow>
                    <TableHead>Member ID</TableHead>
                    <TableHead>Client ID</TableHead>
                    <TableHead>Host</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {group.members.map((member: any, idx: number) => (
                    <TableRow key={idx}>
                      <TableCell className="font-mono text-sm">
                        {member.member_id || member.id || 'N/A'}
                      </TableCell>
                      <TableCell>{member.client_id || 'N/A'}</TableCell>
                      <TableCell>{member.host || 'N/A'}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            ) : (
              <p className="text-sm text-muted-foreground mt-2">No active members</p>
            )}
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Close
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

