# 多 Agent Token 设计

## 1. 目标

在不改变现有用户数据隔离边界的前提下，让一个用户可以为多个数字员工配置独立 MCP Token。

设计后的关系是：

```text
用户（数据所有者）
  ├── Agent A（独立 Token，公众号行业分析）
  ├── Agent B（独立 Token，产品更新）
  └── Agent C（独立 Token，活动运营）
       └── 共享该用户授权的公众号、素材和发布数据
```

必须满足：

1. 一个用户可以创建多个 Agent。
2. 每个 Agent 使用独立 Token，新增或轮换一个 Agent 的 Token 不影响其他 Agent。
3. Token 只能解析出服务端保存的用户和 Agent 身份，调用方不能通过参数伪造。
4. 公众号仍归用户所有，不归某个 Agent 所有。
5. Agent 的文章操作可以追溯，但不得在日志或 API 中泄露 Token 明文或摘要。
6. 现有用户 Token 在升级后继续有效，升级过程不要求所有 Agent 同时更换配置。

## 2. 当前问题

当前 `app_user` 直接保存一组 `api_token_hash`、`api_token_hint` 和 `api_token_created_at`。生成新 Token 会覆盖旧值，因此一个用户实际上只能有一个有效 MCP Token。

当前 MCP 只把 Token 解析为 `user_id`，不识别外部 Agent 的 `agent_id`。多个 Agent 共用同一个 Token 时可以同时工作，但服务端无法判断具体操作者，也无法单独撤销其中一个 Agent。

## 3. 核心边界

### 3.1 用户仍是租户边界

现有业务表中的 `tenant_id` 继续等于 `app_user.id`。所有账号、文章、素材、发布记录查询仍必须传入当前认证用户的 `user_id`。

`agent_id` 只表示用户数据空间中的操作者，不得替代 `tenant_id`，也不得扩大数据访问范围。

### 3.2 Agent 是调用身份

每个 Agent 有独立身份、用途说明和 Token。第一版所有 Agent 继承所属用户现有的公众号能力，不增加细粒度工具权限。

第一版保持用户内数据共享：

- 公众号授权、永久素材和微信实时数据对同一用户的 Agent 共享。
- 本地文章、草稿和发布记录仍属于用户，管理后台可以查看全部。
- 文章记录 Agent 归属，前端和 MCP 可以按 Agent 筛选。
- 第一版不禁止 Agent 读取同一用户中其他 Agent 的文章，避免破坏现有工作流。

如后续需要严格隔离不同 Agent 的草稿，再增加 Agent 级访问策略，不把该策略混入本次身份迁移。

## 4. 数据模型

新增 `app_agent`，一行代表一个用户下的数字员工及其当前凭证：

| 字段 | 类型 | 约束 | 说明 |
|---|---|---|---|
| `id` | TEXT | 主键 | 服务端生成的稳定内部 ID |
| `user_id` | TEXT | 外键、非空 | 所属 `app_user.id` |
| `agent_id` | TEXT | 非空 | Agent 平台提供的外部 ID |
| `name` | TEXT | 非空 | 管理后台展示名称 |
| `purpose` | TEXT | 默认空 | 文章类型或职责说明 |
| `status` | TEXT | active/disabled | Agent 是否允许认证 |
| `api_token_hash` | TEXT | 可空、全局唯一 | SHA-256 摘要；撤销后为空 |
| `api_token_hint` | TEXT | 默认空 | 只用于识别凭证的脱敏提示 |
| `api_token_created_at` | TIMESTAMPTZ | 可空 | 当前 Token 生成时间 |
| `last_used_at` | TIMESTAMPTZ | 可空 | 最近一次认证成功时间 |
| `created_at` | TIMESTAMPTZ | 非空 | 创建时间 |
| `updated_at` | TIMESTAMPTZ | 非空 | 更新时间 |

约束：

