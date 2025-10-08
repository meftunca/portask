import { useEffect, useState, useRef } from 'react';

interface SSEOptions {
  url: string;
  enabled?: boolean;
}

export const useSSE = <T = any>({ url, enabled = true }: SSEOptions) => {
  const [data, setData] = useState<T | null>(null);
  const [isConnected, setIsConnected] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const eventSourceRef = useRef<EventSource | null>(null);
  const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null);

  const connect = () => {
    if (!enabled) return;
    
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
    }

    console.log(`[SSE] Connecting to ${url}`);
    const eventSource = new EventSource(url);

    eventSource.onopen = () => {
      console.log('[SSE] Connected');
      setIsConnected(true);
      setError(null);
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
        reconnectTimeoutRef.current = null;
      }
    };

    eventSource.addEventListener('connected', (event) => {
      console.log('[SSE] Connection confirmed:', event.data);
    });

    eventSource.addEventListener('metrics', (event) => {
      try {
        const parsed = JSON.parse(event.data);
        setData(parsed as T);
      } catch (e) {
        console.error('[SSE] Failed to parse metrics:', e);
      }
    });

    eventSource.addEventListener('health', (event) => {
      try {
        const parsed = JSON.parse(event.data);
        setData(parsed as T);
      } catch (e) {
        console.error('[SSE] Failed to parse health:', e);
      }
    });

    eventSource.onerror = (event) => {
      console.error('[SSE] Error:', event);
      setIsConnected(false);
      setError('Connection error');
      eventSource.close();

      // Attempt to reconnect after 5 seconds
      if (!reconnectTimeoutRef.current) {
        reconnectTimeoutRef.current = setTimeout(() => {
          console.log('[SSE] Reconnecting...');
          connect();
        }, 5000);
      }
    };

    eventSourceRef.current = eventSource;
  };

  useEffect(() => {
    connect();

    return () => {
      if (eventSourceRef.current) {
        console.log('[SSE] Cleaning up...');
        eventSourceRef.current.close();
      }
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
      }
    };
  }, [url, enabled]);

  return { data, isConnected, error };
};

// Specific hook for metrics
export const useMetricsSSE = () => {
  return useSSE({
    url: 'http://localhost:8080/api/v1/sse/metrics',
    enabled: true,
  });
};

// Specific hook for health
export const useHealthSSE = () => {
  return useSSE({
    url: 'http://localhost:8080/api/v1/sse/health',
    enabled: true,
  });
};

