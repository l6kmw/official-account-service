import { adminConfig } from '../config'
import { APIError, type APIErrorCode, setAdminCSRFToken } from './client'

export type AdminSessionStatus = {
  authenticated: boolean
  auth_enabled: boolean
  login_enabled: boolean
  username?: string
  csrf_token?: string
}

export async function getAdminSession(): Promise<AdminSessionStatus> {
  const response = await fetch(toRequestURL('/api/v1/admin/session'), {
    method: 'GET',
    credentials: 'same-origin',
    headers: {
      Accept: 'application/json',
      ...adminAPIKeyHeader()
    }
  })
  const status = await readSessionResponse(response)
  setAdminCSRFToken(status.csrf_token ?? '')
  return status
}

export async function loginAdmin(username: string, password: string): Promise<AdminSessionStatus> {
  const response = await fetch(toRequestURL('/api/v1/admin/session'), {
    method: 'POST',
    credentials: 'same-origin',
    headers: {
      Accept: 'application/json',
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({ username, password })
  })
  const status = await readSessionResponse(response)
  setAdminCSRFToken(status.csrf_token ?? '')
  return status
}

export async function logoutAdmin(): Promise<void> {
  const csrfToken = await currentCSRFToken()
  await fetch(toRequestURL('/api/v1/admin/session'), {
    method: 'DELETE',
    credentials: 'same-origin',
    headers: {
      Accept: 'application/json',
      ...(csrfToken ? { 'X-CSRF-Token': csrfToken } : {})
    }
  })
  setAdminCSRFToken('')
}

async function currentCSRFToken() {
  const status = await getAdminSession()
  return status.csrf_token ?? ''
}

async function readSessionResponse(response: Response): Promise<AdminSessionStatus> {
  if (!response.ok) {
    const body = await readErrorBody(response)
    throw new APIError(response.status, body.error ?? 'internal_error')
  }
  return response.json() as Promise<AdminSessionStatus>
}

async function readErrorBody(response: Response): Promise<{ error?: APIErrorCode }> {
  try {
    return (await response.json()) as { error?: APIErrorCode }
  } catch {
    return {}
  }
}

function adminAPIKeyHeader(): Record<string, string> {
  if (!adminConfig.adminAPIKey) return {}
  return { 'X-Admin-API-Key': adminConfig.adminAPIKey }
}

function toRequestURL(path: string) {
  if (/^https?:\/\//i.test(path)) return path
  return new URL(path, `${window.location.protocol}//${window.location.host}`).toString()
}
