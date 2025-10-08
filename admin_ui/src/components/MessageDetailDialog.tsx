import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Badge } from "@/components/ui/badge"
import { 
  Copy, 
  Clock, 
  Hash, 
  MessageSquare, 
  Tag,
  CheckCircle
} from "lucide-react"
import { useState } from "react"
import { useToast } from "@/hooks/use-toast"

interface Message {
  id: string
  topic: string
  partition: number
  offset: number
  key?: string
  value: any
  headers?: Record<string, string>
  timestamp: string
  size: number
  ttl?: number
}

interface MessageDetailDialogProps {
  message: Message | null
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function MessageDetailDialog({ 
  message, 
  open, 
  onOpenChange 
}: MessageDetailDialogProps) {
  const { toast } = useToast()
  const [copiedField, setCopiedField] = useState<string | null>(null)

  if (!message) return null

  const copyToClipboard = (text: string, field: string) => {
    navigator.clipboard.writeText(text)
    setCopiedField(field)
    toast({
      title: "Copied!",
      description: `${field} copied to clipboard`,
    })
    setTimeout(() => setCopiedField(null), 2000)
  }

  const formatJSON = (data: any) => {
    try {
      return JSON.stringify(data, null, 2)
    } catch {
      return String(data)
    }
  }

  const formatTimestamp = (timestamp: string) => {
    const date = new Date(timestamp)
    return {
      date: date.toLocaleDateString(),
      time: date.toLocaleTimeString(),
      relative: getRelativeTime(date)
    }
  }

  const getRelativeTime = (date: Date) => {
    const now = new Date()
    const diff = now.getTime() - date.getTime()
    const seconds = Math.floor(diff / 1000)
    const minutes = Math.floor(seconds / 60)
    const hours = Math.floor(minutes / 60)
    const days = Math.floor(hours / 24)

    if (days > 0) return `${days}d ago`
    if (hours > 0) return `${hours}h ago`
    if (minutes > 0) return `${minutes}m ago`
    return `${seconds}s ago`
  }

  const formatted = formatTimestamp(message.timestamp)

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-4xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle className="flex items-center space-x-2">
            <MessageSquare className="h-5 w-5" />
            <span>Message Details</span>
          </DialogTitle>
          <DialogDescription>
            Complete information about this message
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-6">
          {/* Basic Info */}
          <div className="grid gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <label className="text-sm font-medium text-muted-foreground flex items-center">
                  <Hash className="h-4 w-4 mr-1" />
                  Message ID
                </label>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => copyToClipboard(message.id, 'Message ID')}
                  className="h-6 px-2"
                >
                  {copiedField === 'Message ID' ? (
                    <CheckCircle className="h-3 w-3 text-green-600" />
                  ) : (
                    <Copy className="h-3 w-3" />
                  )}
                </Button>
              </div>
              <p className="text-sm font-mono bg-muted p-2 rounded truncate">
                {message.id}
              </p>
            </div>

            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <label className="text-sm font-medium text-muted-foreground flex items-center">
                  <Tag className="h-4 w-4 mr-1" />
                  Topic
                </label>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => copyToClipboard(message.topic, 'Topic')}
                  className="h-6 px-2"
                >
                  {copiedField === 'Topic' ? (
                    <CheckCircle className="h-3 w-3 text-green-600" />
                  ) : (
                    <Copy className="h-3 w-3" />
                  )}
                </Button>
              </div>
              <Badge variant="outline" className="font-mono">
                {message.topic}
              </Badge>
            </div>

            <div className="space-y-2">
              <label className="text-sm font-medium text-muted-foreground">
                Partition
              </label>
              <p className="text-sm font-mono bg-muted p-2 rounded">
                {message.partition}
              </p>
            </div>

            <div className="space-y-2">
              <label className="text-sm font-medium text-muted-foreground">
                Offset
              </label>
              <p className="text-sm font-mono bg-muted p-2 rounded">
                {message.offset.toLocaleString()}
              </p>
            </div>

            {message.key && (
              <div className="space-y-2 md:col-span-2">
                <div className="flex items-center justify-between">
                  <label className="text-sm font-medium text-muted-foreground">
                    Key
                  </label>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => copyToClipboard(message.key!, 'Key')}
                    className="h-6 px-2"
                  >
                    {copiedField === 'Key' ? (
                      <CheckCircle className="h-3 w-3 text-green-600" />
                    ) : (
                      <Copy className="h-3 w-3" />
                    )}
                  </Button>
                </div>
                <p className="text-sm font-mono bg-muted p-2 rounded break-all">
                  {message.key}
                </p>
              </div>
            )}
          </div>

          {/* Timestamp */}
          <div className="space-y-2">
            <label className="text-sm font-medium text-muted-foreground flex items-center">
              <Clock className="h-4 w-4 mr-1" />
              Timestamp
            </label>
            <div className="grid gap-2 md:grid-cols-3">
              <div className="bg-muted p-2 rounded">
                <p className="text-xs text-muted-foreground">Date</p>
                <p className="text-sm font-medium">{formatted.date}</p>
              </div>
              <div className="bg-muted p-2 rounded">
                <p className="text-xs text-muted-foreground">Time</p>
                <p className="text-sm font-medium">{formatted.time}</p>
              </div>
              <div className="bg-muted p-2 rounded">
                <p className="text-xs text-muted-foreground">Relative</p>
                <p className="text-sm font-medium">{formatted.relative}</p>
              </div>
            </div>
          </div>

          {/* Metadata */}
          <div className="grid gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <label className="text-sm font-medium text-muted-foreground">
                Size
              </label>
              <p className="text-sm font-mono bg-muted p-2 rounded">
                {message.size.toLocaleString()} bytes
              </p>
            </div>

            {message.ttl && (
              <div className="space-y-2">
                <label className="text-sm font-medium text-muted-foreground">
                  TTL
                </label>
                <p className="text-sm font-mono bg-muted p-2 rounded">
                  {message.ttl}s
                </p>
              </div>
            )}
          </div>

          {/* Headers */}
          {message.headers && Object.keys(message.headers).length > 0 && (
            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <label className="text-sm font-medium text-muted-foreground">
                  Headers ({Object.keys(message.headers).length})
                </label>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => copyToClipboard(formatJSON(message.headers), 'Headers')}
                  className="h-6 px-2"
                >
                  {copiedField === 'Headers' ? (
                    <CheckCircle className="h-3 w-3 text-green-600" />
                  ) : (
                    <Copy className="h-3 w-3" />
                  )}
                </Button>
              </div>
              <div className="bg-muted p-3 rounded space-y-1 max-h-48 overflow-y-auto">
                {Object.entries(message.headers).map(([key, value]) => (
                  <div key={key} className="flex items-start space-x-2 text-sm">
                    <span className="font-mono text-blue-600 dark:text-blue-400">
                      {key}:
                    </span>
                    <span className="font-mono break-all">{value}</span>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Value/Payload */}
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <label className="text-sm font-medium text-muted-foreground">
                Value (Payload)
              </label>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => copyToClipboard(formatJSON(message.value), 'Value')}
                className="h-6 px-2"
              >
                {copiedField === 'Value' ? (
                  <CheckCircle className="h-3 w-3 text-green-600" />
                ) : (
                  <Copy className="h-3 w-3" />
                )}
              </Button>
            </div>
            <div className="bg-muted rounded overflow-hidden">
              <pre className="p-4 text-xs overflow-x-auto font-mono">
                {formatJSON(message.value)}
              </pre>
            </div>
          </div>
        </div>

        <div className="flex justify-end space-x-2 pt-4 border-t">
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Close
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}

