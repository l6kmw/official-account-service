type AdminConfig = {
  publicBaseURL: string
  componentAppID: string
  adminAPIKey: string
}

declare global {
  interface Window {
    __OFFICIAL_ACCOUNT_ADMIN_CONFIG__?: Partial<AdminConfig>
  }
}

// Runtime config for the admin UI. Values here are browser-visible public config
// only — never put AppSecret, EncodingAESKey, database credentials,
// security.admin_api_key, mcp.token or any user token here.
//
// Override per deployment (e.g. edit this file in the served static dir).
const defaultAdminConfig: AdminConfig = {
  publicBaseURL: '',
  componentAppID: '',
  adminAPIKey: ''
}

export const adminConfig: AdminConfig = resolveAdminConfig(window.__OFFICIAL_ACCOUNT_ADMIN_CONFIG__)

function resolveAdminConfig(runtimeConfig: Partial<AdminConfig> | undefined): AdminConfig {
  return {
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

export function buildAuthorizationEntryURL(input: { publicBaseURL: string; componentAppID: string }) {
  const url = new URL('/wechat-authorize.html', `${input.publicBaseURL}/`)
  url.searchParams.set('component_appid', input.componentAppID.trim())
  return url.toString()
}

export function buildWechatOpenPlatformURLs(input: { publicBaseURL: string; componentAppID: string }) {
  const base = normalizePublicBaseURL(input.publicBaseURL) || adminConfig.publicBaseURL
  const componentAppID = input.componentAppID.trim() || '你的ComponentAppID'
  return {
    componentCallback: `${base}/wechat/component/callback`,
    authorizerCallback: `${base}/wechat/authorizer/$APPID$/callback`,
    authorizationCallback: `${base}/api/v1/wechat/authorization-callback`,
    authorizationEntry: buildAuthorizationEntryURL({ publicBaseURL: base, componentAppID })
  }
}
