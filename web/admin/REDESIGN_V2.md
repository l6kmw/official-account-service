# Frontend Complete Redesign — V2

## Rules
- DO NOT touch src/api/ files, keep all hooks/state/handlers logic
- Rewrite ALL 12 files from scratch (theme, global, Card, Button, StatusBadge, AppShell, 6 pages)
- All Chinese text stays as-is
- After changes run: cd web/admin && npm run build

## Tech notes
- @emotion/babel-plugin is installed, vite.config.ts has jsxImportSource: '@emotion/react' + babel plugin
- Component selectors like ${Component} ARE supported now

## Theme (theme.ts)
WeChat green primary + warm neutrals:
- background: oklch(97.5% 0.008 75)
- surface: oklch(100% 0 0)  
- surfaceMuted: oklch(96% 0.01 75)
- text: oklch(25% 0.02 75)
- textMuted: oklch(48% 0.015 75)
- textFaint: oklch(65% 0.012 75)
- border: oklch(90% 0.008 75)
- primary: oklch(52% 0.13 155) (WeChat green)
- primarySoft: oklch(94% 0.04 155)
- primaryStrong: oklch(40% 0.13 155)
- success/warning/danger/info: keep same hues but warm
- shadows: xs, sm, soft, lift (warm-tinted, hue 75)
- radii: sm 8px, md 12px, lg 18px, pill 999px
- Add breakpoints: sm 640px, md 768px, lg 1024px, xl 1280px

## AppShell.tsx — Complete rewrite
Layout: fixed sidebar 248px + main area
Sidebar:
  - Brand: chat bubble SVG logo (primary bg) + "Official Account" / "管理控制台"
  - Nav items with 20x20 stroke SVG icons (grid/users/file/send/gear)
  - Active: 3px left accent bar + primarySoft bg + primaryStrong text
  - Hover: surfaceMuted bg
  - Bottom: tenant badge with green dot
Topbar: sticky, backdrop-blur, left=breadcrumb page name, right=tenant pill
Mobile: sidebar becomes horizontal scroll strip below topbar, icons+labels

## DashboardPage.tsx — "Command Center" layout
NEW LAYOUT (not the old template):
- Welcome hero: greeting + date + system status badge (NOT the old eyebrow+title+desc)
- 2x2 metric grid: each card has icon badge top-right, large number, sub-label, colored bottom accent line matching metric type (green/blue/amber/red)
- Below: 2-column layout
  - Left: "文章状态" panel with horizontal bar-style progress rows (not just text+number)
  - Right: "发布状态" panel with same bar-style rows + colored dots
- Bottom: "配置准备" panel as a 3-step progress card with checkmarks

## AccountsPage.tsx — "Card Gallery" layout  
NEW LAYOUT:
- Header: title + refresh + add button (same as before but cleaner)
- Account list as CARDS in a responsive grid (2-3 per row on desktop), NOT a table
- Each card: avatar (48px, ring border), name, AppID, status badge, last sync, subtle hover lift
- Mobile: single column cards
- Authorization panel: cleaner card with inline form
- Empty state: centered with icon + helpful text + CTA button

## ArticlesPage.tsx — "Editorial Board" layout
NEW LAYOUT:
- Header with filter tabs: 全部 / 草稿 / 发布中 / 已发布 / 失败 (clickable, pill-style, count badges)
- Article list as horizontal wide cards (NOT table):
  - Left: 4px colored status strip (green=published, amber=publishing, red=failed, gray=draft)
  - Title (bold, 16px) + digest (muted, 1 line truncate)
  - Bottom row: status badge + author + time + action buttons (edit/publish/delete)
- Empty state with icon + CTA
- Mobile: same cards, single column

## ArticleEditorPage.tsx — "Studio" layout
NEW LAYOUT:
- Top bar: back button + title + status badge + save button
- Split layout: left 55% form, right 45% sticky preview
- Form: 
  - Title input is LARGE (24px font, no label, placeholder "输入文章标题…")
  - Fields in clean grouped sections with subtle dividers (not cards-within-cards)
  - Upload zones: dashed border drop areas with icon
- Preview panel: looks like a phone screen mockup (375px width, rounded, shadow, white bg)
  - Shows title + author + HTML content rendered

## PublishRecordsPage.tsx — "Timeline + Detail" layout
NEW LAYOUT:
- Split: left 60% record list, right 40% detail panel (sticky)
- Record list as timeline-style items (NOT table):
  - Left: colored status dot connected by vertical line
  - Content: "文章 #ID" + status badge + submit time + wechat publish ID
  - Hover: subtle bg highlight
  - Selected: left border accent + primarySoft bg
- Detail panel: clean key-value list with section dividers
  - Failed: red error box with copy button
- Mobile: stacked, detail below list

## WechatSetupPage.tsx — "Setup Wizard" layout  
NEW LAYOUT:
- Progress indicator: 3 steps with numbered circles + connecting line
  Step 1: 配置清单 (active) → Step 2: 回调地址 → Step 3: 授权URL
- Step 1: Config checklist as cards with status icons (check-circle / circle)
- Step 2: URL cards with copy buttons, domain input at top
- Step 3: Auth URL generator form + result display
- All in clean vertical flow, not cramped grid

## Component updates
Card: border + soft shadow (xs on mobile, sm on desktop), hover optional
Button: 4 variants, compact (small font), scale(0.98) active, nowrap, primary has shadow
StatusBadge: 6px dot, tinted border, pill shape, backdrop-filter blur

## Empty states (all pages)
Each empty state: 64px muted icon SVG + title + description + optional CTA button

## Key visual elements to use everywhere
- Left accent bars (3px) for active/selected states
- Colored dots (8px) for status indicators  
- Subtle hover lifts (translateY -2px + shadow)
- Section dividers: 1px border, not full card wrappers
- Rounded corners everywhere (12px min)
- Soft shadows on cards (not just borders)
