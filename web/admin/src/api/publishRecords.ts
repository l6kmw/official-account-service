import { getJSON, postJSON } from './client'

export type PublishStatus = 'publishing' | 'published' | 'failed'

export type PublishRecord = {
  id: number
  tenant_id: string
  authorizer_id: number
  article_id: number
  wechat_publish_id: string
  wechat_article_id: string
  status: PublishStatus
  error_code: string
  error_message: string
  submitted_at: string
  finished_at: string
  created_at: string
  updated_at: string
}

type PublishRecordListResponse = {
  items: PublishRecord[]
}

export async function listPublishRecords(tenantID?: string): Promise<PublishRecord[]> {
  const response = await getJSON<PublishRecordListResponse>('/api/v1/publish-records', tenantID)
  return response.items
}

export function getPublishRecord(id: number, tenantID?: string): Promise<PublishRecord> {
  return getJSON<PublishRecord>(`/api/v1/publish-records/${id}`, tenantID)
}

export function syncPublishRecordStatus(id: number, tenantID?: string): Promise<PublishRecord> {
  return postJSON<PublishRecord>(`/api/v1/publish-records/${id}/sync-status`, {}, tenantID)
}
