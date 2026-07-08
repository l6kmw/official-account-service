import { postForm } from './client'

export type MaterialAsset = {
  id: number
  tenant_id: string
  authorizer_id: number
  article_id: number
  usage: 'inline_image' | 'cover'
  local_url: string
  wechat_url: string
  media_id: string
  created_at: string
}

type UploadInput = {
  authorizerID: number
  articleID: number
  file: File
}

export function uploadInlineImage(input: UploadInput, tenantID?: string): Promise<MaterialAsset> {
  return postForm<MaterialAsset>('/api/v1/materials/inline-images', toFormData(input), tenantID)
}

export function uploadCover(input: UploadInput, tenantID?: string): Promise<MaterialAsset> {
  return postForm<MaterialAsset>('/api/v1/materials/covers', toFormData(input), tenantID)
}

function toFormData(input: UploadInput) {
  const form = new FormData()
  form.append('authorizer_id', `${input.authorizerID}`)
  form.append('article_id', `${input.articleID}`)
  form.append('file', input.file)
  return form
}
