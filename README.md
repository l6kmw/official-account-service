# Official Account Service

OpenClaw AI 平台的微信公众号授权与文章发布微服务。

当前开发依据：

- `doc/official-account-service-product-design-v1.0.md`
- `doc/02-AI协作开发约束规范.md`
- `doc/04-frontend-development-guide.md`
- `doc/05-frontend-ui-design-spec.md`

## 本地运行

```bash
cp config.yaml.example config.yaml
make test
make run
```

服务默认读取 `config.yaml`，也可以显式指定：

```bash
go run ./cmd/server -config config.yaml
```

重新生成 wire 注入代码：

```bash
make generate
```

## 前端管理后台

前端采用前后端分离，目录位于 `web/admin`，技术栈为 React + Vite + TypeScript + Emotion。

```bash
cd web/admin
npm install
npm run dev
```

开发服务默认运行在 `http://localhost:5173`，通过 Vite proxy 将 `/api` 和 `/wechat` 转发到本地 Go 服务 `http://localhost:8080`。

构建检查：

```bash
cd web/admin
npm run build
```

管理后台会在启动时读取 `/admin-config.js` 注入的公开配置。构建产物里会包含 `dist/admin-config.js`，部署到不同服务器或域名时，可以直接修改这个文件，无需重新构建前端：

```js
window.__OFFICIAL_ACCOUNT_ADMIN_CONFIG__ = {
  tenantID: 'tenant-1',
  publicBaseURL: 'https://oa.example.com',
  componentAppID: 'wx_component_appid',
  adminAPIKey: ''
}
```

字段说明：

- `tenantID`：后台 API 请求使用的租户 ID。
- `publicBaseURL`：公网 HTTPS 服务域名，用于生成微信开放平台回调地址和授权入口页。
- `componentAppID`：微信开放平台第三方平台 Component AppID。
- `adminAPIKey`：兼容旧部署的可选字段。公开站点不要把它当作强保密手段；生产环境优先使用管理员登录会话，或由网关注入该请求头。

前端运行时配置只允许放可公开信息；不要把管理员密码、AppSecret、Verify Token、EncodingAESKey、refresh token 或加密密钥写入 `admin-config.js`。

当前已完成 Phase 8：前端工程骨架、AppShell、现代 SaaS 控制台风格主题、API client、Dashboard 真实数据接入、文章 CRUD 页面、账号列表页、添加公众号授权入口、发布记录页、微信配置检查页、素材上传区和文章发布动作。

## Docker 运行

```bash
cp config.docker.yaml.example config.docker.yaml
make docker-up
curl http://localhost:8080/healthz
```

`docker-compose.yml` 会启动：

- `app`：当前 Go 服务，挂载 `config.docker.yaml` 到容器内 `/app/config.yaml`
- `postgres`：初始化执行 `migrations/*.sql`
- `redis`：用于 token 刷新分布式锁、跨实例 token 缓存和 asynq 队列

关闭：

```bash
make docker-down
```

## Postgres 集成测试

```bash
make docker-up
POSTGRES_TEST_DSN='postgres://official_account:official_account_password@localhost:5432/official_account?sslmode=disable' \
  go test ./internal/infra/persistence/postgres -run TestStoreArticleAndAccountIntegration -count=1
make docker-down
```

## 当前阶段

已建立分层骨架、文章/素材/发布记录领域模型、agent-api 函数入口、OpenAPI 文档、账号读取 API、文章 CRUD HTTP API、Dashboard 统计 API、微信素材上传客户端、微信草稿/发布客户端、发布记录后台 API、发布记录与本地发布状态 API、Token 本地缓存/状态查询 API、发布状态异步兜底同步任务、Docker 部署配置与 Postgres 持久化。

## Agent API

`internal/agent-api` 提供面向 AI Agent 的 Go package 入口，方法对应产品文档里的函数式契约：

- `Authorize` -> `wechat.authorize()`
- `PublishArticle` -> `wechat.publishArticle()`
- `UploadImage` -> `wechat.uploadImage()`
- `ListAccounts` -> `wechat.listAccounts()`
- `ListArticles` -> `wechat.listArticles()`

该层只依赖 application 层，返回 DTO 不包含 token、secret、refresh token 等敏感字段。

如需接入外部 agent，优先使用 stdio MCP 适配层 `cmd/mcp-server`。它通过现有 HTTP API 调用本服务，不直接暴露微信密钥、数据库密码或 refresh token。详细接入方式见 `doc/06-agent-integration-mcp-skill.md`。

## OpenAPI

HTTP API 文档位于 `api/openapi.yaml`。当前测试会解析该文件并检查关键路由是否存在，避免文档格式损坏或遗漏主要入口。

## YAML 配置

应用配置统一走 YAML，不再依赖 `.env`。本地开发使用 `config.yaml`，Docker 使用 `config.docker.yaml`。真实环境可以用同一份 schema，例如：

```yaml
app:
  env: staging
http:
  addr: ":8080"
log:
  level: info
database:
  dsn: "postgres://user:password@postgres:5432/official_account?sslmode=disable"
redis:
  addr: "redis:6379"
security:
  admin_api_key: "change-me"
  admin_username: "admin"
  admin_password_hash: "$2a$12$..."
  admin_session_secret: "change-this-long-random-session-secret"
wechat:
  component_app_id: "wx..."
  component_app_secret: "..."
  api_base_url: ""
  component_verify_token: "..."
  component_encoding_aes_key: "..."
  refresh_token_encryption_key: "..."
```

`wechat.refresh_token_encryption_key` 需要 base64 编码的 32 字节 key：

