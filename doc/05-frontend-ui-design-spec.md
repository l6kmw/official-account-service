# Official Account Service 前端 UI/UX 设计规范

> 本文基于 `frontend-design` 与 `ui-ux-pro-max` 的设计规则，为公众号管理后台定义可执行的视觉方向、页面结构、组件规范、交互状态和验收标准。后续前端实现必须保持前后端分离，技术栈为 React + Vite + TypeScript + Emotion。

## 1. 设计定位

### 1.1 产品目的

这套后台不是泛用 CMS，也不是复杂运营平台。它服务于一个很明确的场景：

```text
让用户 / AI Agent 管理微信公众号授权账号，并完成文章创建、素材准备、发布和状态追踪。
```

核心用户：

- 平台管理员
- 内容运营人员
- 使用 OpenClaw AI 平台的 AI Agent 操作者
- 后续负责真实微信联调和上线运维的人

因此界面要优先支持：

- 快速判断当前配置是否可用
- 快速找到待处理文章和发布失败记录
- 清晰知道下一步该做什么
- 不暴露任何敏感凭证

---

## 2. 视觉方向

### 2.1 方向名称：Modern SaaS Console / 现代 SaaS 管理控制台

第一版采用大多数成熟控制台常见的现代 SaaS 后台风格：结构清晰、信息密度适中、操作入口稳定、状态反馈明确。它不需要做成强风格，也不需要刻意极简，而是优先保证长期可维护、多人可理解、真实业务场景可用。

关键词：

```text
清晰、稳定、专业、中性、信息密度适中、表格友好、状态明确
```

视觉气质：

- 浅色为主，中性背景，蓝色主操作
- 左侧导航 + 顶部上下文 + 主内容区
- Dashboard 使用摘要卡片和状态区
- 列表页以表格、筛选、批量操作为主
- 表单页强调字段分组和保存反馈
- 有现代感，但不追求强装饰或概念化表达

### 2.2 应避免的风格

禁止默认落入以下视觉套路：

- 深色背景 + 蓝紫霓虹渐变
- 大量玻璃拟态卡片
- 强概念化插画或装饰纹理
- 所有内容都包进厚重圆角阴影卡片
- 过度极简导致操作入口不明显
- Hero 区大数字 + 小说明 + 渐变强调的模板化布局
- Emoji 当图标
- 图标堆满页面但不承担信息职责

---

## 3. 色彩系统

使用 OKLCH 定义语义色，不在组件里散落 raw hex。第一版采用成熟控制台常见的中性色 + 蓝色主色 + 语义状态色。

### 3.1 色彩角色

| 角色 | 用途 | 说明 |
|---|---|---|
| `background` | 页面背景 | 接近白色但不使用纯白 |
| `surface` | 表面 / 面板 | 用于主要内容区域 |
| `surfaceMuted` | 次级表面 | 用于筛选区、空状态、轻提示 |
| `text` | 主文字 | 非纯黑 |
| `textMuted` | 次级文字 | 描述、辅助信息 |
| `border` | 分割线 / 边框 | 清晰但不抢眼 |
| `primary` | 主行动 / 当前导航 | 稳定蓝色，管理后台通用且克制 |
| `success` | 成功 / published | 绿色 |
| `warning` | 处理中 / publishing | 琥珀色 |
| `danger` | 失败 / revoked | 红色 |
| `info` | 配置提示 | 蓝色，少量使用 |

### 3.2 推荐 Token

