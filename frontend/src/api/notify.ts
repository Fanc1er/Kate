import { get, post, put, del } from './http'

export interface NotifyChannel {
  id: number
  type: string
  config: string
  enabled: string
}

export interface WebhookConfig {
  url: string
  secret?: string
}

export function listChannels(): Promise<NotifyChannel[]> {
  return get('/notify/channels')
}

export function createChannel(type: string, config: WebhookConfig): Promise<NotifyChannel> {
  return post('/notify/channels', { type, config: JSON.stringify(config) })
}

export function updateChannel(id: number, config: WebhookConfig | null, enabled: string | null): Promise<NotifyChannel> {
  const body: Record<string, unknown> = {}
  if (config) body.config = JSON.stringify(config)
  if (enabled) body.enabled = enabled
  return put(`/notify/channels/${id}`, body)
}

export function deleteChannel(id: number): Promise<void> {
  return del(`/notify/channels/${id}`)
}

export function testChannel(id: number): Promise<void> {
  return post(`/notify/channels/${id}/test`)
}
