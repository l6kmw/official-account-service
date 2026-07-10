import { getJSON } from './client'

export type MCPConnectionConfig = {
  transport: string
  path: string
  header_name: string
  token: string
  configured: boolean
}

export function getMCPConfig() {
  return getJSON<MCPConnectionConfig>('/api/v1/admin/mcp-config')
}
