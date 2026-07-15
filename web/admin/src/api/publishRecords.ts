import { getJSON, postJSON } from './client'

export type PublishStatus = 'publishing' | 'published' | 'failed' | 'deleted'

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
  article_created_by_agent_id: string
  submitted_at: string
  finished_at: string
  created_at: string
  updated_at: string
}

type PublishRecordListResponse = {
  items: PublishRecord[]
}

export async function listPublishRecords(agentRecordID = ''): Promise<PublishRecord[]> {
  const query = agentRecordID.trim() ? `?agent_record_id=${encodeURIComponent(agentRecordID.trim())}` : ''
  const response = await getJSON<PublishRecordListResponse>(`/api/v1/publish-records${query}`)
  return response.items
}

export function getPublishRecord(id: number): Promise<PublishRecord> {
	return getJSON<PublishRecord>(`/api/v1/publish-records/${id}`)
}

export function syncPublishRecordStatus(id: number): Promise<PublishRecord> {
	return postJSON<PublishRecord>(`/api/v1/publish-records/${id}/sync-status`, {})
}

export function deletePublishedRecord(id: number): Promise<PublishRecord> {
	return postJSON<PublishRecord>(`/api/v1/publish-records/${id}/delete-published`, {})
}
