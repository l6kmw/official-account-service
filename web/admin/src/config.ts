type AdminConfig = {
  tenantID: string
  publicBaseURL: string
  componentAppID: string
}

export const adminConfig: AdminConfig = {
  tenantID: 'tenant-1',
  publicBaseURL: 'https://example.com',
  componentAppID: 'wx0000000000000000'
}

export function normalizePublicBaseURL(value: string) {
  const trimmed = value.trim().replace(/\/+$/, '')
  if (!trimmed) return ''
  if (/^https?:\/\//i.test(trimmed)) return trimmed
  return `https://${trimmed}`
}

export function buildAuthorizationEntryURL(input: { publicBaseURL: string; componentAppID: string; tenantID?: string }) {
  const url = new URL('/wechat-authorize.html', `${input.publicBaseURL}/`)
  url.searchParams.set('tenant_id', input.tenantID ?? adminConfig.tenantID)
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
    authorizationCallback: `${base}/api/v1/wechat/authorization-callback?tenant_id=${tenantID}&component_appid=${encodeURIComponent(componentAppID)}`,
    authorizationEntry: buildAuthorizationEntryURL({ publicBaseURL: base, componentAppID, tenantID })
  }
}
