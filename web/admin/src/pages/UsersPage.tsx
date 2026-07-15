import { type FormEvent, useEffect, useState } from 'react'
import styled from '@emotion/styled'
import { createUser, generateUserToken, listUsers, revokeUserToken, type ManagedUser } from '../api/users'
import { getErrorMessage } from '../api/client'
import { Button } from '../components/Button'
import { Card } from '../components/Card'
import { StatusBadge } from '../components/StatusBadge'

const iconAttrs = { viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', strokeWidth: 2, strokeLinecap: 'round' as const, strokeLinejoin: 'round' as const }
function PlusIcon() { return <svg {...iconAttrs} width="16" height="16"><line x1="12" y1="5" x2="12" y2="19" /><line x1="5" y1="12" x2="19" y2="12" /></svg> }
function RefreshIcon() { return <svg {...iconAttrs} width="16" height="16"><polyline points="23 4 23 10 17 10" /><polyline points="1 20 1 14 7 14" /><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15" /></svg> }
function KeyIcon() { return <svg {...iconAttrs} width="16" height="16"><circle cx="7.5" cy="15.5" r="5.5" /><path d="M21 2l-9.6 9.6" /><path d="M15 2h6v6" /></svg> }
function CopyIcon() { return <svg {...iconAttrs} width="15" height="15"><rect x="9" y="9" width="13" height="13" rx="2" /><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" /></svg> }
function EyeIcon() { return <svg {...iconAttrs} width="17" height="17"><path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7-10-7-10-7z" /><circle cx="12" cy="12" r="3" /></svg> }
function EyeOffIcon() { return <svg {...iconAttrs} width="17" height="17"><path d="m2 2 20 20" /><path d="M10.58 10.58A2 2 0 0 0 12 14a2 2 0 0 0 1.42-.58" /><path d="M9.88 4.24A9.8 9.8 0 0 1 12 4c6.5 0 10 8 10 8a18.5 18.5 0 0 1-3.1 4.44" /><path d="M6.61 6.61C3.63 8.62 2 12 2 12s3.5 8 10 8a9.6 9.6 0 0 0 5.39-1.61" /></svg> }
function TrashIcon() { return <svg {...iconAttrs} width="15" height="15"><polyline points="3 6 5 6 21 6" /><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6" /></svg> }

type RevealedToken = { token: string; username: string }

export function UsersPage() {
  const [users, setUsers] = useState<ManagedUser[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showCreate, setShowCreate] = useState(false)
  const [busyUserID, setBusyUserID] = useState('')
  const [revealedToken, setRevealedToken] = useState<RevealedToken | null>(null)
  const [version, setVersion] = useState(0)

  useEffect(() => {
    let active = true
    setLoading(true)
    setError('')
    listUsers()
      .then((items) => { if (active) setUsers(items) })
      .catch((err: unknown) => { if (active) setError(getErrorMessage(err)) })
      .finally(() => { if (active) setLoading(false) })
    return () => { active = false }
  }, [version])

  async function rotateToken(user: ManagedUser) {
    if (user.api_token_configured && !window.confirm(`为「${user.username}」生成新 token 后，旧 token 会立即失效。确定继续吗？`)) return
    setBusyUserID(user.id)
    setError('')
    try {
      const generated = await generateUserToken(user.id)
      setUsers((items) => items.map((item) => item.id === user.id ? generated.user : item))
      setRevealedToken({ token: generated.token, username: user.username })
    } catch (err: unknown) {
      setError(getErrorMessage(err))
    } finally {
      setBusyUserID('')
    }
  }

  async function revokeToken(user: ManagedUser) {
    if (!window.confirm(`撤销「${user.username}」的 token 后，已连接的 agent 会立即失去访问权限。确定继续吗？`)) return
    setBusyUserID(user.id)
    setError('')
    try {
      const updated = await revokeUserToken(user.id)
      setUsers((items) => items.map((item) => item.id === user.id ? updated : item))
      setRevealedToken((current) => current?.username === user.username ? null : current)
    } catch (err: unknown) {
      setError(getErrorMessage(err))
    } finally {
      setBusyUserID('')
    }
  }

  function created(user: ManagedUser) {
    setUsers((items) => [...items, user].sort((a, b) => a.username.localeCompare(b.username)))
    setShowCreate(false)
  }

  return (
    <Page>
      <Header>
        <div>
          <Eyebrow>Access control</Eyebrow>
          <Title>用户管理</Title>
          <Description>创建独立用户，并管理每个用户连接 Agent 所需的 MCP token。</Description>
        </div>
        <HeaderActions>
          <Button variant="secondary" onClick={() => setVersion((value) => value + 1)}><RefreshIcon />刷新</Button>
          <Button onClick={() => setShowCreate((value) => !value)}><PlusIcon />{showCreate ? '收起' : '新建用户'}</Button>
        </HeaderActions>
      </Header>

      {showCreate ? <CreateUserPanel onCreated={created} /> : null}
      {revealedToken ? <TokenReveal token={revealedToken} onClose={() => setRevealedToken(null)} /> : null}
      {error ? <ErrorPanel role="alert">{error}</ErrorPanel> : null}

      <UserPanel>
        <PanelToolbar>
          <Summary>{loading ? '正在加载用户…' : `共 ${users.length} 个用户`}</Summary>
          <StatusBadge tone="info">管理员可见</StatusBadge>
        </PanelToolbar>
        {!loading && users.length === 0 ? <EmptyState>暂无用户</EmptyState> : null}
        <UserList>
          {users.map((user) => (
            <UserRow key={user.id}>
              <Identity>
                <Avatar>{user.username.slice(0, 1).toUpperCase()}</Avatar>
                <IdentityText>
                  <NameLine><UserName>{user.username}</UserName><StatusBadge tone={user.status === 'active' ? 'success' : 'muted'}>{user.status === 'active' ? '正常' : '已停用'}</StatusBadge></NameLine>
                  <UserID>{user.id}</UserID>
                </IdentityText>
              </Identity>
              <TokenState>
                <TokenLabel>API / MCP token</TokenLabel>
                <TokenMeta>{user.api_token_configured ? user.api_token_hint || '已配置' : '未配置'}</TokenMeta>
                {user.api_token_created_at ? <TokenTime>{formatTime(user.api_token_created_at)}</TokenTime> : null}
              </TokenState>
              <RowActions>
                <Button variant="secondary" disabled={busyUserID === user.id || user.status !== 'active'} onClick={() => rotateToken(user)}><KeyIcon />{user.api_token_configured ? '轮换' : '生成'}</Button>
                {user.api_token_configured ? <Button variant="danger" disabled={busyUserID === user.id} onClick={() => revokeToken(user)}><TrashIcon />撤销</Button> : null}
              </RowActions>
            </UserRow>
          ))}
        </UserList>
      </UserPanel>
    </Page>
  )
}

function CreateUserPanel({ onCreated }: { onCreated: (user: ManagedUser) => void }) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setSubmitting(true)
    setError('')
    try {
      onCreated(await createUser(username, password))
    } catch (err: unknown) {
      setError(getErrorMessage(err))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <CreatePanel onSubmit={submit}>
      <CreateCopy><PanelTitle>新建独立用户</PanelTitle><PanelDesc>用户名 3-64 个字符，密码至少 12 个字符。</PanelDesc></CreateCopy>
      <Field><Label>用户名</Label><Input id="new-user-name" autoComplete="off" value={username} onChange={(event) => setUsername(event.target.value)} /></Field>
      <Field><Label>初始密码</Label><Input id="new-user-password" type="password" autoComplete="new-password" value={password} onChange={(event) => setPassword(event.target.value)} /></Field>
      <CreateActions><Button type="submit" disabled={submitting || username.trim().length < 3 || password.length < 12}>{submitting ? '创建中…' : '创建用户'}</Button></CreateActions>
      {error ? <FormError role="alert">{error}</FormError> : null}
    </CreatePanel>
  )
}

function TokenReveal({ token, onClose }: { token: RevealedToken; onClose: () => void }) {
  const [visible, setVisible] = useState(false)
  const [copied, setCopied] = useState(false)
  async function copy() {
    await navigator.clipboard?.writeText(token.token)
    setCopied(true)
    window.setTimeout(() => setCopied(false), 1600)
  }
  return (
    <RevealPanel>
      <RevealHead><div><PanelTitle>{token.username} 的新 token</PanelTitle><PanelDesc>明文仅在本次生成后显示。离开页面前请交付给对应用户。</PanelDesc></div><CloseButton type="button" onClick={onClose} aria-label="关闭 token 展示">×</CloseButton></RevealHead>
      <SecretLine>
        <SecretInput readOnly type={visible ? 'text' : 'password'} value={token.token} aria-label="新 MCP token" />
        <IconButton type="button" onClick={() => setVisible((value) => !value)} aria-label={visible ? '隐藏 token' : '显示 token'}>{visible ? <EyeOffIcon /> : <EyeIcon />}</IconButton>
        <Button variant="secondary" onClick={copy}><CopyIcon />{copied ? '已复制' : '复制'}</Button>
      </SecretLine>
    </RevealPanel>
  )
}

function formatTime(value: string) {
  return new Intl.DateTimeFormat('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }).format(new Date(value))
}

const Page = styled.div`display: grid; gap: ${({ theme }) => theme.space.xl};`
const Header = styled.header`display: flex; align-items: flex-start; justify-content: space-between; gap: ${({ theme }) => theme.space.xl}; flex-wrap: wrap;`
const HeaderActions = styled.div`display: flex; gap: ${({ theme }) => theme.space.md}; flex-wrap: wrap;`
const Eyebrow = styled.p`margin: 0 0 ${({ theme }) => theme.space.sm}; color: ${({ theme }) => theme.colors.primary}; font-size: ${({ theme }) => theme.typeScale.small}; font-weight: 750;`
const Title = styled.h1`margin: 0; font-size: clamp(1.75rem, 3vw, 2.5rem); line-height: 1.08; font-weight: 800;`
const Description = styled.p`max-width: 720px; margin: ${({ theme }) => theme.space.md} 0 0; color: ${({ theme }) => theme.colors.textMuted}; font-size: ${({ theme }) => theme.typeScale.lead};`
const CreatePanel = styled.form`display: grid; grid-template-columns: minmax(180px, 1fr) minmax(180px, 1fr) minmax(220px, 1.2fr) auto; align-items: end; gap: ${({ theme }) => theme.space.lg}; border-block: 1px solid ${({ theme }) => theme.colors.border}; padding: ${({ theme }) => theme.space.xl} 0; @media (max-width: 900px) { grid-template-columns: 1fr 1fr; } @media (max-width: 620px) { grid-template-columns: 1fr; }`
const CreateCopy = styled.div`align-self: center;`
const Field = styled.label`display: grid; gap: ${({ theme }) => theme.space.sm};`
const Label = styled.span`color: ${({ theme }) => theme.colors.textMuted}; font-size: ${({ theme }) => theme.typeScale.small}; font-weight: 650;`
const Input = styled.input`min-width: 0; min-height: 44px; border: 1px solid ${({ theme }) => theme.colors.border}; border-radius: ${({ theme }) => theme.radii.sm}; padding: 0 ${({ theme }) => theme.space.md}; background: ${({ theme }) => theme.colors.surface}; color: ${({ theme }) => theme.colors.text}; &:focus { border-color: ${({ theme }) => theme.colors.primary}; box-shadow: 0 0 0 3px ${({ theme }) => theme.colors.primarySoft}; }`
const CreateActions = styled.div`display: flex;`
const FormError = styled.p`grid-column: 1 / -1; margin: 0; color: ${({ theme }) => theme.colors.danger}; font-size: ${({ theme }) => theme.typeScale.small};`
const PanelTitle = styled.h2`margin: 0; font-size: ${({ theme }) => theme.typeScale.lead}; font-weight: 750;`
const PanelDesc = styled.p`margin: ${({ theme }) => theme.space.xs} 0 0; color: ${({ theme }) => theme.colors.textMuted}; font-size: ${({ theme }) => theme.typeScale.small};`
const RevealPanel = styled(Card)`display: grid; gap: ${({ theme }) => theme.space.lg}; padding: ${({ theme }) => theme.space.xl}; border-color: ${({ theme }) => theme.colors.warning}; background: ${({ theme }) => theme.colors.warningSoft};`
const RevealHead = styled.div`display: flex; align-items: flex-start; justify-content: space-between; gap: ${({ theme }) => theme.space.lg};`
const CloseButton = styled.button`width: 36px; height: 36px; border: 0; background: transparent; color: ${({ theme }) => theme.colors.textMuted}; font-size: 1.5rem; line-height: 1;`
const SecretLine = styled.div`display: grid; grid-template-columns: minmax(0, 1fr) 44px auto; gap: ${({ theme }) => theme.space.sm}; @media (max-width: 620px) { grid-template-columns: minmax(0, 1fr) 44px; > button:last-child { grid-column: 1 / -1; } }`
const SecretInput = styled.input`min-width: 0; min-height: 44px; border: 1px solid ${({ theme }) => theme.colors.border}; border-radius: ${({ theme }) => theme.radii.sm}; padding: 0 ${({ theme }) => theme.space.md}; background: ${({ theme }) => theme.colors.surface}; color: ${({ theme }) => theme.colors.text}; font-family: ui-monospace, SFMono-Regular, Menlo, monospace;`
const IconButton = styled.button`display: grid; width: 44px; height: 44px; place-items: center; border: 1px solid ${({ theme }) => theme.colors.border}; border-radius: ${({ theme }) => theme.radii.sm}; background: ${({ theme }) => theme.colors.surface}; color: ${({ theme }) => theme.colors.textMuted}; &:hover { color: ${({ theme }) => theme.colors.text}; }`
const ErrorPanel = styled.div`border: 1px solid ${({ theme }) => theme.colors.danger}; border-radius: ${({ theme }) => theme.radii.sm}; padding: ${({ theme }) => theme.space.md}; background: ${({ theme }) => theme.colors.dangerSoft}; color: ${({ theme }) => theme.colors.danger}; font-size: ${({ theme }) => theme.typeScale.small};`
const UserPanel = styled(Card)`overflow: hidden;`
const PanelToolbar = styled.div`display: flex; align-items: center; justify-content: space-between; gap: ${({ theme }) => theme.space.lg}; border-bottom: 1px solid ${({ theme }) => theme.colors.border}; padding: ${({ theme }) => theme.space.lg} ${({ theme }) => theme.space.xl};`
const Summary = styled.span`font-size: ${({ theme }) => theme.typeScale.small}; font-weight: 700;`
const UserList = styled.div`display: grid;`
const UserRow = styled.div`display: grid; grid-template-columns: minmax(220px, 1.2fr) minmax(180px, 1fr) auto; align-items: center; gap: ${({ theme }) => theme.space.xl}; padding: ${({ theme }) => theme.space.lg} ${({ theme }) => theme.space.xl}; &:not(:last-child) { border-bottom: 1px solid ${({ theme }) => theme.colors.border}; } @media (max-width: 760px) { grid-template-columns: 1fr; gap: ${({ theme }) => theme.space.md}; }`
const Identity = styled.div`display: flex; align-items: center; gap: ${({ theme }) => theme.space.md}; min-width: 0;`
const Avatar = styled.div`display: grid; width: 40px; height: 40px; flex: 0 0 40px; place-items: center; border-radius: ${({ theme }) => theme.radii.sm}; background: ${({ theme }) => theme.colors.primarySoft}; color: ${({ theme }) => theme.colors.primaryStrong}; font-weight: 800;`
const IdentityText = styled.div`min-width: 0;`
const NameLine = styled.div`display: flex; align-items: center; gap: ${({ theme }) => theme.space.sm}; flex-wrap: wrap;`
const UserName = styled.strong`font-size: ${({ theme }) => theme.typeScale.body};`
const UserID = styled.div`margin-top: 2px; overflow: hidden; color: ${({ theme }) => theme.colors.textMuted}; font-size: ${({ theme }) => theme.typeScale.caption}; text-overflow: ellipsis; white-space: nowrap;`
const TokenState = styled.div`display: grid; gap: 2px; min-width: 0;`
const TokenLabel = styled.span`color: ${({ theme }) => theme.colors.textMuted}; font-size: ${({ theme }) => theme.typeScale.caption};`
const TokenMeta = styled.strong`overflow: hidden; font-size: ${({ theme }) => theme.typeScale.small}; text-overflow: ellipsis; white-space: nowrap;`
const TokenTime = styled.span`color: ${({ theme }) => theme.colors.textFaint}; font-size: ${({ theme }) => theme.typeScale.caption};`
const RowActions = styled.div`display: flex; justify-content: flex-end; gap: ${({ theme }) => theme.space.sm}; flex-wrap: wrap; @media (max-width: 760px) { justify-content: flex-start; }`
const EmptyState = styled.div`padding: ${({ theme }) => theme.space['3xl']}; text-align: center; color: ${({ theme }) => theme.colors.textMuted};`
