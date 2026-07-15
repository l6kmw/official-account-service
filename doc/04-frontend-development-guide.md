# Official Account Service 前端开发文档

> 本文用于后续按用户指令逐步开发公众号管理后台前端。当前阶段只定义方向、边界、页面、接口和验收标准；不默认启动完整前端实现，后续每一步以用户明确指令为准。

## 1. 当前背景

后端 API 已基本完成，微信公众号第三方平台配置仍在申请中。因此前端先做“可联调、可演示、可扩展”的管理后台，不依赖真实微信开放平台配置即可完成大部分页面开发。

当前后端状态：

- 已有账号读取 API
- 已有文章 CRUD API
- 已有素材上传 API
- 已有发布记录 API
- 已有 Dashboard 统计 API
- 已有授权 URL / 授权回调 / 微信回调后端入口
- 暂无正式前端界面

前端第一阶段目标：

- 做一个独立的管理后台工程
- 明确采用前后端分离架构，前端不嵌入 Go 服务、不由 Go 服务托管页面
- 能通过当前后端 API 完成文章管理、账号查看、发布记录查看、Dashboard 数据展示
- 微信开放平台配置没完成时，用空状态 / 不可用状态承接
- 不在前端保存或展示 token、secret、refresh token 等敏感信息

---

## 2. 技术选型

推荐技术栈：

```text
React + Vite + TypeScript + Emotion
```

依赖建议：

```text
@emotion/react
@emotion/styled
```

暂不引入：

- Next.js：当前不是 SEO / SSR 场景，没必要
- 大型后台模板：容易和业务耦合、样式重、改动慢
- Redux：当前状态复杂度不高，先不用
- React Query：第一版可先用简单 fetch；当页面缓存和并发请求变复杂时再引入
- MUI / Ant Design：除非用户明确要求，否则先自建轻量组件，保持视觉可控

推荐目录：

```text
web/admin/
  package.json
  vite.config.ts
  index.html
  src/
    main.tsx
    App.tsx
    api/
      client.ts
      accounts.ts
      articles.ts
      dashboard.ts
      publishRecords.ts
      wechat.ts
    components/
      Button.tsx
      Card.tsx
      EmptyState.tsx
      Field.tsx
      PageShell.tsx
      StatusBadge.tsx
      Table.tsx
      Textarea.tsx
    pages/
      DashboardPage.tsx
      ArticlesPage.tsx
      ArticleEditorPage.tsx
      AccountsPage.tsx
      PublishRecordsPage.tsx
      WeChatSetupPage.tsx
    styles/
      theme.ts
      global.ts
```

---

## 3. 开发边界

### 3.1 明确前后端分离

本项目管理后台采用前后端分离模式：

```text
web/admin 独立前端工程  ->  HTTP API  ->  Go backend
```

要求：

- 前端独立构建、独立部署、独立版本管理
- 后端只提供 HTTP API，不内嵌、不托管前端页面
- 本地开发通过 Vite dev server + proxy 访问后端
- 生产环境建议通过 Nginx / 网关 / 静态站点托管前端，再反向代理 `/api` 到后端
- 不再新增类似 `/admin` 的 Go 内置 HTML 页面

### 3.2 前端只依赖 HTTP API

前端禁止直接访问数据库、Redis、微信 API。所有业务能力都通过后端接口完成。

```text
frontend -> backend HTTP API -> application -> domain -> infra
```

### 3.3 前端不处理敏感凭证

前端不得展示、存储、打印以下字段：

- component app secret
- authorizer access token
- authorizer refresh token
- refresh token encryption key
- 微信回调原始密文
- 任何带 `secret` / `token` / `refresh` 含义的敏感字段

微信开放平台配置页只做“配置状态提示”和“回调地址展示”，不做密钥录入和保存，除非后续明确增加后端安全配置管理能力。

### 3.4 当前用户隔离模型

前端不提供租户输入框，也不发送 `X-Tenant-ID`。用户登录后，后端从签名 session cookie 或用户 API token 解析 `user_id`；所有公众号、文章、素材和发布记录自动归入该用户的数据空间。

