import ReconnectingWebSocket from 'reconnecting-websocket'
import { getToken } from './http'

export type RealtimeEvent =
  | { kind: 'finding.new'; payload: unknown }
  | { kind: 'event.new'; payload: unknown }
  | { kind: 'alert.new'; payload: unknown }

export class EventStream {
  private ws: ReconnectingWebSocket | null = null
  private handlers = new Set<(e: RealtimeEvent) => void>()
  private pingTimer = 0
  private closeTimer = 0
  private listeners: Array<{ el: HTMLElement; cb: (connected: boolean) => void }> = []

  connect(): void {
    const token = getToken()
    if (!token) return
    this.disconnect()
    const proto = window.location.protocol === 'https:' ? 'wss' : 'ws'
    const url = `${proto}://${window.location.host}/api/v1/ws/events?token=${encodeURIComponent(token)}`
    this.ws = new ReconnectingWebSocket(url, [], {
      connectionTimeout: 5000,
      maxRetries: Infinity,
      startClosed: true,
      maxReconnectionDelay: 10000,
      minReconnectionDelay: 1000,
      reconnectionDelayGrowFactor: 1.3,
    })
    this.ws.onopen = () => {
      this.startPing()
      this.emit()
    }
    this.ws.onclose = () => {
      this.stopPing()
      this.emit()
    }
    this.ws.onmessage = (ev) => {
      try {
        const data = JSON.parse(ev.data as string)
        if (data?.type === 'pong') return
        if (data && typeof data === 'object' && 'kind' in data) {
          const event = data as RealtimeEvent
          this.handlers.forEach((h) => h(event))
        }
      } catch {
        // 忽略非 JSON 消息
      }
    }
  }

  private startPing(): void {
    this.stopPing()
    this.pingTimer = window.setInterval(() => {
      if (this.ws?.readyState === WebSocket.OPEN) {
        this.ws.send(JSON.stringify({ type: 'ping' }))
      }
    }, 30000)
  }

  private stopPing(): void {
    if (this.pingTimer) {
      window.clearInterval(this.pingTimer)
      this.pingTimer = 0
    }
  }

  private emit(): void {
    const connected = this.isConnected()
    this.listeners.forEach(({ cb }) => cb(connected))
  }

  isConnected(): boolean {
    return this.ws?.readyState === WebSocket.OPEN
  }

  onStatusChange(el: HTMLElement, cb: (connected: boolean) => void): void {
    this.listeners.push({ el, cb })
    cb(this.isConnected())
  }

  subscribe(handler: (e: RealtimeEvent) => void): () => void {
    this.handlers.add(handler)
    return () => {
      this.handlers.delete(handler)
    }
  }

  disconnect(): void {
    this.stopPing()
    if (this.closeTimer) {
      window.clearTimeout(this.closeTimer)
      this.closeTimer = 0
    }
    if (this.ws) {
      this.ws.close()
      this.ws = null
    }
    this.listeners = []
  }
}

export const eventStream = new EventStream()
