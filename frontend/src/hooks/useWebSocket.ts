import { useCallback, useEffect, useRef, useState } from 'react'

export type WsStatus = 'connecting' | 'connected' | 'disconnected' | 'error'

interface UseWebSocketOptions {
  /** WebSocket URL (ws:// or wss://) */
  url: string | null
  /** Binary type for the WebSocket (default: 'arraybuffer') */
  binaryType?: BinaryType
  /** Heartbeat interval in ms (default: 30000) */
  heartbeatInterval?: number
  /** Pong timeout in ms (default: 60000) */
  pongTimeout?: number
  /** Max reconnect delay in ms (default: 30000) */
  maxReconnectDelay?: number
  /** Called when a message is received */
  onMessage?: (data: string | ArrayBuffer) => void
  /** Called when connection opens */
  onOpen?: () => void
  /** Called when connection closes */
  onClose?: (event: CloseEvent) => void
  /** Called on error */
  onError?: (event: Event) => void
  /** Whether to auto-reconnect (default: true) */
  autoReconnect?: boolean
}

/**
 * WebSocket hook with exponential backoff reconnect and heartbeat.
 *
 * Usage:
 *   const { status, send, close } = useWebSocket({
 *     url: 'ws://host/ws/ssh',
 *     onMessage: (data) => { ... },
 *   })
 */
export function useWebSocket(options: UseWebSocketOptions) {
  const {
    url,
    binaryType = 'arraybuffer',
    heartbeatInterval = 30_000,
    pongTimeout = 60_000,
    maxReconnectDelay = 30_000,
    onMessage,
    onOpen,
    onClose,
    onError,
    autoReconnect = true,
  } = options

  const wsRef = useRef<WebSocket | null>(null)
  const [status, setStatus] = useState<WsStatus>('disconnected')
  const reconnectDelayRef = useRef(1000)
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  const heartbeatTimerRef = useRef<ReturnType<typeof setInterval> | undefined>(undefined)
  const pongTimerRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  const manualCloseRef = useRef(false)
  const urlRef = useRef(url)

  // Keep latest callbacks in refs to avoid reconnect on callback change
  const onMessageRef = useRef<typeof onMessage>(onMessage)
  const onOpenRef = useRef<typeof onOpen>(onOpen)
  const onCloseRef = useRef<typeof onClose>(onClose)
  const onErrorRef = useRef<typeof onError>(onError)
  onMessageRef.current = onMessage
  onOpenRef.current = onOpen
  onCloseRef.current = onClose
  onErrorRef.current = onError
  onMessageRef.current = onMessage
  onOpenRef.current = onOpen
  onCloseRef.current = onClose
  onErrorRef.current = onError

  const clearTimers = useCallback(() => {
    clearTimeout(reconnectTimerRef.current)
    clearInterval(heartbeatTimerRef.current)
    clearTimeout(pongTimerRef.current)
  }, [])

  const startHeartbeat = useCallback(() => {
    clearTimers()
    heartbeatTimerRef.current = setInterval(() => {
      const ws = wsRef.current
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'ping' }))
        // Start pong timeout
        pongTimerRef.current = setTimeout(() => {
          // No pong received — force reconnect
          ws.close()
        }, pongTimeout)
      }
    }, heartbeatInterval)
  }, [heartbeatInterval, pongTimeout, clearTimers])

  const connect = useCallback(() => {
    const currentUrl = urlRef.current
    if (!currentUrl) return

    manualCloseRef.current = false
    clearTimers()

    try {
      setStatus('connecting')
      const ws = new WebSocket(currentUrl)
      ws.binaryType = binaryType
      wsRef.current = ws

      ws.onopen = () => {
        setStatus('connected')
        reconnectDelayRef.current = 1000 // reset backoff
        startHeartbeat()
        onOpenRef.current?.()
      }

      ws.onmessage = (e) => {
        // Any message counts as a pong for heartbeat purposes
        clearTimeout(pongTimerRef.current)

        if (typeof e.data === 'string') {
          // Check for pong response
          try {
            const parsed = JSON.parse(e.data)
            if (parsed.type === 'pong') return
          } catch { /* not JSON, pass through */ }
          onMessageRef.current?.(e.data)
        } else if (e.data instanceof ArrayBuffer) {
          onMessageRef.current?.(e.data)
        }
      }

      ws.onerror = (e) => {
        setStatus('error')
        onErrorRef.current?.(e)
      }

      ws.onclose = (e) => {
        setStatus('disconnected')
        wsRef.current = null
        clearTimers()
        onCloseRef.current?.(e)

        // Reconnect with exponential backoff
        if (!manualCloseRef.current && autoReconnect && urlRef.current) {
          const delay = reconnectDelayRef.current
          reconnectTimerRef.current = setTimeout(() => {
            reconnectDelayRef.current = Math.min(delay * 2, maxReconnectDelay)
            connect()
          }, delay)
        }
      }
    } catch {
      setStatus('error')
    }
  }, [binaryType, autoReconnect, maxReconnectDelay, clearTimers, startHeartbeat])

  /** Send data through the WebSocket */
  const send = useCallback((data: string | ArrayBuffer) => {
    const ws = wsRef.current
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(data)
      return true
    }
    return false
  }, [])

  /** Close the WebSocket and prevent auto-reconnect */
  const close = useCallback(() => {
    manualCloseRef.current = true
    clearTimers()
    if (wsRef.current) {
      try { wsRef.current.close() } catch { /* already closed */ }
      wsRef.current = null
    }
    setStatus('disconnected')
  }, [clearTimers])

  // Connect when url changes; disconnect on unmount
  useEffect(() => {
    urlRef.current = url
    if (url) {
      connect()
    } else {
      close()
    }
    return () => {
      manualCloseRef.current = true
      clearTimers()
      if (wsRef.current) {
        try { wsRef.current.close() } catch { /* already closed */ }
        wsRef.current = null
      }
    }
  }, [url]) // eslint-disable-line react-hooks/exhaustive-deps

  return { status, send, close, reconnect: connect }
}
