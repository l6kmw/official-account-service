import { useEffect, useMemo, useState } from 'react'
import styled from '@emotion/styled'
import { getErrorMessage } from '../api/client'
import { getMCPConfig, type MCPConnectionConfig } from '../api/mcpConfig'
import { Button } from '../components/Button'
import { Card } from '../components/Card'
import { StatusBadge } from '../components/StatusBadge'
import { adminConfig, normalizePublicBaseURL } from '../config'

const svgAttrs = { viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', strokeWidth: 2, strokeLinecap: 'round' as const, strokeLinejoin: 'round' as const }

function CopyIcon() { return <svg {...svgAttrs} width="15" height="15"><rect x="9" y="9" width="13" height="13" rx="2" ry="2" /><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" /></svg> }
function ExternalIcon() { return <svg {...svgAttrs} width="15" height="15"><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" /><polyline points="15 3 21 3 21 9" /><line x1="10" y1="14" x2="21" y2="3" /></svg> }
function KeyIcon() { return <svg {...svgAttrs} width="18" height="18"><circle cx="7.5" cy="15.5" r="5.5" /><path d="M21 2l-9.6 9.6" /><path d="M15 2h6v6" /></svg> }
function ShieldIcon() { return <svg {...svgAttrs} width="18" height="18"><path d="M20 13c0 5-3.5 7.5-7.66 8.95a1 1 0 0 1-.68 0C7.5 20.5 4 18 4 13V6a1 1 0 0 1 1-1c2 0 4.5-1.2 6.24-2.72a1.17 1.17 0 0 1 1.52 0C14.51 3.8 17 5 19 5a1 1 0 0 1 1 1z" /><path d="m9 12 2 2 4-4" /></svg> }
function TerminalIcon() { return <svg {...svgAttrs} width="18" height="18"><polyline points="4 17 10 11 4 5" /><line x1="12" y1="19" x2="20" y2="19" /></svg> }
function ServerIcon() { return <svg {...svgAttrs} width="18" height="18"><rect x="3" y="4" width="18" height="8" rx="2" /><rect x="3" y="12" width="18" height="8" rx="2" /><path d="M7 8h.01" /><path d="M7 16h.01" /></svg> }
function EyeIcon() { return <svg {...svgAttrs} width="17" height="17"><path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7-10-7-10-7z" /><circle cx="12" cy="12" r="3" /></svg> }
function EyeOffIcon() { return <svg {...svgAttrs} width="17" height="17"><path d="m2 2 20 20" /><path d="M10.58 10.58A2 2 0 0 0 12 14a2 2 0 0 0 1.42-.58" /><path d="M9.88 4.24A9.8 9.8 0 0 1 12 4c6.5 0 10 8 10 8a18.5 18.5 0 0 1-3.1 4.44" /><path d="M6.61 6.61C3.63 8.62 2 12 2 12s3.5 8 10 8a9.6 9.6 0 0 0 5.39-1.61" /></svg> }

const tokenPlaceholder = '<USER_MCP_TOKEN>'
const defaultMCPPath = '/mcp'

export function MCPConfigPage() {
  const [copied, setCopied] = useState('')
  const [config, setConfig] = useState<MCPConnectionConfig | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [tokenVisible, setTokenVisible] = useState(false)
  const baseURL = useMemo(resolveDisplayBaseURL, [])
  const endpointPath = config?.path || defaultMCPPath
  const token = config?.token || ''
  const tokenHint = config?.token_hint || ''
  const tokenValue = token || tokenPlaceholder
  const headerName = config?.header_name || 'Authorization'
  const transport = config?.transport || 'streamable-http'
  const displayToken = token ? tokenVisible ? token : maskToken(token) : tokenHint || tokenPlaceholder
  const headerValue = buildHeaderValue(headerName, tokenValue)
  const endpointURL = useMemo(() => new URL(endpointPath, `${baseURL}/`).toString(), [baseURL, endpointPath])
  const healthURL = useMemo(() => new URL(`${endpointPath}/healthz`, `${baseURL}/`).toString(), [baseURL, endpointPath])
  const visibleTokenValue = token ? tokenVisible ? token : maskToken(token) : tokenHint || tokenPlaceholder
  const visibleHeaderValue = buildHeaderValue(headerName, visibleTokenValue)
  const jsonConfig = useMemo(() => buildMCPJSONConfig(endpointURL, headerName, visibleHeaderValue), [endpointURL, headerName, visibleHeaderValue])
  const copyableJSONConfig = useMemo(() => buildMCPJSONConfig(endpointURL, headerName, headerValue), [endpointURL, headerName, headerValue])

  useEffect(() => {
    let active = true
    setLoading(true)
    getMCPConfig()
      .then((next) => {
        if (!active) return
        setConfig(next)
        setError('')
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
  }, [])

  async function copy(label: string, value: string) {
    await navigator.clipboard?.writeText(value)
    setCopied(label)
    window.setTimeout(() => setCopied((current) => current === label ? '' : current), 1600)
  }

  return (
    <Page>
      <Header>
        <HeaderText>
          <Eyebrow>MCP endpoint</Eyebrow>
          <Title>Agent 连接配置</Title>
          <Description>远程 MCP 通过 HTTPS 连接，并按当前登录用户隔离公众号、文章、素材和发布记录。</Description>
        </HeaderText>
        <StatusBadge tone={config?.configured ? 'success' : 'warning'}>{config?.configured ? '已配置 token' : loading ? '读取中' : '待配置 token'}</StatusBadge>
      </Header>

      <HeroCard>
        <HeroContent>
          <HeroIcon><ServerIcon /></HeroIcon>
          <HeroMain>
            <HeroLabel>{transportLabel(transport)}</HeroLabel>
            <EndpointValue>{endpointURL}</EndpointValue>
          </HeroMain>
        </HeroContent>
        <HeroActions>
          <Button variant="secondary" onClick={() => copy('endpoint', endpointURL)}><CopyIcon />{copied === 'endpoint' ? '已复制' : '复制 URL'}</Button>
          <Button variant="secondary" onClick={() => window.open(healthURL, '_blank', 'noopener,noreferrer')}><ExternalIcon />健康检查</Button>
        </HeroActions>
      </HeroCard>

      <ConfigGrid>
        <ConfigPanel>
          <PanelHead>
            <PanelIcon><KeyIcon /></PanelIcon>
            <div>
              <PanelTitle>客户端字段</PanelTitle>
              <PanelDesc>多数客户端只需要 URL 和一个请求头。</PanelDesc>
            </div>
          </PanelHead>
          <FieldList>
            <ConfigRow label="Transport" value={transportLabel(transport)} />
            <ConfigRow label="URL" value={endpointURL} />
            <ConfigRow label="Header Name" value={headerName} />
            <SecretRow
              label="Header Value"
              value={visibleHeaderValue}
              disabled={!token}
              loading={loading}
              visible={tokenVisible}
              onToggle={() => setTokenVisible((value) => !value)}
            />
          </FieldList>
          <ButtonRow>
            <Button onClick={() => copy('json', copyableJSONConfig)}><CopyIcon />{copied === 'json' ? '已复制' : '复制 JSON 模板'}</Button>
            <Button variant="secondary" onClick={() => copy('header', `${headerName}: ${headerValue}`)}><CopyIcon />复制 Header</Button>
          </ButtonRow>
        </ConfigPanel>

        <ConfigPanel>
          <PanelHead>
            <PanelIcon $tone="warning"><ShieldIcon /></PanelIcon>
            <div>
              <PanelTitle>令牌展示</PanelTitle>
              <PanelDesc>{config?.revealable ? '兼容管理员 token，可在当前页面查看。' : '用户 token 只在管理员生成时返回一次。'}</PanelDesc>
            </div>
          </PanelHead>
          <SecretPanel>
            <SecretValue $empty={!config?.configured}>{loading ? '读取中…' : displayToken}</SecretValue>
            <SecretActions>
              <IconButton type="button" onClick={() => setTokenVisible((value) => !value)} disabled={!token || loading} aria-label={tokenVisible ? '隐藏 MCP token' : '显示 MCP token'}>
                {tokenVisible ? <EyeOffIcon /> : <EyeIcon />}
              </IconButton>
              <Button variant="secondary" onClick={() => copy('token', tokenValue)} disabled={!token}><CopyIcon />{copied === 'token' ? '已复制' : '复制 token'}</Button>
            </SecretActions>
          </SecretPanel>
          {error ? <ErrorText role="alert">{error}</ErrorText> : null}
          <SecurityList>
            <SecurityItem><TerminalIcon />客户端选择 Streamable HTTP。</SecurityItem>
            <SecurityItem><ShieldIcon />不要把管理员密码或全局 API key 填给 agent。</SecurityItem>
            <SecurityItem><KeyIcon />请求头使用 Authorization: Bearer 用户 token。</SecurityItem>
          </SecurityList>
        </ConfigPanel>
      </ConfigGrid>

      <SnippetPanel>
        <PanelHead>
          <PanelIcon><TerminalIcon /></PanelIcon>
          <div>
            <PanelTitle>JSON 配置模板</PanelTitle>
            <PanelDesc>{token ? '已使用当前 token 生成，可直接复制到 agent。' : config?.configured ? 'token 已配置；需要明文时请联系管理员重新生成。' : '请先由管理员为当前用户生成 token。'}</PanelDesc>
          </div>
        </PanelHead>
        <CodeBlock>{jsonConfig}</CodeBlock>
      </SnippetPanel>
    </Page>
  )
}

function ConfigRow({ label, value, muted }: { label: string; value: string; muted?: boolean }) {
  return (
    <Row>
      <RowLabel>{label}</RowLabel>
      <RowValue $muted={muted}>{value}</RowValue>
    </Row>
  )
}

function SecretRow({ label, value, disabled, loading, visible, onToggle }: { label: string; value: string; disabled: boolean; loading: boolean; visible: boolean; onToggle: () => void }) {
  return (
    <Row>
      <RowLabel>{label}</RowLabel>
      <SecretRowValue>
        <RowValue $muted={!visible || disabled}>{loading ? '读取中…' : value}</RowValue>
        <IconButton type="button" onClick={onToggle} disabled={loading || disabled} aria-label={visible ? '隐藏 MCP token' : '显示 MCP token'}>
          {visible ? <EyeOffIcon /> : <EyeIcon />}
        </IconButton>
      </SecretRowValue>
    </Row>
  )
}

function resolveDisplayBaseURL() {
  if (window.location.origin && /^https?:\/\//i.test(window.location.origin)) {
    return normalizePublicBaseURL(window.location.origin)
  }
  return normalizePublicBaseURL(adminConfig.publicBaseURL)
}

function buildMCPJSONConfig(endpointURL: string, headerName: string, headerValue: string) {
  return JSON.stringify({
    mcpServers: {
      'official-account': {
        transport: 'streamable-http',
        url: endpointURL,
        headers: {
          [headerName]: headerValue
        }
      }
    }
  }, null, 2)
}

function buildHeaderValue(headerName: string, token: string) {
  if (headerName.toLowerCase() !== 'authorization') return token
  return token.startsWith('Bearer ') ? token : `Bearer ${token}`
}

function maskToken(token: string) {
  if (!token) return tokenPlaceholder
  return '*'.repeat(Math.min(16, Math.max(8, token.length)))
}

function transportLabel(transport: string) {
  return transport === 'streamable-http' ? 'Streamable HTTP' : transport
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

const HeaderText = styled.div`
  min-width: 0;
`

const Eyebrow = styled.p`
  margin: 0 0 ${({ theme }) => theme.space.sm};
  color: ${({ theme }) => theme.colors.primary};
  font-size: ${({ theme }) => theme.typeScale.small};
  font-weight: 750;
`

const Title = styled.h1`
  margin: 0;
  color: ${({ theme }) => theme.colors.text};
  font-size: clamp(1.75rem, 3vw, 2.5rem);
  line-height: 1.08;
  font-weight: 800;
`

const Description = styled.p`
  max-width: 760px;
  margin: ${({ theme }) => theme.space.md} 0 0;
  color: ${({ theme }) => theme.colors.textMuted};
  font-size: ${({ theme }) => theme.typeScale.lead};
`

const HeroCard = styled(Card)`
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: ${({ theme }) => theme.space.xl};
  padding: ${({ theme }) => theme.space['2xl']};
  background:
    linear-gradient(135deg, ${({ theme }) => theme.colors.primarySoft}, ${({ theme }) => theme.colors.surface} 68%),
    ${({ theme }) => theme.colors.surface};

  @media (max-width: 760px) {
    align-items: stretch;
    flex-direction: column;
    padding: ${({ theme }) => theme.space.xl};
  }
`

const HeroContent = styled.div`
  display: flex;
  align-items: center;
  gap: ${({ theme }) => theme.space.lg};
  min-width: 0;
`

const HeroIcon = styled.div`
  display: grid;
  width: 48px;
  height: 48px;
  place-items: center;
  border-radius: ${({ theme }) => theme.radii.md};
  background: ${({ theme }) => theme.colors.primary};
  color: ${({ theme }) => theme.colors.surface};
  flex-shrink: 0;
`

const HeroMain = styled.div`
  min-width: 0;
`

const HeroLabel = styled.div`
  color: ${({ theme }) => theme.colors.primaryStrong};
  font-size: ${({ theme }) => theme.typeScale.small};
  font-weight: 750;
`

const EndpointValue = styled.div`
  overflow-wrap: anywhere;
  color: ${({ theme }) => theme.colors.text};
  font-size: clamp(1.25rem, 2vw, 1.75rem);
  font-weight: 800;
  line-height: 1.2;
`

const HeroActions = styled.div`
  display: flex;
  gap: ${({ theme }) => theme.space.md};
  flex-wrap: wrap;
`

const ConfigGrid = styled.div`
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: ${({ theme }) => theme.space.xl};

  @media (max-width: 960px) {
    grid-template-columns: 1fr;
  }
`

const ConfigPanel = styled(Card)`
  display: grid;
  align-content: start;
  gap: ${({ theme }) => theme.space.xl};
  padding: ${({ theme }) => theme.space.xl};
`

const PanelHead = styled.div`
  display: flex;
  align-items: center;
  gap: ${({ theme }) => theme.space.md};
`

const PanelIcon = styled.div<{ $tone?: 'warning' }>`
  display: grid;
  width: 38px;
  height: 38px;
  place-items: center;
  border-radius: ${({ theme }) => theme.radii.md};
  background: ${({ theme, $tone }) => $tone === 'warning' ? theme.colors.warningSoft : theme.colors.infoSoft};
  color: ${({ theme, $tone }) => $tone === 'warning' ? theme.colors.warning : theme.colors.info};
  flex-shrink: 0;
`

const PanelTitle = styled.h2`
  margin: 0;
  color: ${({ theme }) => theme.colors.text};
  font-size: ${({ theme }) => theme.typeScale.title};
  line-height: 1.15;
`

const PanelDesc = styled.p`
  margin: ${({ theme }) => theme.space.xs} 0 0;
  color: ${({ theme }) => theme.colors.textMuted};
  font-size: ${({ theme }) => theme.typeScale.small};
`

const FieldList = styled.div`
  display: grid;
  border: 1px solid ${({ theme }) => theme.colors.border};
  border-radius: ${({ theme }) => theme.radii.md};
  overflow: hidden;
`

const Row = styled.div`
  display: grid;
  grid-template-columns: minmax(112px, 0.34fr) minmax(0, 1fr);
  gap: ${({ theme }) => theme.space.md};
  padding: ${({ theme }) => theme.space.md} ${({ theme }) => theme.space.lg};
  background: ${({ theme }) => theme.colors.surface};

  & + & {
    border-top: 1px solid ${({ theme }) => theme.colors.border};
  }

  @media (max-width: 540px) {
    grid-template-columns: 1fr;
    gap: ${({ theme }) => theme.space.xs};
  }
`

const RowLabel = styled.div`
  color: ${({ theme }) => theme.colors.textMuted};
  font-size: ${({ theme }) => theme.typeScale.small};
  font-weight: 650;
`

const RowValue = styled.div<{ $muted?: boolean }>`
  overflow-wrap: anywhere;
  color: ${({ theme, $muted }) => $muted ? theme.colors.textFaint : theme.colors.text};
  font-size: ${({ theme }) => theme.typeScale.small};
  font-weight: 720;
`

const ButtonRow = styled.div`
  display: flex;
  gap: ${({ theme }) => theme.space.md};
  flex-wrap: wrap;
`

const SecretRowValue = styled.div`
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: ${({ theme }) => theme.space.md};
  min-width: 0;
`

const SecretPanel = styled.div`
  display: grid;
  gap: ${({ theme }) => theme.space.md};
`

const SecretValue = styled.div<{ $empty?: boolean }>`
  margin: 0;
  border: 1px solid ${({ theme }) => theme.colors.border};
  border-radius: ${({ theme }) => theme.radii.md};
  padding: ${({ theme }) => theme.space.lg};
  background: ${({ theme }) => theme.colors.surfaceMuted};
  color: ${({ theme, $empty }) => $empty ? theme.colors.textFaint : theme.colors.text};
  font-size: ${({ theme }) => theme.typeScale.small};
  font-weight: 720;
  line-height: 1.55;
  overflow-wrap: anywhere;
`

const SecretActions = styled.div`
  display: flex;
  align-items: center;
  gap: ${({ theme }) => theme.space.md};
  flex-wrap: wrap;
`

const IconButton = styled.button`
  display: grid;
  width: 40px;
  height: 40px;
  place-items: center;
  border: 1px solid ${({ theme }) => theme.colors.border};
  border-radius: ${({ theme }) => theme.radii.md};
  background: ${({ theme }) => theme.colors.surface};
  color: ${({ theme }) => theme.colors.textMuted};
  flex-shrink: 0;
  transition:
    background ${({ theme }) => theme.motion.fast} ${({ theme }) => theme.motion.easeOut},
    color ${({ theme }) => theme.motion.fast} ${({ theme }) => theme.motion.easeOut};

  &:hover:not(:disabled) {
    background: ${({ theme }) => theme.colors.primarySoft};
    color: ${({ theme }) => theme.colors.primaryStrong};
  }

  &:disabled {
    cursor: not-allowed;
    opacity: 0.48;
  }
`

const ErrorText = styled.div`
  border: 1px solid ${({ theme }) => theme.colors.dangerSoft};
  border-radius: ${({ theme }) => theme.radii.md};
  padding: ${({ theme }) => theme.space.md} ${({ theme }) => theme.space.lg};
  background: ${({ theme }) => theme.colors.dangerSoft};
  color: ${({ theme }) => theme.colors.danger};
  font-size: ${({ theme }) => theme.typeScale.small};
  font-weight: 650;
`

const SecurityList = styled.div`
  display: grid;
  gap: ${({ theme }) => theme.space.sm};
`

const SecurityItem = styled.div`
  display: flex;
  align-items: center;
  gap: ${({ theme }) => theme.space.sm};
  color: ${({ theme }) => theme.colors.textMuted};
  font-size: ${({ theme }) => theme.typeScale.small};
  font-weight: 650;

  svg {
    color: ${({ theme }) => theme.colors.primary};
    flex-shrink: 0;
  }
`

const SnippetPanel = styled(Card)`
  display: grid;
  gap: ${({ theme }) => theme.space.xl};
  padding: ${({ theme }) => theme.space.xl};
`

const CodeBlock = styled.pre`
  margin: 0;
  overflow-x: auto;
  border-radius: ${({ theme }) => theme.radii.md};
  padding: ${({ theme }) => theme.space.xl};
  background: oklch(25% 0.02 75);
  color: oklch(96% 0.01 75);
  font-size: ${({ theme }) => theme.typeScale.small};
  line-height: 1.65;
`