```sql
UNIQUE (user_id, agent_id)
UNIQUE (api_token_hash) WHERE api_token_hash IS NOT NULL
CHECK (api_token_hash IS NULL OR LENGTH(api_token_hash) = 64)
```

Agent 不做物理删除，只允许停用或撤销 Token，保证历史操作仍能关联到原 Agent。

### 4.1 文章归属

第二阶段给 `wechat_article` 增加可空字段：

```text
created_by_agent_id
updated_by_agent_id
```

字段引用 `app_agent.id`。浏览器用户、旧静态 Token 和迁移前文章可以为空。Agent 更新文章时只更新 `updated_by_agent_id`，不改变创建归属。

发布记录继续通过文章找到创建 Agent；真实发布动作另外写入审计日志，避免把“作者”和“发布者”混为一谈。

### 4.2 操作审计

新增追加写 `app_agent_audit_log`：

| 字段 | 说明 |
|---|---|
| `id` | 审计记录 ID |
| `user_id` | 数据所有者 |
| `agent_record_id` | `app_agent.id`，浏览器操作可空 |
| `action` | create_article、update_article、upload_material、publish_article、delete_article 等 |
| `resource_type` | article、material、publish_record、account |
| `resource_id` | 资源 ID 的字符串形式 |
| `created_at` | 操作时间 |

审计日志不保存 Token、正文、图片 URL、微信 access token、refresh token 或请求体。

## 5. Token 规则

1. 新 Token 沿用高熵 `oat_` 前缀，明文只在生成或轮换成功时返回一次。
2. 数据库只保存 SHA-256 摘要和提示，不保存可恢复明文。
3. 创建 Agent 时生成第一枚 Token。
4. 轮换 Token 只更新目标 `app_agent`，该 Agent 的旧 Token 立即失效，其他 Agent 不受影响。
5. 撤销 Token 只清空目标 Agent 的凭证字段，Agent 记录和历史审计保留。
6. 停用用户时，其下所有 Agent 即使仍有 Token 也不能认证。
7. 停用 Agent 时，只影响该 Agent。

## 6. 认证主体

应用层不再只返回 `identity.User`，而是返回明确的认证主体：

```go
type Principal struct {
    User      User
    ActorType ActorType // user_token, agent_token, browser_session, legacy_token
    Agent     *Agent
}
```

MCP Agent Token 的认证流程：

```text
Authorization: Bearer oat_xxx
  -> SHA-256
  -> app_agent.api_token_hash
  -> 检查 Agent active
  -> app_user
  -> 检查 User active
  -> Principal{UserID, AgentID}
  -> user_id 作为 tenant_id，agent.id 作为操作者
```

HTTP 上下文分别保存：

```text
current_user_id
current_user_role
current_actor_type
current_agent_record_id
current_agent_id
```

任何业务接口都不得接受客户端传入的 `user_id`、`tenant_id` 或 `agent_id` 来覆盖这些值。

HTTP handler 需要把认证结果转换为显式应用层输入，应用层不能读取 Gin context：

```go
type Actor struct {
    UserID        string
    ActorType     ActorType
    AgentRecordID string
}
```

例如创建文章时由 handler 传入 `CreateArticleInput{TenantID: actor.UserID, Actor: actor}`。应用服务再负责写文章归属和审计，避免身份逻辑渗透到 infra 或依赖 HTTP 框架。

## 7. 管理 API

以下接口只允许平台管理员调用：

### 7.1 列出用户的 Agent

```http
GET /api/v1/admin/users/:user_id/agents
```

只返回 Agent 元数据、Token 配置状态和提示，不返回 Token 摘要。

### 7.2 创建 Agent

```http
POST /api/v1/admin/users/:user_id/agents
Content-Type: application/json

{
  "agent_id": "alma-writer-01",
  "name": "行业分析 Agent",
  "purpose": "行业趋势与深度分析"
}
```

成功时返回 Agent 和一次性 Token。`agent_id` 在同一用户内唯一，长度 1-128；名称长度 1-64；用途长度不超过 200。

