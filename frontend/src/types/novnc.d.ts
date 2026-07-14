declare module '@novnc/novnc/core/rfb.js' {
  interface RFBOptions {
    credentials?: Record<string, string>
    shared?: boolean
    repeaterID?: string
    wsProtocols?: string[]
  }

  interface RFBDisconnectEvent extends Event {
    detail?: { clean?: boolean }
  }

  class RFB {
    constructor(target: HTMLElement, url: string, options?: RFBOptions)
    addEventListener(type: 'connect', handler: () => void): void
    addEventListener(type: 'disconnect', handler: (e: RFBDisconnectEvent) => void): void
    addEventListener(type: 'credentialsrequired', handler: () => void): void
    addEventListener(type: string, handler: EventListener): void
    removeEventListener(type: string, handler: EventListener): void
    disconnect(): void
    sendCredentials(credentials: Record<string, string>): void
    sendCtrlAltDel(): void
    focus(): void
    blur(): void
    machineShutdown(): void
    machineReboot(): void
    machineReset(): void
    scaleViewport: boolean
    resizeSession: boolean
    clipViewport: boolean
    dragViewport: boolean
    focusOnClick: boolean
    background: string
    qualityLevel: number
    compressionLevel: number
    readonly connected: boolean
  }

  export default RFB
}
