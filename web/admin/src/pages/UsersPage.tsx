import { type FormEvent, useEffect, useMemo, useState } from 'react'
import styled from '@emotion/styled'
import {
  createAgent,
  listAgents,
  revokeAgentToken,
  rotateAgentToken,
  updateAgentStatus,
  type GeneratedAgentToken,
  type ManagedAgent
} from '../api/agents'
import { APIError, getErrorMessage } from '../api/client'
import { createUser, listUsers, type ManagedUser } from '../api/users'
import { Button } from '../components/Button'
import { Card } from '../components/Card'
import { StatusBadge } from '../components/StatusBadge'

const iconAttrs = { viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', strokeWidth: 2, strokeLinecap: 'round' as const, strokeLinejoin: 'round' as const }
function PlusIcon() { return <svg {...iconAttrs} width="16" height="16" aria-hidden="true"><line x1="12" y1="5" x2="12" y2="19" /><line x1="5" y1="12" x2="19" y2="12" /></svg> }
function RefreshIcon() { return <svg {...iconAttrs} width="16" height="16" aria-hidden="true"><polyline points="23 4 23 10 17 10" /><polyline points="1 20 1 14 7 14" /><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15" /></svg> }
function KeyIcon() { return <svg {...iconAttrs} width="16" height="16" aria-hidden="true"><circle cx="7.5" cy="15.5" r="5.5" /><path d="M21 2l-9.6 9.6" /><path d="M15 2h6v6" /></svg> }
function CopyIcon() { return <svg {...iconAttrs} width="15" height="15" aria-hidden="true"><rect x="9" y="9" width="13" height="13" rx="2" /><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" /></svg> }
function EyeIcon() { return <svg {...iconAttrs} width="17" height="17" aria-hidden="true"><path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7-10-7-10-7z" /><circle cx="12" cy="12" r="3" /></svg> }
function EyeOffIcon() { return <svg {...iconAttrs} width="17" height="17" aria-hidden="true"><path d="m2 2 20 20" /><path d="M10.58 10.58A2 2 0 0 0 12 14a2 2 0 0 0 1.42-.58" /><path d="M9.88 4.24A9.8 9.8 0 0 1 12 4c6.5 0 10 8 10 8a18.5 18.5 0 0 1-3.1 4.44" /><path d="M6.61 6.61C3.63 8.62 2 12 2 12s3.5 8 10 8a9.6 9.6 0 0 0 5.39-1.61" /></svg> }
function TrashIcon() { return <svg {...iconAttrs} width="15" height="15" aria-hidden="true"><polyline points="3 6 5 6 21 6" /><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6" /></svg> }
function PowerIcon() { return <svg {...iconAttrs} width="16" height="16" aria-hidden="true"><path d="M18.36 6.64a9 9 0 1 1-12.73 0" /><line x1="12" y1="2" x2="12" y2="12" /></svg> }
function ChevronIcon({ expanded }: { expanded: boolean }) { return <svg {...iconAttrs} width="18" height="18" aria-hidden="true" style={{ transform: expanded ? 'rotate(180deg)' : 'none' }}><polyline points="6 9 12 15 18 9" /></svg> }

type RevealedToken = {
  token: string
  username: string
  agentName: string
  agentRecordID: string
  reason: 'created' | 'rotated'
}

type PendingAgentAction = {
  userID: string
  agentRecordID: string
  kind: 'rotate' | 'revoke' | 'disable'
}

type AgentsByUser = Record<string, ManagedAgent[]>
type ErrorsByUser = Record<string, string>

export function UsersPage() {
  const [users, setUsers] = useState<ManagedUser[]>([])
  const [agentsByUser, setAgentsByUser] = useState<AgentsByUser>({})
  const [agentErrors, setAgentErrors] = useState<ErrorsByUser>({})
  const [expandedUserIDs, setExpandedUserIDs] = useState<string[]>([])
  const [creatingForUserID, setCreatingForUserID] = useState('')
  const [busyAgentKey, setBusyAgentKey] = useState('')
  const [pendingAgentAction, setPendingAgentAction] = useState<PendingAgentAction | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showCreateUser, setShowCreateUser] = useState(false)
  const [revealedToken, setRevealedToken] = useState<RevealedToken | null>(null)
  const [version, setVersion] = useState(0)

  useEffect(() => {
    let active = true

    async function load() {
      setLoading(true)
      setError('')
      try {
        const items = await listUsers()
        const results = await Promise.all(items.map(async (user) => {
          try {
            return { userID: user.id, agents: await listAgents(user.id), error: '' }
          } catch (loadError: unknown) {
            return { userID: user.id, agents: [] as ManagedAgent[], error: getErrorMessage(loadError) }
          }
        }))
        if (!active) return
        setUsers(items)
        setAgentsByUser(Object.fromEntries(results.map((result) => [result.userID, result.agents])))
        setAgentErrors(Object.fromEntries(results.filter((result) => result.error).map((result) => [result.userID, result.error])))
        setExpandedUserIDs((current) => {
          const available = new Set(items.map((user) => user.id))
          const retained = current.filter((userID) => available.has(userID))
          return retained.length > 0 || items.length === 0 ? retained : [items[0].id]
        })
      } catch (loadError: unknown) {
        if (active) setError(getErrorMessage(loadError))
      } finally {
        if (active) setLoading(false)
      }
    }

    void load()
    return () => { active = false }
  }, [version])

  const totalAgents = useMemo(
    () => Object.values(agentsByUser).reduce((total, items) => total + items.length, 0),
    [agentsByUser]
  )

  function toggleUser(userID: string) {
    setExpandedUserIDs((current) => current.includes(userID) ? current.filter((id) => id !== userID) : [...current, userID])
  }

  function openAgentForm(user: ManagedUser) {
    setPendingAgentAction(null)
    setExpandedUserIDs((current) => current.includes(user.id) ? current : [...current, user.id])
    setCreatingForUserID((current) => current === user.id ? '' : user.id)
  }

  function replaceAgent(userID: string, updated: ManagedAgent) {
    setAgentsByUser((current) => ({
      ...current,
      [userID]: (current[userID] ?? []).map((agent) => agent.id === updated.id ? updated : agent)
    }))
  }

  function agentCreated(user: ManagedUser, generated: GeneratedAgentToken) {
    setAgentsByUser((current) => ({
      ...current,
      [user.id]: [...(current[user.id] ?? []), generated.agent]
        .sort((left, right) => left.name.localeCompare(right.name, 'zh-CN'))
    }))
    setCreatingForUserID('')
    setRevealedToken({
      token: generated.token,
      username: user.username,
      agentName: generated.agent.name,
      agentRecordID: generated.agent.id,
      reason: 'created'
    })
  }

  async function rotateToken(user: ManagedUser, agent: ManagedAgent) {
    const key = `${user.id}:${agent.id}`
    setBusyAgentKey(key)
    setError('')
    try {
      const generated = await rotateAgentToken(user.id, agent.id)
      replaceAgent(user.id, generated.agent)
      setRevealedToken({
        token: generated.token,
        username: user.username,
        agentName: generated.agent.name,
        agentRecordID: generated.agent.id,
        reason: 'rotated'
      })
    } catch (actionError: unknown) {
      setError(getErrorMessage(actionError))
    } finally {
      setBusyAgentKey('')
    }
  }

  async function revokeToken(user: ManagedUser, agent: ManagedAgent) {
    const key = `${user.id}:${agent.id}`
    setBusyAgentKey(key)
    setError('')
    try {
      const updated = await revokeAgentToken(user.id, agent.id)
      replaceAgent(user.id, updated)
      setRevealedToken((current) => current?.agentRecordID === agent.id ? null : current)
    } catch (actionError: unknown) {
      setError(getErrorMessage(actionError))
    } finally {
      setBusyAgentKey('')
    }
  }

  async function toggleAgentStatus(user: ManagedUser, agent: ManagedAgent) {
    const nextStatus = agent.status === 'active' ? 'disabled' : 'active'
    const key = `${user.id}:${agent.id}`
    setBusyAgentKey(key)
    setError('')
    try {
      const updated = await updateAgentStatus(user.id, agent, nextStatus)
      replaceAgent(user.id, updated)
      if (nextStatus === 'disabled') {
        setRevealedToken((current) => current?.agentRecordID === agent.id ? null : current)
      }
    } catch (actionError: unknown) {
      setError(getErrorMessage(actionError))
    } finally {
      setBusyAgentKey('')
    }
  }

  function confirmAgentAction(user: ManagedUser, agent: ManagedAgent, kind: PendingAgentAction['kind']) {
    setPendingAgentAction(null)
    if (kind === 'rotate') {
      void rotateToken(user, agent)
    } else if (kind === 'revoke') {
      void revokeToken(user, agent)
    } else {
      void toggleAgentStatus(user, agent)
    }
  }

  function userCreated(user: ManagedUser) {
    setUsers((items) => [...items, user].sort((left, right) => left.username.localeCompare(right.username)))
    setAgentsByUser((current) => ({ ...current, [user.id]: [] }))
    setExpandedUserIDs((current) => [...current, user.id])
    setShowCreateUser(false)
    setCreatingForUserID(user.id)
  }

  return (
    <Page>
      <Header>
        <div>
          <Eyebrow>Access control</Eyebrow>
          <Title>用户与 Agent</Title>
          <Description>用户决定数据边界，每个 Agent 使用独立凭证和内容职责。</Description>
        </div>
        <HeaderActions>
          <Button variant="secondary" onClick={() => { setRevealedToken(null); setPendingAgentAction(null); setCreatingForUserID(''); setVersion((value) => value + 1) }}><RefreshIcon />刷新</Button>
          <Button onClick={() => setShowCreateUser((value) => !value)}><PlusIcon />{showCreateUser ? '收起' : '新建用户'}</Button>
        </HeaderActions>
      </Header>

      {showCreateUser ? <CreateUserPanel onCreated={userCreated} /> : null}
      {revealedToken ? <TokenReveal token={revealedToken} onClose={() => setRevealedToken(null)} /> : null}
      {error ? <ErrorPanel role="alert">{error}</ErrorPanel> : null}

      <UserPanel>
        <PanelToolbar>
          <Summary>{loading ? '正在加载身份…' : `${users.length} 个用户 · ${totalAgents} 个 Agent`}</Summary>
          <StatusBadge tone="info">仅管理员可见</StatusBadge>
        </PanelToolbar>
        {!loading && users.length === 0 ? <EmptyState><strong>暂无用户</strong><span>创建用户后，可继续为其添加独立 Agent。</span></EmptyState> : null}
        <UserList>
          {users.map((user) => {
            const agents = agentsByUser[user.id] ?? []
            const expanded = expandedUserIDs.includes(user.id)
            return (
              <UserGroup key={user.id} data-user-id={user.id}>
                <UserHeader>
                  <Identity>
                    <Avatar>{user.username.slice(0, 1).toUpperCase()}</Avatar>
                    <IdentityText>
                      <NameLine>
                        <UserName>{user.username}</UserName>
                        <StatusBadge tone={user.status === 'active' ? 'success' : 'muted'}>{user.status === 'active' ? '正常' : '已停用'}</StatusBadge>
                        <RoleLabel>{user.role === 'admin' ? '管理员' : '用户'}</RoleLabel>
                      </NameLine>
                      <UserID title={user.id}>{user.id}</UserID>
                    </IdentityText>
                  </Identity>
                  <AgentCount><strong>{agents.length}</strong><span>Agent</span></AgentCount>
                  <UserActions>
                    <Button variant="secondary" disabled={user.status !== 'active'} onClick={() => openAgentForm(user)}><PlusIcon />添加 Agent</Button>
                    <ExpandButton type="button" onClick={() => toggleUser(user.id)} aria-expanded={expanded} aria-label={expanded ? `收起 ${user.username} 的 Agent` : `展开 ${user.username} 的 Agent`} title={expanded ? '收起 Agent' : '展开 Agent'}>
                      <ChevronIcon expanded={expanded} />
                    </ExpandButton>
                  </UserActions>
                </UserHeader>

                {expanded ? (
                  <AgentArea>
                    {creatingForUserID === user.id ? <CreateAgentPanel user={user} onCancel={() => setCreatingForUserID('')} onCreated={(generated) => agentCreated(user, generated)} /> : null}
                    {agentErrors[user.id] ? <InlineError role="alert">{agentErrors[user.id]}</InlineError> : null}
                    {agents.length === 0 && creatingForUserID !== user.id && !agentErrors[user.id] ? <AgentEmpty>尚未创建 Agent</AgentEmpty> : null}
                    <AgentList>
                      {agents.map((agent) => {
                        const busy = busyAgentKey === `${user.id}:${agent.id}`
                        const pendingAction = pendingAgentAction?.userID === user.id && pendingAgentAction.agentRecordID === agent.id
                          ? pendingAgentAction.kind
                          : null
                        return (
                          <AgentRow key={agent.id} data-agent-record-id={agent.id}>
                            <AgentIdentity>
                              <AgentMark>{agent.name.slice(0, 1).toUpperCase()}</AgentMark>
                              <AgentIdentityText>
                                <AgentNameLine>
                                  <AgentName>{agent.name}</AgentName>
                                  <StatusBadge tone={agent.status === 'active' ? 'success' : 'muted'}>{agent.status === 'active' ? '运行中' : '已停用'}</StatusBadge>
                                </AgentNameLine>
                                <AgentExternalID title={agent.agent_id}>ID · {agent.agent_id}</AgentExternalID>
                              </AgentIdentityText>
                            </AgentIdentity>
                            <AgentPurpose>
                              <MetaLabel>内容职责</MetaLabel>
                              <PurposeText>{agent.purpose || '未填写'}</PurposeText>
                            </AgentPurpose>
                            <TokenState>
                              <MetaLabel>MCP Token</MetaLabel>
                              <TokenMeta>{agent.api_token_configured ? agent.api_token_hint || '已配置' : '未配置'}</TokenMeta>
                              <TokenTime>{agent.last_used_at ? `最近使用 ${formatTime(agent.last_used_at)}` : agent.api_token_created_at ? `创建于 ${formatTime(agent.api_token_created_at)}` : '尚未使用'}</TokenTime>
                            </TokenState>
                            <RowActions>
                              <Button variant="secondary" disabled={busy || user.status !== 'active' || agent.status !== 'active'} onClick={() => agent.api_token_configured ? setPendingAgentAction({ userID: user.id, agentRecordID: agent.id, kind: 'rotate' }) : void rotateToken(user, agent)}><KeyIcon />{agent.api_token_configured ? '轮换' : '生成'}</Button>
                              {agent.api_token_configured ? <Button variant="danger" disabled={busy} onClick={() => setPendingAgentAction({ userID: user.id, agentRecordID: agent.id, kind: 'revoke' })}><TrashIcon />撤销</Button> : null}
                              <Button variant="ghost" disabled={busy || user.status !== 'active'} onClick={() => agent.status === 'active' ? setPendingAgentAction({ userID: user.id, agentRecordID: agent.id, kind: 'disable' }) : void toggleAgentStatus(user, agent)}><PowerIcon />{agent.status === 'active' ? '停用' : '启用'}</Button>
                            </RowActions>
                            {pendingAction ? (
                              <AgentConfirmation role="group" aria-label={`${agent.name} 操作确认`}>
                                <ConfirmationText>
                                  {pendingAction === 'rotate' ? '轮换后，这个 Agent 的旧 Token 会立即失效。' : null}
                                  {pendingAction === 'revoke' ? '撤销后，这个 Agent 会立即失去 MCP 访问权限。' : null}
                                  {pendingAction === 'disable' ? '停用后请求会被拒绝；重新启用后原 Token 仍可使用。' : null}
                                </ConfirmationText>
                                <ConfirmationActions>
                                  <Button type="button" variant="ghost" onClick={() => setPendingAgentAction(null)}>取消</Button>
                                  <Button type="button" variant={pendingAction === 'rotate' ? 'primary' : 'danger'} onClick={() => confirmAgentAction(user, agent, pendingAction)}>
                                    {pendingAction === 'rotate' ? '确认轮换' : pendingAction === 'revoke' ? '确认撤销' : '确认停用'}
                                  </Button>
                                </ConfirmationActions>
                              </AgentConfirmation>
                            ) : null}
                          </AgentRow>
                        )
                      })}
                    </AgentList>
                  </AgentArea>
                ) : null}
              </UserGroup>
            )
          })}
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
      onCreated(await createUser(username.trim(), password))
    } catch (submitError: unknown) {
      setError(getErrorMessage(submitError))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <CreatePanel onSubmit={submit}>
      <CreateCopy><PanelTitle>新建独立用户</PanelTitle><PanelDesc>用户名 3-64 个字符，密码至少 12 个字符。</PanelDesc></CreateCopy>
      <Field><Label htmlFor="new-user-name">用户名</Label><Input id="new-user-name" autoComplete="off" minLength={3} maxLength={64} value={username} onChange={(event) => setUsername(event.target.value)} /></Field>
      <Field><Label htmlFor="new-user-password">初始密码</Label><Input id="new-user-password" type="password" autoComplete="new-password" minLength={12} value={password} onChange={(event) => setPassword(event.target.value)} /></Field>
      <CreateActions><Button type="submit" disabled={submitting || username.trim().length < 3 || password.length < 12}>{submitting ? '创建中…' : '创建用户'}</Button></CreateActions>
      {error ? <FormError role="alert">{error}</FormError> : null}
    </CreatePanel>
  )
}

function CreateAgentPanel({ user, onCancel, onCreated }: { user: ManagedUser; onCancel: () => void; onCreated: (generated: GeneratedAgentToken) => void }) {
  const [agentID, setAgentID] = useState('')
  const [name, setName] = useState('')
  const [purpose, setPurpose] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setSubmitting(true)
    setError('')
    try {
      onCreated(await createAgent(user.id, { agent_id: agentID.trim(), name: name.trim(), purpose: purpose.trim() }))
    } catch (submitError: unknown) {
      if (submitError instanceof APIError && submitError.code === 'conflict') {
        setError('这个 Agent ID 已被当前用户使用，请换一个。')
      } else {
        setError(getErrorMessage(submitError))
      }
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <AgentCreateForm onSubmit={submit}>
      <AgentCreateHeading><PanelTitle>添加 Agent</PanelTitle><PanelDesc>归属于 {user.username}</PanelDesc></AgentCreateHeading>
      <Field><Label htmlFor={`agent-id-${user.id}`}>Agent ID</Label><Input id={`agent-id-${user.id}`} maxLength={128} placeholder="writer-news" value={agentID} onChange={(event) => setAgentID(event.target.value)} /></Field>
      <Field><Label htmlFor={`agent-name-${user.id}`}>显示名称</Label><Input id={`agent-name-${user.id}`} maxLength={64} placeholder="行业资讯编辑" value={name} onChange={(event) => setName(event.target.value)} /></Field>
      <Field><Label htmlFor={`agent-purpose-${user.id}`}>内容职责</Label><Input id={`agent-purpose-${user.id}`} maxLength={200} placeholder="行业资讯与每周观察" value={purpose} onChange={(event) => setPurpose(event.target.value)} /></Field>
      <AgentCreateActions>
        <Button type="button" variant="ghost" onClick={onCancel}>取消</Button>
        <Button type="submit" disabled={submitting || !agentID.trim() || !name.trim()}>{submitting ? '创建中…' : '创建并生成 Token'}</Button>
      </AgentCreateActions>
      {error ? <FormError role="alert">{error}</FormError> : null}
    </AgentCreateForm>
  )
}

function TokenReveal({ token, onClose }: { token: RevealedToken; onClose: () => void }) {
  const [visible, setVisible] = useState(false)
  const [copied, setCopied] = useState(false)
  const [copyError, setCopyError] = useState('')

  async function copy() {
    setCopyError('')
    try {
      await copyText(token.token)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1600)
    } catch {
      setCopyError('复制失败，请显示 Token 后手动复制。')
    }
  }

  return (
    <RevealPanel aria-live="polite">
      <RevealHead>
        <div>
          <PanelTitle>{token.agentName} 的{token.reason === 'created' ? '访问 Token' : '新 Token'}</PanelTitle>
          <PanelDesc>{token.username} · 明文仅本次显示，关闭或刷新后无法找回。</PanelDesc>
        </div>
        <CloseButton type="button" onClick={onClose} aria-label="关闭 Token 展示" title="关闭">×</CloseButton>
      </RevealHead>
      <SecretLine>
        <SecretInput readOnly type={visible ? 'text' : 'password'} value={token.token} aria-label="新 MCP Token" />
        <IconButton type="button" onClick={() => setVisible((value) => !value)} aria-label={visible ? '隐藏 Token' : '显示 Token'} title={visible ? '隐藏 Token' : '显示 Token'}>{visible ? <EyeOffIcon /> : <EyeIcon />}</IconButton>
        <Button variant="secondary" onClick={copy}><CopyIcon />{copied ? '已复制' : '复制'}</Button>
      </SecretLine>
      {copyError ? <FormError role="alert">{copyError}</FormError> : null}
    </RevealPanel>
  )
}

async function copyText(value: string) {
  if (navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(value)
      return
    } catch {
      // Fall through for browsers that block the async clipboard API.
    }
  }
  const textarea = document.createElement('textarea')
  textarea.value = value
  textarea.style.position = 'fixed'
  textarea.style.opacity = '0'
  document.body.appendChild(textarea)
  textarea.select()
  const copied = document.execCommand('copy')
  textarea.remove()
  if (!copied) throw new Error('copy failed')
}

function formatTime(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '未知时间'
  return new Intl.DateTimeFormat('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }).format(date)
}

const Page = styled.div`display: grid; gap: ${({ theme }) => theme.space.xl};`
const Header = styled.header`display: flex; align-items: flex-start; justify-content: space-between; gap: ${({ theme }) => theme.space.xl}; flex-wrap: wrap;`
const HeaderActions = styled.div`display: flex; gap: ${({ theme }) => theme.space.md}; flex-wrap: wrap;`
const Eyebrow = styled.p`margin: 0 0 ${({ theme }) => theme.space.sm}; color: ${({ theme }) => theme.colors.primary}; font-size: ${({ theme }) => theme.typeScale.small}; font-weight: 750;`
const Title = styled.h1`margin: 0; font-size: ${({ theme }) => theme.typeScale.section}; line-height: 1.12; font-weight: 800;`
const Description = styled.p`max-width: 720px; margin: ${({ theme }) => theme.space.md} 0 0; color: ${({ theme }) => theme.colors.textMuted}; font-size: ${({ theme }) => theme.typeScale.lead};`
const CreatePanel = styled.form`display: grid; grid-template-columns: minmax(180px, 1fr) minmax(180px, 1fr) minmax(220px, 1.2fr) auto; align-items: end; gap: ${({ theme }) => theme.space.lg}; border-block: 1px solid ${({ theme }) => theme.colors.border}; padding: ${({ theme }) => theme.space.xl} 0; @media (max-width: 900px) { grid-template-columns: 1fr 1fr; } @media (max-width: 620px) { grid-template-columns: 1fr; }`
const CreateCopy = styled.div`align-self: center;`
const Field = styled.div`display: grid; gap: ${({ theme }) => theme.space.sm}; min-width: 0;`
const Label = styled.label`color: ${({ theme }) => theme.colors.textMuted}; font-size: ${({ theme }) => theme.typeScale.small}; font-weight: 650;`
const Input = styled.input`width: 100%; min-width: 0; min-height: 44px; border: 1px solid ${({ theme }) => theme.colors.border}; border-radius: ${({ theme }) => theme.radii.sm}; padding: 0 ${({ theme }) => theme.space.md}; background: ${({ theme }) => theme.colors.surface}; color: ${({ theme }) => theme.colors.text}; &:focus { border-color: ${({ theme }) => theme.colors.primary}; box-shadow: 0 0 0 3px ${({ theme }) => theme.colors.primarySoft}; }`
const CreateActions = styled.div`display: flex;`
const FormError = styled.p`grid-column: 1 / -1; margin: 0; color: ${({ theme }) => theme.colors.danger}; font-size: ${({ theme }) => theme.typeScale.small};`
const PanelTitle = styled.h2`margin: 0; font-size: ${({ theme }) => theme.typeScale.lead}; font-weight: 750;`
const PanelDesc = styled.p`margin: ${({ theme }) => theme.space.xs} 0 0; color: ${({ theme }) => theme.colors.textMuted}; font-size: ${({ theme }) => theme.typeScale.small};`
const RevealPanel = styled(Card)`display: grid; gap: ${({ theme }) => theme.space.lg}; padding: ${({ theme }) => theme.space.xl}; border-color: ${({ theme }) => theme.colors.warning}; background: ${({ theme }) => theme.colors.warningSoft};`
const RevealHead = styled.div`display: flex; align-items: flex-start; justify-content: space-between; gap: ${({ theme }) => theme.space.lg};`
const CloseButton = styled.button`display: grid; width: 36px; height: 36px; flex: 0 0 36px; place-items: center; border: 0; background: transparent; color: ${({ theme }) => theme.colors.textMuted}; font-size: 1.5rem; line-height: 1; &:hover { color: ${({ theme }) => theme.colors.text}; }`
const SecretLine = styled.div`display: grid; grid-template-columns: minmax(0, 1fr) 44px auto; gap: ${({ theme }) => theme.space.sm}; @media (max-width: 620px) { grid-template-columns: minmax(0, 1fr) 44px; > button:last-child { grid-column: 1 / -1; } }`
const SecretInput = styled.input`min-width: 0; min-height: 44px; border: 1px solid ${({ theme }) => theme.colors.border}; border-radius: ${({ theme }) => theme.radii.sm}; padding: 0 ${({ theme }) => theme.space.md}; background: ${({ theme }) => theme.colors.surface}; color: ${({ theme }) => theme.colors.text}; font-family: ui-monospace, SFMono-Regular, Menlo, monospace;`
const IconButton = styled.button`display: grid; width: 44px; height: 44px; place-items: center; border: 1px solid ${({ theme }) => theme.colors.border}; border-radius: ${({ theme }) => theme.radii.sm}; background: ${({ theme }) => theme.colors.surface}; color: ${({ theme }) => theme.colors.textMuted}; &:hover { color: ${({ theme }) => theme.colors.text}; }`
const ErrorPanel = styled.div`border: 1px solid ${({ theme }) => theme.colors.danger}; border-radius: ${({ theme }) => theme.radii.sm}; padding: ${({ theme }) => theme.space.md}; background: ${({ theme }) => theme.colors.dangerSoft}; color: ${({ theme }) => theme.colors.danger}; font-size: ${({ theme }) => theme.typeScale.small};`
const UserPanel = styled(Card)`overflow: hidden;`
const PanelToolbar = styled.div`display: flex; align-items: center; justify-content: space-between; gap: ${({ theme }) => theme.space.lg}; border-bottom: 1px solid ${({ theme }) => theme.colors.border}; padding: ${({ theme }) => theme.space.lg} ${({ theme }) => theme.space.xl}; @media (max-width: 520px) { align-items: flex-start; flex-direction: column; }`
const Summary = styled.span`font-size: ${({ theme }) => theme.typeScale.small}; font-weight: 700;`
const UserList = styled.div`display: grid;`
const UserGroup = styled.section`min-width: 0; &:not(:last-child) { border-bottom: 1px solid ${({ theme }) => theme.colors.border}; }`
const UserHeader = styled.div`display: grid; grid-template-columns: minmax(240px, 1fr) auto auto; align-items: center; gap: ${({ theme }) => theme.space.xl}; padding: ${({ theme }) => theme.space.lg} ${({ theme }) => theme.space.xl}; @media (max-width: 760px) { grid-template-columns: minmax(0, 1fr) auto; gap: ${({ theme }) => theme.space.md}; } @media (max-width: 520px) { grid-template-columns: 1fr; }`
const Identity = styled.div`display: flex; align-items: center; gap: ${({ theme }) => theme.space.md}; min-width: 0;`
const Avatar = styled.div`display: grid; width: 42px; height: 42px; flex: 0 0 42px; place-items: center; border-radius: ${({ theme }) => theme.radii.sm}; background: ${({ theme }) => theme.colors.primarySoft}; color: ${({ theme }) => theme.colors.primaryStrong}; font-weight: 800;`
const IdentityText = styled.div`min-width: 0;`
const NameLine = styled.div`display: flex; align-items: center; gap: ${({ theme }) => theme.space.sm}; flex-wrap: wrap;`
const UserName = styled.strong`overflow-wrap: anywhere; font-size: ${({ theme }) => theme.typeScale.body};`
const UserID = styled.div`margin-top: 3px; overflow: hidden; color: ${({ theme }) => theme.colors.textMuted}; font-size: ${({ theme }) => theme.typeScale.caption}; text-overflow: ellipsis; white-space: nowrap;`
const RoleLabel = styled.span`color: ${({ theme }) => theme.colors.textFaint}; font-size: ${({ theme }) => theme.typeScale.caption};`
const AgentCount = styled.div`display: grid; min-width: 72px; text-align: right; strong { font-size: ${({ theme }) => theme.typeScale.lead}; } span { color: ${({ theme }) => theme.colors.textMuted}; font-size: ${({ theme }) => theme.typeScale.caption}; } @media (max-width: 760px) { display: none; }`
const UserActions = styled.div`display: flex; justify-content: flex-end; gap: ${({ theme }) => theme.space.sm}; @media (max-width: 760px) { grid-column: 1 / -1; justify-content: flex-start; } @media (max-width: 520px) { grid-column: auto; > button:first-of-type { flex: 1; } }`
const ExpandButton = styled.button`display: grid; width: 44px; height: 44px; flex: 0 0 44px; place-items: center; border: 1px solid ${({ theme }) => theme.colors.border}; border-radius: ${({ theme }) => theme.radii.sm}; background: ${({ theme }) => theme.colors.surface}; color: ${({ theme }) => theme.colors.textMuted}; svg { transition: transform ${({ theme }) => theme.motion.base} ${({ theme }) => theme.motion.easeOut}; } &:hover { background: ${({ theme }) => theme.colors.surfaceMuted}; color: ${({ theme }) => theme.colors.text}; }`
const AgentArea = styled.div`border-top: 1px solid ${({ theme }) => theme.colors.border}; background: ${({ theme }) => theme.colors.surfaceMuted};`
const AgentCreateForm = styled.form`display: grid; grid-template-columns: minmax(130px, .8fr) repeat(3, minmax(150px, 1fr)) auto; align-items: end; gap: ${({ theme }) => theme.space.lg}; border-bottom: 1px solid ${({ theme }) => theme.colors.border}; padding: ${({ theme }) => theme.space.xl}; background: ${({ theme }) => theme.colors.primarySoft}; @media (max-width: 1080px) { grid-template-columns: 1fr 1fr; } @media (max-width: 620px) { grid-template-columns: 1fr; padding: ${({ theme }) => theme.space.lg}; }`
const AgentCreateHeading = styled.div`align-self: center;`
const AgentCreateActions = styled.div`display: flex; gap: ${({ theme }) => theme.space.sm}; @media (max-width: 620px) { > button { flex: 1; } }`
const InlineError = styled.div`border-bottom: 1px solid ${({ theme }) => theme.colors.border}; padding: ${({ theme }) => theme.space.md} ${({ theme }) => theme.space.xl}; color: ${({ theme }) => theme.colors.danger}; font-size: ${({ theme }) => theme.typeScale.small};`
const AgentList = styled.div`display: grid;`
const AgentRow = styled.div`display: grid; grid-template-columns: minmax(220px, 1.1fr) minmax(170px, .9fr) minmax(180px, .9fr) minmax(250px, auto); align-items: center; gap: ${({ theme }) => theme.space.xl}; padding: ${({ theme }) => theme.space.lg} ${({ theme }) => theme.space.xl}; &:not(:last-child) { border-bottom: 1px solid ${({ theme }) => theme.colors.border}; } @media (max-width: 1080px) { grid-template-columns: 1fr 1fr; } @media (max-width: 620px) { grid-template-columns: minmax(0, 1fr); gap: ${({ theme }) => theme.space.md}; padding: ${({ theme }) => theme.space.lg}; }`
const AgentIdentity = styled.div`display: flex; align-items: center; gap: ${({ theme }) => theme.space.md}; min-width: 0;`
const AgentMark = styled.div`display: grid; width: 36px; height: 36px; flex: 0 0 36px; place-items: center; border: 1px solid ${({ theme }) => theme.colors.border}; border-radius: ${({ theme }) => theme.radii.sm}; background: ${({ theme }) => theme.colors.surface}; color: ${({ theme }) => theme.colors.info}; font-weight: 800;`
const AgentIdentityText = styled.div`min-width: 0;`
const AgentNameLine = styled.div`display: flex; align-items: center; gap: ${({ theme }) => theme.space.sm}; flex-wrap: wrap;`
const AgentName = styled.strong`overflow-wrap: anywhere; font-size: ${({ theme }) => theme.typeScale.small};`
const AgentExternalID = styled.div`margin-top: 3px; overflow: hidden; color: ${({ theme }) => theme.colors.textMuted}; font-size: ${({ theme }) => theme.typeScale.caption}; text-overflow: ellipsis; white-space: nowrap;`
const AgentPurpose = styled.div`display: grid; gap: 3px; min-width: 0;`
const MetaLabel = styled.span`color: ${({ theme }) => theme.colors.textFaint}; font-size: ${({ theme }) => theme.typeScale.caption};`
const PurposeText = styled.span`overflow-wrap: anywhere; color: ${({ theme }) => theme.colors.text}; font-size: ${({ theme }) => theme.typeScale.small};`
const TokenState = styled.div`display: grid; gap: 2px; min-width: 0;`
const TokenMeta = styled.strong`overflow: hidden; font-size: ${({ theme }) => theme.typeScale.small}; text-overflow: ellipsis; white-space: nowrap;`
const TokenTime = styled.span`overflow-wrap: anywhere; color: ${({ theme }) => theme.colors.textMuted}; font-size: ${({ theme }) => theme.typeScale.caption};`
const RowActions = styled.div`display: flex; justify-content: flex-end; gap: ${({ theme }) => theme.space.sm}; flex-wrap: wrap; @media (max-width: 1080px) { justify-content: flex-start; } @media (max-width: 620px) { > button { flex: 1 1 auto; } }`
const AgentConfirmation = styled.div`display: flex; grid-column: 1 / -1; align-items: center; justify-content: space-between; gap: ${({ theme }) => theme.space.lg}; border-top: 1px solid ${({ theme }) => theme.colors.border}; padding-top: ${({ theme }) => theme.space.md}; @media (max-width: 620px) { align-items: stretch; flex-direction: column; }`
const ConfirmationText = styled.p`margin: 0; overflow-wrap: anywhere; color: ${({ theme }) => theme.colors.textMuted}; font-size: ${({ theme }) => theme.typeScale.small};`
const ConfirmationActions = styled.div`display: flex; flex: 0 0 auto; gap: ${({ theme }) => theme.space.sm}; @media (max-width: 620px) { > button { flex: 1; } }`
const EmptyState = styled.div`display: grid; justify-items: center; gap: ${({ theme }) => theme.space.sm}; padding: ${({ theme }) => theme.space['3xl']}; text-align: center; color: ${({ theme }) => theme.colors.textMuted}; strong { color: ${({ theme }) => theme.colors.text}; }`
const AgentEmpty = styled.div`padding: ${({ theme }) => theme.space.xl}; color: ${({ theme }) => theme.colors.textMuted}; font-size: ${({ theme }) => theme.typeScale.small}; text-align: center;`