管理员角色只增加“用户管理”权限，不允许跨用户读取业务数据。用户管理页可以创建用户、生成/轮换 token、撤销 token；token 明文只在生成成功后展示一次。

---

## 4. 视觉方向

采用“精致工具台”风格，而不是普通 SaaS 模板。

关键词：

```text
清爽、克制、内容优先、中文排版友好、状态明确、少装饰但有质感
```

建议：

- 浅色主题优先
- 主色可围绕墨绿 / 暖米色 / 赤陶色做低饱和组合
- 页面留白充足
- 卡片不要过度嵌套
- 状态色明确：draft / publishing / published / failed / revoked
- 表格密度适中，避免过度压缩
- 空状态要告诉用户下一步该做什么

不建议：

- 默认深色霓虹风
- 大面积渐变字
- 大量玻璃拟态
- 套模板式 dashboard 卡片堆砌
- 复杂动效优先于业务可用性

---

## 5. 页面规划

### 5.1 Dashboard 首页

路径建议：

```text
/dashboard
```

后端接口：

```http
GET /api/v1/dashboard/stats
```

鉴权：使用登录 session cookie；写请求自动附带 `X-CSRF-Token`。

展示：

- 账号总数
- 活跃账号数
- 文章总数
- 草稿数
- 发布中数量
- 已发布数量
- 发布失败数量

空状态 / 错误状态：

- 没有数据时展示 0 值和引导创建文章
- 后端返回 `501 not_implemented` 时展示“服务依赖未配置”提示
- 后端返回 `500` 时展示通用错误，不展示内部错误详情

验收：

- 能正确读取 dashboard stats
- 请求不包含可控租户参数
- 不展示 token / secret / refresh 字段

---

### 5.2 账号管理页

路径建议：

```text
/accounts
```

后端接口：

```http
GET /api/v1/accounts
GET /api/v1/accounts/:id
GET /api/v1/accounts/:id/token-status?component_appid=...
```

展示：

- 公众号名称
- AppID
- 头像
- 状态
- 最近同步时间
- token 缓存状态，只展示状态和过期时间，不展示 token 值

空状态文案：

```text
暂无授权公众号
微信开放平台第三方平台配置完成后，可以从“微信配置”页面生成授权链接。
```

验收：

- 列表可正常展示
- 不返回 / 不展示敏感字段
- 不同用户数据互相隔离，由后端根据登录身份保证，前端不能提交或切换用户数据空间

---

### 5.3 文章列表页

路径建议：

```text
/articles
```

后端接口：

```http
GET /api/v1/articles
DELETE /api/v1/articles/:id
POST /api/v1/articles/:id/publish
```

展示：

- 标题
- 作者
- 摘要
- 状态
- 创建时间
- 更新时间
- 操作：编辑、删除、发布、查看发布记录

状态：

```text
draft
publishing
published
failed
```

验收：

- 能展示文章列表
- 删除前必须二次确认
- 发布按钮在微信配置未就绪时可以点击，但要友好展示 `501 not_implemented`
- 发布失败时展示后端错误摘要，不暴露内部敏感信息

---

### 5.4 文章编辑页

路径建议：

```text
/articles/new
/articles/:id/edit
```

后端接口：

```http
POST /api/v1/articles
GET /api/v1/articles/:id
PUT /api/v1/articles/:id
```

字段：

- authorizer_id
- title
- author
- digest
- content_html
- cover_media_asset_id

第一版编辑器：

- 暂用 textarea 编辑 HTML
- 提供基础预览区域
- 不引入富文本编辑器

后续可升级：

- Markdown -> HTML
- 富文本编辑器
- 图片插入
- 封面选择器

验收：

- 新建文章时 title 和 authorizer_id 必填
- 编辑文章时 title 必填
- 保存失败展示字段级或页面级错误
- 不在前端绕过后端校验

---

### 5.5 素材上传区