### 7.3 轮换单个 Agent Token

```http
POST /api/v1/admin/users/:user_id/agents/:id/api-token
```

只让该 Agent 的旧 Token 失效。

### 7.4 撤销单个 Agent Token

```http
DELETE /api/v1/admin/users/:user_id/agents/:id/api-token
```

### 7.5 更新或停用 Agent

```http
PATCH /api/v1/admin/users/:user_id/agents/:id

{
  "name": "产品更新 Agent",
  "purpose": "产品功能更新",
  "status": "disabled"
}
```

服务端必须同时校验路径中的 `user_id` 和 Agent 记录归属，禁止管理员路径参数错配后操作其他用户的 Agent。

## 8. MCP 行为

### 8.1 身份传播

MCP Streamable HTTP 验证 Token 后，将 `user_id`、`agent_record_id` 和 `agent_id` 写入 `TokenInfo.Extra`。MCP 调用后端 HTTP API 时继续转发原始 Bearer Token，由 API 再次完成真实身份解析。

MCP 工具参数不新增 `agent_id`。这能避免 Agent 伪装成另一个 Agent。

### 8.2 身份查询工具

新增只读工具：

```text
official_account_get_identity
```

返回当前用户名、用户 ID、Agent ID、Agent 名称和用途，不返回 Token 或 Token 提示。Agent 可以在工作流开始时确认自己正在使用的身份和内容职责。

### 8.3 公众号授权

公众号授权 `state` 继续绑定 `user_id`，而不是 Agent ID。一个用户只需授权一次公众号，其下所有 Agent 即可使用。

不同 Agent 同时生成的授权链接使用独立随机 `state`，互不覆盖。可以在审计日志中记录由哪个 Agent 发起授权，但公众号最终仍归用户所有。

### 8.4 MCP 配置接口

现有 `/api/v1/admin/mcp-config` 不再假设一个用户只有一枚 Token：

- 使用 Agent Token 调用时，返回当前 Agent 的名称、Agent ID、Token 提示和连接模板。
- 使用浏览器 session 调用时，只返回 MCP URL、Header 名称和 Agent 管理入口，不猜测用户想配置哪一个 Agent。
- 任何场景都不通过该接口恢复 Token 明文。

## 9. 管理后台

“用户管理”页改为两级结构：

```text
用户
  ├── Agent 名称 / Agent ID / 用途
  ├── Token 状态 / 提示 / 生成时间 / 最近使用时间
  └── 轮换 / 撤销 / 停用
```

交互要求：

1. “新建 Agent”时填写 Agent ID、名称和用途，并生成第一枚 Token。
2. Token 明文仅在创建或轮换后展示一次，默认使用密码样式遮挡并提供显示、复制按钮。
3. 轮换确认文案明确说明“只影响当前 Agent”。
4. 撤销和停用是两个不同动作：撤销只处理凭证，停用禁止该 Agent 认证。
5. 用户级旧 Token 在兼容期显示为“默认 Agent”，不继续展示“每用户一个 Token”的旧界面。

## 10. 兼容迁移

迁移文件建议使用 `013_multi_agent_token.sql`，只新增表和索引，不立即删除 `app_user` 的旧 Token 字段。

迁移步骤：

1. 创建 `app_agent`。
2. 为每个已有 `api_token_hash` 的用户创建一个 `agent_id=default`、名称为“默认 Agent”的记录，并复制原 Token 摘要和提示。
3. API 先查询 `app_agent`，兼容期内查询不到时再回退旧 `app_user.api_token_hash`。
4. 管理后台使用新 Agent API，不再轮换用户级 Token。
5. 完成线上验证并观察一个发布周期后，再单独迁移删除 `app_user` 旧 Token 字段和旧接口。

由于迁移复制的是摘要，已有 Token 明文无需重新获取，升级后应继续可用。

旧接口兼容策略：

