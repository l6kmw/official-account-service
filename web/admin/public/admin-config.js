// Runtime config for the admin UI. Values here are browser-visible public config
// only — never put AppSecret, EncodingAESKey, database credentials,
// security.admin_api_key, mcp.token or any user token here.
//
// Fill in per deployment, e.g. edit this file in the served static dir.
window.__OFFICIAL_ACCOUNT_ADMIN_CONFIG__ = {
  publicBaseURL: '',
  componentAppID: '',
  adminAPIKey: ''
}
