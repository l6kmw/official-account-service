import { useCallback, useEffect, useRef, useState } from 'react'
import styled from '@emotion/styled'
import { getAdminSession, logoutAdmin } from './api/auth'
import { setAdminCSRFToken } from './api/client'
import { AppShell, type PageID } from './components/AppShell'
import { adminConfig } from './config'
import { AccountsPage } from './pages/AccountsPage'
import { ArticleEditorPage } from './pages/ArticleEditorPage'
import { ArticlesPage } from './pages/ArticlesPage'
import { DashboardPage } from './pages/DashboardPage'
import { LoginPage } from './pages/LoginPage'
import { MCPConfigPage } from './pages/MCPConfigPage'
import { PublishRecordsPage } from './pages/PublishRecordsPage'
import { WechatSetupPage } from './pages/WechatSetupPage'

type View =
  | { page: 'dashboard' }
  | { page: 'accounts' }
  | { page: 'articles' }
  | { page: 'publishes' }
  | { page: 'wechat-setup' }
  | { page: 'mcp-config' }
  | { page: 'article-new' }
  | { page: 'article-edit'; id: number }

type NavigateOptions = {
  skipDirtyCheck?: boolean
}

type AuthState =
  | { checking: true }
  | { checking: false; authenticated: true; loginEnabled: boolean }
  | { checking: false; authenticated: false; loginEnabled: boolean }

const discardEditorChangesMessage = '文章有未保存改动，确定离开吗？'

export function App() {
  const [view, setView] = useState<View>(() => parseHashView(window.location.hash))
  const [editorDirty, setEditorDirty] = useState(false)
  const [authState, setAuthState] = useState<AuthState>({ checking: true })
  const viewRef = useRef(view)
  const editorDirtyRef = useRef(editorDirty)
  const currentPage: PageID = view.page === 'article-new' || view.page === 'article-edit' ? 'articles' : view.page

  useEffect(() => {
    viewRef.current = view
  }, [view])

  useEffect(() => {
    editorDirtyRef.current = editorDirty
  }, [editorDirty])

  useEffect(() => {
    const currentHash = viewToHash(viewRef.current)
    if (window.location.hash !== currentHash) {
      replaceHash(currentHash)
    }

    function handleHashChange() {
      const next = parseHashView(window.location.hash)
      const nextHash = viewToHash(next)
      const current = viewRef.current
      if (nextHash === viewToHash(current)) {
        if (window.location.hash !== nextHash) replaceHash(nextHash)
        return
      }

      if (editorDirtyRef.current && !window.confirm(discardEditorChangesMessage)) {
        replaceHash(viewToHash(current))
        return
      }

      editorDirtyRef.current = false
      setEditorDirty(false)
      viewRef.current = next
      setView(next)
      if (window.location.hash !== nextHash) replaceHash(nextHash)
    }

    window.addEventListener('hashchange', handleHashChange)
    return () => window.removeEventListener('hashchange', handleHashChange)
  }, [])

  useEffect(() => {
    let active = true
    if (adminConfig.adminAPIKey) {
      setAuthState({ checking: false, authenticated: true, loginEnabled: false })
      return () => {
        active = false
      }
    }

    getAdminSession()
      .then((session) => {
        if (!active) return
        setAuthState({
          checking: false,
          authenticated: session.authenticated || !session.auth_enabled,
          loginEnabled: session.login_enabled
        })
      })
      .catch(() => {
        if (active) setAuthState({ checking: false, authenticated: false, loginEnabled: true })
      })

    return () => {
      active = false
    }
  }, [])

  useEffect(() => {
    function handleSessionExpired() {
      setAdminCSRFToken('')
      setAuthState({ checking: false, authenticated: false, loginEnabled: true })
    }
    window.addEventListener('admin-session-expired', handleSessionExpired)
    return () => window.removeEventListener('admin-session-expired', handleSessionExpired)
  }, [])

  const navigate = useCallback((next: View, options: NavigateOptions = {}) => {
    if (!options.skipDirtyCheck && editorDirtyRef.current && !window.confirm(discardEditorChangesMessage)) return
    setEditorDirty(false)
    editorDirtyRef.current = false

    const nextHash = viewToHash(next)
    if (window.location.hash === nextHash) {
      viewRef.current = next
      setView(next)
      return
    }
    window.location.hash = nextHash
  }, [])

  const handleAuthenticated = useCallback(() => {
    setAuthState({ checking: false, authenticated: true, loginEnabled: true })
  }, [])

  const handleLogout = useCallback(() => {
    logoutAdmin().finally(() => {
      setEditorDirty(false)
      editorDirtyRef.current = false
      setAuthState({ checking: false, authenticated: false, loginEnabled: true })
    })
  }, [])

  if (authState.checking) {
    return <LoadingShell>正在检查登录状态…</LoadingShell>
  }

  if (!authState.authenticated) {
    return <LoginPage loginEnabled={authState.loginEnabled} onAuthenticated={handleAuthenticated} />
  }

  return (
    <AppShell currentPage={currentPage} onLogout={authState.loginEnabled ? handleLogout : undefined} onNavigate={(page) => navigate({ page })}>
      {view.page === 'dashboard' ? <DashboardPage /> : null}
      {view.page === 'accounts' ? <AccountsPage /> : null}
      {view.page === 'publishes' ? <PublishRecordsPage /> : null}
      {view.page === 'wechat-setup' ? <WechatSetupPage /> : null}
      {view.page === 'mcp-config' ? <MCPConfigPage /> : null}
      {view.page === 'articles' ? (
        <ArticlesPage
          onCreate={() => navigate({ page: 'article-new' })}
          onEdit={(id) => navigate({ page: 'article-edit', id })}
        />
      ) : null}
      {view.page === 'article-new' ? (
        <ArticleEditorPage onBack={(options) => navigate({ page: 'articles' }, options)} onDirtyChange={setEditorDirty} />
      ) : null}
      {view.page === 'article-edit' ? (
        <ArticleEditorPage articleID={view.id} onBack={(options) => navigate({ page: 'articles' }, options)} onDirtyChange={setEditorDirty} />
      ) : null}
    </AppShell>
  )
}

