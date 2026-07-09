import { useEffect, useMemo, useState } from 'react'
import styled from '@emotion/styled'
import { listAccounts, type Account } from '../api/accounts'
import { createArticle, getArticle, updateArticle, type Article, type ArticleFormInput } from '../api/articles'
import { uploadCover, uploadInlineImage } from '../api/materials'
import { getErrorMessage } from '../api/client'
import { Button } from '../components/Button'
import { Card } from '../components/Card'
import { StatusBadge } from '../components/StatusBadge'
import { adminConfig } from '../config'
import { accountByID, accountDisplayName, accountOptionLabel } from '../utils/accounts'

const tenantID = adminConfig.tenantID
const emptyForm: ArticleFormInput = {
  authorizer_id: 0,
  title: '',
  author: '',
  digest: '',
  content_html: '',
  cover_media_asset_id: 0
}

type FieldErrors = Partial<Record<keyof ArticleFormInput, string>>

const svgAttrs = { viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', strokeWidth: 2, strokeLinecap: 'round' as const, strokeLinejoin: 'round' as const }

function ArrowLeftIcon() { return <svg {...svgAttrs} width="16" height="16"><line x1="19" y1="12" x2="5" y2="12" /><polyline points="12 19 5 12 12 5" /></svg> }
function UploadIcon() { return <svg {...svgAttrs} width="22" height="22"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" /><polyline points="17 8 12 3 7 8" /><line x1="12" y1="3" x2="12" y2="15" /></svg> }
function AlertIcon() { return <svg {...svgAttrs} width="20" height="20"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" /><line x1="12" y1="9" x2="12" y2="13" /><line x1="12" y1="17" x2="12.01" y2="17" /></svg> }
function CheckIcon() { return <svg {...svgAttrs} width="20" height="20"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" /><polyline points="22 4 12 14.01 9 11.01" /></svg> }
function ArticlePreviewIcon() { return <svg {...svgAttrs} width="16" height="16"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" /><polyline points="14 2 14 8 20 8" /><line x1="8" y1="13" x2="16" y2="13" /><line x1="8" y1="17" x2="14" y2="17" /></svg> }

type BackOptions = {
  skipDirtyCheck?: boolean
}

const emptyPreviewContentHTML = `
<section style="max-width: 677px; margin: 0 auto; padding: 24px 16px; background: #ffffff; box-sizing: border-box; font-family: -apple-system, BlinkMacSystemFont, 'Helvetica Neue', Helvetica, Arial, sans-serif;">
  <section style="margin: 0; padding: 28px 20px; background: #F6F7F6; border: 1px solid #DDEBDD; border-radius: 16px; box-sizing: border-box; text-align: center;">
    <p style="margin: 0; color: #7A817A; font-size: 14px; line-height: 1.8;"><span leaf="">正文预览会显示在这里。</span></p>
  </section>
</section>
`

function extractArticleContentHTML(contentHTML: string) {
  const trimmed = contentHTML.trim()
  if (!trimmed) return ''
  if (typeof DOMParser === 'undefined') return trimmed

  const parsed = new DOMParser().parseFromString(trimmed, 'text/html')
  const wrappedContent = parsed.getElementById('content')
  const wrappedHTML = wrappedContent?.innerHTML.trim()
  return wrappedHTML || trimmed
}

function buildArticlePreviewSrcDoc(contentHTML: string) {
  const previewContentHTML = extractArticleContentHTML(contentHTML) || emptyPreviewContentHTML
  return `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=760, initial-scale=1">
  <meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src https: http: data: blob:; style-src 'unsafe-inline'; font-src data:; media-src https: http: data: blob:; script-src 'none'; connect-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none';">
  <style>
    * { box-sizing: border-box; }
    html { margin: 0; min-height: 100%; background: #f6f7f6; }
    body { margin: 0; min-height: 100%; background: #f6f7f6; color: #2f332f; font-family: -apple-system, BlinkMacSystemFont, "Helvetica Neue", Helvetica, Arial, sans-serif; }
    .page { padding: 20px 0 28px; }
    .wrap { width: 760px; max-width: 100%; min-height: calc(100vh - 48px); margin: 0 auto; background: #fff; }
    img { max-width: 100%; height: auto; }
  </style>
</head>
<body>
  <main class="page">
    <div id="content" class="wrap">${previewContentHTML}</div>
  </main>
</body>
</html>`
}

export function ArticleEditorPage({ articleID, onBack, onDirtyChange }: { articleID?: number; onBack: (options?: BackOptions) => void; onDirtyChange: (dirty: boolean) => void }) {
  const editing = articleID !== undefined
  const [form, setForm] = useState<ArticleFormInput>(emptyForm)
  const [accounts, setAccounts] = useState<Account[]>([])
  const [accountsLoading, setAccountsLoading] = useState(true)
  const [loaded, setLoaded] = useState<Article | null>(null)
  const [loading, setLoading] = useState(editing)
  const [saving, setSaving] = useState(false)
  const [uploading, setUploading] = useState<'inline' | 'cover' | ''>('')
  const [error, setError] = useState('')
  const [savedMessage, setSavedMessage] = useState('')
  const [dirty, setDirty] = useState(false)
  const activeAccounts = useMemo(() => accounts.filter((account) => account.status === 'active'), [accounts])
  const accountsByID = useMemo(() => accountByID(accounts), [accounts])
  const selectedAccount = accountsByID.get(form.authorizer_id)
  const fieldErrors = useMemo(() => validate(form, editing), [form, editing])
  const canSave = Object.keys(fieldErrors).length === 0 && !saving && !loading && !accountsLoading
  const previewSrcDoc = useMemo(() => buildArticlePreviewSrcDoc(form.content_html), [form.content_html])

  useEffect(() => {
    onDirtyChange(dirty)
  }, [dirty, onDirtyChange])

  useEffect(() => {
    let active = true
    setAccountsLoading(true)

    listAccounts(tenantID)
      .then((items) => {
        if (!active) return
        setAccounts(items)
        const activeItems = items.filter((account) => account.status === 'active')
        if (!editing && activeItems.length === 1) {
          setForm((current) => current.authorizer_id > 0 ? current : { ...current, authorizer_id: activeItems[0].id })
        }
      })
      .catch((err: unknown) => {
        if (!active) return
        setError(getErrorMessage(err))
      })
      .finally(() => {
        if (active) setAccountsLoading(false)
      })

    return () => {
      active = false
    }
  }, [editing])

  useEffect(() => {
    if (!editing || articleID === undefined) return
    let active = true
    setLoading(true)
    setError('')

    getArticle(articleID, tenantID)
      .then((article) => {
        if (!active) return
        setLoaded(article)
        setForm({
          authorizer_id: article.authorizer_id,
          title: article.title,
          author: article.author,
          digest: article.digest,
          content_html: article.content_html,
          cover_media_asset_id: article.cover_media_asset_id
        })
        setDirty(false)
      })
      .catch((err: unknown) => {
        if (!active) return
        setError(getErrorMessage(err))
      })
      .finally(() => {
        if (active) setLoading(false)
      })

    return () => {
      active = false
    }
  }, [articleID, editing])

  useEffect(() => {
    const handler = (event: BeforeUnloadEvent) => {
      if (!dirty) return
      event.preventDefault()
    }
    window.addEventListener('beforeunload', handler)
    return () => window.removeEventListener('beforeunload', handler)
  }, [dirty])

  function update<K extends keyof ArticleFormInput>(key: K, value: ArticleFormInput[K]) {
    setForm((current) => ({ ...current, [key]: value }))
    setDirty(true)
    setSavedMessage('')
  }

  async function uploadMaterial(kind: 'inline' | 'cover', file: File | undefined) {
    if (!file || articleID === undefined) return
    if (form.authorizer_id <= 0) {
      setError('请先选择公众号。')
      return
    }

    setUploading(kind)
    setError('')
    setSavedMessage('')

    try {
      if (kind === 'inline') {
        const asset = await uploadInlineImage({ authorizerID: form.authorizer_id, articleID, file }, tenantID)
        update('content_html', `${form.content_html}\n<p><img src="${asset.wechat_url}" alt="" /></p>`)
        setSavedMessage('正文图片已上传，已插入 HTML，请保存文章。')
      } else {
        const asset = await uploadCover({ authorizerID: form.authorizer_id, articleID, file }, tenantID)
        update('cover_media_asset_id', asset.id)
        setSavedMessage('封面已上传，已写入封面素材 ID，请保存文章。')
      }
    } catch (err: unknown) {
      setError(getErrorMessage(err))
    } finally {
      setUploading('')
    }
  }

  async function save() {
    const nextErrors = validate(form, editing)
    if (Object.keys(nextErrors).length > 0) return

    setSaving(true)
    setError('')
    setSavedMessage('')

    try {
      if (editing && articleID !== undefined) {
        await updateArticle(articleID, form, tenantID)
      } else {
        const created = await createArticle(form, tenantID)
        if (form.cover_media_asset_id > 0) await updateArticle(created.id, form, tenantID)
      }
      setDirty(false)
      setSavedMessage(editing ? '文章已保存。' : '文章已创建。')
      if (!editing) onBack({ skipDirtyCheck: true })
    } catch (err: unknown) {
      setError(getErrorMessage(err))
    } finally {
      setSaving(false)
    }
  }

  return (
    <Page>
      <TopBar>
        <TopBarLeft>
          <Button variant="ghost" onClick={() => onBack()}><ArrowLeftIcon />返回列表</Button>
          <TopTitle>{editing ? '编辑文章' : '新建文章'}</TopTitle>
          {loaded ? <StatusBadge tone="muted">{loaded.status}</StatusBadge> : null}
        </TopBarLeft>
        <Button disabled={!canSave} onClick={save}>{saving ? '保存中…' : '保存'}</Button>
      </TopBar>

      {error ? <Notice $danger role="alert"><NoticeIconWrap $danger><AlertIcon /></NoticeIconWrap><div><PanelTitle>保存失败</PanelTitle><PanelDesc>{error}</PanelDesc></div></Notice> : null}
      {savedMessage ? <Notice><NoticeIconWrap><CheckIcon /></NoticeIconWrap><div><PanelTitle>{savedMessage}</PanelTitle><PanelDesc>可以返回列表查看最新更新时间。</PanelDesc></div></Notice> : null}

      <EditorGrid>
        <FormPanel>
          <BigTitleWrap>
            <BigTitleInput
              id="article-title"
              placeholder="输入文章标题…"
              value={form.title}
              onChange={(event) => update('title', event.target.value)}
            />
            {fieldErrors.title ? <FieldError>{fieldErrors.title}</FieldError> : <BigTitleHint>微信文章标题，保存前必须填写。</BigTitleHint>}
          </BigTitleWrap>

          <Divider />

          <SectionLabel>基础信息</SectionLabel>
          <TwoColumns>
            <Field>
              <Label htmlFor="authorizer-id">公众号 *</Label>
              {editing ? (
                <ReadonlyAccount>
                  <strong>{accountDisplayName(selectedAccount, form.authorizer_id)}</strong>
                  <span>ID {form.authorizer_id || '—'}</span>
                </ReadonlyAccount>
              ) : (
                <Select
                  disabled={accountsLoading || activeAccounts.length === 0}
                  id="authorizer-id"
                  value={form.authorizer_id || ''}
                  onChange={(event) => update('authorizer_id', Number(event.target.value))}
                >
                  <option value="">{accountsLoading ? '正在加载公众号…' : '选择公众号'}</option>
                  {activeAccounts.map((account) => (
                    <option key={account.id} value={account.id}>{accountOptionLabel(account)}</option>
                  ))}
                </Select>
              )}
              {fieldErrors.authorizer_id ? (
                <FieldError>{fieldErrors.authorizer_id}</FieldError>
              ) : (
                <Helper>{editing ? '已创建文章不能切换公众号。' : activeAccounts.length === 0 ? '没有可用公众号，请先在账号管理页添加或恢复授权。' : '只展示可用状态的授权公众号。'}</Helper>
              )}
            </Field>
            <Field>
              <Label htmlFor="article-author">作者</Label>
              <Input id="article-author" value={form.author} onChange={(event) => update('author', event.target.value)} />
              <Helper>可选。</Helper>
            </Field>
          </TwoColumns>
          <Field>
            <Label htmlFor="article-digest">摘要</Label>
            <Textarea id="article-digest" rows={3} value={form.digest} onChange={(event) => update('digest', event.target.value)} />
            <Helper>列表页和微信摘要展示用。</Helper>
          </Field>
          <Field>
            <Label htmlFor="cover-media-asset-id">封面素材 ID</Label>
            <Input
              id="cover-media-asset-id"
              min="0"
              type="number"
              value={form.cover_media_asset_id || ''}
              onChange={(event) => update('cover_media_asset_id', Number(event.target.value))}
            />
            {fieldErrors.cover_media_asset_id ? <FieldError>{fieldErrors.cover_media_asset_id}</FieldError> : <Helper>没有封面时留空；上传封面后会自动写入本地素材 ID。</Helper>}
          </Field>

          <Divider />

          <SectionLabel>素材上传</SectionLabel>
          <UploadGrid>
            <Field>
              <Label>正文图片</Label>
              <UploadControl
                disabled={!editing || uploading !== ''}
                id="inline-image-upload"
                loading={uploading === 'inline'}
                note={editing ? '上传成功后会把 wechat_url 插入正文 HTML。' : '请先保存文章，再上传正文图片。'}
                title="选择正文图片"
                onFile={(file) => uploadMaterial('inline', file)}
              />
            </Field>
            <Field>
              <Label>封面图</Label>
              <UploadControl
                disabled={!editing || uploading !== ''}
                id="cover-upload"
                loading={uploading === 'cover'}
                note={editing ? '上传成功后会把本地素材 ID 写入封面字段。' : '请先保存文章，再上传封面。'}
                title="选择封面图"
                onFile={(file) => uploadMaterial('cover', file)}
              />
            </Field>
          </UploadGrid>
          {uploading ? <Helper>正在上传{uploading === 'inline' ? '正文图片' : '封面'}…</Helper> : null}

          <Divider />

          <SectionLabel>正文 HTML</SectionLabel>
          <Field>
            <Label htmlFor="content-html">HTML 内容</Label>
            <CodeTextarea id="content-html" rows={16} value={form.content_html} onChange={(event) => update('content_html', extractArticleContentHTML(event.target.value))} />
            <Helper>可粘贴 gzh 正文 section；误粘完整预览页时会提取正文。不要粘贴 token、secret、refresh 等敏感内容。</Helper>
          </Field>
        </FormPanel>

        <PreviewColumn>
          <PreviewHeader><ArticlePreviewIcon /><PreviewHeaderTitle>公众号排版预览</PreviewHeaderTitle></PreviewHeader>
          <PreviewCanvas>
            <PreviewFrame title="公众号文章 HTML 预览" sandbox="" srcDoc={previewSrcDoc} />
          </PreviewCanvas>
        </PreviewColumn>
      </EditorGrid>
    </Page>
  )
}

function UploadControl({ id, title, note, disabled, loading, onFile }: { id: string; title: string; note: string; disabled: boolean; loading: boolean; onFile: (file: File) => void }) {
  return (
    <UploadBox data-disabled={disabled}>
      <UploadMain>
        <UploadIconWrap><UploadIcon /></UploadIconWrap>
        <UploadText>
          <strong>{loading ? '上传中…' : title}</strong>
          <span>{note}</span>
        </UploadText>
      </UploadMain>
      <UploadButton htmlFor={id} aria-disabled={disabled}>{loading ? '处理中' : '选择文件'}</UploadButton>
      <HiddenFileInput
        id={id}
        disabled={disabled}
        type="file"
        accept="image/*"
        onChange={(event) => {
          const file = event.target.files?.[0]
          if (file) onFile(file)
          event.target.value = ''
        }}
      />
    </UploadBox>
  )
}

function validate(form: ArticleFormInput, editing: boolean): FieldErrors {
  const errors: FieldErrors = {}
  if (!form.title.trim()) errors.title = '标题必填。'
  if (!editing && form.authorizer_id <= 0) errors.authorizer_id = '请选择要发布的公众号。'
  if (form.cover_media_asset_id < 0) errors.cover_media_asset_id = '封面素材 ID 不能小于 0。'
  return errors
}

const Page = styled.div`
  display: grid;
  gap: ${({ theme }) => theme.space.xl};
`

const TopBar = styled.div`
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: ${({ theme }) => theme.space.lg};
  flex-wrap: wrap;
  padding: ${({ theme }) => theme.space.lg} ${({ theme }) => theme.space.xl};
  border: 1px solid ${({ theme }) => theme.colors.border};
  border-radius: ${({ theme }) => theme.radii.lg};
  background: ${({ theme }) => theme.colors.surface};
  box-shadow: ${({ theme }) => theme.shadows.xs};
  position: sticky;
  top: ${({ theme }) => theme.space.lg};
  z-index: 5;
`

const TopBarLeft = styled.div`
  display: flex;
  align-items: center;
  gap: ${({ theme }) => theme.space.lg};
  flex-wrap: wrap;
  min-width: 0;

  button {
    display: inline-flex;
    align-items: center;
    gap: ${({ theme }) => theme.space.xs};
  }
`

const TopTitle = styled.h1`
  margin: 0;
  font-size: ${({ theme }) => theme.typeScale.title};
  font-weight: 800;
  letter-spacing: -0.02em;
  color: ${({ theme }) => theme.colors.text};
`

const EditorGrid = styled.div`
  display: grid;
  gap: ${({ theme }) => theme.space.xl};

  @media (min-width: 1180px) {
    grid-template-columns: minmax(420px, 0.78fr) minmax(620px, 1.22fr);
    align-items: start;
  }
`

const FormPanel = styled(Card)`
  display: grid;
  gap: ${({ theme }) => theme.space.lg};
  padding: ${({ theme }) => theme.space.xl};
`

const BigTitleWrap = styled.div`
  display: grid;
  gap: ${({ theme }) => theme.space.xs};
`

const BigTitleInput = styled.input`
  width: 100%;
  border: none;
  background: transparent;
  color: ${({ theme }) => theme.colors.text};
  font-size: 24px;
  font-weight: 800;
  letter-spacing: -0.02em;
  padding: ${({ theme }) => theme.space.xs} 0;
  border-bottom: 2px solid ${({ theme }) => theme.colors.border};
  transition: border-color ${({ theme }) => theme.motion.fast} ${({ theme }) => theme.motion.easeOut};

  &::placeholder {
    color: ${({ theme }) => theme.colors.textFaint};
  }

  &:focus {
    outline: none;
    border-bottom-color: ${({ theme }) => theme.colors.primary};
  }
`

const BigTitleHint = styled.p`
  margin: 0;
  color: ${({ theme }) => theme.colors.textFaint};
  font-size: ${({ theme }) => theme.typeScale.small};
`

const Divider = styled.hr`
  border: none;
  border-top: 1px solid ${({ theme }) => theme.colors.border};
  margin: ${({ theme }) => theme.space.sm} 0;
`

const SectionLabel = styled.div`
  display: flex;
  align-items: center;
  gap: ${({ theme }) => theme.space.sm};
  color: ${({ theme }) => theme.colors.primary};
  font-size: ${({ theme }) => theme.typeScale.small};
  font-weight: 750;
  letter-spacing: 0.03em;
  text-transform: uppercase;
  margin-bottom: ${({ theme }) => theme.space.xs};

  &::before {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: ${({ theme }) => theme.colors.primary};
    content: '';
  }
`

const Field = styled.div`
  display: grid;
  gap: ${({ theme }) => theme.space.sm};
`

const Label = styled.label`
  color: ${({ theme }) => theme.colors.text};
  font-weight: 700;
`

const Input = styled.input`
  min-height: 44px;
  border: 1px solid ${({ theme }) => theme.colors.border};
  border-radius: ${({ theme }) => theme.radii.md};
  padding: 0 ${({ theme }) => theme.space.md};
  background: ${({ theme }) => theme.colors.surface};
  color: ${({ theme }) => theme.colors.text};
  font: inherit;

  &:focus {
    outline: 3px solid ${({ theme }) => theme.colors.primarySoft};
    border-color: ${({ theme }) => theme.colors.primary};
  }

  &:disabled {
    background: ${({ theme }) => theme.colors.surfaceMuted};
    color: ${({ theme }) => theme.colors.textMuted};
  }
`

const Select = styled.select`
  min-height: 44px;
  border: 1px solid ${({ theme }) => theme.colors.border};
  border-radius: ${({ theme }) => theme.radii.md};
  padding: 0 ${({ theme }) => theme.space.md};
  background: ${({ theme }) => theme.colors.surface};
  color: ${({ theme }) => theme.colors.text};
  font: inherit;

  &:focus {
    outline: 3px solid ${({ theme }) => theme.colors.primarySoft};
    border-color: ${({ theme }) => theme.colors.primary};
  }

  &:disabled {
    background: ${({ theme }) => theme.colors.surfaceMuted};
    color: ${({ theme }) => theme.colors.textMuted};
  }
`

const ReadonlyAccount = styled.div`
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: ${({ theme }) => theme.space.md};
  min-height: 44px;
  border: 1px solid ${({ theme }) => theme.colors.border};
  border-radius: ${({ theme }) => theme.radii.md};
  background: ${({ theme }) => theme.colors.surfaceMuted};
  padding: 0 ${({ theme }) => theme.space.md};
  color: ${({ theme }) => theme.colors.text};

  strong {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  span {
    color: ${({ theme }) => theme.colors.textMuted};
    font-size: ${({ theme }) => theme.typeScale.small};
    white-space: nowrap;
  }
`

const Textarea = styled.textarea`
  border: 1px solid ${({ theme }) => theme.colors.border};
  border-radius: ${({ theme }) => theme.radii.md};
  padding: ${({ theme }) => theme.space.md};
  background: ${({ theme }) => theme.colors.surface};
  color: ${({ theme }) => theme.colors.text};
  font: inherit;
  resize: vertical;

  &:focus {
    outline: 3px solid ${({ theme }) => theme.colors.primarySoft};
    border-color: ${({ theme }) => theme.colors.primary};
  }
`

const CodeTextarea = styled(Textarea)`
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: ${({ theme }) => theme.typeScale.small};
  line-height: 1.6;
`

const TwoColumns = styled.div`
  display: grid;
  gap: ${({ theme }) => theme.space.lg};

  @media (min-width: 720px) {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
`

const UploadGrid = styled(TwoColumns)``

const Helper = styled.p`
  margin: 0;
  color: ${({ theme }) => theme.colors.textMuted};
  font-size: ${({ theme }) => theme.typeScale.small};
`

const FieldError = styled(Helper)`
  color: ${({ theme }) => theme.colors.danger};
  font-weight: 650;
`

const UploadBox = styled.div`
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: ${({ theme }) => theme.space.lg};
  min-height: 78px;
  border: 1px dashed ${({ theme }) => theme.colors.border};
  border-radius: ${({ theme }) => theme.radii.lg};
  background: ${({ theme }) => theme.colors.surface};
  padding: ${({ theme }) => theme.space.lg};
  transition: border-color ${({ theme }) => theme.motion.fast} ${({ theme }) => theme.motion.easeOut}, background ${({ theme }) => theme.motion.fast} ${({ theme }) => theme.motion.easeOut};

  &:hover {
    border-color: ${({ theme }) => theme.colors.primary};
    background: ${({ theme }) => theme.colors.primarySoft};
  }

  &[data-disabled='true'] {
    background: ${({ theme }) => theme.colors.surfaceMuted};
    opacity: 0.72;
  }

  @media (max-width: 560px) {
    grid-template-columns: 1fr;
  }
`

const UploadMain = styled.div`
  display: flex;
  align-items: center;
  min-width: 0;
  gap: ${({ theme }) => theme.space.md};
`

const UploadIconWrap = styled.span`
  display: grid;
  flex: 0 0 auto;
  width: 44px;
  height: 44px;
  place-items: center;
  border-radius: ${({ theme }) => theme.radii.md};
  background: ${({ theme }) => theme.colors.primarySoft};
  color: ${({ theme }) => theme.colors.primary};
`

const UploadText = styled.div`
  display: grid;
  min-width: 0;
  gap: ${({ theme }) => theme.space.xs};

  strong {
    color: ${({ theme }) => theme.colors.text};
    font-weight: 750;
  }

  span {
    color: ${({ theme }) => theme.colors.textMuted};
    font-size: ${({ theme }) => theme.typeScale.small};
    line-height: 1.5;
  }
`

const UploadButton = styled.label`
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 40px;
  border: 1px solid ${({ theme }) => theme.colors.border};
  border-radius: ${({ theme }) => theme.radii.md};
  background: ${({ theme }) => theme.colors.surface};
  color: ${({ theme }) => theme.colors.text};
  cursor: pointer;
  font-weight: 650;
  padding: 0 ${({ theme }) => theme.space.lg};
  white-space: nowrap;

  &[aria-disabled='true'] {
    cursor: not-allowed;
    opacity: 0.55;
    pointer-events: none;
  }
`

const HiddenFileInput = styled.input`
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0 0 0 0);
  clip-path: inset(50%);
  white-space: nowrap;
`

const Notice = styled(Card)<{ $danger?: boolean }>`
  display: flex;
  align-items: center;
  gap: ${({ theme }) => theme.space.lg};
  padding: ${({ theme }) => theme.space.lg} ${({ theme }) => theme.space.xl};
  border-left: 3px solid ${({ theme, $danger }) => ($danger ? theme.colors.danger : theme.colors.success)};
`

const NoticeIconWrap = styled.span<{ $danger?: boolean }>`
  display: grid;
  width: 40px;
  height: 40px;
  place-items: center;
  border-radius: ${({ theme }) => theme.radii.md};
  background: ${({ theme, $danger }) => ($danger ? theme.colors.dangerSoft : theme.colors.successSoft)};
  color: ${({ theme, $danger }) => ($danger ? theme.colors.danger : theme.colors.success)};
  flex-shrink: 0;
`

const PanelTitle = styled.h2`
  margin: 0;
  font-size: ${({ theme }) => theme.typeScale.title};
  letter-spacing: -0.02em;
  font-weight: 700;
`

const PanelDesc = styled.p`
  margin: ${({ theme }) => theme.space.xs} 0 0;
  color: ${({ theme }) => theme.colors.textMuted};
  font-size: ${({ theme }) => theme.typeScale.small};
`

const PreviewColumn = styled.div`
  display: grid;
  gap: ${({ theme }) => theme.space.lg};
  min-width: 0;
  justify-items: stretch;

  @media (min-width: 1180px) {
    position: sticky;
    top: 100px;
  }
`

const PreviewHeader = styled.div`
  display: flex;
  align-items: center;
  gap: ${({ theme }) => theme.space.sm};
  color: ${({ theme }) => theme.colors.textMuted};
  font-size: ${({ theme }) => theme.typeScale.small};
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.03em;
  justify-self: start;
`

const PreviewHeaderTitle = styled.span``

const PreviewCanvas = styled.div`
  width: 100%;
  min-width: 0;
  overflow-x: auto;
  border: 1px solid ${({ theme }) => theme.colors.border};
  border-radius: ${({ theme }) => theme.radii.lg};
  background: #f6f7f6;
  box-shadow: ${({ theme }) => theme.shadows.xs};
`

const PreviewFrame = styled.iframe`
  display: block;
  width: 760px;
  height: clamp(560px, 72vh, 860px);
  border: 0;
  background: #f6f7f6;
`
