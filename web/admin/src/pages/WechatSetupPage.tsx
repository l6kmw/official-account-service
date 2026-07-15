import { useMemo, useState } from 'react'
import styled from '@emotion/styled'
import { Button } from '../components/Button'
import { Card } from '../components/Card'
import { StatusBadge } from '../components/StatusBadge'
import { adminConfig, buildWechatOpenPlatformURLs, normalizePublicBaseURL } from '../config'

const yamlItems = [
  ['wechat.component_app_id', 'Component AppID', '待确认'],
  ['wechat.component_app_secret', 'Component AppSecret', '只显示配置状态，不显示值'],
  ['wechat.component_verify_token', 'Verify Token', '只显示配置状态，不显示值'],
  ['wechat.component_encoding_aes_key', 'EncodingAESKey', '只显示配置状态，不显示值'],
  ['wechat.refresh_token_encryption_key', 'Refresh Token Encryption Key', '只显示配置状态，不显示值']
]

const svgAttrs = { viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', strokeWidth: 2, strokeLinecap: 'round' as const, strokeLinejoin: 'round' as const }

function CopyIcon() { return <svg {...svgAttrs} width="15" height="15"><rect x="9" y="9" width="13" height="13" rx="2" ry="2" /><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" /></svg> }
function ExternalIcon() { return <svg {...svgAttrs} width="15" height="15"><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" /><polyline points="15 3 21 3 21 9" /><line x1="10" y1="14" x2="21" y2="3" /></svg> }
function AlertIcon() { return <svg {...svgAttrs} width="20" height="20"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" /><line x1="12" y1="9" x2="12" y2="13" /><line x1="12" y1="17" x2="12.01" y2="17" /></svg> }
function LinkIcon() { return <svg {...svgAttrs} width="16" height="16"><path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71" /><path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71" /></svg> }
function CheckCircleIcon() { return <svg {...svgAttrs} width="20" height="20"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" /><polyline points="22 4 12 14.01 9 11.01" /></svg> }
function CircleIcon() { return <svg {...svgAttrs} width="20" height="20"><circle cx="12" cy="12" r="10" /></svg> }

const STEPS = [
  { num: 1, label: '配置清单', desc: '核对第三方平台密钥' },
  { num: 2, label: '回调地址', desc: '填写到微信开放平台' },
  { num: 3, label: '授权入口', desc: '打开扫码入口页' }
]

export function WechatSetupPage() {
  const [baseURL, setBaseURL] = useState(adminConfig.publicBaseURL)
  const [componentAppID, setComponentAppID] = useState(adminConfig.componentAppID)
  const [authorizationEntryURL, setAuthorizationEntryURL] = useState('')
  const [error, setError] = useState('')
  const urls = useMemo(() => buildWechatOpenPlatformURLs({ publicBaseURL: baseURL, componentAppID }), [baseURL, componentAppID])

  function openAuthorizationEntry() {
    const publicBaseURL = normalizePublicBaseURL(baseURL)
    if (!publicBaseURL) {
      setError('请先填写公网服务域名。')
      return
    }
    if (!componentAppID.trim()) {
      setError('请先填写 Component AppID。')
      return
    }
    setError('')
    setAuthorizationEntryURL('')

    try {
      const nextEntryURL = buildWechatOpenPlatformURLs({ publicBaseURL, componentAppID }).authorizationEntry
      setAuthorizationEntryURL(nextEntryURL)
      window.open(nextEntryURL, '_blank', 'noopener,noreferrer')
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : '授权入口地址生成失败，请检查公网服务域名。')
    }
  }

  return (
    <Page>
      <Header>
        <HeaderLeft>
          <Eyebrow>WeChat setup</Eyebrow>
          <Title>开放平台配置检查</Title>
          <Description>本页供系统管理员核对第三方平台配置；普通公众号管理员只需在账号管理页点击“添加公众号”并扫码授权。</Description>
        </HeaderLeft>
        <StatusBadge tone="warning">申请准备中</StatusBadge>
      </Header>

      <ProgressIndicator>
        {STEPS.map((step, index) => (
          <ProgressSegment key={step.num}>
            <ProgressCircle data-active={index === 0}>{index < 1 ? <CheckCircleIcon /> : step.num}</ProgressCircle>
            <ProgressText>
              <ProgressLabel data-active={index === 0}>{step.label}</ProgressLabel>
              <ProgressDesc>{step.desc}</ProgressDesc>
            </ProgressText>
            {index < STEPS.length - 1 ? <ProgressConnector /> : null}
          </ProgressSegment>
        ))}
      </ProgressIndicator>

      <StepSection>
        <StepHeader>
          <StepBadge>1</StepBadge>
          <StepHeading>
            <PanelTitle>开放平台配置清单</PanelTitle>
            <PanelDesc>当前后端还没有配置状态 API，所以这里不读取真实密钥；secret 类字段只展示状态，不展示值。</PanelDesc>
          </StepHeading>
        </StepHeader>
        <Checklist>
          {yamlItems.map(([key, label, helper]) => (
            <CheckItem key={key}>
              <CheckIconWrap><CircleIcon /></CheckIconWrap>
              <CheckInfo>
                <Strong>{label}</Strong>
                <Meta>{key}</Meta>
                <Meta>{helper}</Meta>
              </CheckInfo>
              <StatusBadge tone="muted">待确认</StatusBadge>
            </CheckItem>
          ))}
        </Checklist>
      </StepSection>

      <StepSection>
        <StepHeader>
          <StepBadge>2</StepBadge>
          <StepHeading>
            <PanelTitle>填写到微信开放平台的 URL</PanelTitle>
            <PanelDesc>将域名替换成公网 HTTPS 域名后，复制到微信开放平台第三方平台配置。</PanelDesc>
          </StepHeading>
        </StepHeader>
        <Field>
          <Label htmlFor="base-url">公网服务域名</Label>
          <Input id="base-url" value={baseURL} onChange={(event) => setBaseURL(event.target.value)} />
          <Helper>示例：https://oa.example.com</Helper>
        </Field>
        <URLList>
          <URLItem label="授权事件接收 URL" value={urls.componentCallback} />
          <URLItem label="授权后公众号消息与事件接收 URL" value={urls.authorizerCallback} />
          <URLItem label="授权回调 URL" value={urls.authorizationCallback} />
        </URLList>
      </StepSection>

      <StepSection>
        <StepHeader>
          <StepBadge>3</StepBadge>
          <StepHeading>
            <PanelTitle>授权 URL 生成</PanelTitle>
            <PanelDesc>这里生成公网授权入口页地址，入口页会再向后端请求 pre_auth_code 并跳转微信官方授权页。</PanelDesc>
          </StepHeading>
        </StepHeader>
        <AuthForm>
          <Field>
            <Label htmlFor="component-appid">Component AppID</Label>
            <Input id="component-appid" value={componentAppID} onChange={(event) => setComponentAppID(event.target.value)} placeholder="wx_component_appid" />
            <Helper>AppID 可展示；不要在这里输入 AppSecret。</Helper>
          </Field>
          <ActionBox>
            <Button onClick={openAuthorizationEntry}>打开授权入口</Button>
          </ActionBox>
        </AuthForm>

        {error ? <Notice $danger role="alert"><NoticeIconWrap $danger><AlertIcon /></NoticeIconWrap><div><PanelTitle>生成失败</PanelTitle><PanelDesc>{error}</PanelDesc></div></Notice> : null}
        {authorizationEntryURL ? (
          <ResultBox>
            <ResultHead>
              <Strong>授权入口 URL</Strong>
              <Meta>与账号管理页使用同一个入口。</Meta>
            </ResultHead>
            <URLValue>{authorizationEntryURL}</URLValue>
            <RowActions>
              <Button variant="secondary" onClick={() => navigator.clipboard?.writeText(authorizationEntryURL)}><CopyIcon />复制</Button>
              <Button onClick={() => window.open(authorizationEntryURL, '_blank', 'noopener,noreferrer')}><ExternalIcon />重新打开入口</Button>
            </RowActions>
          </ResultBox>
        ) : null}
      </StepSection>
    </Page>
  )
}

function URLItem({ label, value }: { label: string; value: string }) {
  return (
    <URLCard>
      <URLCardLeft>
        <URLCardIcon><LinkIcon /></URLCardIcon>
        <div>
          <Strong>{label}</Strong>
          <URLValue>{value}</URLValue>
        </div>
      </URLCardLeft>
      <Button variant="secondary" onClick={() => navigator.clipboard?.writeText(value)}><CopyIcon />复制</Button>
    </URLCard>
  )
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

const ProgressIndicator = styled.div`
  display: flex;
  align-items: stretch;
  gap: 0;
  padding: ${({ theme }) => theme.space.xl} ${({ theme }) => theme.space['2xl']};
  border: 1px solid ${({ theme }) => theme.colors.border};
  border-radius: ${({ theme }) => theme.radii.lg};
  background: ${({ theme }) => theme.colors.surface};
  box-shadow: ${({ theme }) => theme.shadows.xs};

  @media (max-width: 768px) {
    flex-direction: column;
    gap: ${({ theme }) => theme.space.md};
  }
`

const ProgressSegment = styled.div`
  display: flex;
  align-items: center;
  gap: ${({ theme }) => theme.space.md};
  flex: 1;
  min-width: 0;

  @media (max-width: 768px) {
    width: 100%;
  }
`

const ProgressCircle = styled.div`
  display: grid;
  width: 44px;
  height: 44px;
  place-items: center;
  border-radius: 50%;
  border: 2px solid ${({ theme }) => theme.colors.border};
  background: ${({ theme }) => theme.colors.surfaceMuted};
  color: ${({ theme }) => theme.colors.textMuted};
  font-weight: 800;
  font-size: ${({ theme }) => theme.typeScale.body};
  flex-shrink: 0;

  &[data-active='true'] {
    border-color: ${({ theme }) => theme.colors.primary};
    background: ${({ theme }) => theme.colors.primarySoft};
    color: ${({ theme }) => theme.colors.primaryStrong};
  }
`

const ProgressText = styled.div`
  display: grid;
  gap: 2px;
  min-width: 0;
`

const ProgressLabel = styled.div`
  font-weight: 750;
  color: ${({ theme }) => theme.colors.textMuted};

  &[data-active='true'] {
    color: ${({ theme }) => theme.colors.text};
  }
`

const ProgressDesc = styled.div`
  color: ${({ theme }) => theme.colors.textFaint};
  font-size: ${({ theme }) => theme.typeScale.caption};
`

const ProgressConnector = styled.div`
  flex: 1;
  height: 2px;
  background: ${({ theme }) => theme.colors.border};
  margin: 0 ${({ theme }) => theme.space.md};
  min-width: 16px;

  @media (max-width: 768px) {
    display: none;
  }
`

const StepSection = styled(Card)`
  display: grid;
  gap: ${({ theme }) => theme.space.lg};
  padding: ${({ theme }) => theme.space.xl};
`

const StepHeader = styled.div`
  display: flex;
  align-items: flex-start;
  gap: ${({ theme }) => theme.space.lg};
  padding-bottom: ${({ theme }) => theme.space.lg};
  border-bottom: 1px solid ${({ theme }) => theme.colors.border};
`

const StepBadge = styled.div`
  display: grid;
  width: 36px;
  height: 36px;
  place-items: center;
  border-radius: 50%;
  background: ${({ theme }) => theme.colors.primarySoft};
  color: ${({ theme }) => theme.colors.primaryStrong};
  font-weight: 800;
  font-size: ${({ theme }) => theme.typeScale.body};
  flex-shrink: 0;
`

const StepHeading = styled.div`
  min-width: 0;
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
  font-size: ${({ theme }) => theme.typeScale.small};
`

const Checklist = styled.div`
  display: grid;
  gap: ${({ theme }) => theme.space.md};
`

const CheckItem = styled.div`
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: ${({ theme }) => theme.space.md};
  border: 1px solid ${({ theme }) => theme.colors.border};
  border-radius: ${({ theme }) => theme.radii.md};
  padding: ${({ theme }) => theme.space.lg};
  transition: background ${({ theme }) => theme.motion.fast} ${({ theme }) => theme.motion.easeOut};

  &:hover {
    background: ${({ theme }) => theme.colors.surfaceMuted};
  }
`

const CheckIconWrap = styled.span`
  display: grid;
  width: 36px;
  height: 36px;
  place-items: center;
  border-radius: 50%;
  background: ${({ theme }) => theme.colors.surfaceMuted};
  color: ${({ theme }) => theme.colors.textFaint};
  flex-shrink: 0;
`

const CheckInfo = styled.div`
  min-width: 0;
`

const Strong = styled.div`
  font-weight: 750;
`

const Meta = styled.div`
  margin-top: ${({ theme }) => theme.space.xs};
  color: ${({ theme }) => theme.colors.textMuted};
  font-size: ${({ theme }) => theme.typeScale.small};
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

const Helper = styled(Meta)`
  margin-top: 0;
`

const URLList = styled.div`
  display: grid;
  gap: ${({ theme }) => theme.space.md};
`

const URLCard = styled.div`
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: ${({ theme }) => theme.space.lg};
  border: 1px solid ${({ theme }) => theme.colors.border};
  border-radius: ${({ theme }) => theme.radii.md};
  padding: ${({ theme }) => theme.space.lg};

  button {
    display: inline-flex;
    align-items: center;
    gap: ${({ theme }) => theme.space.xs};
    flex-shrink: 0;
  }

  @media (max-width: 640px) {
    flex-direction: column;
    align-items: stretch;
  }
`

const URLCardLeft = styled.div`
  display: flex;
  align-items: flex-start;
  gap: ${({ theme }) => theme.space.md};
  min-width: 0;
`

const URLCardIcon = styled.span`
  display: grid;
  width: 36px;
  height: 36px;
  place-items: center;
  border-radius: ${({ theme }) => theme.radii.md};
  background: ${({ theme }) => theme.colors.primarySoft};
  color: ${({ theme }) => theme.colors.primary};
  flex-shrink: 0;
`

const URLValue = styled.code`
  display: block;
  margin-top: ${({ theme }) => theme.space.xs};
  color: ${({ theme }) => theme.colors.textMuted};
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: ${({ theme }) => theme.typeScale.small};
  overflow-wrap: anywhere;
`

const AuthForm = styled.div`
  display: grid;
  gap: ${({ theme }) => theme.space.lg};

  @media (min-width: 768px) {
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: start;
  }
`

const ActionBox = styled.div`
  display: flex;
  align-items: flex-start;

  @media (min-width: 768px) {
    padding-top: calc(1.4em + ${({ theme }) => theme.space.sm});
  }
`

const ResultBox = styled.div`
  display: grid;
  gap: ${({ theme }) => theme.space.md};
  border-radius: ${({ theme }) => theme.radii.lg};
  border-left: 3px solid ${({ theme }) => theme.colors.success};
  background: ${({ theme }) => theme.colors.successSoft};
  padding: ${({ theme }) => theme.space.lg};
`

const ResultHead = styled.div`
  display: flex;
  align-items: baseline;
  gap: ${({ theme }) => theme.space.md};
`

const RowActions = styled.div`
  display: flex;
  flex-wrap: wrap;
  gap: ${({ theme }) => theme.space.sm};

  button {
    display: inline-flex;
    align-items: center;
    gap: ${({ theme }) => theme.space.xs};
  }
`

const Notice = styled(Card)<{ $danger?: boolean }>`
  display: flex;
  align-items: center;
  gap: ${({ theme }) => theme.space.lg};
  padding: ${({ theme }) => theme.space.lg} ${({ theme }) => theme.space.xl};
  border-left: 3px solid ${({ theme, $danger }) => ($danger ? theme.colors.danger : theme.colors.info)};
`

const NoticeIconWrap = styled.span<{ $danger?: boolean }>`
  display: grid;
  width: 40px;
  height: 40px;
  place-items: center;
  border-radius: ${({ theme }) => theme.radii.md};
  background: ${({ theme, $danger }) => ($danger ? theme.colors.dangerSoft : theme.colors.infoSoft)};
  color: ${({ theme, $danger }) => ($danger ? theme.colors.danger : theme.colors.info)};
  flex-shrink: 0;
`
