import { useEffect, useState } from 'react'
import styled from '@emotion/styled'
import { listAccounts, type Account, type AccountStatus } from '../api/accounts'
import { getErrorMessage } from '../api/client'
import { Button } from '../components/Button'
import { Card } from '../components/Card'
import { StatusBadge } from '../components/StatusBadge'

const tenantID = 'tenant-1'
const defaultPublicBaseURL = 'https://example.com'
const defaultComponentAppID = 'wx0000000000000000'

const svgAttrs = { viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', strokeWidth: 2, strokeLinecap: 'round' as const, strokeLinejoin: 'round' as const }

function RefreshIcon() { return <svg {...svgAttrs} width="16" height="16"><polyline points="23 4 23 10 17 10" /><polyline points="1 20 1 14 7 14" /><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15" /></svg> }
function PlusIcon() { return <svg {...svgAttrs} width="16" height="16"><line x1="12" y1="5" x2="12" y2="19" /><line x1="5" y1="12" x2="19" y2="12" /></svg> }
function ClockIcon() { return <svg {...svgAttrs} width="15" height="15"><circle cx="12" cy="12" r="10" /><polyline points="12 6 12 12 16 14" /></svg> }
function FingerprintIcon() { return <svg {...svgAttrs} width="15" height="15"><path d="M12 11c0 1 .5 6-2 9" /><path d="M8 11c0 2 1 4 1 6" /><path d="M16 11c0 3-.5 6-1 7" /><path d="M5 11a7 7 0 0 1 14 0" /><path d="M19 11c0 1.5-1 3-1 5" /></svg> }
function KeyGlyph() { return <svg {...svgAttrs} width="15" height="15"><path d="M21 2l-2 2m-7.61 7.61a5.5 5.5 0 1 1-7.778 7.778 5.5 5.5 0 0 1 7.777-7.777zm0 0L15.5 7.5m0 0l3 3L22 7l-3-3m-3.5 3.5L19 4" /></svg> }
function UsersLargeIcon() { return <svg {...svgAttrs} width="64" height="64"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" /><circle cx="9" cy="7" r="4" /><path d="M23 21v-2a4 4 0 0 0-3-3.87" /><path d="M16 3.13a4 4 0 0 1 0 7.75" /></svg> }
function AlertIcon() { return <svg {...svgAttrs} width="20" height="20"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" /><line x1="12" y1="9" x2="12" y2="13" /><line x1="12" y1="17" x2="12.01" y2="17" /></svg> }

export function AccountsPage() {
  const { accounts, loading, error, reload } = useAccounts(tenantID)
  const [showAuthorizePanel, setShowAuthorizePanel] = useState(false)

  return (
    <Page>
      <Header>
        <HeaderLeft>
          <Eyebrow>Accounts</Eyebrow>
          <Title>账号管理</Title>
          <Description>查看已授权公众号及其可用状态。这里只展示账号元信息，不展示任何 token、secret 或 refresh 字段。</Description>
        </HeaderLeft>
        <HeaderActions>
          <Button variant="secondary" onClick={reload}><RefreshIcon />刷新</Button>
          <Button onClick={() => setShowAuthorizePanel((value) => !value)}><PlusIcon />{showAuthorizePanel ? '收起授权' : '添加公众号'}</Button>
        </HeaderActions>
      </Header>

      {error ? <ErrorPanel message={error} onRetry={reload} /> : null}
      {showAuthorizePanel ? <AuthorizePanel /> : null}

      {!loading && accounts.length === 0 ? (
        <EmptyState onAdd={() => setShowAuthorizePanel(true)} />
      ) : (
        <AccountSection>
          <SectionToolbar>
            <Summary>{loading ? '正在加载账号…' : `共 ${accounts.length} 个授权账号`}</Summary>
            <StatusBadge tone="info">tenant-1</StatusBadge>
          </SectionToolbar>
          {loading ? <LoadingGrid /> : <AccountGrid accounts={accounts} />}
        </AccountSection>
      )}
    </Page>
  )
}

function useAccounts(currentTenantID: string) {
  const [accounts, setAccounts] = useState<Account[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [version, setVersion] = useState(0)

  useEffect(() => {
    let active = true
    setLoading(true)
    setError('')

    listAccounts(currentTenantID)
      .then((items) => {
        if (!active) return
        setAccounts(items)
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

  return { accounts, loading, error, reload: () => setVersion((value) => value + 1) }
}

function AuthorizePanel() {
  const [baseURL, setBaseURL] = useState(defaultPublicBaseURL)
  const [componentAppID, setComponentAppID] = useState(defaultComponentAppID)
  const [entryURL, setEntryURL] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  function authorize() {
    const publicBaseURL = normalizePublicBaseURL(baseURL)
    if (!publicBaseURL) {
      setError('请先填写公网服务域名。')
      return
    }
    if (!componentAppID.trim()) {
      setError('请先填写 Component AppID。')
      return
    }
    setLoading(true)
    setError('')
    setEntryURL('')

    try {
      const nextEntryURL = buildAuthorizationEntryURL(publicBaseURL, componentAppID)
      setEntryURL(nextEntryURL)
      window.open(nextEntryURL, '_blank', 'noopener,noreferrer')
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : '授权入口地址生成失败，请检查公网服务域名。')
    } finally {
      setLoading(false)
    }
  }

  return (
    <AuthorizeCard>
      <AuthorizeCopy>
        <PanelTitle>添加公众号</PanelTitle>
        <PanelDesc>普通公众号管理员只需要在微信官方页面扫码确认授权。这里会先打开公网授权入口页，再跳转到微信官方授权页。</PanelDesc>
      </AuthorizeCopy>
      <AuthorizeForm>
        <Field>
          <Label htmlFor="authorize-base-url">公网服务域名</Label>
          <Input id="authorize-base-url" value={baseURL} onChange={(event) => setBaseURL(event.target.value)} />
          <Meta>当前测试域名：https://example.com</Meta>
        </Field>
        <Field>
          <Label htmlFor="authorize-component-appid">Component AppID</Label>
          <Input id="authorize-component-appid" value={componentAppID} onChange={(event) => setComponentAppID(event.target.value)} placeholder="wx_component_appid" />
          <Meta>不要填写 AppSecret、token 或 refresh token。</Meta>
        </Field>
        <Button disabled={loading} onClick={authorize}>{loading ? '打开中…' : '去微信扫码授权'}</Button>
      </AuthorizeForm>
      {error ? <InlineError role="alert">{error}</InlineError> : null}
      {entryURL ? (
        <ResultBox>
          <URLValue>{entryURL}</URLValue>
          <Button variant="secondary" onClick={() => window.open(entryURL, '_blank', 'noopener,noreferrer')}>重新打开授权入口</Button>
        </ResultBox>
      ) : null}
    </AuthorizeCard>
  )
}

function normalizePublicBaseURL(value: string) {
  const trimmed = value.trim().replace(/\/+$/, '')
  if (!trimmed) return ''
  if (/^https?:\/\//i.test(trimmed)) return trimmed
  return `https://${trimmed}`
}

function buildAuthorizationEntryURL(baseURL: string, componentAppID: string) {
  const url = new URL('/wechat-authorize.html', `${baseURL}/`)
  url.searchParams.set('tenant_id', tenantID)
  url.searchParams.set('component_appid', componentAppID.trim())
  return url.toString()
}

function AccountGrid({ accounts }: { accounts: Account[] }) {
  if (accounts.length === 0) return null

  return (
    <Cards>
      {accounts.map((account) => (
        <AccountCard key={account.id}>
          <CardTop>
            <AccountIdentity account={account} />
            <AccountStatusBadge status={account.status} />
          </CardTop>
          <CardMeta>
            <MetaRow><FingerprintIcon /><MetaLabel>AppID</MetaLabel><MetaValue>{account.app_id}</MetaValue></MetaRow>
            <MetaRow><ClockIcon /><MetaLabel>最近同步</MetaLabel><MetaValue>{formatTime(account.last_synced_at)}</MetaValue></MetaRow>
            <MetaRow><KeyGlyph /><MetaLabel>Token 缓存</MetaLabel><MetaValue>后续接入</MetaValue></MetaRow>
          </CardMeta>
        </AccountCard>
      ))}
    </Cards>
  )
}

function AccountIdentity({ account }: { account: Account }) {
  const name = account.name || '未命名公众号'
  return (
    <Identity>
      {account.avatar_url ? (
        <Avatar src={account.avatar_url} alt="" />
      ) : (
        <AvatarPlaceholder>{name.slice(0, 1)}</AvatarPlaceholder>
      )}
      <IdentityText>
        <Name>{name}</Name>
        <Meta>ID：{account.id}</Meta>
      </IdentityText>
    </Identity>
  )
}

function AccountStatusBadge({ status }: { status: AccountStatus }) {
  switch (status) {
    case 'active':
      return <StatusBadge tone="success">可用</StatusBadge>
    case 'revoked':
      return <StatusBadge tone="danger">已取消授权</StatusBadge>
    case 'refresh_failed':
      return <StatusBadge tone="danger">Token 刷新失败</StatusBadge>
    case 'unknown':
      return <StatusBadge tone="muted">未知</StatusBadge>
    default:
      return <StatusBadge tone="muted">未知</StatusBadge>
  }
}

function LoadingGrid() {
  return (
    <SkeletonGrid aria-label="正在加载账号">
      {Array.from({ length: 4 }).map((_, index) => <SkeletonCard key={index} />)}
    </SkeletonGrid>
  )
}

function EmptyState({ onAdd }: { onAdd: () => void }) {
  return (
    <EmptyPanel>
      <EmptyIcon><UsersLargeIcon /></EmptyIcon>
      <PanelTitle>暂无授权公众号</PanelTitle>
      <PanelDesc>点击下方按钮，公众号管理员在微信官方页面扫码确认后即可授权。</PanelDesc>
      <Button onClick={onAdd}><PlusIcon />添加公众号</Button>
    </EmptyPanel>
  )
}

function ErrorPanel({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <NoticePanel role="alert">
      <NoticeLeft><NoticeIconWrap><AlertIcon /></NoticeIconWrap><div><PanelTitle>账号加载失败</PanelTitle><PanelDesc>{message}</PanelDesc></div></NoticeLeft>
      <Button variant="secondary" onClick={onRetry}>重试</Button>
    </NoticePanel>
  )
}

function formatTime(value: string) {
  if (!value || value.startsWith('0001-')) return '—'
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
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

const HeaderActions = styled.div`
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: ${({ theme }) => theme.space.sm};

  button {
    display: inline-flex;
    align-items: center;
    gap: ${({ theme }) => theme.space.xs};
  }
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

const AccountSection = styled(Card)`
  overflow: hidden;
`

const SectionToolbar = styled.div`
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: ${({ theme }) => theme.space.lg};
  border-bottom: 1px solid ${({ theme }) => theme.colors.border};
  padding: ${({ theme }) => theme.space.lg} ${({ theme }) => theme.space.xl};
`

const Summary = styled.div`
  color: ${({ theme }) => theme.colors.textMuted};
  font-weight: 650;
`

const AuthorizeCard = styled(Card)`
  display: grid;
  gap: ${({ theme }) => theme.space.lg};
  padding: ${({ theme }) => theme.space.xl};
`

const AuthorizeCopy = styled.div`
  max-width: 860px;
`

const AuthorizeForm = styled.div`
  display: grid;
  align-items: end;
  gap: ${({ theme }) => theme.space.lg};

  @media (min-width: 920px) {
    grid-template-columns: minmax(220px, 1fr) minmax(220px, 1fr) auto;
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
`

const InlineError = styled.div`
  border: 1px solid ${({ theme }) => theme.colors.dangerSoft};
  border-radius: ${({ theme }) => theme.radii.md};
  background: ${({ theme }) => theme.colors.dangerSoft};
  color: ${({ theme }) => theme.colors.danger};
  font-weight: 650;
  padding: ${({ theme }) => theme.space.md} ${({ theme }) => theme.space.lg};
`

const ResultBox = styled.div`
  display: grid;
  gap: ${({ theme }) => theme.space.md};
  border: 1px solid ${({ theme }) => theme.colors.border};
  border-radius: ${({ theme }) => theme.radii.lg};
  padding: ${({ theme }) => theme.space.lg};
  background: ${({ theme }) => theme.colors.surfaceMuted};
`

const URLValue = styled.code`
  overflow-wrap: anywhere;
  color: ${({ theme }) => theme.colors.text};
  font-size: ${({ theme }) => theme.typeScale.small};
`

const Cards = styled.div`
  display: grid;
  gap: ${({ theme }) => theme.space.lg};
  padding: ${({ theme }) => theme.space.xl};

  @media (min-width: 640px) {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  @media (min-width: 1024px) {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
`

const AccountCard = styled.div`
  display: grid;
  gap: ${({ theme }) => theme.space.lg};
  border: 1px solid ${({ theme }) => theme.colors.border};
  border-radius: ${({ theme }) => theme.radii.lg};
  padding: ${({ theme }) => theme.space.xl};
  background: ${({ theme }) => theme.colors.surface};
  box-shadow: ${({ theme }) => theme.shadows.xs};
  transition: transform ${({ theme }) => theme.motion.base} ${({ theme }) => theme.motion.easeOut}, box-shadow ${({ theme }) => theme.motion.base} ${({ theme }) => theme.motion.easeOut};

  &:hover {
    transform: translateY(-2px);
    box-shadow: ${({ theme }) => theme.shadows.soft};
  }
`

const CardTop = styled.div`
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: ${({ theme }) => theme.space.lg};
`

const Identity = styled.div`
  display: flex;
  align-items: center;
  gap: ${({ theme }) => theme.space.md};
  min-width: 0;
`

const Avatar = styled.img`
  width: 48px;
  height: 48px;
  border-radius: ${({ theme }) => theme.radii.pill};
  background: ${({ theme }) => theme.colors.primarySoft};
  object-fit: cover;
  border: 2px solid ${({ theme }) => theme.colors.primarySoft};
  flex-shrink: 0;
`

const AvatarPlaceholder = styled.div`
  display: grid;
  width: 48px;
  height: 48px;
  place-items: center;
  border-radius: ${({ theme }) => theme.radii.pill};
  background: ${({ theme }) => theme.colors.primarySoft};
  color: ${({ theme }) => theme.colors.primaryStrong};
  font-weight: 800;
  font-size: ${({ theme }) => theme.typeScale.title};
  border: 2px solid ${({ theme }) => theme.colors.primarySoft};
  flex-shrink: 0;
`

const IdentityText = styled.div`
  min-width: 0;
`

const Name = styled.div`
  font-weight: 750;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
`

const Meta = styled.div`
  margin-top: ${({ theme }) => theme.space.xs};
  color: ${({ theme }) => theme.colors.textMuted};
  font-size: ${({ theme }) => theme.typeScale.small};
`

const CardMeta = styled.div`
  display: grid;
  gap: ${({ theme }) => theme.space.md};
  padding-top: ${({ theme }) => theme.space.lg};
  border-top: 1px solid ${({ theme }) => theme.colors.border};
`

const MetaRow = styled.div`
  display: flex;
  align-items: center;
  gap: ${({ theme }) => theme.space.sm};
  color: ${({ theme }) => theme.colors.textMuted};
  font-size: ${({ theme }) => theme.typeScale.small};

  svg {
    color: ${({ theme }) => theme.colors.textFaint};
    flex-shrink: 0;
  }
`

const MetaLabel = styled.span`
  color: ${({ theme }) => theme.colors.textFaint};
  min-width: 56px;
`

const MetaValue = styled.span`
  color: ${({ theme }) => theme.colors.text};
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
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

const NoticeIconWrap = styled.span`
  display: grid;
  width: 40px;
  height: 40px;
  place-items: center;
  border-radius: ${({ theme }) => theme.radii.md};
  background: ${({ theme }) => theme.colors.infoSoft};
  color: ${({ theme }) => theme.colors.info};
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

const SkeletonGrid = styled(Cards)`
  @media (min-width: 640px) {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  @media (min-width: 1024px) {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
`

const SkeletonCard = styled.div`
  height: 168px;
  border-radius: ${({ theme }) => theme.radii.lg};
  background: linear-gradient(90deg, ${({ theme }) => theme.colors.surfaceMuted}, ${({ theme }) => theme.colors.primarySoft}, ${({ theme }) => theme.colors.surfaceMuted});
  background-size: 220% 100%;
  animation: shimmer 1.2s linear infinite;

  @keyframes shimmer {
    from { background-position: 220% 0; }
    to { background-position: -220% 0; }
  }
`