第一阶段建议作为文章编辑页的一部分，而不是单独页面。

后端接口：

```http
POST /api/v1/materials/inline-images
POST /api/v1/materials/covers
```

约束：

- 正文图片接口返回 `wechat_url`
- 封面接口返回 `media_id`
- 两者不能混用

微信配置未就绪时：

- 后端可能返回 `501 not_implemented`
- 前端提示：

```text
微信素材上传暂不可用，请完成第三方平台配置和公众号授权后重试。
```

验收：

- 使用 multipart 上传
- 请求带 authorizer_id、article_id、file
- 不在日志或页面中展示文件原始内容

---

### 5.6 发布记录页

路径建议：

```text
/publish-records
/articles/:id/publish-records
```

后端接口：

```http
GET /api/v1/publish-records
GET /api/v1/publish-records/:id
GET /api/v1/articles/:id/publish-records
POST /api/v1/publish-records/:id/sync-status
```

展示：

- 发布记录 ID
- 文章 ID
- Authorizer ID
- 微信 publish id
- 微信 article id
- 状态
- 错误码
- 错误信息
- 提交时间
- 完成时间

验收：

- 能展示当前用户的所有发布记录
- 能手动同步发布状态
- 发布状态同步不可用时友好提示 `not_implemented`
- 错误信息不展示 token / secret / refresh

---

### 5.7 微信配置检查页

路径建议：

```text
/wechat-setup
```

用途：第三方平台申请期间，作为配置准备清单页面。

展示内容：

- 第三方平台 AppID：是否已填写到后端 YAML
- AppSecret：只展示“已配置 / 未配置”，不展示值
- Verify Token：只展示“已配置 / 未配置”
- EncodingAESKey：只展示“已配置 / 未配置”
- Refresh Token Encryption Key：只展示“已配置 / 未配置”
- 授权事件接收 URL
- 授权后公众号消息与事件接收 URL
- 授权回调 URL 模板

后端当前没有配置状态 API，第一版可先静态展示推荐路径：

```text
授权事件接收 URL:
https://你的域名/wechat/component/callback

授权后公众号消息与事件接收 URL:
https://你的域名/wechat/authorizer/$APPID$/callback

授权回调 URL:
https://你的域名/api/v1/wechat/authorization-callback
```

后续可新增后端安全状态 API：

```http
GET /api/v1/wechat/config-status
```

该 API 只返回 boolean，不返回真实密钥。

---

## 6. API Client 规范

统一封装：

```text
src/api/client.ts
```

要求：

- 所有业务请求使用当前登录 session，不接受页面传入租户 ID
- 统一解析 JSON
- 统一处理错误码
- 不把敏感响应写入 console
- 对 `400` / `404` / `500` / `501` 做用户可读提示

后端错误格式：

```json
{"error":"invalid_request"}
{"error":"not_found"}
{"error":"internal_error"}
{"error":"not_implemented"}
```

前端建议映射：

| error | 文案 |
|---|---|
| invalid_request | 请求参数不完整，请检查输入 |
| not_found | 资源不存在或无权访问 |
| internal_error | 服务暂时异常，请稍后重试 |
| not_implemented | 相关能力尚未配置，完成微信配置后可用 |

---

## 7. 开发命令建议

进入前端目录：

```bash
cd web/admin
```

安装依赖：

```bash
npm install
```

开发：

```bash
npm run dev
```

构建：

```bash
npm run build
```

类型检查：

```bash
npm run typecheck
```

Lint 可后续再接，第一版先保证：

- TypeScript 类型通过
- Vite build 通过
- 核心页面能调通本地后端

---

## 8. Vite 代理建议

开发环境代理后端：

```ts
server: {
  proxy: {
    '/api': 'http://localhost:8080',
    '/wechat': 'http://localhost:8080'
  }
}
```

前端调用时使用相对路径：

```ts
fetch('/api/v1/articles')
```

不要在页面代码里硬编码后端域名。

---

## 9. 构建与部署策略

全阶段保持前后端分离。

本地开发：

