# Official Account Service 未完成功能对照与开发路线

本文对照 `official-account-service-product-design-v1.0.md` 和 `02-AI协作开发约束规范.md`，记录当前代码已完成/未完成内容，并作为后续逐项开发清单。

## 0. 当前已完成

- 工程骨架：`cmd/server`、`internal/domain`、`internal/application`、`internal/infra`、`internal/interfaces/http`
- 基础配置：环境变量、zap 日志、HTTP health check
- Docker：`app`、`postgres`、`redis`
- 领域模型：授权账号、文章、素材、发布记录
- memory repository：授权账号、文章、素材、发布记录
- Postgres repository：授权账号读取/写入、文章 CRUD、素材记录、发布记录
- 授权账号 application service：读取、保存、状态更新
- 文章 application service：创建、列表、详情、更新、删除
- 素材 application service：正文配图、封面图分路径上传骨架
- 发布 application service：本地发布记录创建、查询、状态更新与文章状态同步
- 微信第三方平台授权基础闭环：component callback、授权 URL、授权回调、账号同步、取消授权
- Token 体系：authorizer access token 刷新、本地缓存、Redis 分布式锁、跨实例缓存、token 状态查询
- 微信素材真实客户端：正文图片上传、永久封面上传、超时、有限重试、errcode 处理
- 微信发布客户端：draft.add、freepublish.submit、发布状态同步、发布失败原因记录、发布幂等控制
- 微信回调处理：authorizer callback、发布结果回调、去重、审计落库、日志脱敏
- Redis/asynq 异步任务：发布状态兜底轮询、token 自调度刷新、归档任务查看与重试
- P2 能力：Dashboard 统计、agent-api、OpenAPI、wire、HTTP 集成测试
- 文章 HTTP CRUD API：`/api/v1/articles`
- 公众号账号读取 API：`/api/v1/accounts`
- 初始数据库 migration：文章、素材、发布记录表
- 基础单元测试：文章/素材/发布 service、HTTP router、微信客户端、队列与集成测试

## 1. P0 开发清单

### P0-3 公众号账号读取 API（已完成）

目标：先让后台/API 能读取已授权账号，为后续真实授权回调落库提供入口。

范围：

- `AuthorizationService` / `AccountService`：已完成 `AccountService`
- 账号列表 API：已完成 `GET /api/v1/accounts`
- 账号详情 API：已完成 `GET /api/v1/accounts/:id`
- tenant 隔离：已完成
- 不返回 token/secret/refresh 字段：已完成

验收：

- 能按 `X-Tenant-ID` 查询账号列表
- 不同 tenant 互相不可见
- 不存在账号返回 `404 not_found`
- HTTP 入参校验失败返回 `400 invalid_request`
- 有 service 和 HTTP 测试

### P0-4 Postgres 持久化接入（已完成）

目标：让当前文章 CRUD 和账号读取不再依赖 memory store。

范围：

- Postgres connection 初始化：已完成
- repository 实现：已完成账号与文章 repository
- app 根据 `DB_DSN` 使用 Postgres，否则 fallback memory：已完成
- 保持 repository 方法强制 `tenantID`：已完成

验收：

- Docker Compose 启动后可持久化文章：已验证
- 重启 app 后文章不丢：已验证
- migration 可初始化表结构：已完成 `001`/`002`
- 有 repository/integration 测试或明确的 Docker 验证步骤：已完成 `POSTGRES_TEST_DSN` 集成测试

### P0-5 授权账号写入基础能力（已完成）

目标：为真实微信授权回调接入前，补齐账号落库用例。

范围：

- 创建/更新授权账号 application 方法：已完成 `SaveAccount`
- 账号状态更新：已完成 `UpdateAccountStatus`
- 仅供内部授权流程调用，不开放手工创建公网 API，避免假授权数据污染：已遵守，未新增写入 HTTP API

验收：

- 授权流程可复用该 service 保存账号：已完成
- 状态更新 tenant 隔离：已完成
- 错误均 wrap：已完成

### P0-6 素材上传骨架（已完成）

目标：按文档强制区分正文配图和封面图。

范围：

- `MaterialService`：已完成
- 正文配图上传入口：已完成，返回并保存 `wechat_url`
- 封面上传入口：已完成，返回并保存 `media_id`
- 先定义 WeChat client interface，infra 后续实现：已完成 `material.Uploader`
- HTTP multipart 入参校验：已完成

