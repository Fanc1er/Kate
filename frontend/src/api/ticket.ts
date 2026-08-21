import { get, post, put, del } from './http'
import type { PageResult, PageQuery } from '../types'
import type { EventItem } from './event'
import type { Vulnerability } from './event'

export interface Ticket {
  id: number
  org_id: number
  event_id: number
  vuln_id: number
  assignee: string
  status: string
  due_at?: string
  notes: string
  version: number
  created_at: string
  updated_at: string
}

export interface TicketDetail {
  ticket: Ticket
  event?: EventItem
  vulnerability?: Vulnerability
}

export function listTickets(params: PageQuery & { status?: string; source?: string }): Promise<PageResult<Ticket>> {
  return get('/tickets', params)
}

export function getTicket(id: number): Promise<TicketDetail> {
  return get(`/tickets/${id}`)
}

export function createTicket(data: { event_id?: number; vuln_id?: number; assignee?: string; notes?: string; due_at?: string }): Promise<Ticket> {
  return post('/tickets', data)
}

export function updateTicketStatus(id: number, status: string, version?: number): Promise<Ticket> {
  return put(`/tickets/${id}/status`, { status, version })
}

export function assignTicket(id: number, assignee: string, version?: number): Promise<Ticket> {
  return put(`/tickets/${id}/assign`, { assignee, version })
}

export function batchUpdateTicketStatus(ids: number[], status: string): Promise<{ success: number[]; failed: number[] }> {
  return post('/tickets/batch/status', { ids, status })
}

export function deleteTicket(id: number): Promise<null> {
  return del(`/tickets/${id}`)
}

export function listTicketSources(): Promise<{ event_tickets: number; vuln_tickets: number }> {
  return get('/tickets/sources')
}

export function batchUpdateEventStatus(ids: number[], status: string): Promise<{ success: number[]; failed: number[] }> {
  return post('/events/batch/status', { ids, status })
}

export function getEventDetail(id: number): Promise<{ event: EventItem; findings?: unknown[]; tickets?: Ticket[] }> {
  return get(`/events/${id}`)
}

export function batchResolveAlert(ids: number[]): Promise<{ success: number[]; failed: number[] }> {
  return post('/alerts/batch/resolve', { ids })
}