const LoadingShell = styled.main`
  min-height: 100dvh;
  display: grid;
  place-items: center;
  background: ${({ theme }) => theme.colors.background};
  color: ${({ theme }) => theme.colors.textMuted};
  font-size: ${({ theme }) => theme.typeScale.small};
  font-weight: 650;
`

function parseHashView(hash: string): View {
  const path = normalizeHashPath(hash)

  switch (path) {
    case '/':
    case '/dashboard':
      return { page: 'dashboard' }
    case '/accounts':
      return { page: 'accounts' }
    case '/articles':
      return { page: 'articles' }
    case '/articles/new':
      return { page: 'article-new' }
    case '/publishes':
      return { page: 'publishes' }
    case '/wechat-setup':
      return { page: 'wechat-setup' }
    case '/mcp-config':
      return { page: 'mcp-config' }
  }

  const editMatch = path.match(/^\/articles\/(\d+)\/edit$/)
  if (editMatch) {
    const id = Number(editMatch[1])
    if (Number.isSafeInteger(id) && id > 0) return { page: 'article-edit', id }
  }

  if (path.startsWith('/articles')) return { page: 'articles' }
  return { page: 'dashboard' }
}

function normalizeHashPath(hash: string) {
  const withoutHash = hash.startsWith('#') ? hash.slice(1) : hash
  const pathOnly = withoutHash.split('?')[0] ?? ''
  const withLeadingSlash = pathOnly.startsWith('/') ? pathOnly : `/${pathOnly}`
  return withLeadingSlash.replace(/\/{2,}/g, '/').replace(/\/+$/, '') || '/'
}

function viewToHash(view: View) {
  switch (view.page) {
    case 'dashboard':
      return '#/'
    case 'accounts':
      return '#/accounts'
    case 'articles':
      return '#/articles'
    case 'article-new':
      return '#/articles/new'
    case 'article-edit':
      return `#/articles/${view.id}/edit`
    case 'publishes':
      return '#/publishes'
    case 'wechat-setup':
      return '#/wechat-setup'
    case 'mcp-config':
      return '#/mcp-config'
  }
}

function replaceHash(hash: string) {
  try {
    const url = new URL(window.location.href)
    url.hash = hash
    window.history.replaceState(null, '', url)
  } catch {
    if (window.location.hash !== hash) window.location.hash = hash
  }
}
