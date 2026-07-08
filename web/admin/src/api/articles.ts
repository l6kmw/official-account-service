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

export async function listArticles(tenantID?: string): Promise<Article[]> {
  const response = await getJSON<ArticleListResponse>('/api/v1/articles', tenantID)
  return response.items
}

export function getArticle(id: number, tenantID?: string): Promise<Article> {
  return getJSON<Article>(`/api/v1/articles/${id}`, tenantID)
}

export function createArticle(input: ArticleFormInput, tenantID?: string): Promise<Article> {
  return postJSON<Article>('/api/v1/articles', {
    authorizer_id: input.authorizer_id,
    title: input.title,
    author: input.author,
    digest: input.digest,
    content_html: input.content_html
  }, tenantID)
}

export function updateArticle(id: number, input: ArticleFormInput, tenantID?: string): Promise<Article> {
  return putJSON<Article>(`/api/v1/articles/${id}`, input, tenantID)
}

export function deleteArticle(id: number, tenantID?: string): Promise<void> {
  return deleteJSON(`/api/v1/articles/${id}`, tenantID)
}

export function publishArticle(id: number, tenantID?: string): Promise<PublishRecord> {
  return postJSON<PublishRecord>(`/api/v1/articles/${id}/publish`, {}, tenantID)
}
