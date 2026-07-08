import { useEffect, useState } from 'react'
import styled from '@emotion/styled'
import { getDashboardStats, type DashboardStats } from '../api/dashboard'
import { getErrorMessage } from '../api/client'
import { Button } from '../components/Button'
import { Card } from '../components/Card'
import { StatusBadge } from '../components/StatusBadge'

const tenantID = 'tenant-1'
type Tone = 'success' | 'warning' | 'danger' | 'info' | 'muted'
const toneColor: Record<Tone, string> = { success: 'success', warning: 'warning', danger: 'danger', info: 'info', muted: 'textMuted' }
const toneSoft: Record<Tone, string> = { success: 'successSoft', warning: 'warningSoft', danger: 'dangerSoft', info: 'infoSoft', muted: 'surfaceMuted' }

function MetricIcon({ index }: { index: number }) {
  const c = { width: 18, height: 18, viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', strokeWidth: 2, strokeLinecap: 'round' as const, strokeLinejoin: 'round' as const }
  switch (index) {
    case 0: return <svg {...c}><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2" /><circle cx="9" cy="7" r="4" /><path d="M22 21v-2a4 4 0 0 0-3-3.87" /><path d="M16 3.13a4 4 0 0 1 0 7.75" /></svg>
    case 1: return <svg {...c}><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" /><polyline points="14 2 14 8 20 8" /><line x1="8" y1="13" x2="16" y2="13" /><line x1="8" y1="17" x2="14" y2="17" /></svg>
    case 2: return <svg {...c}><circle cx="12" cy="12" r="10" /><polyline points="12 6 12 12 16 14" /></svg>
    case 3: return <svg {...c}><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" /><line x1="12" y1="9" x2="12" y2="13" /><line x1="12" y1="17" x2="12.01" y2="17" /></svg>
    default: return null
  }
}

export function DashboardPage() {
  const { stats, loading, error, reload } = useDashboardStats(tenantID)
  const empty = stats ? isEmptyStats(stats) : false
  const hour = new Date().getHours()
  const greeting = hour < 12 ? '早上好' : hour < 18 ? '下午好' : '晚上好'
  const today = new Intl.DateTimeFormat('zh-CN', { month: 'long', day: 'numeric', weekday: 'long' }).format(new Date())
  const metrics = getMetrics(stats)

  return (
    <Page>
      <WelcomeStrip>
        <WelcomeLeft>
          <Greeting>{greeting}</Greeting>
          <DateText>{today}</DateText>
        </WelcomeLeft>
        <StatusBadge tone={error ? 'danger' : 'success'}>{error ? '连接异常' : '系统正常运行中'}</StatusBadge>
      </WelcomeStrip>
      {error ? <ErrorPanel message={error} onRetry={reload} /> : null}
      {empty ? <EmptyPanel /> : null}
      <MetricGrid>
        {metrics.map((m, i) => (
          <MetricCard key={m.label} aria-busy={loading}>
            <MetricTop>
              <MetricLabel>{m.label}</MetricLabel>
              <MetricIconWrap tone={m.tone}><MetricIcon index={i} /></MetricIconWrap>
            </MetricTop>
            <MetricValue>{loading ? '—' : m.value}</MetricValue>
            <MetricHelper>{m.helper}</MetricHelper>
            <MetricAccent tone={m.tone} />
          </MetricCard>
        ))}
      </MetricGrid>
      <ContentGrid>
        <Panel>
          <PanelHeader><PanelDot tone="info" /><PanelTitle>文章状态</PanelTitle></PanelHeader>
          <BarList>
            {getArticleRows(stats).map((row) => {
              const max = stats?.article_total || 1
              const pct = loading ? 0 : Math.round((row.value / max) * 100)
              return (
                <BarRow key={row.label}>
                  <BarInfo><BarLabel>{row.label}</BarLabel><BarMeta>{row.helper}</BarMeta></BarInfo>
                  <BarTrack><BarFill tone={row.tone} style={{ width: `${pct}%` }} /></BarTrack>
                  <BarCount>{loading ? '—' : row.value}</BarCount>
                </BarRow>
              )
            })}
          </BarList>
        </Panel>
        <Panel>
          <PanelHeader><PanelDot tone="warning" /><PanelTitle>发布状态</PanelTitle></PanelHeader>
          <BarList>
            {getPublishRows(stats).map((row) => {
              const max = stats?.publish_total || 1
              const pct = loading ? 0 : Math.round((row.value / max) * 100)
              return (
                <BarRow key={row.label}>
                  <BarInfo><BarLabel>{row.label}</BarLabel><BarMeta>{row.helper}</BarMeta></BarInfo>
                  <BarTrack><BarFill tone={row.tone} style={{ width: `${pct}%` }} /></BarTrack>
                  <StatusBadge tone={row.tone}>{loading ? '—' : row.value}</StatusBadge>
                </BarRow>
              )
            })}
          </BarList>
        </Panel>
      </ContentGrid>
      <SetupCard>
        <SetupHeader><PanelTitle>微信配置准备</PanelTitle><StatusBadge tone="warning">进行中</StatusBadge></SetupHeader>
        <SetupSteps>
          <SetupStep><StepNum>1</StepNum><StepText>配置 component app id / secret</StepText></SetupStep>
          <SetupStep><StepNum>2</StepNum><StepText>配置 verify token / encoding aes key</StepText></SetupStep>
          <SetupStep><StepNum>3</StepNum><StepText>填写授权事件和 authorizer 回调 URL</StepText></SetupStep>
        </SetupSteps>
      </SetupCard>
    </Page>
  )
}

function useDashboardStats(currentTenantID: string) {
  const [stats, setStats] = useState<DashboardStats | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [version, setVersion] = useState(0)
  useEffect(() => {
    let active = true
    setLoading(true)
    setError('')
    getDashboardStats(currentTenantID).then((data) => { if (active) setStats(data) }).catch((err: unknown) => { if (active) setError(getErrorMessage(err)) }).finally(() => { if (active) setLoading(false) })
    return () => { active = false }
  }, [currentTenantID, version])
  return { stats, loading, error, reload: () => setVersion((v) => v + 1) }
}

function getMetrics(stats: DashboardStats | null) {
  return [
    { label: '授权账号', value: `${stats?.account_total ?? 0}`, helper: `${stats?.active_account_total ?? 0} 个可用`, tone: 'success' as Tone },
    { label: '文章总数', value: `${stats?.article_total ?? 0}`, helper: `${stats?.draft_article_total ?? 0} 篇草稿`, tone: 'info' as Tone },
    { label: '发布中', value: `${stats?.publishing_publish_total ?? 0}`, helper: '等待微信侧结果', tone: 'warning' as Tone },
    { label: '发布失败', value: `${stats?.failed_publish_total ?? 0}`, helper: '需要人工检查', tone: 'danger' as Tone },
  ]
}

function getArticleRows(stats: DashboardStats | null) {
  return [
    { label: '草稿', value: stats?.draft_article_total ?? 0, helper: '还未提交微信发布', tone: 'muted' as Tone },
    { label: '发布中', value: stats?.publishing_article_total ?? 0, helper: '已提交，等待结果', tone: 'warning' as Tone },
    { label: '已发布', value: stats?.published_article_total ?? 0, helper: '微信侧发布成功', tone: 'success' as Tone },
    { label: '失败', value: stats?.failed_article_total ?? 0, helper: '需要排查错误', tone: 'danger' as Tone },
  ]
}

function getPublishRows(stats: DashboardStats | null) {
  return [
    { label: '发布中', value: stats?.publishing_publish_total ?? 0, helper: '轮询或回调会更新', tone: 'warning' as Tone },
    { label: '已发布', value: stats?.published_publish_total ?? 0, helper: '最终结果成功', tone: 'success' as Tone },
    { label: '失败', value: stats?.failed_publish_total ?? 0, helper: '查看失败原因', tone: 'danger' as Tone },
  ]
}

function isEmptyStats(stats: DashboardStats) { return stats.account_total === 0 && stats.article_total === 0 && stats.publish_total === 0 }

function ErrorPanel({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (<NoticePanel role="alert"><div><PanelTitle>Dashboard 数据加载失败</PanelTitle><PanelDesc>{message}</PanelDesc></div><Button variant="secondary" onClick={onRetry}>重试</Button></NoticePanel>)
}
function EmptyPanel() {
  return (<NoticePanel><div><PanelTitle>还没有业务数据</PanelTitle><PanelDesc>先创建文章或完成公众号授权，之后这里会展示统计数据。</PanelDesc></div><StatusBadge tone="muted">空状态</StatusBadge></NoticePanel>)
}

const Page = styled.div`display: grid; gap: ${({ theme }) => theme.space.xl};`
const WelcomeStrip = styled.div`
  display: flex; align-items: center; justify-content: space-between; gap: ${({ theme }) => theme.space.lg};
  padding: ${({ theme }) => theme.space.xl} ${({ theme }) => theme.space['2xl']};
  border-radius: ${({ theme }) => theme.radii.lg};
  background: linear-gradient(135deg, ${({ theme }) => theme.colors.primarySoft}, ${({ theme }) => theme.colors.surfaceMuted});
`
const WelcomeLeft = styled.div`display: grid; gap: ${({ theme }) => theme.space.xs};`
const Greeting = styled.h1`margin: 0; font-size: clamp(1.5rem, 3vw, 2.25rem); font-weight: 800; letter-spacing: -0.03em; color: ${({ theme }) => theme.colors.text};`
const DateText = styled.p`margin: 0; color: ${({ theme }) => theme.colors.textMuted}; font-size: ${({ theme }) => theme.typeScale.small};`
const MetricGrid = styled.div`display: grid; gap: ${({ theme }) => theme.space.lg}; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));`
const MetricCard = styled(Card)`
  position: relative; padding: ${({ theme }) => theme.space.xl}; overflow: hidden;
  transition: transform ${({ theme }) => theme.motion.base} ${({ theme }) => theme.motion.easeOut}, box-shadow ${({ theme }) => theme.motion.base} ${({ theme }) => theme.motion.easeOut};
  &:hover { transform: translateY(-2px); box-shadow: ${({ theme }) => theme.shadows.soft}; }
`
const MetricAccent = styled.div<{ tone: Tone }>`position: absolute; bottom: 0; left: 0; right: 0; height: 3px; background: ${({ theme, tone }) => (theme.colors as Record<string, string>)[toneColor[tone]]}; opacity: 0.5;`
const MetricTop = styled.div`display: flex; align-items: center; justify-content: space-between; gap: ${({ theme }) => theme.space.md};`
const MetricIconWrap = styled.span<{ tone: Tone }>`
  display: grid; width: 36px; height: 36px; place-items: center; border-radius: ${({ theme }) => theme.radii.md};
  background: ${({ theme, tone }) => (theme.colors as Record<string, string>)[toneSoft[tone]]};
  color: ${({ theme, tone }) => (theme.colors as Record<string, string>)[toneColor[tone]]}; flex-shrink: 0;
`
const MetricLabel = styled.div`color: ${({ theme }) => theme.colors.textMuted}; font-size: ${({ theme }) => theme.typeScale.small}; font-weight: 650;`
const MetricValue = styled.div`margin-top: ${({ theme }) => theme.space.sm}; font-size: clamp(1.75rem, 4vw, 2.5rem); font-variant-numeric: tabular-nums; font-weight: 800; letter-spacing: -0.04em; line-height: 1;`
const MetricHelper = styled.div`margin-top: ${({ theme }) => theme.space.md}; color: ${({ theme }) => theme.colors.textMuted}; font-size: ${({ theme }) => theme.typeScale.small};`
const ContentGrid = styled.div`display: grid; gap: ${({ theme }) => theme.space.xl}; @media (min-width: 1024px) { grid-template-columns: 1fr 1fr; }`
const Panel = styled(Card)`overflow: hidden;`
const PanelHeader = styled.div`display: flex; align-items: center; gap: ${({ theme }) => theme.space.md}; border-bottom: 1px solid ${({ theme }) => theme.colors.border}; padding: ${({ theme }) => theme.space.lg} ${({ theme }) => theme.space.xl};`
const PanelDot = styled.span<{ tone: Tone }>`width: 10px; height: 10px; border-radius: 50%; background: ${({ theme, tone }) => (theme.colors as Record<string, string>)[toneColor[tone]]}; flex-shrink: 0;`
const PanelTitle = styled.h2`margin: 0; font-size: ${({ theme }) => theme.typeScale.title}; font-weight: 700; letter-spacing: -0.02em;`
const PanelDesc = styled.p`margin: ${({ theme }) => theme.space.sm} 0 0; color: ${({ theme }) => theme.colors.textMuted}; font-size: ${({ theme }) => theme.typeScale.small};`
const BarList = styled.div`display: grid; padding: ${({ theme }) => theme.space.lg} ${({ theme }) => theme.space.xl}; gap: ${({ theme }) => theme.space.lg};`
const BarRow = styled.div`display: grid; grid-template-columns: minmax(80px, 1fr) minmax(60px, 1.5fr) auto; align-items: center; gap: ${({ theme }) => theme.space.lg};`
const BarInfo = styled.div`min-width: 0;`
const BarLabel = styled.div`font-weight: 650; font-size: ${({ theme }) => theme.typeScale.small};`
const BarMeta = styled.div`color: ${({ theme }) => theme.colors.textMuted}; font-size: ${({ theme }) => theme.typeScale.caption}; margin-top: 2px;`
const BarTrack = styled.div`height: 8px; border-radius: ${({ theme }) => theme.radii.pill}; background: ${({ theme }) => theme.colors.surfaceMuted}; overflow: hidden;`
const BarFill = styled.div<{ tone: Tone }>`height: 100%; border-radius: ${({ theme }) => theme.radii.pill}; background: ${({ theme, tone }) => (theme.colors as Record<string, string>)[toneColor[tone]]}; transition: width ${({ theme }) => theme.motion.slow} ${({ theme }) => theme.motion.easeOut};`
const BarCount = styled.span`font-size: ${({ theme }) => theme.typeScale.title}; font-variant-numeric: tabular-nums; font-weight: 800; min-width: 2ch; text-align: right;`
const NoticePanel = styled(Card)`display: flex; align-items: center; justify-content: space-between; gap: ${({ theme }) => theme.space.lg}; padding: ${({ theme }) => theme.space.xl}; border-left: 3px solid ${({ theme }) => theme.colors.info};`
const SetupCard = styled(Card)`display: grid; gap: ${({ theme }) => theme.space.lg}; padding: ${({ theme }) => theme.space.xl};`
const SetupHeader = styled.div`display: flex; align-items: center; justify-content: space-between; gap: ${({ theme }) => theme.space.lg};`
const SetupSteps = styled.div`display: grid; gap: ${({ theme }) => theme.space.md};`
const SetupStep = styled.div`display: flex; align-items: center; gap: ${({ theme }) => theme.space.md};`
const StepNum = styled.span`display: grid; width: 28px; height: 28px; place-items: center; border-radius: 50%; background: ${({ theme }) => theme.colors.primarySoft}; color: ${({ theme }) => theme.colors.primaryStrong}; font-weight: 700; font-size: ${({ theme }) => theme.typeScale.small};`
const StepText = styled.span`font-weight: 650;`
