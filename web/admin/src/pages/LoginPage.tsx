import { FormEvent, useState } from 'react'
import styled from '@emotion/styled'
import { loginAdmin, type AdminSessionStatus } from '../api/auth'
import { getErrorMessage } from '../api/client'
import { Button } from '../components/Button'
import { Card } from '../components/Card'

type LoginPageProps = {
  loginEnabled: boolean
	onAuthenticated: (session: AdminSessionStatus) => void
}

const iconAttrs = { viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', strokeWidth: 2, strokeLinecap: 'round' as const, strokeLinejoin: 'round' as const }

function LogoIcon() {
  return <svg {...iconAttrs} width="22" height="22"><path d="M21 11.5a8.38 8.38 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7 8.38 8.38 0 0 1-3.8-.9L3 21l1.9-5.7a8.38 8.38 0 0 1-.9-3.8 8.5 8.5 0 0 1 4.7-7.6 8.38 8.38 0 0 1 3.8-.9h.5a8.48 8.48 0 0 1 8 8v.5z" /></svg>
}

function ShieldIcon() {
  return <svg {...iconAttrs} width="18" height="18"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" /><path d="M9 12l2 2 4-4" /></svg>
}

export function LoginPage({ loginEnabled, onAuthenticated }: LoginPageProps) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!loginEnabled || submitting) return
    setError('')
    setSubmitting(true)
    try {
      const session = await loginAdmin(username, password)
      if (session.authenticated) {
		onAuthenticated(session)
        return
      }
      setError('登录未完成，请确认账号密码后重试。')
    } catch (err: unknown) {
      setError(getErrorMessage(err))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Shell>
      <LoginPanel>
        <Brand>
          <LogoMark><LogoIcon /></LogoMark>
          <BrandText>
            <strong>Official Account</strong>
            <span>管理控制台</span>
          </BrandText>
        </Brand>
        <CopyBlock>
          <SecurityBadge><ShieldIcon />用户访问</SecurityBadge>
          <Title>登录控制台</Title>
          <Description>请输入你的账号进入独立数据空间。</Description>
        </CopyBlock>

        {!loginEnabled ? (
          <ConfigNotice role="alert">
            用户登录尚未完成服务端配置，请先配置初始管理员账号和会话密钥。
          </ConfigNotice>
        ) : null}

        <Form onSubmit={submit}>
          <Field>
            <Label>账号</Label>
            <Input
              autoComplete="username"
              disabled={!loginEnabled || submitting}
              id="admin-username"
              onChange={(event) => setUsername(event.target.value)}
              value={username}
            />
          </Field>
          <Field>
            <Label>密码</Label>
            <Input
              autoComplete="current-password"
              disabled={!loginEnabled || submitting}
              id="admin-password"
              onChange={(event) => setPassword(event.target.value)}
              type="password"
              value={password}
            />
          </Field>
          {error ? <InlineError role="alert">{error}</InlineError> : null}
          <Button disabled={!loginEnabled || submitting || !username.trim() || !password} type="submit">
            {submitting ? '登录中…' : '登录'}
          </Button>
        </Form>
      </LoginPanel>
    </Shell>
  )
}

const Shell = styled.main`
  min-height: 100dvh;
  display: grid;
  place-items: center;
  padding: clamp(1rem, 4vw, 3rem);
  background:
    linear-gradient(135deg, ${({ theme }) => theme.colors.primarySoft}, transparent 38%),
    ${({ theme }) => theme.colors.background};
`

const LoginPanel = styled(Card)`
  width: min(100%, 440px);
  display: grid;
  gap: ${({ theme }) => theme.space.xl};
  padding: clamp(1.25rem, 4vw, 2rem);
`

const Brand = styled.div`
  display: flex;
  align-items: center;
  gap: ${({ theme }) => theme.space.md};
`

const LogoMark = styled.div`
  display: grid;
  width: 40px;
  height: 40px;
  place-items: center;
  border-radius: ${({ theme }) => theme.radii.md};
  background: ${({ theme }) => theme.colors.primary};
  color: ${({ theme }) => theme.colors.surface};
  flex-shrink: 0;
`

const BrandText = styled.div`
  display: grid;
  line-height: 1.25;

  strong {
    font-size: ${({ theme }) => theme.typeScale.small};
  }

  span {
    color: ${({ theme }) => theme.colors.textMuted};
    font-size: ${({ theme }) => theme.typeScale.caption};
  }
`

const CopyBlock = styled.div`
  display: grid;
  gap: ${({ theme }) => theme.space.sm};
`

const SecurityBadge = styled.div`
  width: fit-content;
  display: inline-flex;
  align-items: center;
  gap: ${({ theme }) => theme.space.sm};
  border-radius: ${({ theme }) => theme.radii.pill};
  background: ${({ theme }) => theme.colors.successSoft};
  color: ${({ theme }) => theme.colors.primaryStrong};
  font-size: ${({ theme }) => theme.typeScale.caption};
  font-weight: 700;
  padding: ${({ theme }) => theme.space.xs} ${({ theme }) => theme.space.md};
`

const Title = styled.h1`
  margin: 0;
  color: ${({ theme }) => theme.colors.text};
  font-size: ${({ theme }) => theme.typeScale.section};
  line-height: 1.2;
`

const Description = styled.p`
  margin: 0;
  color: ${({ theme }) => theme.colors.textMuted};
  font-size: ${({ theme }) => theme.typeScale.small};
`

const ConfigNotice = styled.div`
  border: 1px solid ${({ theme }) => theme.colors.warning};
  border-radius: ${({ theme }) => theme.radii.md};
  background: ${({ theme }) => theme.colors.warningSoft};
  color: ${({ theme }) => theme.colors.text};
  font-size: ${({ theme }) => theme.typeScale.small};
  padding: ${({ theme }) => theme.space.md};
`

const Form = styled.form`
  display: grid;
  gap: ${({ theme }) => theme.space.lg};
`

const Field = styled.label`
  display: grid;
  gap: ${({ theme }) => theme.space.sm};
`

const Label = styled.span`
  color: ${({ theme }) => theme.colors.textMuted};
  font-size: ${({ theme }) => theme.typeScale.small};
  font-weight: 650;
`

const Input = styled.input`
  width: 100%;
  min-height: 44px;
  border: 1px solid ${({ theme }) => theme.colors.border};
  border-radius: ${({ theme }) => theme.radii.md};
  background: ${({ theme }) => theme.colors.surface};
  color: ${({ theme }) => theme.colors.text};
  padding: 0 ${({ theme }) => theme.space.md};

  &:focus {
    border-color: ${({ theme }) => theme.colors.primary};
    box-shadow: 0 0 0 3px ${({ theme }) => theme.colors.primarySoft};
  }

  &:disabled {
    cursor: not-allowed;
    opacity: 0.7;
  }
`

const InlineError = styled.div`
  border-radius: ${({ theme }) => theme.radii.md};
  background: ${({ theme }) => theme.colors.dangerSoft};
  color: ${({ theme }) => theme.colors.danger};
  font-size: ${({ theme }) => theme.typeScale.small};
  font-weight: 650;
  padding: ${({ theme }) => theme.space.md};
`
