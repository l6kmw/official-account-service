import { useEffect, useState } from 'react'
import styled from '@emotion/styled'
import { listAccounts, type Account } from '../api/accounts'
import { deleteArticle, listArticles, publishArticle, type Article, type ArticleStatus } from '../api/articles'
import { getErrorMessage } from '../api/client'
import { Button } from '../components/Button'
import { Card } from '../components/Card'
import { StatusBadge } from '../components/StatusBadge'
import { adminConfig } from '../config'
import { accountByID, accountDisplayName, accountOptionLabel } from '../utils/accounts'

const tenantID = adminConfig.tenantID

type Tone = 'success' | 'warning' | 'danger' | 'info' | 'muted'
const toneColor: Record<Tone, string> = { success: 'success', warning: 'warning', danger: 'danger', info: 'info', muted: 'textMuted' }

type FilterKey = 'all' | 'draft' | 'publishing' | 'published' | 'failed'

const svgAttrs = { viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', strokeWidth: 2, strokeLinecap: 'round' as const, strokeLinejoin: 'round' as const }

function PlusIcon() { return <svg {...svgAttrs} width="16" height="16"><line x1="12" y1="5" x2="12" y2="19" /><line x1="5" y1="12" x2="19" y2="12" /></svg> }
function RefreshIcon() { return <svg {...svgAttrs} width="16" height="16"><polyline points="23 4 23 10 17 10" /><polyline points="1 20 1 14 7 14" /><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15" /></svg> }
function EditIcon() { return <svg {...svgAttrs} width="15" height="15"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7" /><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z" /></svg> }
function SendIcon() { return <svg {...svgAttrs} width="15" height="15"><line x1="22" y1="2" x2="11" y2="13" /><polygon points="22 2 15 22 11 13 2 9 22 2" /></svg> }
function TrashIcon() { return <svg {...svgAttrs} width="15" height="15"><polyline points="3 6 5 6 21 6" /><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" /></svg> }
function ClockIcon() { return <svg {...svgAttrs} width="15" height="15"><circle cx="12" cy="12" r="10" /><polyline points="12 6 12 12 16 14" /></svg> }
function UserIcon() { return <svg {...svgAttrs} width="15" height="15"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2" /><circle cx="12" cy="7" r="4" /></svg> }
function FileLargeIcon() { return <svg {...svgAttrs} width="64" height="64"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" /><polyline points="14 2 14 8 20 8" /><line x1="8" y1="13" x2="16" y2="13" /><line x1="8" y1="17" x2="14" y2="17" /></svg> }
function AlertIcon() { return <svg {...svgAttrs} width="20" height="20"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" /><line x1="12" y1="9" x2="12" y2="13" /><line x1="12" y1="17" x2="12.01" y2="17" /></svg> }
function InfoIcon() { return <svg {...svgAttrs} width="20" height="20"><circle cx="12" cy="12" r="10" /><line x1="12" y1="16" x2="12" y2="12" /><line x1="12" y1="8" x2="12.01" y2="8" /></svg> }

const FILTER_TABS: { key: FilterKey; label: string }[] = [
  { key: 'all', label: '全部' },
  { key: 'draft', label: '草稿' },
  { key: 'publishing', label: '发布中' },
  { key: 'published', label: '已发布' },
  { key: 'failed', label: '失败' }
]

export function ArticlesPage({ onCreate, onEdit }: { onCreate: () => void; onEdit: (id: number) => void }) {
  const { articles, accounts, loading, error, notice, publishingID, reload, remove, publish } = useArticles(tenantID)
  const [filter, setFilter] = useState<FilterKey>('all')
  const [accountFilter, setAccountFilter] = useState('all')
  const accountsByID = accountByID(accounts)
  const accountScopedArticles = accountFilter === 'all' ? articles : articles.filter((article) => article.authorizer_id === Number(accountFilter))
  const counts = countByStatus(accountScopedArticles)
  const visible = filter === 'all' ? accountScopedArticles : accountScopedArticles.filter((article) => article.status === filter)

  return (
    <Page>
      <Header>
        <HeaderLeft>
          <Eyebrow>Articles</Eyebrow>
          <Title>文章列表</Title>
          <Description>查看租户下的公众号文章草稿、发布状态和最近更新时间。</Description>
        </HeaderLeft>
        <Button onClick={onCreate}><PlusIcon />新建文章</Button>
      </Header>

      <FilterTabs>
        {FILTER_TABS.map((tab) => (
          <FilterPill key={tab.key} data-active={filter === tab.key} onClick={() => setFilter(tab.key)}>
            <span>{tab.label}</span>
            <FilterCount data-active={filter === tab.key}>{counts[tab.key]}</FilterCount>
          </FilterPill>
        ))}
      </FilterTabs>

      <AccountFilterBar>
        <AccountFilterLabel htmlFor="article-account-filter">公众号</AccountFilterLabel>
        <AccountSelect id="article-account-filter" value={accountFilter} onChange={(event) => setAccountFilter(event.target.value)}>
          <option value="all">全部公众号</option>
          {accounts.map((account) => (
            <option key={account.id} value={account.id}>{accountOptionLabel(account)}</option>
          ))}
        </AccountSelect>
      </AccountFilterBar>

      {error ? <ErrorPanel message={error} onRetry={reload} /> : null}
      {notice ? <NoticePanel><NoticeLeft><NoticeIconWrap tone="info"><InfoIcon /></NoticeIconWrap><div><PanelTitle>{notice}</PanelTitle><PanelDesc>发布或删除结果会同步显示在列表和发布记录页。</PanelDesc></div></NoticeLeft></NoticePanel> : null}

      {!loading && visible.length === 0 ? (
        <EmptyState onCreate={onCreate} />
      ) : (
        <Panel>
          <Toolbar>
            <Summary>{loading ? '正在加载文章…' : `共 ${visible.length} 篇文章`}</Summary>
            <Button variant="secondary" onClick={reload}><RefreshIcon />刷新</Button>
          </Toolbar>
          {loading ? <LoadingRows /> : <ArticleList accountsByID={accountsByID} articles={visible} onDelete={remove} onEdit={onEdit} onPublish={publish} publishingID={publishingID} />}
        </Panel>
      )}
    </Page>
  )
}

function useArticles(currentTenantID: string) {
  const [articles, setArticles] = useState<Article[]>([])
  const [accounts, setAccounts] = useState<Account[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [publishingID, setPublishingID] = useState<number | null>(null)
  const [version, setVersion] = useState(0)

  useEffect(() => {
    let active = true
    setLoading(true)
    setError('')
    setNotice('')

    Promise.all([listArticles(currentTenantID), listAccounts(currentTenantID)])
      .then(([articleItems, accountItems]) => {
        if (!active) return
        setArticles(articleItems)
        setAccounts(accountItems)
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
  }, [currentTenantID, version])

  async function remove(article: Article) {
    const confirmed = window.confirm(deleteConfirmText(article))
    if (!confirmed) return

    try {
      await deleteArticle(article.id, currentTenantID)
      setArticles((items) => items.filter((item) => item.id !== article.id))
      setNotice(article.status === 'published' ? `「${article.title}」已删除，本地记录和公众号发布内容已同步处理。` : `「${article.title}」已删除。`)
    } catch (err: unknown) {
      setError(getErrorMessage(err))
    }
  }

  async function publish(article: Article) {
    if (!canPublishArticle(article)) return
    try {
      setError('')
      setNotice('')
      setPublishingID(article.id)
      await publishArticle(article.id, currentTenantID)
      setArticles((items) => items.map((item) => (item.id === article.id ? { ...item, status: 'publishing' } : item)))
      setNotice(article.status === 'published' ? `「${article.title}」的修订版已提交发布。` : `「${article.title}」已提交发布。`)
    } catch (err: unknown) {
      setError(getErrorMessage(err))
    } finally {
      setPublishingID(null)
    }
  }

  return { articles, accounts, loading, error, notice, publishingID, reload: () => setVersion((value) => value + 1), remove, publish }
}

function ArticleList({ accountsByID, articles, onDelete, onEdit, onPublish, publishingID }: { accountsByID: Map<number, Account>; articles: Article[]; onDelete: (article: Article) => void; onEdit: (id: number) => void; onPublish: (article: Article) => void; publishingID: number | null }) {
  if (articles.length === 0) return null

  return (
    <ArticleCards>
      {articles.map((article) => {
        const tone = statusTone(article.status)
        const account = accountsByID.get(article.authorizer_id)
        return (
          <ArticleCard key={article.id}>
            <StatusStrip tone={tone} />
            <ArticleBody>
              <ArticleHead>
                <ArticleText>
                  <ArticleTitle>{article.title}</ArticleTitle>
                  <Digest>{article.digest || '暂无摘要'}</Digest>
                </ArticleText>
                <ArticleStatusBadge status={article.status} />
              </ArticleHead>
              <ArticleFoot>
                <FootMeta><AccountDot />{accountDisplayName(account, article.authorizer_id)}</FootMeta>
                <FootMeta><UserIcon />{article.author || '—'}</FootMeta>
                <FootMeta><KeyGlyph />{article.authorizer_id}</FootMeta>
                <FootMeta><ClockIcon />{formatTime(article.updated_at)}</FootMeta>
                <RowActions>
                  <Button variant="ghost" onClick={() => onEdit(article.id)}><EditIcon />编辑</Button>
                  {canPublishArticle(article) ? (
                    <Button disabled={publishingID === article.id || article.status === 'publishing'} variant="ghost" onClick={() => onPublish(article)}><SendIcon />{publishActionText(article, publishingID === article.id)}</Button>
                  ) : null}
                  <Button disabled={article.status === 'publishing'} variant="danger" onClick={() => onDelete(article)}><TrashIcon />删除</Button>
                </RowActions>
              </ArticleFoot>
            </ArticleBody>
          </ArticleCard>
        )
      })}
    </ArticleCards>
  )
}

function canPublishArticle(article: Article) {
  return article.status !== 'publishing'
}

function publishActionText(article: Article, submitting: boolean) {
  if (submitting) return '提交中…'
  if (article.status === 'published') return '发布修订版'
  return '发布'
}

function deleteConfirmText(article: Article) {
  if (article.status === 'published') {
    return `确定删除「${article.title}」吗？这会同时删除公众号里已发布的内容和本地文章记录，且不可恢复。`
  }
  if (article.status === 'publishing') {
    return `「${article.title}」正在发布中，暂不支持删除。`
  }
  return `确定删除「${article.title}」吗？此操作不可撤销。`
}

function KeyGlyph() { return <svg {...svgAttrs} width="15" height="15"><path d="M21 2l-2 2m-7.61 7.61a5.5 5.5 0 1 1-7.778 7.778 5.5 5.5 0 0 1 7.777-7.777zm0 0L15.5 7.5m0 0l3 3L22 7l-3-3m-3.5 3.5L19 4" /></svg> }
function AccountDot() { return <svg {...svgAttrs} width="15" height="15"><rect x="3" y="4" width="18" height="16" rx="2" /><path d="M7 8h10" /><path d="M7 12h6" /><path d="M7 16h8" /></svg> }

function statusTone(status: ArticleStatus): Tone {
  switch (status) {
    case 'published': return 'success'
    case 'publishing': return 'warning'
    case 'failed': return 'danger'
    case 'draft': return 'muted'
  }
}

function countByStatus(articles: Article[]): Record<FilterKey, number> {
  const counts: Record<FilterKey, number> = { all: articles.length, draft: 0, publishing: 0, published: 0, failed: 0 }
  for (const a of articles) {
    if (a.status === 'draft') counts.draft += 1
    else if (a.status === 'publishing') counts.publishing += 1
    else if (a.status === 'published') counts.published += 1
    else if (a.status === 'failed') counts.failed += 1
  }
  return counts
}

function ArticleStatusBadge({ status }: { status: ArticleStatus }) {
  switch (status) {
    case 'published':
      return <StatusBadge tone="success">已发布</StatusBadge>
    case 'publishing':
      return <StatusBadge tone="warning">发布中</StatusBadge>
    case 'failed':
      return <StatusBadge tone="danger">失败</StatusBadge>
    case 'draft':
      return <StatusBadge tone="muted">草稿</StatusBadge>
  }
}

function LoadingRows() {
  return (
    <SkeletonList aria-label="正在加载文章">
      {Array.from({ length: 4 }).map((_, index) => <SkeletonRow key={index} />)}
    </SkeletonList>
  )
}

function EmptyState({ onCreate }: { onCreate: () => void }) {
  return (
    <EmptyPanel>
      <EmptyIcon><FileLargeIcon /></EmptyIcon>
      <PanelTitle>暂无文章</PanelTitle>
      <PanelDesc>创建第一篇文章后，可以在这里查看草稿、发布状态和失败记录。</PanelDesc>
      <Button onClick={onCreate}><PlusIcon />新建文章</Button>
    </EmptyPanel>
  )
}

function ErrorPanel({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <NoticePanel role="alert">
      <NoticeLeft><NoticeIconWrap tone="danger"><AlertIcon /></NoticeIconWrap><div><PanelTitle>文章加载失败</PanelTitle><PanelDesc>{message}</PanelDesc></div></NoticeLeft>
      <Button variant="secondary" onClick={onRetry}>重试</Button>
    </NoticePanel>
  )
}

function formatTime(value: string) {
  if (!value) return '—'
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  }).format(new Date(value))
}

const Page = styled.div`
  display: grid;
  gap: ${({ theme }) => theme.space.xl};
`

const Header = styled.div`
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: ${({ theme }) => theme.space.xl};
  flex-wrap: wrap;
`

const HeaderLeft = styled.div`
  min-width: 0;
`

const Eyebrow = styled.p`
  margin: 0 0 ${({ theme }) => theme.space.sm};
  color: ${({ theme }) => theme.colors.primary};
  font-size: ${({ theme }) => theme.typeScale.small};
  font-weight: 750;
  letter-spacing: 0.03em;
`

const Title = styled.h1`
  margin: 0;
  color: ${({ theme }) => theme.colors.text};
  font-size: clamp(1.75rem, 3vw, 2.5rem);
  letter-spacing: -0.04em;
  line-height: 1.08;
  font-weight: 800;
`

const Description = styled.p`
  max-width: 760px;
  margin: ${({ theme }) => theme.space.md} 0 0;
  color: ${({ theme }) => theme.colors.textMuted};
  font-size: ${({ theme }) => theme.typeScale.lead};
`

const FilterTabs = styled.div`
  display: flex;
  flex-wrap: wrap;
  gap: ${({ theme }) => theme.space.sm};
`

const FilterPill = styled.button`
  display: inline-flex;
  align-items: center;
  gap: ${({ theme }) => theme.space.sm};
  min-height: 38px;
  border: 1px solid ${({ theme }) => theme.colors.border};
  border-radius: ${({ theme }) => theme.radii.pill};
  background: ${({ theme }) => theme.colors.surface};
  color: ${({ theme }) => theme.colors.textMuted};
  font-size: ${({ theme }) => theme.typeScale.small};
  font-weight: 650;
  padding: 0 ${({ theme }) => theme.space.md};
  cursor: pointer;
  transition: all ${({ theme }) => theme.motion.fast} ${({ theme }) => theme.motion.easeOut};

  &:hover {
    background: ${({ theme }) => theme.colors.surfaceMuted};
    color: ${({ theme }) => theme.colors.text};
  }

  &[data-active='true'] {
    border-color: ${({ theme }) => theme.colors.primary};
    background: ${({ theme }) => theme.colors.primarySoft};
    color: ${({ theme }) => theme.colors.primaryStrong};
  }
`

const FilterCount = styled.span`
  display: grid;
  place-items: center;
  min-width: 22px;
  height: 22px;
  padding: 0 ${({ theme }) => theme.space.xs};
  border-radius: ${({ theme }) => theme.radii.pill};
  background: ${({ theme }) => theme.colors.surfaceMuted};
  color: ${({ theme }) => theme.colors.textMuted};
  font-size: ${({ theme }) => theme.typeScale.caption};
  font-weight: 700;
  font-variant-numeric: tabular-nums;

  &[data-active='true'] {
    background: ${({ theme }) => theme.colors.primary};
    color: ${({ theme }) => theme.colors.surface};
  }
`

const AccountFilterBar = styled.div`
  display: flex;
  align-items: center;
  gap: ${({ theme }) => theme.space.md};
  flex-wrap: wrap;
`

const AccountFilterLabel = styled.label`
  color: ${({ theme }) => theme.colors.text};
  font-size: ${({ theme }) => theme.typeScale.small};
  font-weight: 750;
`

const AccountSelect = styled.select`
  min-width: min(360px, 100%);
  min-height: 40px;
  border: 1px solid ${({ theme }) => theme.colors.border};
  border-radius: ${({ theme }) => theme.radii.md};
  background: ${({ theme }) => theme.colors.surface};
  color: ${({ theme }) => theme.colors.text};
  font: inherit;
  padding: 0 ${({ theme }) => theme.space.md};

  &:focus {
    outline: 3px solid ${({ theme }) => theme.colors.primarySoft};
    border-color: ${({ theme }) => theme.colors.primary};
  }
`

const Panel = styled(Card)`
  overflow: hidden;
`

const Toolbar = styled.div`
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: ${({ theme }) => theme.space.lg};
  border-bottom: 1px solid ${({ theme }) => theme.colors.border};
  padding: ${({ theme }) => theme.space.lg} ${({ theme }) => theme.space.xl};

  button {
    display: inline-flex;
    align-items: center;
    gap: ${({ theme }) => theme.space.xs};
  }
`

const Summary = styled.div`
  color: ${({ theme }) => theme.colors.textMuted};
  font-weight: 650;
`

const ArticleCards = styled.div`
  display: grid;
  padding: ${({ theme }) => theme.space.lg};
  gap: ${({ theme }) => theme.space.md};
`

const ArticleCard = styled.div`
  display: flex;
  border: 1px solid ${({ theme }) => theme.colors.border};
  border-radius: ${({ theme }) => theme.radii.lg};
  background: ${({ theme }) => theme.colors.surface};
  box-shadow: ${({ theme }) => theme.shadows.xs};
  overflow: hidden;
  transition: transform ${({ theme }) => theme.motion.base} ${({ theme }) => theme.motion.easeOut}, box-shadow ${({ theme }) => theme.motion.base} ${({ theme }) => theme.motion.easeOut};

  &:hover {
    transform: translateY(-2px);
    box-shadow: ${({ theme }) => theme.shadows.soft};
  }
`

const StatusStrip = styled.div<{ tone: Tone }>`
  flex: 0 0 4px;
  background: ${({ theme, tone }) => (theme.colors as Record<string, string>)[toneColor[tone]]};
`

const ArticleBody = styled.div`
  display: grid;
  gap: ${({ theme }) => theme.space.lg};
  padding: ${({ theme }) => theme.space.lg} ${({ theme }) => theme.space.xl};
  min-width: 0;
  flex: 1;
`

const ArticleHead = styled.div`
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: ${({ theme }) => theme.space.lg};
`

const ArticleText = styled.div`
  min-width: 0;
`

const ArticleTitle = styled.div`
  font-weight: 750;
  font-size: 1rem;
  letter-spacing: -0.01em;
`

const Digest = styled.div`
  max-width: 64ch;
  margin-top: ${({ theme }) => theme.space.xs};
  color: ${({ theme }) => theme.colors.textMuted};
  font-size: ${({ theme }) => theme.typeScale.small};
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
`

const ArticleFoot = styled.div`
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: ${({ theme }) => theme.space.lg};
`

const FootMeta = styled.span`
  display: inline-flex;
  align-items: center;
  gap: ${({ theme }) => theme.space.xs};
  color: ${({ theme }) => theme.colors.textMuted};
  font-size: ${({ theme }) => theme.typeScale.small};

  svg {
    color: ${({ theme }) => theme.colors.textFaint};
  }
`

const RowActions = styled.div`
  display: flex;
  flex-wrap: wrap;
  gap: ${({ theme }) => theme.space.sm};
  margin-left: auto;

  button {
    display: inline-flex;
    align-items: center;
    gap: ${({ theme }) => theme.space.xs};
  }
`

const EmptyPanel = styled.div`
  display: grid;
  gap: ${({ theme }) => theme.space.md};
  justify-items: center;
  padding: ${({ theme }) => theme.space['3xl']} ${({ theme }) => theme.space.xl};
  text-align: center;
`

const EmptyIcon = styled.div`
  color: ${({ theme }) => theme.colors.textFaint};
  opacity: 0.6;
`

const NoticePanel = styled(Card)`
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: ${({ theme }) => theme.space.lg};
  padding: ${({ theme }) => theme.space.xl};
  border-left: 3px solid ${({ theme }) => theme.colors.info};
`

const NoticeLeft = styled.div`
  display: flex;
  align-items: center;
  gap: ${({ theme }) => theme.space.lg};
`

const NoticeIconWrap = styled.span<{ tone: Tone }>`
  display: grid;
  width: 40px;
  height: 40px;
  place-items: center;
  border-radius: ${({ theme }) => theme.radii.md};
  background: ${({ theme, tone }) => (theme.colors as Record<string, string>)[tone === 'danger' ? 'dangerSoft' : 'infoSoft']};
  color: ${({ theme, tone }) => (theme.colors as Record<string, string>)[toneColor[tone]]};
  flex-shrink: 0;
`

const PanelTitle = styled.h2`
  margin: 0;
  font-size: ${({ theme }) => theme.typeScale.title};
  letter-spacing: -0.02em;
  font-weight: 700;
`

const PanelDesc = styled.p`
  margin: ${({ theme }) => theme.space.sm} 0 0;
  color: ${({ theme }) => theme.colors.textMuted};
`

const SkeletonList = styled.div`
  display: grid;
  padding: ${({ theme }) => theme.space.lg};
  gap: ${({ theme }) => theme.space.md};
`

const SkeletonRow = styled.div`
  height: 92px;
  border-radius: ${({ theme }) => theme.radii.lg};
  background: linear-gradient(90deg, ${({ theme }) => theme.colors.surfaceMuted}, ${({ theme }) => theme.colors.primarySoft}, ${({ theme }) => theme.colors.surfaceMuted});
  background-size: 220% 100%;
  animation: shimmer 1.2s linear infinite;

  @keyframes shimmer {
    from { background-position: 220% 0; }
    to { background-position: -220% 0; }
  }
`
