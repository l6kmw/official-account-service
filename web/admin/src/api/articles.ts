import { deleteJSON, getJSON, postJSON, putJSON } from './client'
import type { PublishRecord } from './publishRecords'

export type ArticleStatus = 'draft' | 'publishing' | 'published' | 'failed'

export type Article = {
  id: number
  tenant_id: string
  authorizer_id: number
  title: string
  author: string
  digest: string
  content_html: string
  cover_media_asset_id: number
  status: ArticleStatus
  created_by_agent_id: string
  updated_by_agent_id: string
  version: number
  created_at: string
  updated_at: string
}

export type ArticleFormInput = {
  authorizer_id: number
  title: string
  author: string
  digest: string
  content_html: string
  cover_media_asset_id: number
}

type ArticleListResponse = {
  items: Article[]
}

export async function listArticles(agentRecordID = ''): Promise<Article[]> {
  const query = agentRecordID.trim() ? `?agent_record_id=${encodeURIComponent(agentRecordID.trim())}` : ''
  const response = await getJSON<ArticleListResponse>(`/api/v1/articles${query}`)
  return response.items
}

export function getArticle(id: number): Promise<Article> {
	return getJSON<Article>(`/api/v1/articles/${id}`)
}

export function createArticle(input: ArticleFormInput): Promise<Article> {
  return postJSON<Article>('/api/v1/articles', {
    authorizer_id: input.authorizer_id,
    title: input.title,
    author: input.author,
    digest: input.digest,
    content_html: input.content_html
	})
}

export function updateArticle(id: number, input: ArticleFormInput, version: number): Promise<Article> {
  return putJSON<Article>(`/api/v1/articles/${id}`, { ...input, version })
}

export function deleteArticle(id: number): Promise<void> {
	return deleteJSON(`/api/v1/articles/${id}`)
}

export function publishArticle(id: number): Promise<PublishRecord> {
	return postJSON<PublishRecord>(`/api/v1/articles/${id}/publish`, {})
}