```ts
export const colors = {
  background: 'oklch(98% 0.006 250)',
  surface: 'oklch(100% 0 0)',
  surfaceMuted: 'oklch(96% 0.008 250)',

  text: 'oklch(24% 0.018 250)',
  textMuted: 'oklch(48% 0.018 250)',
  textFaint: 'oklch(65% 0.014 250)',
  border: 'oklch(88% 0.01 250)',

  primary: 'oklch(48% 0.12 255)',
  primarySoft: 'oklch(93% 0.035 255)',
  primaryStrong: 'oklch(36% 0.12 255)',

  success: 'oklch(46% 0.11 150)',
  successSoft: 'oklch(94% 0.045 150)',
  warning: 'oklch(62% 0.13 78)',
  warningSoft: 'oklch(95% 0.055 78)',
  danger: 'oklch(52% 0.14 25)',
  dangerSoft: 'oklch(94% 0.05 25)',
  info: 'oklch(50% 0.11 245)',
  infoSoft: 'oklch(94% 0.04 245)'
}
```

### 3.3 色彩使用规则

- 主页面背景用 `background`
- 主要按钮只用 `primary`
- 配置缺失、错误、失败必须用语义色 + 文案说明
- 状态色必须配合文字，不允许只靠颜色表达含义
- 表格行 hover 用 `surfaceMuted` 或 `primarySoft`
- 禁止在组件内直接写任意 hex 色值

---

## 4. 字体与排版

### 4.1 字体方向

由于这是中文为主的管理后台，第一版优先使用系统字体，保证加载快、稳定、易维护。

推荐：

```ts
export const fonts = {
  heading: 'system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif',
  body: 'system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif',
  numeric: 'system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif'
}
```

说明：

- 第一版不引入 Web Font，减少构建和加载复杂度
- 标题通过字号、字重、间距建立层级，不靠特殊字体
- 数字列启用 `font-variant-numeric: tabular-nums`
- 后续如果需要品牌化，再单独评估字体方案

### 4.2 字号系统

```ts
export const typeScale = {
  caption: '0.75rem',   // 12px
  small: '0.875rem',    // 14px
  body: '1rem',         // 16px
  lead: '1.125rem',     // 18px
  title: '1.5rem',      // 24px
  section: '2rem',      // 32px
  display: 'clamp(2.5rem, 5vw, 4.75rem)'
}
```

### 4.3 排版规则

- body 最小 16px
- 行高默认 1.6
- 页面主标题可以使用 fluid type
- 表格、按钮、标签不使用 fluid type
- 中文段落每行不要过长，详情页正文区域控制在 72ch 左右
- 表格数字列使用 tabular numbers，避免数值跳动

---

## 5. 空间与布局

### 5.1 基础间距

采用 4pt spacing system：

```ts
export const space = {
  xs: '0.25rem',  // 4
  sm: '0.5rem',   // 8
  md: '0.75rem',  // 12
  lg: '1rem',     // 16
  xl: '1.5rem',   // 24
  '2xl': '2rem',  // 32
  '3xl': '3rem',  // 48
  '4xl': '4rem'   // 64
}
```

### 5.2 页面布局

桌面端采用典型控制台结构：

```text
左侧导航 240px
顶部上下文栏 / 操作栏
右侧内容自适应
页面最大内容宽度 1440px
```

移动端：

```text
顶部栏 + 抽屉导航
关键操作不隐藏
表格改为卡片式列表
```

### 5.3 布局原则

- Dashboard 摘要信息可以使用卡片，但不要嵌套卡片
- 页面首屏要能看到当前系统状态和主要待办
- 内容区以“清晰分组 + 适度分割线”为主
- Dashboard 可以采用常见两栏布局：主要指标区 + 配置提醒区
- 表格页面保持标准控制台体验：筛选、搜索、分页、行操作清晰

---

## 6. 全局信息架构

### 6.1 一级导航

建议一级导航：

```text
Dashboard
账号
文章
发布记录
微信配置
任务队列
```

对应路由：

```text
/dashboard
/accounts
/articles
/publish-records
/wechat-setup
/task-queues
```

任务队列可后置，如果第一版不做页面，可以在导航中暂不出现。

### 6.2 页面优先级

第一批实现优先级：

1. Dashboard
2. 文章列表
3. 文章编辑
4. 账号列表
5. 发布记录
6. 微信配置检查

