import type { Account } from '../api/accounts'

export function accountDisplayName(account: Account | undefined, authorizerID?: number) {
  if (account) return account.name || `未命名公众号 #${account.id}`
  if (authorizerID && authorizerID > 0) return `公众号 #${authorizerID}`
  return '未选择公众号'
}

export function accountOptionLabel(account: Account) {
  const name = account.name || `未命名公众号 #${account.id}`
  return `${name} / ID ${account.id}`
}

export function accountByID(accounts: Account[]) {
  return new Map(accounts.map((account) => [account.id, account]))
}
