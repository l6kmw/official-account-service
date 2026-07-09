import type { ReactNode } from 'react'
import styled from '@emotion/styled'
import { adminConfig } from '../config'

export type PageID = 'dashboard' | 'accounts' | 'articles' | 'publishes' | 'wechat-setup'

const pageTitles: Record<PageID, string> = {
  dashboard: '首页',
  accounts: '账号管理',
  articles: '文章',
  publishes: '发布记录',
  'wechat-setup': '微信配置'
}

function NavIcon({ id }: { id: PageID }) {
  const common = { width: 20, height: 20, viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', strokeWidth: 2, strokeLinecap: 'round' as const, strokeLinejoin: 'round' as const }
  switch (id) {
    case 'dashboard':
      return (<svg {...common}><rect x="3" y="3" width="7" height="7" rx="1.5" /><rect x="14" y="3" width="7" height="7" rx="1.5" /><rect x="3" y="14" width="7" height="7" rx="1.5" /><rect x="14" y="14" width="7" height="7" rx="1.5" /></svg>)
    case 'accounts':
      return (<svg {...common}><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2" /><circle cx="9" cy="7" r="4" /><path d="M22 21v-2a4 4 0 0 0-3-3.87" /><path d="M16 3.13a4 4 0 0 1 0 7.75" /></svg>)
    case 'articles':
      return (<svg {...common}><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" /><polyline points="14 2 14 8 20 8" /><line x1="8" y1="13" x2="16" y2="13" /><line x1="8" y1="17" x2="14" y2="17" /></svg>)
    case 'publishes':
      return (<svg {...common}><line x1="22" y1="2" x2="11" y2="13" /><polygon points="22 2 15 22 11 13 2 9 22 2" /></svg>)
    case 'wechat-setup':
      return (<svg {...common}><circle cx="12" cy="12" r="3" /><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" /></svg>)
  }
}

function LogoIcon() {
  return (
    <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
      <path d="M21 11.5a8.38 8.38 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7 8.38 8.38 0 0 1-3.8-.9L3 21l1.9-5.7a8.38 8.38 0 0 1-.9-3.8 8.5 8.5 0 0 1 4.7-7.6 8.38 8.38 0 0 1 3.8-.9h.5a8.48 8.48 0 0 1 8 8v.5z" />
    </svg>
  )
}

const navItems: Array<{ id: PageID; label: string }> = [
  { id: 'dashboard', label: '首页' },
  { id: 'accounts', label: '账号' },
  { id: 'articles', label: '文章' },
  { id: 'publishes', label: '发布记录' },
  { id: 'wechat-setup', label: '微信配置' }
]

export function AppShell({ children, currentPage, onNavigate }: { children: ReactNode; currentPage: PageID; onNavigate: (page: PageID) => void }) {
  return (
    <Shell>
      <SkipLink href="#main-content">跳到主内容</SkipLink>
      <AppHeader>
        <HeaderInner>
          <Brand>
            <LogoMark><LogoIcon /></LogoMark>
            <BrandText>
              <strong>Official Account</strong>
              <span>管理控制台</span>
            </BrandText>
          </Brand>
          <NavList aria-label="主导航">
            {navItems.map((item) => (
              <NavItem
                aria-current={item.id === currentPage ? 'page' : undefined}
                key={item.id}
                onClick={() => onNavigate(item.id)}
                type="button"
              >
                {item.id === currentPage ? <NavItemActive aria-hidden="true"><NavIcon id={item.id} /></NavItemActive> : <NavIconWrap aria-hidden="true"><NavIcon id={item.id} /></NavIconWrap>}
                {item.label}
              </NavItem>
            ))}
          </NavList>
        </HeaderInner>
      </AppHeader>

      <MainArea>
        <Topbar>
          <Breadcrumb>{pageTitles[currentPage]}</Breadcrumb>
          <TenantBadge>
            <TenantDot />
            {adminConfig.tenantID}
          </TenantBadge>
        </Topbar>
        <Main id="main-content">{children}</Main>
      </MainArea>
    </Shell>
  )
}

const Shell = styled.div`
  min-height: 100dvh;
  background: ${({ theme }) => theme.colors.background};
`

const SkipLink = styled.a`
  position: fixed;
  top: ${({ theme }) => theme.space.lg};
  left: ${({ theme }) => theme.space.lg};
  z-index: 10;
  border-radius: ${({ theme }) => theme.radii.md};
  padding: ${({ theme }) => theme.space.sm} ${({ theme }) => theme.space.lg};
  background: ${({ theme }) => theme.colors.primary};
  color: ${({ theme }) => theme.colors.surface};
  transform: translateY(-150%);

  &:focus {
    transform: translateY(0);
  }
`

const AppHeader = styled.header`
  display: flex;
  flex-direction: column;
  border-bottom: 1px solid ${({ theme }) => theme.colors.border};
  background: ${({ theme }) => theme.colors.surface};
  padding: ${({ theme }) => theme.space.lg} clamp(1rem, 3vw, 2rem);
`

const HeaderInner = styled.div`
  display: grid;
  width: min(100%, 1440px);
  margin: 0 auto;
  gap: ${({ theme }) => theme.space.md};
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

const NavList = styled.nav`
  display: flex;
  gap: ${({ theme }) => theme.space.sm};
  overflow-x: auto;
  padding-bottom: 1px;
`

const NavItem = styled.button`
  display: flex;
  align-items: center;
  flex: 0 0 auto;
  gap: ${({ theme }) => theme.space.md};
  min-height: 44px;
  border: 0;
  border-radius: ${({ theme }) => theme.radii.md};
  padding: 0 ${({ theme }) => theme.space.lg};
  background: transparent;
  color: ${({ theme }) => theme.colors.textMuted};
  font-weight: 650;
  position: relative;
  transition:
    background ${({ theme }) => theme.motion.fast} ${({ theme }) => theme.motion.easeOut},
    color ${({ theme }) => theme.motion.fast} ${({ theme }) => theme.motion.easeOut};

  &::before {
    position: absolute;
    left: 0;
    top: 50%;
    transform: translateY(-50%) scaleY(0);
    width: 3px;
    height: 60%;
    border-radius: ${({ theme }) => theme.radii.pill};
    background: ${({ theme }) => theme.colors.primary};
    content: '';
    transition: transform ${({ theme }) => theme.motion.base} ${({ theme }) => theme.motion.easeOut};
  }

  &[aria-current='page'] {
    background: ${({ theme }) => theme.colors.primarySoft};
    color: ${({ theme }) => theme.colors.primaryStrong};

    &::before {
      transform: translateY(-50%) scaleY(1);
    }
  }

  &:not([aria-current='page']):hover {
    background: ${({ theme }) => theme.colors.surfaceMuted};
    color: ${({ theme }) => theme.colors.text};
  }
`

const NavIconWrap = styled.span`
  display: grid;
  place-items: center;
  flex-shrink: 0;
  color: ${({ theme }) => theme.colors.textFaint};
  transition: color ${({ theme }) => theme.motion.fast} ${({ theme }) => theme.motion.easeOut};
`

const NavItemActive = styled.span`
  display: grid;
  place-items: center;
  flex-shrink: 0;
  color: ${({ theme }) => theme.colors.primary};
`

const TenantDot = styled.span`
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: ${({ theme }) => theme.colors.success};
  flex-shrink: 0;
`

const MainArea = styled.div`
  min-width: 0;
`

const Topbar = styled.header`
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: ${({ theme }) => theme.space.lg};
  border-bottom: 1px solid ${({ theme }) => theme.colors.border};
  background: oklch(100% 0 0 / 0.8);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  padding: ${({ theme }) => theme.space.md} clamp(1rem, 3vw, 2rem);
  position: sticky;
  top: 0;
  z-index: 5;
`

const Breadcrumb = styled.span`
  color: ${({ theme }) => theme.colors.textMuted};
  font-size: ${({ theme }) => theme.typeScale.small};
  font-weight: 650;
`

const TenantBadge = styled.div`
  display: flex;
  align-items: center;
  gap: ${({ theme }) => theme.space.sm};
  border-radius: ${({ theme }) => theme.radii.pill};
  background: ${({ theme }) => theme.colors.surfaceMuted};
  color: ${({ theme }) => theme.colors.text};
  font-size: ${({ theme }) => theme.typeScale.caption};
  font-weight: 650;
  padding: ${({ theme }) => theme.space.xs} ${({ theme }) => theme.space.md};
`

const Main = styled.main`
  width: min(100%, 1440px);
  margin: 0 auto;
  padding: clamp(1rem, 3vw, 2rem);
`