---

## 7. 页面设计规范

## 7.1 Dashboard

### 页面目的

让用户 10 秒内知道：

- 当前有没有公众号授权
- 有多少文章
- 有多少发布中 / 发布失败
- 微信配置是否还缺东西

### 布局建议

```text
顶部：页面标题 + 当前 tenant selector
主体：状态摘要 + 配置提醒
下方：最近文章 / 最近发布记录
```

### 关键组件

- `MetricStrip`：横向指标条，不做模板化大卡片堆砌
- `SetupNotice`：配置提醒，使用 info / warning 语义色
- `RecentArticles`：最近文章列表
- `RecentPublishes`：最近发布记录

### 空状态

如果没有文章：

```text
还没有文章草稿
先创建一篇文章，微信配置完成后即可上传封面并发布。
```

如果没有账号：

```text
还没有授权公众号
第三方平台审核通过后，从“微信配置”页面生成授权链接。
```

---

## 7.2 账号页

### 页面目的

展示已授权公众号及其可用性状态。

### 列表字段

- 头像
- 名称
- AppID
- 状态
- 最近同步时间
- token 缓存状态入口

### 状态 Badge

| 状态 | 颜色 | 文案 |
|---|---|---|
| active | success | 可用 |
| revoked | danger | 已取消授权 |
| refresh_failed | danger | Token 刷新失败 |
| unknown | muted | 未知 |

### 禁止

- 不展示 access token
- 不展示 refresh token
- 不展示 app secret

---

## 7.3 文章列表页

### 页面目的

快速管理草稿、定位发布失败文章。

### 布局

桌面：表格。

移动：文章卡片列表。

字段：

- 标题
- 作者
- 摘要
- Authorizer ID
- 状态
- 更新时间
- 操作

### 操作

- 编辑
- 删除
- 发布
- 查看发布记录

### 删除交互

优先使用 Undo Toast，而不是默认 confirm。

第一版如果实现成本较高，可先用确认对话，但必须在后续优化为 Undo。

---

## 7.4 文章编辑页

### 页面目的

完成文章草稿创建与编辑。

### 布局建议

```text
左侧：编辑表单
右侧：预览 / 发布准备状态
```

移动端：

```text
上方表单，下方预览，使用分段切换
```

### 字段分组

基础信息：

- 标题
- 作者
- 摘要
- Authorizer ID

正文：

- HTML textarea
- 预览区域

封面：

- cover_media_asset_id
- 后续接封面上传

### 设计细节

- 标题输入要明显，像编辑器标题区
- 正文 textarea 用清晰等宽字体，但不要让整个产品变成“开发者工具”风格
- 预览区使用干净的浅色表面
- 未保存改动时离开页面要提示

---

## 7.5 发布记录页

### 页面目的

让用户定位发布状态、失败原因和微信侧 ID。

### 列表字段

- 发布记录 ID
- 文章 ID
- Authorizer ID
- 微信 Publish ID
- 微信 Article ID
- 状态
- 错误信息
- 提交时间
- 完成时间

### 状态色

| 状态 | 色彩 |
|---|---|
| publishing | warning |
| published | success |
| failed | danger |

### 交互

- publishing 状态可显示“同步状态”按钮
- failed 状态展开失败原因
- 错误信息要可复制，但不展示敏感字段

---

## 7.6 微信配置检查页

### 页面目的

在第三方平台申请阶段，帮助用户知道还缺什么。

### 页面结构

```text
1. 开放平台配置清单
2. 需要填写到微信后台的 URL
3. 需要填写到 YAML 的配置项
4. 授权 URL 生成区域，配置未就绪时禁用或显示说明
```

### 配置清单 UI

用 checklist，而不是表格。

项目：

- Component AppID
- Component AppSecret
- Verify Token
- EncodingAESKey
- Refresh Token Encryption Key
- 授权事件接收 URL
- 授权后公众号消息与事件接收 URL
- 授权回调 URL