```bash
openssl rand -base64 32
```

管理员登录使用服务端会话 cookie，不需要把密码或 API key 写入前端。生成密码 hash 示例：

```bash
htpasswd -bnBC 12 "" "your-admin-password" | tr -d ':\n'
```

`security.admin_session_secret` 用于签名登录会话，建议使用至少 32 字节随机值：

```bash
openssl rand -base64 32
```

当 `app.env` 为 `prod` 时，必须配置 `security.admin_api_key`，或同时配置 `security.admin_username`、`security.admin_password_hash` 和 `security.admin_session_secret`。配置管理员登录后，管理台会通过 `POST /api/v1/admin/session` 登录，服务端签发 HttpOnly + SameSite cookie；写操作还需要 `X-CSRF-Token`。保留 `security.admin_api_key` 时，管理 API 仍兼容请求头 `X-Admin-API-Key: <admin_api_key>` 或 `Authorization: Bearer <admin_api_key>`，适合网关或脚本调用。

HTTP 服务会统一限制请求体最大 10 MiB，超限返回 `413 request_too_large`。微信 component/authorizer 回调正文解析仍按 1 MiB 上限处理。

## 微信授权配置

默认不配置微信密钥时，授权 URL API 会返回 `501 not_implemented`。配置 `wechat.component_app_secret` 后，服务会使用已保存的 `component_verify_ticket` 获取 `component_access_token` 和 `pre_auth_code`。

如需接收微信加密回调，需要同时配置 `wechat.component_verify_token` 和 `wechat.component_encoding_aes_key`；可选配置 `wechat.component_app_id` 用于校验解密明文里的接收方 appid。

授权回调落库前会加密 `authorizer_refresh_token`；启用真实授权回调处理时，需要配置 base64 编码的 32 字节 `wechat.refresh_token_encryption_key`。

Token 状态查询使用 `GET /api/v1/accounts/:id/token-status?component_appid=...`，需要 `X-Tenant-ID` 请求头；响应只包含缓存状态和过期时间，不返回 access token 或 refresh token。

Dashboard 统计使用 `GET /api/v1/dashboard/stats`，需要 `X-Tenant-ID` 请求头；响应包含账号、文章、发布记录的总量与状态分布，不返回 token、secret 或 refresh token。

配置 `redis.addr` 后，服务启动时会连接 Redis，并在刷新 authorizer access token 时使用分布式锁和跨实例缓存；同时会启动 asynq worker 处理 `publish` 和 `token` 队列。不配置时仍使用进程内 singleflight 和本地内存缓存，异步发布状态兜底任务和 token 自调度刷新任务也不会入队。

成功刷新 authorizer access token 后，如果配置了 `redis.addr`，服务会按过期时间提前 5 分钟入队 `token:refresh_authorizer_access_token`，后续由 asynq 按 tenant/account/component 继续自调度刷新。

配置 `wechat.component_app_id` 后，素材上传接口会通过 TokenService 获取 authorizer access token，并调用微信正文图片上传和永久素材上传接口；未配置时素材上传保持 `501 not_implemented`。

同样在配置 `wechat.component_app_id` 后，可用 `POST /api/v1/articles/:id/publish` 创建微信草稿并提交发布；已发布文章再次调用该接口会按当前本地内容提交一条新的修订版发布记录，修订版发布成功后会自动删除同一文章的上一版已发布内容。`POST /api/v1/publish-records/:id/sync-status` 可主动轮询微信发布状态并同步本地记录。发布提交成功只代表进入微信异步处理，最终结果仍以后续状态同步或回调为准。

删除文章时，如果该文章已有成功发布且本地保存了微信 `article_id`，后台会先调用微信 `freepublish/delete` 删除公众号侧图文，再删除本地文章；正在发布中的文章会被拒绝删除，避免公众号侧异步发布结果变成孤儿内容。已经删除本地文章但仍保留的历史发布记录，可通过 `POST /api/v1/publish-records/:id/delete-published` 或管理台发布记录页删除公众号侧内容。

发布记录后台查询使用 `GET /api/v1/publish-records` 和 `GET /api/v1/publish-records/:id`，需要 `X-Tenant-ID` 请求头；也可继续使用 `GET /api/v1/articles/:id/publish-records` 查看单篇文章的发布记录。

如果同时配置了 `redis.addr`，真实发布成功后会自动入队 `publish:sync_status` 延迟任务，用 asynq 兜底轮询微信发布状态；微信仍返回 `publishing` 或任务失败时会按 asynq 重试策略重试，重试耗尽后归档。

归档任务运维入口：

- `GET /api/v1/task-queues/:queue/archived-tasks?limit=30`：查看当前 `X-Tenant-ID` 下 `publish` 或 `token` 队列的 archived task 安全摘要
- `POST /api/v1/task-queues/:queue/archived-tasks/:task_id/retry`：校验 archived task 属于当前 `X-Tenant-ID` 后，将其重新放回 pending 等待 worker 处理

这些接口仅返回 payload 白名单字段（如 `tenant_id`、`publish_record_id`、`account_id`、`component_app_id`），疑似包含敏感字段的 `last_error` 会脱敏，不返回 token、secret、refresh token 或原始回调 payload。未配置 `redis.addr` 时会返回 `501 not_implemented`。

发布结果回调入口为 `POST /wechat/authorizer/:app_id/callback`，支持明文和 `encrypt_type=aes` 加密回调。服务会按 authorizer appid 找到 tenant，处理 `PUBLISHJOBFINISH` 事件，按 `publish_id` 去重，并保存解密后的回调正文用于审计，默认留存 30 天。
