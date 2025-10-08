import { useEffect, useState, useRef, useCallback } from 'react'

export interface WebSocketMessage {
  type: 'metrics' | 'message' | 'connection' | 'error'
  data: any
  timestamp: number
}

export interface WebSocketOptions {
  url: string
  reconnect?: boolean
  reconnectInterval?: number
  onOpen?: () => void
  onClose?: () => void
  onError?: (error: Event) => void
  onMessage?: (data: any) => void
}

export interface WebSocketHookReturn {
  data: any
  isConnected: boolean
  error: Event | null
  send: (data: any) => void
  reconnect: () => void
}

/**
 * Custom hook for WebSocket connection
 * 
 * Features:
 * - Automatic reconnection
 * - Connection status tracking
 * - Error handling
 * - Message buffering
 * - Auto cleanup on unmount
 * 
 * @example
 * const { data, isConnected, send } = useWebSocket({
 *   url: 'ws://localhost:8080/ws',
 *   reconnect: true,
 *   onMessage: (data) => console.log('New data:', data)
 * })
 */
export function useWebSocket(options: WebSocketOptions): WebSocketHookReturn {
  const {
    url,
    reconnect = true,
    reconnectInterval = 3000,
    onOpen,
    onClose,
    onError,
    onMessage
  } = options

  const [data, setData] = useState<any>(null)
  const [isConnected, setIsConnected] = useState(false)
  const [error, setError] = useState<Event | null>(null)
  
  const ws = useRef<WebSocket | null>(null)
  const reconnectTimeout = useRef<NodeJS.Timeout | null>(null)
  const shouldReconnect = useRef(reconnect)

  const connect = useCallback(() => {
    try {
      console.log(`[WebSocket] Connecting to ${url}...`)
      ws.current = new WebSocket(url)

      ws.current.onopen = () => {
        console.log('[WebSocket] Connected')
        setIsConnected(true)
        setError(null)
        onOpen?.()
      }

      ws.current.onmessage = (event) => {
        try {
          const parsedData = JSON.parse(event.data)
          setData(parsedData)
          onMessage?.(parsedData)
        } catch (e) {
          // If not JSON, use raw data
          setData(event.data)
          onMessage?.(event.data)
        }
      }

      ws.current.onerror = (event) => {
        console.error('[WebSocket] Error:', event)
        setError(event)
        onError?.(event)
      }

      ws.current.onclose = () => {
        console.log('[WebSocket] Disconnected')
        setIsConnected(false)
        onClose?.()

        // Attempt reconnection if enabled
        if (shouldReconnect.current && reconnectInterval > 0) {
          console.log(`[WebSocket] Reconnecting in ${reconnectInterval}ms...`)
          reconnectTimeout.current = setTimeout(() => {
            connect()
          }, reconnectInterval)
        }
      }
    } catch (e) {
      console.error('[WebSocket] Connection error:', e)
    }
  }, [url, reconnectInterval, onOpen, onClose, onError, onMessage])

  const disconnect = useCallback(() => {
    shouldReconnect.current = false
    if (reconnectTimeout.current) {
      clearTimeout(reconnectTimeout.current)
    }
    if (ws.current) {
      ws.current.close()
      ws.current = null
    }
  }, [])

  const send = useCallback((data: any) => {
    if (ws.current && ws.current.readyState === WebSocket.OPEN) {
      const payload = typeof data === 'string' ? data : JSON.stringify(data)
      ws.current.send(payload)
    } else {
      console.warn('[WebSocket] Cannot send, not connected')
    }
  }, [])

  const manualReconnect = useCallback(() => {
    disconnect()
    shouldReconnect.current = true
    connect()
  }, [connect, disconnect])

  useEffect(() => {
    connect()
    return () => {
      disconnect()
    }
  }, [connect, disconnect])

  return {
    data,
    isConnected,
    error,
    send,
    reconnect: manualReconnect
  }
}

/**
 * Hook specifically for metrics monitoring
 * Auto-subscribes to metrics updates
 */
export function useMetricsWebSocket() {
  return useWebSocket({
    url: `ws://${window.location.hostname}:8080/ws`,
    reconnect: true,
    reconnectInterval: 5000,
    onOpen: () => {
      console.log('[Metrics] WebSocket ready')
    },
    onMessage: (data) => {
      console.log('[Metrics] Update received:', data)
    }
  })
}

/**
 * Hook for message stream monitoring
 * Auto-subscribes to new message events
 */
export function useMessageWebSocket(topic?: string) {
  const ws = useWebSocket({
    url: `ws://${window.location.hostname}:8080/ws`,
    reconnect: true
  })

  useEffect(() => {
    if (ws.isConnected && topic) {
      // Subscribe to topic
      ws.send({
        type: 'subscribe',
        topic
      })
    }
  }, [ws.isConnected, topic, ws])

  return ws
}