### 安全原则

只展示：

```text
已配置 / 未配置
```

不展示真实 secret 值。

---

## 8. 组件系统

## 8.1 AppShell

职责：

- 左侧导航
- 顶部当前 tenant
- 当前页面标题
- 主内容区域

要求：

- 桌面固定侧边栏
- 移动端导航折叠
- 支持键盘跳转主内容 skip link

## 8.2 Button

Variant：

```text
primary
secondary
ghost
danger
```

状态：

```text
default
hover
focus-visible
active
disabled
loading
```

要求：

- 高度不低于 44px
- 有 visible focus ring
- loading 时禁用重复点击

## 8.3 Field

要求：

- 必须有 visible label
- helper text 常驻
- error text 在字段下方
- 错误状态使用 `aria-describedby`
- 不只用 placeholder 当标签

## 8.4 StatusBadge

输入：

```ts
type Status =
  | 'draft'
  | 'publishing'
  | 'published'
  | 'failed'
  | 'active'
  | 'revoked'
  | 'refresh_failed'
```

输出：

- 颜色 + 文案 + 轻量图形标记
- 不只靠颜色表达状态

## 8.5 DataTable

要求：

- 桌面表格
- 移动端转卡片
- loading skeleton
- empty state
- error state
- 数字列 tabular nums
- 行操作不依赖 hover 才能发现

## 8.6 EmptyState

每个空状态必须回答：

1. 当前为什么为空
2. 用户下一步可以做什么
3. 如果依赖微信配置，去哪里完成配置

示例：

```text
暂无发布记录
创建文章并完成封面上传后，可以在这里跟踪微信发布结果。
```

## 8.7 Toast

用途：

- 保存成功
- 删除后撤销
- 网络错误提示

要求：

- 自动消失 3~5 秒
- 不抢焦点
- `aria-live="polite"`
- destructive action 优先支持 undo

---

## 9. 交互规范

### 9.1 Loading

- 首屏数据用 skeleton，不用大 spinner
- 按钮提交用局部 loading
- 表格加载时保留表头，表体 skeleton

### 9.2 Error

错误文案必须包含恢复路径。

示例：

```text
文章保存失败。请检查标题和 Authorizer ID 后重试。
```

而不是：

```text
Invalid request
```

### 9.3 Not Implemented

后端 `501 not_implemented` 对用户翻译为：

```text
该能力还没有完成配置。请先完成微信开放平台配置和公众号授权。
```

### 9.4 表单校验

- 必填项显式标记
- blur 后校验
- submit 时聚焦第一个错误字段
- 不在每个字符输入时疯狂报错

### 9.5 动效

动效只用于表达状态变化。

建议：

- 页面进入：一次轻微 stagger reveal
- hover：轻微 transform / 色彩变化
- toast：slide + fade
- 折叠区域：使用 grid-template-rows 或 opacity/transform

禁止：

- 动画宽高导致 layout shift
- 装饰性循环动效
- 大范围 parallax
- 超过 500ms 的慢动画

必须支持：

```css
@media (prefers-reduced-motion: reduce) {
  * {
    animation-duration: 0.01ms !important;
    transition-duration: 0.01ms !important;
  }
}
```

---

## 10. 响应式规范

### 10.1 Breakpoints

```ts
export const breakpoints = {
  sm: '640px',
  md: '768px',
  lg: '1024px',
  xl: '1280px'
}
```

### 10.2 移动端规则

- 不允许横向滚动
- 表格转卡片
- 主要 CTA 仍然可见
- 表单字段单列展示
- 触控目标不低于 44px
- 不依赖 hover 展示关键操作

### 10.3 桌面端规则

- 左侧导航固定
- 内容宽度不要无限拉长
- 数据表格可横向容纳更多字段，但不要压缩到不可读
- 详情页使用左右分栏提升效率