```text
Go backend: localhost:8080
Vite frontend: localhost:5173
```

生产部署建议：

```text
Nginx / CDN / 静态站点服务 -> web/admin/dist
API 网关 / Nginx             -> Go backend
```

路由建议：

```text
https://console.example.com/        -> 前端后台
https://api.example.com/api/v1/...  -> 后端 API
```

或同域反代：

```text
https://console.example.com/        -> 前端后台
https://console.example.com/api/... -> 反向代理到 Go backend
```

明确不采用：

```text
Go backend 直接托管 /admin 静态页面
```

---

## 10. 分阶段开发计划

### Phase 1：前端工程骨架

目标：创建 `web/admin`，跑通 Vite + React + TypeScript + Emotion。

验收：

- `npm run dev` 可启动
- 首页可显示基本布局
- Vite proxy 可访问 `/api/v1/healthz`

### Phase 2：基础壳与主题

目标：完成布局、导航、主题、基础组件。

页面：

- Dashboard
- 账号
- 文章
- 发布记录
- 微信配置
- 用户管理（仅管理员）

验收：

- 页面可切换
- 用户身份由登录态确定，页面不可切换租户 ID
- Emotion 主题生效
- 移动端不崩

### Phase 3：Dashboard + 账号列表

目标：接入只读接口。

接口：

- `GET /api/v1/dashboard/stats`
- `GET /api/v1/accounts`

验收：

- 能展示统计
- 能展示账号空状态 / 列表
- 请求不带 tenant header

### Phase 4：文章 CRUD

目标：完成文章列表、新建、编辑、删除。

接口：

- `GET /api/v1/articles`
- `POST /api/v1/articles`
- `GET /api/v1/articles/:id`
- `PUT /api/v1/articles/:id`
- `DELETE /api/v1/articles/:id`

验收：

- 可完整管理本地文章草稿
- 表单校验可用
- 删除有确认

### Phase 5：发布记录

目标：完成发布记录列表和状态同步入口。

接口：

- `GET /api/v1/publish-records`
- `GET /api/v1/publish-records/:id`
- `POST /api/v1/publish-records/:id/sync-status`

验收：

- 可查看发布状态
- 可手动同步
- 微信配置未就绪时提示明确

### Phase 6：微信配置准备页

目标：做配置清单和授权 URL 准备。

接口：

- 第一版可先不调接口，只展示说明
- 微信配置完成后接 `GET /api/v1/wechat/authorization-url`

验收：

- 用户知道去微信开放平台填哪些 URL
- 用户知道 YAML 里需要哪些配置
- 不展示任何密钥值

### Phase 7：素材上传与发布

目标：等微信配置和授权账号可用后再做。

接口：

- `POST /api/v1/materials/inline-images`
- `POST /api/v1/materials/covers`
- `POST /api/v1/articles/:id/publish`

验收：

- 封面上传后能更新文章 cover_media_asset_id
- 正文图片上传后能插入 `wechat_url`
- 发布后能看到发布记录

---

## 11. 每次前端开发任务的执行规则

后续每次用户发指令时，按以下规则执行：

1. 先确认本次任务属于哪个 Phase
2. 只改本次任务需要的文件
3. 不顺手扩大范围
4. 每个页面接 API 时必须处理 loading / empty / error 三种状态
5. 所有请求必须复用登录态，禁止恢复 `X-Tenant-ID` 或租户选择器
6. 不展示任何敏感字段
7. 完成后至少运行：

```bash
npm run build
```

如果本次任务涉及 TypeScript 类型：

```bash
npm run typecheck
```

如果还没有前端工程，则先完成 Phase 1。

---

## 12. 当前建议的下一条用户指令

如果要开始开发，建议下一步指令是：

```text
开始 Phase 1，创建 web/admin 前端工程，使用 React + Vite + TypeScript + Emotion，只搭骨架和首页，不接业务接口。
```

或者如果想先看视觉稿：

```text
先设计前端 Dashboard 和文章列表的视觉方案，不写代码。
```
