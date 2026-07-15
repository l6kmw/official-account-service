type AdminConfig = {
  tenantID: string
  publicBaseURL: string
  componentAppID: string
  adminAPIKey: string
}

declare global {
  interface Window {
    __OFFICIAL_ACCOUNT_ADMIN_CONFIG__?: Partial<AdminConfig>
  }
}

const defaultAdminConfig: AdminConfig = {
  tenantID: 'tenant-1',
  publicBaseURL: 'https://example.com',
  componentAppID: 'wx0000000000000000',
  adminAPIKey: ''
}

export const adminConfig: AdminConfig = resolveAdminConfig(window.__OFFICIAL_ACCOUNT_ADMIN_CONFIG__)

function resolveAdminConfig(runtimeConfig: Partial<AdminConfig> | undefined): AdminConfig {
  return {
    tenantID: nonEmpty(runtimeConfig?.tenantID) || defaultAdminConfig.tenantID,
    publicBaseURL: normalizePublicBaseURL(runtimeConfig?.publicBaseURL ?? '') || defaultAdminConfig.publicBaseURL,
    componentAppID: nonEmpty(runtimeConfig?.componentAppID) || defaultAdminConfig.componentAppID,
    adminAPIKey: nonEmpty(runtimeConfig?.adminAPIKey)
  }
}

function nonEmpty(value: string | undefined) {
  return value?.trim() ?? ''
}

export function normalizePublicBaseURL(value: string) {
  const trimmed = value.trim().replace(/\/+$/, '')
  if (!trimmed) return ''
  if (/^https?:\/\//i.test(trimmed)) return trimmed
  return `https://${trimmed}`
}

export function buildAuthorizationEntryURL(input: { publicBaseURL: string; componentAppID: string; tenantID?: string }) {
  const url = new URL('/wechat-authorize.html', `${input.publicBaseURL}/`)
  url.searchParams.set('component_appid', input.componentAppID.trim())
  return url.toString()
}

export function buildWechatOpenPlatformURLs(input: { publicBaseURL: string; componentAppID: string; tenantID?: string }) {
  const base = normalizePublicBaseURL(input.publicBaseURL) || adminConfig.publicBaseURL
  const componentAppID = input.componentAppID.trim() || '你的ComponentAppID'
  const tenantID = input.tenantID ?? adminConfig.tenantID

  return {
    componentCallback: `${base}/wechat/component/callback`,
    authorizerCallback: `${base}/wechat/authorizer/$APPID$/callback`,
    authorizationCallback: `${base}/api/v1/wechat/authorization-callback`,
    authorizationEntry: buildAuthorizationEntryURL({ publicBaseURL: base, componentAppID, tenantID })
  }
}
