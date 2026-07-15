import { getJSON } from './client'

export type DashboardStats = {
  account_total: number
  active_account_total: number
  revoked_account_total: number
  refresh_failed_account_total: number
  article_total: number
  draft_article_total: number
  publishing_article_total: number
  published_article_total: number
  failed_article_total: number
  publish_total: number
  publishing_publish_total: number
  published_publish_total: number
  failed_publish_total: number
}

export function getDashboardStats(): Promise<DashboardStats> {
	return getJSON<DashboardStats>('/api/v1/dashboard/stats')
}
