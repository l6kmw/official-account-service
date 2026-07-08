import { getJSON } from './client'

export type AccountStatus = 'active' | 'revoked' | 'refresh_failed' | 'unknown'

export type Account = {
  id: number
  tenant_id: string
  app_id: string
  name: string
  avatar_url: string
  status: AccountStatus
  last_synced_at: string
  created_at: string
  updated_at: string
}

type AccountListResponse = {
  items: Account[]
}

export async function listAccounts(tenantID?: string): Promise<Account[]> {
  const response = await getJSON<AccountListResponse>('/api/v1/accounts', tenantID)
  return response.items
}