---

## 11. Accessibility 清单

每个页面必须满足：

- 语义化 heading：h1 -> h2 -> h3 不跳级
- 所有按钮有文本或 aria-label
- icon-only 按钮必须有 aria-label
- 可键盘导航
- focus-visible 明显
- 正文文本对比度 ≥ 4.5:1
- UI 组件对比度 ≥ 3:1
- 表单 label 可见
- 错误提示与字段关联
- Toast 使用 aria-live
- 动效尊重 prefers-reduced-motion
- 不禁用浏览器缩放

---

## 12. Emotion 实现约定

### 12.1 Theme 结构

```ts
export const theme = {
  colors,
  fonts,
  typeScale,
  space,
  radii: {
    none: '0',
    sm: '6px',
    md: '10px',
    lg: '18px',
    pill: '999px'
  },
  shadows: {
    hairline: '0 0 0 1px var(--line)',
    soft: '0 18px 50px oklch(30% 0.04 70 / 0.10)',
    lift: '0 24px 70px oklch(30% 0.04 70 / 0.14)'
  },
  motion: {
    fast: '140ms',
    base: '220ms',
    slow: '320ms',
    easeOut: 'cubic-bezier(0.16, 1, 0.3, 1)'
  }
}
```

### 12.2 组件规则

- 组件只使用 theme token
- 不在组件中直接写散落颜色
- 复杂页面布局可用 Emotion `css`
- 可复用组件优先用 `styled`
- variant 通过 props 控制

### 12.3 CSS Reset / Global

Global styles 应包含：

- box-sizing
- body font
- background
- color
- focus-visible
- reduced motion
- button/input font inherit
- responsive media defaults

---

## 13. 首批页面的验收标准

### Dashboard

- [ ] stats 可加载
- [ ] loading / empty / error 都有状态
- [ ] 移动端不横向滚动
- [ ] 不使用模板化大卡片堆砌

### 文章列表

- [ ] 可查看文章列表
- [ ] 状态 badge 可识别
- [ ] 删除有保护机制
- [ ] 发布不可用时能解释原因

### 文章编辑

- [ ] title / authorizer_id 有校验
- [ ] HTML 正文可编辑
- [ ] 有预览区域
- [ ] 未保存离开有提示

### 账号页

- [ ] 空状态可指导用户去微信配置页
- [ ] 不展示敏感字段
- [ ] 状态文案清楚

### 发布记录页

- [ ] 可查看发布记录
- [ ] failed 展示原因
- [ ] publishing 可触发同步
- [ ] 不展示敏感字段

### 微信配置页

- [ ] 清楚展示微信后台要填哪些 URL
- [ ] 清楚展示 YAML 要配置哪些 key
- [ ] secret 只显示配置状态，不显示值

---

## 14. 设计质量检查

每次实现后做以下检查：

- [ ] 页面符合现代 SaaS 控制台习惯，不刻意极简，也不过度装饰
- [ ] 没有默认蓝紫渐变霓虹风
- [ ] 没有滥用卡片、阴影、插画和装饰纹理
- [ ] 页面主任务一眼可见
- [ ] 空状态能指导下一步
- [ ] 所有交互都有 loading / error / success 反馈
- [ ] 键盘可操作
- [ ] 移动端可用
- [ ] 不泄露 token / secret / refresh

---

## 15. 下一步建议

下一步不要直接写所有页面。建议先做一个设计原型级实现：

```text
Phase 1A：创建 web/admin 工程 + AppShell + Dashboard 静态版
```

范围：

- React + Vite + TypeScript + Emotion
- theme / global styles
- AppShell
- Sidebar
- Dashboard 静态数据页面
- 不接真实 API

验收：

- `npm run dev` 可启动
- `npm run build` 通过
- 页面风格符合“现代 SaaS 管理控制台”方向
- 前后端完全分离
