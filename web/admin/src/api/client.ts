import { adminConfig } from '../config'

export const DEFAULT_TENANT_ID = adminConfig.tenantID

type APIErrorCode = 'invalid_request' | 'request_too_large' | 'unauthorized' | 'not_found' | 'not_implemented' | 'internal_error'

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

export async function getJSON<T>(path: string, tenantID = DEFAULT_TENANT_ID): Promise<T> {
  const response = await request(path, { method: 'GET', tenantID })
  return response.json() as Promise<T>
}

export async function postJSON<T>(path: string, body: unknown, tenantID = DEFAULT_TENANT_ID): Promise<T> {
  const response = await request(path, { method: 'POST', tenantID, body })
  return response.json() as Promise<T>
}

export async function putJSON<T>(path: string, body: unknown, tenantID = DEFAULT_TENANT_ID): Promise<T> {
  const response = await request(path, { method: 'PUT', tenantID, body })
  return response.json() as Promise<T>
}

export async function deleteJSON(path: string, tenantID = DEFAULT_TENANT_ID): Promise<void> {
  await request(path, { method: 'DELETE', tenantID })
}

export async function postForm<T>(path: string, body: FormData, tenantID = DEFAULT_TENANT_ID): Promise<T> {
  const response = await fetch(toRequestURL(path), {
    method: 'POST',
    headers: {
      Accept: 'application/json',
      'X-Tenant-ID': tenantID,
      ...adminAuthHeaders()
    },
    body
  })

  if (!response.ok) {
    throw await toAPIError(response)
  }

  return response.json() as Promise<T>
}

async function request(path: string, options: { method: 'GET' | 'POST' | 'PUT' | 'DELETE'; tenantID: string; body?: unknown }): Promise<Response> {
  const headers: Record<string, string> = {
    Accept: 'application/json',
    'X-Tenant-ID': options.tenantID,
    ...adminAuthHeaders()
  }

  if (options.body !== undefined) {
    headers['Content-Type'] = 'application/json'
  }

  const response = await fetch(toRequestURL(path), {
    method: options.method,
    headers,
    body: options.body === undefined ? undefined : JSON.stringify(options.body)
  })

  if (!response.ok) {
    throw await toAPIError(response)
  }

  return response
}

function toRequestURL(path: string) {
  if (/^https?:\/\//i.test(path)) return path
  return new URL(path, `${window.location.protocol}//${window.location.host}`).toString()
}

function adminAuthHeaders(): Record<string, string> {
  if (!adminConfig.adminAPIKey) return {}
  return { 'X-Admin-API-Key': adminConfig.adminAPIKey }
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
      return '请求参数不完整，请检查租户和输入内容。'
    case 'request_too_large':
      return '请求内容过大，请压缩图片或减少正文内容后重试。'
    case 'unauthorized':
      return '管理 API 未授权，请检查 adminAPIKey 或网关注入的鉴权请求头。'
    case 'not_found':
      return '资源不存在或无权访问。'
    case 'not_implemented':
      return '该能力还没有完成配置，请先完成微信开放平台配置和公众号授权。'
    case 'internal_error':
      return '服务暂时异常，请稍后重试。'
  }
}
