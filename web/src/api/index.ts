import { request } from './client'
import type {
  FamilyView,
  Me,
  Profile,
  ProfileInput,
  Reading,
  ReadingInput,
  Trend,
  TrendMode,
} from './types'

export const api = {
  register: (username: string, password: string, displayName: string) =>
    request<Me>('POST', '/auth/register', { username, password, displayName }),
  login: (username: string, password: string) => request<Me>('POST', '/auth/login', { username, password }),
  logout: () => request<void>('POST', '/auth/logout'),
  me: () => request<Me>('GET', '/me'),
  changePassword: (oldPassword: string, newPassword: string) =>
    request<void>('PUT', '/me/password', { oldPassword, newPassword }),

  family: () => request<FamilyView>('GET', '/family'),
  createFamily: (name: string) => request<FamilyView>('POST', '/family', { name }),
  joinFamily: (inviteCode: string) => request<FamilyView>('POST', '/family/join', { inviteCode }),
  leaveFamily: () => request<void>('POST', '/family/leave'),
  resetInviteCode: () => request<FamilyView>('POST', '/family/invite-code/reset'),

  profiles: () => request<Profile[]>('GET', '/profiles'),
  createProfile: (input: ProfileInput) => request<Profile>('POST', '/profiles', input),
  updateProfile: (id: number, input: ProfileInput) => request<Profile>('PUT', `/profiles/${id}`, input),
  deleteProfile: (id: number) => request<void>('DELETE', `/profiles/${id}`),

  readings: (profileId: number, params: { from?: string; to?: string; limit?: number; offset?: number } = {}) =>
    request<Reading[]>('GET', `/profiles/${profileId}/readings${query(params)}`),
  reading: (id: number) => request<Reading>('GET', `/readings/${id}`),
  createReading: (profileId: number, input: ReadingInput) =>
    request<Reading>('POST', `/profiles/${profileId}/readings`, input),
  updateReading: (id: number, input: ReadingInput) => request<Reading>('PUT', `/readings/${id}`, input),
  deleteReading: (id: number) => request<void>('DELETE', `/readings/${id}`),

  trend: (profileId: number, mode: TrendMode, from: string, to: string) =>
    request<Trend>('GET', `/profiles/${profileId}/trend${query({ mode, from, to })}`),
}

function query(params: Record<string, string | number | undefined>): string {
  const q = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== '') q.set(k, String(v))
  }
  const s = q.toString()
  return s ? `?${s}` : ''
}
