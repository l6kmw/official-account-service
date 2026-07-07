# Official Account Service

OpenClaw AI 平台的微信公众号授权与文章发布微服务。

当前开发依据：

- `doc/official-account-service-product-design-v1.0.md`
- `doc/02-AI协作开发约束规范.md`

## 本地运行

```bash
cp .env.example .env
make test
make run
```

重新生成 wire 注入代码：

```bash
make generate
```

## Docker 运行

```bash
cp .env.example .env
make docker-up
curl http://localhost:8080/healthz
```

`docker-compose.yml` 会启动：

- `app`：当前 Go 服务，使用 Postgres 持久化
- `postgres`：初始化执行 `migrations/*.sql`
- `redis`：用于 token 刷新分布式锁、跨实例 token 缓存，并为后续队列预留

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

## OpenAPI

HTTP API 文档位于 `api/openapi.yaml`。当前测试会解析该文件并检查关键路由是否存在，避免文档格式损坏或遗漏主要入口。

## 微信授权配置

默认不配置微信密钥时，授权 URL API 会返回 `501 not_implemented`。配置 `WECHAT_COMPONENT_APP_SECRET` 后，服务会使用已保存的 `component_verify_ticket` 获取 `component_access_token` 和 `pre_auth_code`。

如需接收微信加密回调，需要同时配置 `WECHAT_COMPONENT_VERIFY_TOKEN` 和 `WECHAT_COMPONENT_ENCODING_AES_KEY`；可选配置 `WECHAT_COMPONENT_APP_ID` 用于校验解密明文里的接收方 appid。

授权回调落库前会加密 `authorizer_refresh_token`；启用真实授权回调处理时，需要配置 base64 编码的 32 字节 `WECHAT_REFRESH_TOKEN_ENCRYPTION_KEY`。

Token 状态查询使用 `GET /api/v1/accounts/:id/token-status?component_appid=...`，需要 `X-Tenant-ID` 请求头；响应只包含缓存状态和过期时间，不返回 access token 或 refresh token。

Dashboard 统计使用 `GET /api/v1/dashboard/stats`，需要 `X-Tenant-ID` 请求头；响应包含账号、文章、发布记录的总量与状态分布，不返回 token、secret 或 refresh token。

配置 `REDIS_ADDR` 后，服务启动时会连接 Redis，并在刷新 authorizer access token 时使用分布式锁和跨实例缓存；同时会启动 asynq worker 处理 `publish` 和 `token` 队列。不配置时仍使用进程内 singleflight 和本地内存缓存，异步发布状态兜底任务和 token 自调度刷新任务也不会入队。

成功刷新 authorizer access token 后，如果配置了 `REDIS_ADDR`，服务会按过期时间提前 5 分钟入队 `token:refresh_authorizer_access_token`，后续由 asynq 按 tenant/account/component 继续自调度刷新。

配置 `WECHAT_COMPONENT_APP_ID` 后，素材上传接口会通过 TokenService 获取 authorizer access token，并调用微信正文图片上传和永久素材上传接口；未配置时素材上传保持 `501 not_implemented`。

同样在配置 `WECHAT_COMPONENT_APP_ID` 后，可用 `POST /api/v1/articles/:id/publish` 创建微信草稿并提交发布；`POST /api/v1/publish-records/:id/sync-status` 可主动轮询微信发布状态并同步本地记录。发布提交成功只代表进入微信异步处理，最终结果仍以后续状态同步或回调为准。

发布记录后台查询使用 `GET /api/v1/publish-records` 和 `GET /api/v1/publish-records/:id`，需要 `X-Tenant-ID` 请求头；也可继续使用 `GET /api/v1/articles/:id/publish-records` 查看单篇文章的发布记录。

如果同时配置了 `REDIS_ADDR`，真实发布成功后会自动入队 `publish:sync_status` 延迟任务，用 asynq 兜底轮询微信发布状态；微信仍返回 `publishing` 或任务失败时会按 asynq 重试策略重试，重试耗尽后归档。

归档任务运维入口：

- `GET /api/v1/task-queues/:queue/archived-tasks?limit=30`：查看 `publish` 或 `token` 队列的 archived task 安全摘要
- `POST /api/v1/task-queues/:queue/archived-tasks/:task_id/retry`：将 archived task 重新放回 pending 等待 worker 处理

这些接口仅返回 payload 白名单字段（如 `tenant_id`、`publish_record_id`、`account_id`、`component_app_id`），疑似包含敏感字段的 `last_error` 会脱敏，不返回 token、secret、refresh token 或原始回调 payload。未配置 `REDIS_ADDR` 时会返回 `501 not_implemented`。

发布结果回调入口为 `POST /wechat/authorizer/:app_id/callback`，支持明文和 `encrypt_type=aes` 加密回调。服务会按 authorizer appid 找到 tenant，处理 `PUBLISHJOBFINISH` 事件，按 `publish_id` 去重，并保存解密后的回调正文用于审计，默认留存 30 天。
