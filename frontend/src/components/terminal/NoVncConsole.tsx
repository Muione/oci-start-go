import { useEffect, useRef, useCallback, type CSSProperties } from 'react'

interface NoVncConsoleProps {
  /** WebSocket URL for the VNC connection (null = not connected) */
  wsUrl: string | null
  /** Additional className */
  className?: string
  /** Inline styles */
  style?: CSSProperties
  /** Called when connected */
  onConnect?: () => void
  /** Called when disconnected */
  onDisconnect?: () => void
  /** Called on error */
  onError?: (msg: string) => void
  /** Connection status change */
  onStatusChange?: (status: string) => void
}

/**
 * noVNC React wrapper.
 * Uses a raw WebSocket (via useWebSocket) and renders VNC canvas
 * into a container div using the noVNC RFB class.
 */
export default function NoVncConsole({
  wsUrl,
  className,
  style,
  onConnect,
  onDisconnect,
  onError,
  onStatusChange,
}: NoVncConsoleProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const rfbRef = useRef<any>(null)

  // Track connection status for parent
  const statusRef = useRef('disconnected')

  const updateStatus = useCallback(
    (s: string) => {
      statusRef.current = s
      onStatusChange?.(s)
    },
    [onStatusChange],
  )

  // Initialize noVNC RFB
  useEffect(() => {
    if (!containerRef.current || !wsUrl) return

    let disposed = false
    const init = async () => {
      try {
        // @novnc/novnc exports RFB as the default export from core/rfb.js
        const RFBModule = await import('@novnc/novnc/core/rfb.js') as any
        const RFB = RFBModule.default ?? RFBModule
        if (disposed || !containerRef.current) return

        // Clean up previous instance
        if (rfbRef.current) {
          try { rfbRef.current.disconnect() } catch { /* ignore */ }
          rfbRef.current = null
        }

        const rfb = new RFB(containerRef.current, wsUrl, {
          // noVNC will create its own WebSocket connection
        })

        rfb.addEventListener('connect', () => {
          updateStatus('connected')
          onConnect?.()
        })

        rfb.addEventListener('disconnect', (e: any) => {
          updateStatus('disconnected')
          onDisconnect?.()
          // If disconnected with error detail
          if (e?.detail?.clean === false) {
            onError?.('VNC connection lost')
          }
        })

        rfb.addEventListener('credentialsrequired', () => {
          updateStatus('credentials-required')
          onStatusChange?.('credentials-required')
        })

        rfb.scaleViewport = true
        rfb.resizeSession = true
        rfbRef.current = rfb
        updateStatus('connecting')
      } catch (err) {
        onError?.(`Failed to load noVNC: ${(err as Error).message}`)
      }
    }

    init()

    return () => {
      disposed = true
      if (rfbRef.current) {
        try { rfbRef.current.disconnect() } catch { /* ignore */ }
        rfbRef.current = null
      }
    }
  }, [wsUrl]) // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <div
      ref={containerRef}
      className={className}
      style={{
        width: '100%',
        height: 560,
        background: '#1e1e1e',
        borderRadius: 4,
        overflow: 'hidden',
        ...style,
      }}
    />
  )
}

export type { NoVncConsoleProps }
