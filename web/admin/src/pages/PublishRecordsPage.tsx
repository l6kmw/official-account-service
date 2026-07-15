import { useEffect, useMemo, useState } from 'react'
import styled from '@emotion/styled'
import { deletePublishedRecord, getPublishRecord, listPublishRecords, syncPublishRecordStatus, type PublishRecord, type PublishStatus } from '../api/publishRecords'
import { listCurrentAgents, type AgentSummary } from '../api/agents'
import { getErrorMessage } from '../api/client'
import { Button } from '../components/Button'
import { Card } from '../components/Card'
import { StatusBadge } from '../components/StatusBadge'

type Tone = 'success' | 'warning' | 'danger' | 'info' | 'muted'
const toneColor: Record<Tone, string> = { success: 'success', warning: 'warning', danger: 'danger', info: 'info', muted: 'textMuted' }

const svgAttrs = { viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', strokeWidth: 2, strokeLinecap: 'round' as const, strokeLinejoin: 'round' as const }

function RefreshIcon() { return <svg {...svgAttrs} width="16" height="16"><polyline points="23 4 23 10 17 10" /><polyline points="1 20 1 14 7 14" /><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15" /></svg> }
function ClockIcon() { return <svg {...svgAttrs} width="14" height="14"><circle cx="12" cy="12" r="10" /><polyline points="12 6 12 12 16 14" /></svg> }
function HashIcon() { return <svg {...svgAttrs} width="14" height="14"><line x1="4" y1="9" x2="20" y2="9" /><line x1="4" y1="15" x2="20" y2="15" /><line x1="10" y1="3" x2="8" y2="21" /><line x1="16" y1="3" x2="14" y2="21" /></svg> }
function SendIcon() { return <svg {...svgAttrs} width="16" height="16"><line x1="22" y1="2" x2="11" y2="13" /><polygon points="22 2 15 22 11 13 2 9 22 2" /></svg> }
function AlertIcon() { return <svg {...svgAttrs} width="20" height="20"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" /><line x1="12" y1="9" x2="12" y2="13" /><line x1="12" y1="17" x2="12.01" y2="17" /></svg> }
function CopyIcon() { return <svg {...svgAttrs} width="15" height="15"><rect x="9" y="9" width="13" height="13" rx="2" ry="2" /><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" /></svg> }
function TrashIcon() { return <svg {...svgAttrs} width="15" height="15"><polyline points="3 6 5 6 21 6" /><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" /></svg> }
function InboxLargeIcon() { return <svg {...svgAttrs} width="64" height="64"><polyline points="22 12 16 12 14 15 10 15 8 12 2 12" /><path d="M5.45 5.11L2 12v6a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-6l-3.45-6.89A2 2 0 0 0 16.76 4H7.24a2 2 0 0 0-1.79 1.11z" /></svg> }
function AgentIcon() { return <svg {...svgAttrs} width="14" height="14"><rect x="4" y="6" width="16" height="12" rx="2" /><path d="M9 10h.01M15 10h.01" /><path d="M9 14h6M12 2v4" /></svg> }

export function PublishRecordsPage() {
  const [agentFilter, setAgentFilter] = useState('all')
  const { records, agents, selected, loading, syncingID, deletingID, error, select, reload, sync, deletePublished } = usePublishRecords(agentFilter === 'all' ? '' : agentFilter)
  const agentsByID = useMemo(() => new Map(agents.map((agent) => [agent.id, agent])), [agents])

  return (
    <Page>
      <Header>
        <HeaderLeft>
          <Eyebrow>Publish records</Eyebrow>
          <Title>发布记录</Title>
          <Description>查看微信侧发布状态、失败原因和关联文章。发布中的记录可以手动同步状态。</Description>
        </HeaderLeft>
        <Button variant="secondary" onClick={reload}><RefreshIcon />刷新</Button>
      </Header>

      {error ? <ErrorPanel message={error} onRetry={reload} /> : null}

      <AgentFilterBar>
        <AgentFilterLabel htmlFor="publish-agent-filter">创建 Agent</AgentFilterLabel>
        <AgentSelect id="publish-agent-filter" value={agentFilter} onChange={(event) => setAgentFilter(event.target.value)}>
          <option value="all">全部 Agent</option>
          {agents.map((agent) => <option key={agent.id} value={agent.id}>{agent.name} · {agent.agent_id}</option>)}
        </AgentSelect>
      </AgentFilterBar>

      <Grid>
        <ListPanel>
          <Toolbar>
            <Summary>{loading ? '正在加载发布记录…' : `共 ${records.length} 条发布记录`}</Summary>
          </Toolbar>
          {!loading && records.length === 0 ? <EmptyState /> : null}
          {loading ? <LoadingRows /> : <Timeline agentsByID={agentsByID} records={records} selectedID={selected?.id} onSelect={select} onSync={sync} onDeletePublished={deletePublished} syncingID={syncingID} deletingID={deletingID} />}
        </ListPanel>

        <DetailPanel>
          <DetailHeader><DetailHeaderTitle>记录详情</DetailHeaderTitle></DetailHeader>
          <DetailDesc>失败信息可复制，但不要粘贴包含 token / secret 的内部日志。</DetailDesc>
          {selected ? <RecordDetail agent={agentsByID.get(selected.article_created_by_agent_id)} record={selected} /> : <DetailEmpty>选择一条发布记录查看详情。</DetailEmpty>}
        </DetailPanel>
      </Grid>
    </Page>
  )
}

function usePublishRecords(agentRecordID: string) {
  const [records, setRecords] = useState<PublishRecord[]>([])
  const [agents, setAgents] = useState<AgentSummary[]>([])
  const [selected, setSelected] = useState<PublishRecord | null>(null)
  const [loading, setLoading] = useState(true)
  const [syncingID, setSyncingID] = useState<number | null>(null)
  const [deletingID, setDeletingID] = useState<number | null>(null)
  const [error, setError] = useState('')
  const [version, setVersion] = useState(0)

  useEffect(() => {
    let active = true
    setLoading(true)
    setRecords([])
    setSelected(null)
    setError('')

    Promise.all([listPublishRecords(agentRecordID), listCurrentAgents()])
      .then(([items, agentItems]) => {
        if (!active) return
        setRecords(items)
        setAgents(agentItems)
        setSelected((current) => {
          if (!current) return items[0] ?? null
          return items.find((item) => item.id === current.id) ?? items[0] ?? null
        })
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
  }, [agentRecordID, version])

  async function select(id: number) {
    try {
      setError('')
      const record = await getPublishRecord(id)
      setSelected(record)
      setRecords((items) => items.map((item) => (item.id === record.id ? record : item)))
    } catch (err: unknown) {
      setError(getErrorMessage(err))
    }
  }

  async function sync(record: PublishRecord) {
    if (record.status !== 'publishing') return
    try {
      setError('')
      setSyncingID(record.id)
      const updated = await syncPublishRecordStatus(record.id)
      setRecords((items) => items.map((item) => (item.id === updated.id ? updated : item)))
      setSelected(updated)
    } catch (err: unknown) {
      setError(getErrorMessage(err))
    } finally {
      setSyncingID(null)
    }
  }

  async function deletePublished(record: PublishRecord) {
    if (record.status !== 'published' || !record.wechat_article_id) return
    const confirmed = window.confirm(`确定删除发布记录 #${record.id} 对应的公众号内容吗？这只删除微信侧已发布图文，不会删除本地文章记录。`)
    if (!confirmed) return

    try {
      setError('')
      setDeletingID(record.id)
      const updated = await deletePublishedRecord(record.id)
      setRecords((items) => items.map((item) => (item.id === updated.id ? updated : item)))
      setSelected(updated)
    } catch (err: unknown) {
      setError(getErrorMessage(err))
    } finally {
      setDeletingID(null)
    }
  }

  return { records, agents, selected, loading, syncingID, deletingID, error, select, reload: () => setVersion((value) => value + 1), sync, deletePublished }
}

function statusTone(status: PublishStatus): Tone {
  if (status === 'publishing') return 'warning'
  if (status === 'published') return 'success'
  if (status === 'deleted') return 'muted'
  return 'danger'
}

function Timeline({ agentsByID, records, selectedID, onSelect, onSync, onDeletePublished, syncingID, deletingID }: { agentsByID: Map<string, AgentSummary>; records: PublishRecord[]; selectedID?: number; onSelect: (id: number) => void; onSync: (record: PublishRecord) => void; onDeletePublished: (record: PublishRecord) => void; syncingID: number | null; deletingID: number | null }) {
  if (records.length === 0) return null

  return (
    <TimelineList>
      {records.map((record, index) => {
        const tone = statusTone(record.status)
        const isLast = index === records.length - 1
        const agent = agentsByID.get(record.article_created_by_agent_id)
        return (
          <TimelineItem key={record.id} data-selected={record.id === selectedID} onClick={() => onSelect(record.id)}>
            <TimelineRail>
              <TimelineDot tone={tone} />
              {!isLast ? <TimelineLine /> : null}
            </TimelineRail>
            <TimelineContent>
              <TimelineHead>
                <TimelineTitle><HashIcon />文章 #{record.article_id}</TimelineTitle>
                <PublishStatusBadge status={record.status} />
              </TimelineHead>
              <TimelineMeta>
                <FootMeta><ClockIcon />{formatTime(record.submitted_at)}</FootMeta>
                <FootMeta title={record.article_created_by_agent_id || undefined}><AgentIcon />{agentDisplayName(agent, record.article_created_by_agent_id)}</FootMeta>
                <FootMeta><SendIcon />{record.wechat_publish_id || '无 Publish ID'}</FootMeta>
              </TimelineMeta>
              <TimelineActions onClick={(event) => event.stopPropagation()}>
                <Button variant="ghost" onClick={() => onSelect(record.id)}>详情</Button>
                {record.status === 'publishing' ? (
                  <Button disabled={syncingID === record.id} variant="secondary" onClick={() => onSync(record)}>
                    {syncingID === record.id ? '同步中…' : '同步状态'}
                  </Button>
                ) : null}
                {record.status === 'published' && record.wechat_article_id ? (
                  <Button disabled={deletingID === record.id} variant="danger" onClick={() => onDeletePublished(record)}>
                    <TrashIcon />{deletingID === record.id ? '删除中…' : '删除公众号内容'}
                  </Button>
                ) : null}
              </TimelineActions>
            </TimelineContent>
          </TimelineItem>
        )
      })}
    </TimelineList>
  )
}

function RecordDetail({ agent, record }: { agent?: AgentSummary; record: PublishRecord }) {
  const errorText = [record.error_code, record.error_message].filter(Boolean).join('：')

  return (
    <DetailBody>
      <DetailSection>
        <DetailSectionTitle>基本信息</DetailSectionTitle>
        <DetailRow label="发布记录 ID" value={`${record.id}`} />
        <DetailRow label="文章 ID" value={`${record.article_id}`} />
        <DetailRow label="Authorizer ID" value={`${record.authorizer_id}`} />
        <DetailRow label="创建 Agent" value={agentDisplayName(agent, record.article_created_by_agent_id)} />
        <DetailRow label="状态" value={statusText(record.status)} />
      </DetailSection>
      <DetailSection>
        <DetailSectionTitle>微信侧信息</DetailSectionTitle>
        <DetailRow label="Publish ID" value={record.wechat_publish_id || '—'} />
        <DetailRow label="Article ID" value={record.wechat_article_id || '—'} />
      </DetailSection>
      <DetailSection>
        <DetailSectionTitle>时间</DetailSectionTitle>
        <DetailRow label="提交时间" value={formatTime(record.submitted_at)} />
        <DetailRow label="完成时间" value={formatTime(record.finished_at)} />
        <DetailRow label="更新时间" value={formatTime(record.updated_at)} />
      </DetailSection>
      {record.status === 'failed' ? (
        <ErrorBox>
          <ErrorBoxHead><AlertIcon /><PanelTitle>失败原因</PanelTitle></ErrorBoxHead>
          <ErrorText>{errorText || '后端没有返回失败详情。'}</ErrorText>
          {errorText ? <Button variant="secondary" onClick={() => navigator.clipboard?.writeText(errorText)}><CopyIcon />复制错误</Button> : null}
        </ErrorBox>
      ) : null}
    </DetailBody>
  )
}

function agentDisplayName(agent: AgentSummary | undefined, agentRecordID: string) {
  if (agent) return `${agent.name} · ${agent.agent_id}`
  if (agentRecordID) return agentRecordID
  return '网页用户 / 旧文章'
}

function DetailRow({ label, value }: { label: string; value: string }) {
  return (
    <DetailItem>
      <DetailLabel>{label}</DetailLabel>
      <DetailValue>{value}</DetailValue>
    </DetailItem>
  )
}

function PublishStatusBadge({ status }: { status: PublishStatus }) {
  switch (status) {
    case 'publishing':
      return <StatusBadge tone="warning">发布中</StatusBadge>
    case 'published':
      return <StatusBadge tone="success">已发布</StatusBadge>
    case 'deleted':
      return <StatusBadge tone="muted">已删除</StatusBadge>
    case 'failed':
      return <StatusBadge tone="danger">失败</StatusBadge>
  }
}

function statusText(status: PublishStatus) {
  if (status === 'publishing') return '发布中'
  if (status === 'published') return '已发布'
  if (status === 'deleted') return '已删除'
  return '失败'
}

function LoadingRows() {
  return (
    <SkeletonList aria-label="正在加载发布记录">
      {Array.from({ length: 4 }).map((_, index) => <SkeletonRow key={index} />)}
    </SkeletonList>
  )
}

function EmptyState() {
  return (
    <EmptyPanel>
      <EmptyIcon><InboxLargeIcon /></EmptyIcon>
      <PanelTitle>暂无发布记录</PanelTitle>
      <PanelDesc>文章提交发布后，会在这里展示微信侧发布 ID、状态和失败原因。</PanelDesc>
    </EmptyPanel>
  )
}

function ErrorPanel({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <NoticePanel role="alert">
      <NoticeLeft><NoticeIconWrap><AlertIcon /></NoticeIconWrap><div><PanelTitle>发布记录加载失败</PanelTitle><PanelDesc>{message}</PanelDesc></div></NoticeLeft>
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

const AgentFilterBar = styled.div`
  display: flex;
  align-items: center;
  gap: ${({ theme }) => theme.space.md};
  flex-wrap: wrap;
`

const AgentFilterLabel = styled.label`
  color: ${({ theme }) => theme.colors.text};
  font-size: ${({ theme }) => theme.typeScale.small};
  font-weight: 750;
`

const AgentSelect = styled.select`
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

const Grid = styled.div`
  display: grid;
  gap: ${({ theme }) => theme.space.xl};

  @media (min-width: 1024px) {
    grid-template-columns: minmax(0, 1.5fr) minmax(360px, 1fr);
    align-items: start;
  }
`

const ListPanel = styled(Card)`
  overflow: hidden;
`

const Toolbar = styled.div`
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

const TimelineList = styled.div`
  display: grid;
  padding: ${({ theme }) => theme.space.lg} ${({ theme }) => theme.space.xl};
  gap: 0;
`

const TimelineItem = styled.div`
  display: grid;
  grid-template-columns: 28px 1fr;
  gap: ${({ theme }) => theme.space.md};
  padding: ${({ theme }) => theme.space.md} ${({ theme }) => theme.space.lg};
  margin: ${({ theme }) => theme.space.xs} -${({ theme }) => theme.space.lg};
  border-radius: ${({ theme }) => theme.radii.md};
  cursor: pointer;
  border-left: 3px solid transparent;
  transition: background ${({ theme }) => theme.motion.fast} ${({ theme }) => theme.motion.easeOut};

  &:hover {
    background: ${({ theme }) => theme.colors.surfaceMuted};
  }

  &[data-selected='true'] {
    background: ${({ theme }) => theme.colors.primarySoft};
    border-left-color: ${({ theme }) => theme.colors.primary};
  }
`

const TimelineRail = styled.div`
  display: flex;
  flex-direction: column;
  align-items: center;
  padding-top: 4px;
`

const TimelineDot = styled.span<{ tone: Tone }>`
  width: 12px;
  height: 12px;
  border-radius: 50%;
  background: ${({ theme, tone }) => (theme.colors as Record<string, string>)[toneColor[tone]]};
  border: 2px solid ${({ theme }) => theme.colors.surface};
  box-shadow: 0 0 0 2px ${({ theme, tone }) => (theme.colors as Record<string, string>)[toneColor[tone]]}40;
  flex-shrink: 0;
`

const TimelineLine = styled.span`
  flex: 1;
  width: 2px;
  background: ${({ theme }) => theme.colors.border};
  margin-top: 4px;
`

const TimelineContent = styled.div`
  display: grid;
  gap: ${({ theme }) => theme.space.sm};
  min-width: 0;
  padding-bottom: ${({ theme }) => theme.space.md};
`

const TimelineHead = styled.div`
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: ${({ theme }) => theme.space.md};
`

const TimelineTitle = styled.div`
  display: inline-flex;
  align-items: center;
  gap: ${({ theme }) => theme.space.xs};
  font-weight: 750;

  svg {
    color: ${({ theme }) => theme.colors.textFaint};
  }
`

const TimelineMeta = styled.div`
  display: flex;
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

const TimelineActions = styled.div`
  display: flex;
  flex-wrap: wrap;
  gap: ${({ theme }) => theme.space.sm};
`

const DetailPanel = styled(Card)`
  position: sticky;
  top: ${({ theme }) => theme.space.xl};
  display: grid;
  gap: ${({ theme }) => theme.space.lg};
  padding: ${({ theme }) => theme.space.xl};

  @media (min-width: 1024px) {
    position: sticky;
  }
`

const DetailHeader = styled.div`
  display: flex;
  align-items: center;
  gap: ${({ theme }) => theme.space.sm};
`

const DetailHeaderTitle = styled.h2`
  margin: 0;
  font-size: ${({ theme }) => theme.typeScale.title};
  font-weight: 700;
  letter-spacing: -0.02em;
`

const DetailDesc = styled.p`
  margin: 0;
  color: ${({ theme }) => theme.colors.textMuted};
  font-size: ${({ theme }) => theme.typeScale.small};
`

const DetailBody = styled.div`
  display: grid;
  gap: ${({ theme }) => theme.space.lg};
`

const DetailSection = styled.div`
  display: grid;
  gap: ${({ theme }) => theme.space.xs};
`

const DetailSectionTitle = styled.div`
  color: ${({ theme }) => theme.colors.primary};
  font-size: ${({ theme }) => theme.typeScale.caption};
  font-weight: 750;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  margin-bottom: ${({ theme }) => theme.space.xs};
`

const DetailItem = styled.div`
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: ${({ theme }) => theme.space.lg};
  border-bottom: 1px solid ${({ theme }) => theme.colors.border};
  padding: ${({ theme }) => theme.space.sm} 0;
`

const DetailLabel = styled.span`
  color: ${({ theme }) => theme.colors.textMuted};
  font-size: ${({ theme }) => theme.typeScale.small};
`

const DetailValue = styled.strong`
  overflow-wrap: anywhere;
  text-align: right;
  font-weight: 650;
`

const DetailEmpty = styled.div`
  border-radius: ${({ theme }) => theme.radii.lg};
  background: ${({ theme }) => theme.colors.surfaceMuted};
  color: ${({ theme }) => theme.colors.textMuted};
  padding: ${({ theme }) => theme.space.xl};
  text-align: center;
`

const ErrorBox = styled.div`
  display: grid;
  gap: ${({ theme }) => theme.space.md};
  border-radius: ${({ theme }) => theme.radii.lg};
  border-left: 3px solid ${({ theme }) => theme.colors.danger};
  background: ${({ theme }) => theme.colors.dangerSoft};
  color: ${({ theme }) => theme.colors.danger};
  padding: ${({ theme }) => theme.space.lg};
`

const ErrorBoxHead = styled.div`
  display: flex;
  align-items: center;
  gap: ${({ theme }) => theme.space.sm};
`

const ErrorText = styled.div`
  overflow-wrap: anywhere;
  font-size: ${({ theme }) => theme.typeScale.small};
  line-height: 1.6;
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
  font-weight: 700;
  letter-spacing: -0.02em;
`

const PanelDesc = styled.p`
  margin: ${({ theme }) => theme.space.sm} 0 0;
  color: ${({ theme }) => theme.colors.textMuted};
`

const SkeletonList = styled.div`
  display: grid;
  padding: ${({ theme }) => theme.space.lg} ${({ theme }) => theme.space.xl};
  gap: ${({ theme }) => theme.space.md};
`

const SkeletonRow = styled.div`
  height: 88px;
  border-radius: ${({ theme }) => theme.radii.md};
  background: linear-gradient(90deg, ${({ theme }) => theme.colors.surfaceMuted}, ${({ theme }) => theme.colors.primarySoft}, ${({ theme }) => theme.colors.surfaceMuted});
  background-size: 220% 100%;
  animation: shimmer 1.2s linear infinite;

  @keyframes shimmer {
    from { background-position: 220% 0; }
    to { background-position: -220% 0; }
  }
`