验收：

- `inline_image` 和 `cover` 两条路径类型不可混用：已完成
- 有 service 测试：已完成
- 不把文件内容打日志：已完成

### P0-7 发布记录与本地发布状态 API（已完成）

目标：先补齐本地发布记录和状态查询，再接微信真实发布。

范围：

- `PublishService`：已完成
- 创建发布记录：已完成 `POST /api/v1/publish-records`
- 查询文章发布记录：已完成 `GET /api/v1/articles/:id/publish-records`
- 更新发布状态：已完成 `PUT /api/v1/publish-records/:id/status`
- article status 同步更新：已完成

验收：

- 发布记录 tenant 隔离：已完成
- 重复更新同一发布记录幂等：已完成
- 状态流转有测试：已完成

## 2. P1 开发清单

### P1-1 微信第三方平台授权流程（已完成）

已完成：

- component_verify_ticket 明文回调入口：已完成 `POST /wechat/component/callback`
- component_verify_ticket application 保存用例：已完成
- component_verify_ticket memory/Postgres 持久化：已完成
- component_verify_ticket migration：已完成 `003`
- 授权 URL 生成 application 用例：已完成
- 授权 URL HTTP API：已完成 `GET /api/v1/wechat/authorization-url`
- 预授权码生成接口契约：已完成 `PreAuthCodeCreator`
- component_access_token 基础获取客户端：已完成 `ComponentClient`
- 真实 pre_auth_code 微信客户端：已完成，复用 `ComponentClient`
- 微信授权客户端 HTTP timeout/retry/熔断：已完成基础实现
- 微信授权客户端配置接入：已完成 `WECHAT_COMPONENT_APP_SECRET` / `WECHAT_API_BASE_URL`
- component_verify_ticket 回调验签/解密：已完成 `ComponentCallbackCrypto`
- component_verify_ticket 加密回调 HTTP 接入：已完成 `msg_signature` / `timestamp` / `nonce` / `Encrypt`
- 微信加密回调配置接入：已完成 `WECHAT_COMPONENT_VERIFY_TOKEN` / `WECHAT_COMPONENT_ENCODING_AES_KEY`
- 授权回调处理：已完成 `GET /api/v1/wechat/authorization-callback`
- authorizer_refresh_token 加密存储：已完成 AES-GCM 加密后落库
- authorizer 信息同步：已完成授权回调后调用微信资料接口并保存账号名称/头像
- 取消授权事件处理：已完成 `unauthorized` component callback，账号置为 `revoked` 并清空加密 refresh token

缺失：无，P1-1 已完成基础闭环；真实微信环境联调仍需有效第三方平台配置。

### P1-2 Token 体系

已完成：

- authorizer_access_token 刷新：已完成 `TokenService.RefreshAuthorizerAccessToken`
- 本地 singleflight 并发去重：已完成
- refresh 失败账号状态更新：已完成，刷新失败置为 `refresh_failed`
- 并发刷新测试：已完成
- refresh token 解密能力：已完成，复用 AES-GCM codec
- access token 本地缓存：已完成，`TokenService.GetAuthorizerAccessToken` 优先返回有效缓存
- token 过期主动续期：已完成请求时提前 5 分钟刷新
- access token 状态查询：已完成 `GET /api/v1/accounts/:id/token-status`
- Redis 分布式锁：已完成，配置 `REDIS_ADDR` 后刷新 token 使用 Redis SET NX 锁
- 跨实例共享 token 缓存：已完成，配置 `REDIS_ADDR` 后 Redis 缓存 authorizer access token

缺失：无，P1-2 已完成基础闭环；真实多实例场景仍需在部署环境联调 Redis 可用性和锁等待表现。

### P1-3 微信素材客户端

已完成：

- 正文图片上传接口：已完成 `MaterialUploader.UploadInlineImage`，调用 `media/uploadimg`
- 永久素材封面上传接口：已完成 `MaterialUploader.UploadCover`，调用 `material/add_material?type=image`
- HTTP timeout：已完成，默认复用 5 秒 HTTP timeout
- 微信 errcode 处理：已完成，非 0 `errcode` 映射为素材上传失败
- 幂等接口重试策略：已完成基础传输/5xx 有限重试，业务 errcode 不重试

缺失：无，P1-3 已完成基础闭环；真实微信环境仍需用有效 authorizer access token 联调上传结果。

