import { useMemo, useState } from 'react'
import styled from '@emotion/styled'
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

const tokenPlaceholder = '<OFFICIAL_ACCOUNT_MCP_TOKEN>'

export function MCPConfigPage() {
  const [copied, setCopied] = useState('')
  const baseURL = useMemo(resolveDisplayBaseURL, [])
  const endpointURL = useMemo(() => new URL('/mcp', `${baseURL}/`).toString(), [baseURL])
  const healthURL = useMemo(() => new URL('/mcp/healthz', `${baseURL}/`).toString(), [baseURL])
  const jsonConfig = useMemo(() => buildMCPJSONConfig(endpointURL), [endpointURL])
  const tokenCommand = `ssh -i /Users/CHANGE_ME/.ssh/deploy_ed25519 root@203.0.113.10 'sed -n "s/^OFFICIAL_ACCOUNT_MCP_TOKEN=//p" /opt/official-account-service/mcp.env'`

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
          <Description>远程 MCP 已通过 HTTPS 暴露，后台只展示连接参数；访问令牌保留在服务器环境文件中。</Description>
        </HeaderText>
        <StatusBadge tone="success">已部署</StatusBadge>
      </Header>

      <HeroCard>
        <HeroContent>
          <HeroIcon><ServerIcon /></HeroIcon>
          <HeroMain>
            <HeroLabel>Streamable HTTP</HeroLabel>
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
              <PanelDesc>适合 Cherry Studio 或支持远程 MCP 的 agent。</PanelDesc>
            </div>
          </PanelHead>
          <FieldList>
            <ConfigRow label="Transport" value="Streamable HTTP" />
            <ConfigRow label="URL" value={endpointURL} />
            <ConfigRow label="Auth Type" value="API Key" />
            <ConfigRow label="Header Name" value="X-API-Key" />
            <ConfigRow label="API Key" value={tokenPlaceholder} muted />
          </FieldList>
          <ButtonRow>
            <Button onClick={() => copy('json', jsonConfig)}><CopyIcon />{copied === 'json' ? '已复制' : '复制 JSON 模板'}</Button>
            <Button variant="secondary" onClick={() => copy('header', `X-API-Key: ${tokenPlaceholder}`)}><CopyIcon />复制 Header</Button>
          </ButtonRow>
        </ConfigPanel>

        <ConfigPanel>
          <PanelHead>
            <PanelIcon $tone="warning"><ShieldIcon /></PanelIcon>
            <div>
              <PanelTitle>令牌位置</PanelTitle>
              <PanelDesc>token 不进入前端运行时配置。</PanelDesc>
            </div>
          </PanelHead>
          <CommandBox>
            <CommandText>{tokenCommand}</CommandText>
            <Button variant="secondary" onClick={() => copy('command', tokenCommand)}><CopyIcon />{copied === 'command' ? '已复制' : '复制命令'}</Button>
          </CommandBox>
          <SecurityList>
            <SecurityItem><TerminalIcon />不要选择 OAuth 登录流。</SecurityItem>
            <SecurityItem><ShieldIcon />不要把后台 admin API key 填给 agent。</SecurityItem>
            <SecurityItem><KeyIcon />只把 MCP token 填到 API Key 字段。</SecurityItem>
          </SecurityList>
        </ConfigPanel>
      </ConfigGrid>

      <SnippetPanel>
        <PanelHead>
          <PanelIcon><TerminalIcon /></PanelIcon>
          <div>
            <PanelTitle>JSON 配置模板</PanelTitle>
            <PanelDesc>将占位符替换为服务器上的 MCP token。</PanelDesc>
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

function resolveDisplayBaseURL() {
  if (window.location.origin && /^https?:\/\//i.test(window.location.origin)) {
    return normalizePublicBaseURL(window.location.origin)
  }
  return normalizePublicBaseURL(adminConfig.publicBaseURL)
}

function buildMCPJSONConfig(endpointURL: string) {
  return JSON.stringify({
    mcpServers: {
      'official-account': {
        transport: 'streamable-http',
        url: endpointURL,
        headers: {
          'X-API-Key': tokenPlaceholder
        }
      }
    }
  }, null, 2)
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

const CommandBox = styled.div`
  display: grid;
  gap: ${({ theme }) => theme.space.md};
`

const CommandText = styled.pre`
  max-width: 100%;
  margin: 0;
  overflow-x: auto;
  border: 1px solid ${({ theme }) => theme.colors.border};
  border-radius: ${({ theme }) => theme.radii.md};
  padding: ${({ theme }) => theme.space.lg};
  background: ${({ theme }) => theme.colors.surfaceMuted};
  color: ${({ theme }) => theme.colors.text};
  font-size: ${({ theme }) => theme.typeScale.small};
  line-height: 1.55;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
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
