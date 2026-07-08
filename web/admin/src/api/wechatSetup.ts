import { getJSON } from './client'

export type AuthorizationURLResult = {
  authorization_url: string
  pre_auth_code_expires_in_sec: number
}

export function generateAuthorizationURL(input: { componentAppID: string; redirectURI: string; authType: number }, tenantID?: string): Promise<AuthorizationURLResult> {
  const query = new URLSearchParams({
    component_appid: input.componentAppID,
    redirect_uri: input.redirectURI,
    auth_type: `${input.authType}`
  })
  return getJSON<AuthorizationURLResult>(`/api/v1/wechat/authorization-url?${query.toString()}`, tenantID)
}