### P1-4 微信发布流程

已完成：

- draft.add：已完成 `Publisher.AddDraft`
- freepublish.submit：已完成 `Publisher.SubmitFreePublish`
- 异步发布状态处理：已完成主动轮询 `freepublish/get` 并更新本地发布记录
- 发布失败原因记录：已完成失败状态写入 `error_code` / `error_message`
- 发布幂等控制：已完成同一文章已有 `publishing` 记录时直接返回现有记录

缺失：无，P1-4 已完成基础闭环；发布结果回调处理归入 P1-5。

### P1-5 微信回调处理

已完成：

- 回调验签/解密：已完成，authorizer 回调复用 `ComponentCallbackCrypto` 处理 `encrypt_type=aes` / `msg_signature` / `timestamp` / `nonce` / `Encrypt`
- 发布结果回调：已完成 `CallbackService.HandleAuthorizerCallback` 和 `POST /wechat/authorizer/:app_id/callback`，处理 `PUBLISHJOBFINISH` 并同步发布记录与文章状态
- 回调去重：已完成，按 `tenant_id + event_type + publish_id` 去重，memory/Postgres repository 均实现重复检测
- 回调原文审计存储与留存策略：已完成 `wechat_callback_event`，保存解密后的回调正文、接收时间和 30 天 `retain_until`
- 日志脱敏：已完成 `logging.RedactedString` 辅助方法，当前回调处理不记录 token、secret、密文或原始回调正文

缺失：无，P1-5 已完成基础闭环；真实微信环境仍需配置 authorizer 回调地址并联调加密回调与发布结果事件格式。

### P1-6 Redis/asynq 异步任务

已完成：

- asynq client/server：已完成，配置 `REDIS_ADDR` 后发布服务会创建 asynq client，服务启动时会启动 `publish` 队列 worker
- 发布状态兜底轮询：已完成，真实发布成功后自动入队 `publish:sync_status` 延迟任务，worker 调用 `PublishService.SyncPublishStatus`；微信仍返回 `publishing` 时任务返回可重试错误继续轮询
- token 定时刷新：已完成，成功刷新 authorizer access token 后按 `expires_at - refresh_before` 自调度 `token:refresh_authorizer_access_token`，任务携带 tenant/account/component，不做跨 tenant 全库扫描
- 失败重试和死信处理：已完成基础能力，`publish:sync_status` 与 `token:refresh_authorizer_access_token` 配置 `MaxRetry`，handler 返回错误或发布仍未终态时由 asynq 重试，重试耗尽后归档
- archived task 管理入口：已完成 `GET /api/v1/task-queues/:queue/archived-tasks` 查看归档任务安全摘要，`POST /api/v1/task-queues/:queue/archived-tasks/:task_id/retry` 将归档任务重新放回 pending；仅开放 `publish` / `token` 队列，payload 只返回白名单字段，疑似包含敏感字段的 `last_error` 会脱敏，不暴露 token/secret/raw callback payload

缺失：无，P1-6 已完成基础闭环；真实 Redis/asynq 运维场景仍需在部署环境联调 archived task 查看、告警接入和人工重放权限控制。

## 3. P2 开发清单

已完成：

- Dashboard 统计 API：已完成 `GET /api/v1/dashboard/stats`，按 `X-Tenant-ID` 返回账号、文章、发布记录的总量与状态分布，不返回 token/secret/refresh 字段
- Token 状态 API：已完成 `GET /api/v1/accounts/:id/token-status`，响应只包含缓存状态和过期时间
- 发布记录后台 API：已完成 `GET /api/v1/publish-records` tenant 级列表和 `GET /api/v1/publish-records/:id` 详情，后台可不依赖文章详情页查看发布记录
- agent-api：已完成 `internal/agent-api` Go package，提供 `wechat.authorize()`、`wechat.publishArticle()`、`wechat.uploadImage()`、`wechat.listAccounts()`、`wechat.listArticles()` 对应方法，只依赖 application 层并返回安全 DTO
- OpenAPI 文档：已完成 `api/openapi.yaml`，覆盖当前 HTTP API、微信回调入口、Dashboard、任务队列运维入口，并用测试校验 YAML 可解析和关键路径存在
- wire 依赖注入：已完成基础接入，`cmd/server/wire.go` 定义 injector，`cmd/server/wire_gen.go` 生成 HTTP `Dependencies` 组装，`make generate` 可重复生成
- 更完整的集成测试：已完成 `TestHTTPIntegrationTenantPublishingWorkflow`，基于内存仓储和真实 HTTP router 覆盖授权回调建号、账号列表、token 状态、文章创建、封面上传、发布提交、发布状态同步、发布记录后台列表/详情、Dashboard 统计和跨 tenant 隔离

