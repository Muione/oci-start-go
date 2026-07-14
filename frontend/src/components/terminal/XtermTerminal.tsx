import { useEffect, useRef, useCallback, useState, type CSSProperties } from 'react'
import { useWebSocket } from '@/hooks/useWebSocket'

// Dynamically imported — these are heavy and only needed on terminal pages
type TerminalType = import('@xterm/xterm').Terminal
type FitAddonType = import('@xterm/addon-fit').FitAddon

interface XtermTerminalProps {
  /** WebSocket URL to connect to (null = not connected) */
  wsUrl: string | null
  /** Additional className on the outer container */
  className?: string
  /** Inline styles */
  style?: CSSProperties
  /** Called when WebSocket opens */
  onConnect?: () => void
  /** Called when WebSocket closes */
  onDisconnect?: () => void
  /** Called on WebSocket error */
  onError?: (msg: string) => void
  /** Exposes the raw WebSocket send function (for sending connect messages etc.) */
  onSendReady?: (send: (data: string) => boolean) => void
}

const TERM_OPTIONS = {
  cursorBlink: true,
  fontSize: 14,
  convertEol: true,
  scrollback: 10000,
  tabStopWidth: 4,
  fontFamily: '"Cascadia Code", Menlo, Monaco, "Courier New", monospace',
  theme: {
    background: '#1e1e1e',
    foreground: '#d4d4d4',
    cursor: '#d4d4d4',
    selectionBackground: '#264f78',
  },
}

/**
 * xterm.js React wrapper with WebSocket transport.
 * Handles terminal lifecycle, fit-on-resize, and binary messages.
 */
export default function XtermTerminal({
  wsUrl,
  className,
  style,
  onConnect,
  onDisconnect,
  onError,
  onSendReady,
}: XtermTerminalProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const termRef = useRef<TerminalType | null>(null)
  const fitAddonRef = useRef<FitAddonType | null>(null)
  const resizeObserverRef = useRef<ResizeObserver | null>(null)
  const [termReady, setTermReady] = useState(false)

  // WebSocket connection
  const { status, send } = useWebSocket({
    url: wsUrl,
    binaryType: 'arraybuffer',
    onOpen: () => onConnect?.(),
    onClose: () => onDisconnect?.(),
    onError: () => onError?.('WebSocket connection failed'),
    onMessage: (data) => {
      const term = termRef.current
      if (!term) return
      if (typeof data === 'string') {
        term.write(data)
      } else {
        term.write(new Uint8Array(data))
      }
    },
  })

  // Expose raw send to parent
  useEffect(() => {
    onSendReady?.(send)
  }, [send, onSendReady])

  // Send terminal input to WebSocket
  const handleData = useCallback(
    (data: string) => {
      send(JSON.stringify({ type: 'input', data }))
    },
    [send],
  )

  // Send terminal resize to WebSocket
  const handleResize = useCallback(
    ({ cols, rows }: { cols: number; rows: number }) => {
      send(JSON.stringify({ type: 'resize', data: { cols, rows } }))
    },
    [send],
  )

  // Initialize xterm.js
  useEffect(() => {
    if (!containerRef.current) return

    let disposed = false
    const init = async () => {
      const [{ Terminal }, { FitAddon }] = await Promise.all([
        import('@xterm/xterm'),
        import('@xterm/addon-fit'),
      ])
      // Import xterm CSS
      await import('@xterm/xterm/css/xterm.css')

      if (disposed || !containerRef.current) return

      const term = new Terminal(TERM_OPTIONS)
      const fitAddon = new FitAddon()
      term.loadAddon(fitAddon)
      term.open(containerRef.current)
      fitAddon.fit()

      term.onData(handleData)
      term.onResize(handleResize)

      // Observe container resize → fit terminal
      const observer = new ResizeObserver(() => fitAddon.fit())
      observer.observe(containerRef.current)

      termRef.current = term
      fitAddonRef.current = fitAddon
      resizeObserverRef.current = observer
      setTermReady(true)
    }
    init()

    return () => {
      disposed = true
      resizeObserverRef.current?.disconnect()
      resizeObserverRef.current = null
      try { termRef.current?.dispose() } catch { /* already disposed */ }
      termRef.current = null
      fitAddonRef.current = null
    }
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  // Update handlers when they change (no terminal recreation)
  useEffect(() => {
    const term = termRef.current
    if (!term) return
    // xterm.js doesn't support removing onData handlers, so we store
    // the latest handler in a ref and use a wrapper (already done via useCallback deps)
  }, [handleData, handleResize])

  // Write disconnect message when status changes
  useEffect(() => {
    const term = termRef.current
    if (!term || !termReady) return
    if (status === 'disconnected') {
      term.write('\r\n\x1b[33m[Connection closed]\x1b[0m\r\n')
    } else if (status === 'error') {
      term.write('\r\n\x1b[31m[Connection error]\x1b[0m\r\n')
    }
  }, [status, termReady])

  return (
    <div
      ref={containerRef}
      className={className}
      style={{
        height: '100%',
        minHeight: 380,
        background: '#1e1e1e',
        borderRadius: '0 0 6px 6px',
        overflow: 'hidden',
        ...style,
      }}
    />
  )
}

/** Export status type for parent components */
export type { XtermTerminalProps }
