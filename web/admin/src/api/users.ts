import { deleteJSONWithResponse, getJSON, postJSON } from './client'

export type ManagedUser = {
  id: string
  username: string
  role: 'admin' | 'user'
  status: 'active' | 'disabled'
  api_token_configured: boolean
  api_token_hint?: string
  api_token_created_at?: string
  created_at: string
  updated_at: string
}

type UserListResponse = { items: ManagedUser[] }
type GeneratedTokenResponse = { token: string; user: ManagedUser }

export async function listUsers(): Promise<ManagedUser[]> {
  const response = await getJSON<UserListResponse>('/api/v1/admin/users')
  return response.items
}

export function createUser(username: string, password: string): Promise<ManagedUser> {
  return postJSON<ManagedUser>('/api/v1/admin/users', { username, password })
}

export function generateUserToken(userID: string): Promise<GeneratedTokenResponse> {
  return postJSON<GeneratedTokenResponse>(`/api/v1/admin/users/${encodeURIComponent(userID)}/api-token`, {})
}

export function revokeUserToken(userID: string): Promise<ManagedUser> {
  return deleteJSONWithResponse<ManagedUser>(`/api/v1/admin/users/${encodeURIComponent(userID)}/api-token`)
}
