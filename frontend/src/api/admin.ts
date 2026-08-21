import { get, post, put, del } from './http'
import type { PageResult, PageQuery } from '../types'

export interface Member {
  id: number
  username: string
  email: string
  role: string
  status: string
  created_at: string
}

export interface InvitePayload {
  email: string
  role: string
}

export function listMembers(params: PageQuery): Promise<PageResult<Member>> {
  return get('/members', params)
}

export function inviteMember(data: InvitePayload): Promise<Member> {
  return post('/members', data)
}

export function setMemberRole(userId: number, role: string): Promise<null> {
  return put(`/members/${userId}/role`, { role })
}

export function setMemberStatus(userId: number, status: string): Promise<null> {
  return put(`/members/${userId}/status`, { status })
}

export function removeMember(userId: number): Promise<null> {
  return del(`/members/${userId}`)
}