- `POST /admin/users/:user_id/api-token` 在兼容期改为轮换“默认 Agent”。
- `DELETE /admin/users/:user_id/api-token` 在兼容期改为撤销“默认 Agent”。
- README 标记旧接口已弃用，但至少保留一个版本周期。

## 11. 并发与冲突

1. 两个 Agent 创建不同文章：允许并行。
2. 两个 Agent发布同一文章：继续由现有 `tenant_id + article_id + publishing` 唯一约束阻止重复发布。
3. 两个 Agent 更新同一文章：当前是后写覆盖先写。多 Agent 上线后应增加文章 `version` 乐观锁，旧版本更新返回 `409 conflict`。
4. 同时轮换同一个 Agent Token：数据库行锁或带版本条件的更新只允许一个结果成为最终有效 Token。
5. 同时创建相同 `(user_id, agent_id)`：唯一约束返回冲突，不生成重复 Agent。

文章乐观锁属于多 Agent 数据一致性任务，必须在开放多个 Agent 写入生产前完成。

## 12. 分阶段实现

### 阶段 A：Agent 身份与多 Token

允许修改：domain identity、memory/PostgreSQL identity store、迁移文件。

验收：

- 同一用户创建两个 Agent 后，两枚 Token 均可认证。
- 创建或轮换 Agent B 不影响 Agent A。
- 用户 B 的 Agent 不能访问用户 A 的任何资源。
- 已有用户 Token 迁移后仍可认证。

### 阶段 B：应用服务与管理 API

允许修改：application identity、HTTP 管理接口及其测试。

验收：

- Token 解析出正确的 `user_id + agent_id`。
- 调用方提交伪造 `agent_id` 不会改变服务端身份。
- 创建、列表、轮换、撤销和停用接口具备权限与归属测试。

### 阶段 C：MCP 身份传播

允许修改：MCP verifier、MCP client/server 及其测试。

验收：

- 两个 Agent Token 可以同时建立 Streamable HTTP 连接。
- MCP 转发请求后，API 解析出的 Agent 与入口 Token 一致。
- `official_account_get_identity` 返回正确身份且不包含任何 Token 信息。
- 轮换 Agent B 后，Agent A 的既有连接和新请求不受影响。

### 阶段 D：管理后台

允许修改：`web/admin` 用户管理 API 和页面。

验收：

- 一个用户下可管理多个 Agent。
- Token 明文只显示一次。
- 桌面和移动视口无文字溢出、按钮重叠。
- 浏览器完成创建、复制、轮换、撤销流程测试。

### 阶段 E：文章归属、审计和乐观锁

按 domain/application、infra、interfaces 分开提交，避免跨层大改。

验收：

- Agent 创建、更新、发布和删除文章均有操作者记录。
- 管理后台可以按 Agent 筛选文章和发布记录。
- 两个 Agent 用相同版本同时更新文章时，一个成功、另一个收到 `409`。
- 审计返回和日志中没有 Token、正文、图片 URL 或微信凭证。

### 阶段 F：生产部署验证

1. 备份 PostgreSQL。
2. 检查重复 `(user_id, agent_id)` 和 Token 摘要。
3. 执行迁移。
4. 依次部署 API、MCP 和前端。
5. 使用两个真实 Agent Token 完成：连接、列账号、各建一篇草稿、上传封面、发布状态同步、删除测试文章。
6. 轮换其中一个 Token，确认另一个 Agent 连接和数据访问不受影响。

## 13. 第一版明确不做

- 不让一个公众号归属于多个用户。
- 不允许 Agent 自己创建或提升权限。
- 不实现按 MCP 工具划分的细粒度 scopes。
- 不按 Agent 隔离公众号授权或永久素材。
- 不保存 Token 明文。
- 不立即删除旧用户 Token 字段和兼容接口。

这些边界用于保证第一版迁移可回滚，并把最重要的“独立凭证、不互相失效、可追溯”先做正确。