待完成：无

## 4. 开发顺序

按当前代码状态，后续逐项开发顺序：

1. P0-3 公众号账号读取 API（已完成）
2. P0-4 Postgres 持久化接入（已完成）
3. P0-5 授权账号写入基础能力（已完成）
4. P0-6 素材上传骨架（已完成）
5. P0-7 发布记录与本地发布状态 API（已完成）
6. P1 微信真实授权/token/素材/发布/回调（已完成基础闭环，待真实环境联调）
7. P2 agent-api、后台增强、OpenAPI、wire（已完成）

下一步不再是继续补 P0/P1/P2 功能，而是进入真实环境联调、可靠性加固和上线准备。

每完成一项必须执行完成度自查：

- 分层 import 是否符合约束
- error 是否 wrap
- tenant 数据访问是否强制传 `tenantID`
- HTTP 入参是否校验
- token/secret 是否没有出现在日志或返回体
- 是否有对应测试
- `go test ./...` 是否通过

## 5. 上线前待补齐 / 联调清单

以下不是新的产品功能，而是从基础闭环到可上线服务必须完成的联调、运维和安全确认。

### 5.1 微信真实环境联调

- 第三方平台配置确认：`component_appid`、`component_appsecret`、verify token、EncodingAESKey
- component callback URL 配置与加密回调联调
- authorizer callback URL 配置与发布结果事件联调
- 授权 URL、授权回调、账号资料同步、取消授权事件联调
- authorizer access token 刷新联调，确认 refresh token 解密、刷新失败状态和 Redis 锁表现
- 正文图片上传 `media/uploadimg` 联调，确认返回 `url` 可嵌入正文 HTML
- 永久封面上传 `material/add_material?type=image` 联调，确认返回 `media_id` 可作为封面
- 草稿创建 `draft.add`、提交发布 `freepublish.submit`、发布状态查询 `freepublish/get` 联调
- 发布成功、发布失败、微信仍返回 `publishing` 三类路径联调

### 5.2 部署与运维确认

- Postgres migration 生产执行策略确认，包括已有旧 volume/旧 schema 的升级步骤
- Redis/asynq worker 部署验证，确认 `publish` 和 `token` 队列均能消费
- asynq archived task 查看、重试流程演练
- archived task 运维入口权限控制和审计接入
- 发布状态轮询、token 刷新失败、回调处理失败的告警接入
- 回调审计数据 `retain_until` 清理任务确认
- OpenAPI 与前端、后台、agent 调用方对齐

### 5.3 安全与合规确认

- 日志抽样检查：不输出 token、secret、refresh token、回调密文、原始回调正文
- API 响应抽样检查：账号、token 状态、队列 payload、agent-api DTO 不返回敏感字段
- 配置检查：密钥只走环境变量/密钥系统，不进入代码库或镜像层
- 租户隔离抽样测试：账号、文章、素材、发布记录、回调事件、任务摘要均不能跨 tenant 查询

## 6. 已知技术债与后续加固项

- 当前 `database/sql + pgx` 已满足 MVP/P0-P2 基础闭环；产品设计提到 ent，后续如需严格对齐可评估迁移，但不是上线前 blocker。
- 发布流程涉及发布记录、文章状态、素材、token、微信接口等多资源；当前已保证应用层幂等，后续可用 DB transaction / unit of work 加强跨 repository 一致性。
- 微信错误码矩阵当前覆盖基础 errcode 和传输/5xx 重试；真实联调后应补充常见业务错误码的分类、告警和用户可读失败原因。
- asynq archived task 已有基础查看与重试入口；生产环境还需补权限、审计、告警和人工重放 SOP。
- 回调审计已保存 `retain_until`；上线前需确认定时清理任务或数据库侧清理策略。
- 当前 Redis 锁和缓存已接入；真实多实例场景仍需压测锁等待、缓存过期、worker 重启后的表现。
- OpenAPI 已有关键路径校验；后续接口字段稳定后应补充示例响应、错误码说明和调用方契约版本策略。
