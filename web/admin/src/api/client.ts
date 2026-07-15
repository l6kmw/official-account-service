import { adminConfig } from '../config'

export type APIErrorCode = 'invalid_request' | 'request_too_large' | 'unauthorized' | 'forbidden' | 'conflict' | 'rate_limited' | 'not_found' | 'not_implemented' | 'internal_error'

type APIErrorBody = {
  error?: APIErrorCode
}

export class APIError extends Error {
  status: number
  code: APIErrorCode

  constructor(status: number, code: APIErrorCode) {
    super(toUserMessage(code))
    this.name = 'APIError'
    this.status = status
    this.code = code
  }
}

export async function getJSON<T>(path: string): Promise<T> {
  const response = await request(path, { method: 'GET' })
  return response.json() as Promise<T>
}

export async function postJSON<T>(path: string, body: unknown): Promise<T> {
  const response = await request(path, { method: 'POST', body })
  return response.json() as Promise<T>
}

export async function putJSON<T>(path: string, body: unknown): Promise<T> {
  const response = await request(path, { method: 'PUT', body })
  return response.json() as Promise<T>
}

export async function deleteJSON(path: string): Promise<void> {
  await request(path, { method: 'DELETE' })
}

export async function deleteJSONWithResponse<T>(path: string): Promise<T> {
  const response = await request(path, { method: 'DELETE' })
  return response.json() as Promise<T>
}

export async function postForm<T>(path: string, body: FormData): Promise<T> {
  const response = await fetch(toRequestURL(path), {
    method: 'POST',
    credentials: 'same-origin',
    headers: {
      Accept: 'application/json',
      ...adminAuthHeaders()
    },
    body
  })

  if (!response.ok) {
    const error = await toAPIError(response)
    if (error.status === 401) {
      window.dispatchEvent(new Event('admin-session-expired'))
    }
    throw error
  }

  return response.json() as Promise<T>
}

async function request(path: string, options: { method: 'GET' | 'POST' | 'PUT' | 'DELETE'; body?: unknown }): Promise<Response> {
  const headers: Record<string, string> = {
    Accept: 'application/json',
    ...adminAuthHeaders()
  }

  if (options.body !== undefined) {
    headers['Content-Type'] = 'application/json'
  }

  const response = await fetch(toRequestURL(path), {
    method: options.method,
    credentials: 'same-origin',
    headers,
    body: options.body === undefined ? undefined : JSON.stringify(options.body)
  })

  if (!response.ok) {
    const error = await toAPIError(response)
    if (error.status === 401) {
      window.dispatchEvent(new Event('admin-session-expired'))
    }
    throw error
  }

  return response
}

function toRequestURL(path: string) {
  if (/^https?:\/\//i.test(path)) return path
  return new URL(path, `${window.location.protocol}//${window.location.host}`).toString()
}

function adminAuthHeaders(): Record<string, string> {
  const headers: Record<string, string> = {}
  if (adminConfig.adminAPIKey) headers['X-Admin-API-Key'] = adminConfig.adminAPIKey
  if (adminCSRFToken) headers['X-CSRF-Token'] = adminCSRFToken
  return headers
}

let adminCSRFToken = ''

export function setAdminCSRFToken(token: string) {
  adminCSRFToken = token.trim()
}

export function getErrorMessage(error: unknown): string {
  if (error instanceof APIError) return error.message
  if (error instanceof Error) return '网络请求失败，请确认后端服务已启动后重试。'
  return '请求失败，请稍后重试。'
}

async function toAPIError(response: Response): Promise<APIError> {
  const body = await readErrorBody(response)
  return new APIError(response.status, body.error ?? 'internal_error')
}

async function readErrorBody(response: Response): Promise<APIErrorBody> {
  try {
    return (await response.json()) as APIErrorBody
  } catch {
    return {}
  }
}

function toUserMessage(code: APIErrorCode): string {
  switch (code) {
    case 'invalid_request':
      return '请求参数不完整，请检查输入内容。'
    case 'request_too_large':
      return '请求内容过大，请压缩图片或减少正文内容后重试。'
    case 'unauthorized':
      return '登录已失效，请重新登录后继续操作。'
    case 'forbidden':
      return '当前用户没有执行此操作的权限。'
    case 'conflict':
      return '该资源已属于其他用户。'
    case 'rate_limited':
      return '登录尝试过于频繁，请稍后再试。'
    case 'not_found':
      return '资源不存在或无权访问。'
    case 'not_implemented':
      return '该能力还没有完成配置，请先完成微信开放平台配置和公众号授权。'
    case 'internal_error':
      return '服务暂时异常，请稍后重试。'
  }
}
