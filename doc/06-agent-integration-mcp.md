# 公众号能力接入 Agent：MCP 方案

## 结论

公众号的真实执行能力用 MCP 接入即可。agent 通过 MCP 工具快速创建、更新、上传封面、发布和同步公众号文章；不再额外做 Skill。

MCP server 只调用本服务已有 HTTP API，不直接接触微信 AppSecret、数据库密码、authorizer refresh token 或 access token。agent 看到的是受控工具，不看到底层密钥。

## MCP Server

本仓库提供两种 MCP transport：

- `stdio`：本机 agent 直接拉起进程，默认模式。
- `streamable-http`：线上部署为 HTTPS MCP endpoint，适合远程 agent 调用。

本机 stdio 启动：

```bash
go run ./cmd/mcp-server
```

生产建议先构建二进制：

```bash
go build -trimpath -ldflags="-s -w" -o official-account-mcp ./cmd/mcp-server
```

线上 Streamable HTTP 启动：

```bash
OFFICIAL_ACCOUNT_MCP_TRANSPORT="streamable-http"
OFFICIAL_ACCOUNT_MCP_ADDR="127.0.0.1:8091"
OFFICIAL_ACCOUNT_MCP_PATH="/mcp"
./official-account-mcp -config /opt/official-account-service/config.yaml
```

Streamable HTTP 进程仍需要一个兼容启动 token。它只用于已有 `tenant-1` 管理员连接，不应分发给新用户：

```yaml
mcp:
  token: "replace-with-a-long-random-mcp-token"
  path: "/mcp"
```

`OFFICIAL_ACCOUNT_MCP_TOKEN` 仍可覆盖这个兼容 token。新用户 token 由管理员在“用户管理”中生成，数据库只保存 SHA-256 摘要，明文只返回一次。

公网连接地址：

```text
URL: https://mp.example.com/mcp
Transport: streamable-http
Header name: Authorization
Header value: Bearer <USER_MCP_TOKEN>
```

如果客户端只提供 API Key Header 配置，也可以使用：

```text
Header name: X-API-Key
Header value: <USER_MCP_TOKEN>
```

健康检查地址：

```text
https://mp.example.com/mcp/healthz
```

## 环境变量

```bash
OFFICIAL_ACCOUNT_BASE_URL="https://mp.example.com"
OFFICIAL_ACCOUNT_PUBLIC_BASE_URL="https://mp.example.com"
OFFICIAL_ACCOUNT_COMPONENT_APP_ID="wx_component_appid"
# 以下两项仅用于旧静态 token 或 stdio 的 tenant-1 兼容路径。
OFFICIAL_ACCOUNT_TENANT_ID="tenant-1"
OFFICIAL_ACCOUNT_ADMIN_API_KEY="replace-with-a-long-server-side-api-key"
OFFICIAL_ACCOUNT_MCP_ALLOWED_ROOT="/path/to/agent/workspace"
OFFICIAL_ACCOUNT_MCP_TRANSPORT="stdio"
OFFICIAL_ACCOUNT_MCP_ADDR="127.0.0.1:8091"
OFFICIAL_ACCOUNT_MCP_PATH="/mcp"
```

说明：

- `OFFICIAL_ACCOUNT_BASE_URL`：MCP 调用后端 API 的服务地址。
- `OFFICIAL_ACCOUNT_PUBLIC_BASE_URL`：用户浏览器可访问的公网 HTTPS 地址，用来生成授权接入链接和回调 URL；不填时默认使用 `OFFICIAL_ACCOUNT_BASE_URL`。
- `OFFICIAL_ACCOUNT_COMPONENT_APP_ID`：微信第三方平台 Component AppID，用来生成授权入口。
- `OFFICIAL_ACCOUNT_TENANT_ID`：仅用于旧静态 token / stdio 的兼容用户；用户 token 请求不会采用它。
- `OFFICIAL_ACCOUNT_ADMIN_API_KEY`：仅用于旧静态 token / stdio 调用后台 API。Streamable HTTP 用户 token 会直接绑定当前用户并转发给后端。
- `OFFICIAL_ACCOUNT_MCP_ALLOWED_ROOT`：可选。限制 `file_path` 图片上传只能读取该目录下的文件；线上 Agent 上传的附件建议通过 `image_url` 传入。
- `OFFICIAL_ACCOUNT_MCP_TRANSPORT`：默认 `stdio`；线上远程连接使用 `streamable-http`。
- `OFFICIAL_ACCOUNT_MCP_ADDR`：Streamable HTTP 监听地址。线上建议只监听 `127.0.0.1`，再由 Nginx 提供 HTTPS。
- `OFFICIAL_ACCOUNT_MCP_PATH`：Streamable HTTP MCP 路径，默认 `/mcp`。
- `OFFICIAL_ACCOUNT_MCP_TOKEN`：可选兼容环境变量，覆盖 YAML 中的旧管理员 token。不要把它当成多人共享 token。

## API Key

stdio 模式通过本机进程通信，不需要给 agent 暴露 MCP API key。

streamable-http 模式会暴露公网 HTTPS endpoint。管理员为用户下的每个 Agent 单独生成 token，Agent 连接时用：

```text
Authorization: Bearer <USER_MCP_TOKEN>
```

部分 MCP 客户端把 API key 作为 `X-API-Key` 发送；服务端也兼容这种格式。不要选择 OAuth 登录流，除非后续单独实现 OAuth 授权服务器。

MCP 会先向后端验证 Agent token，再用同一个 token 调用后台 API。后端从 token 再次解析 `user_id + agent_id`，忽略客户端提供的 `X-Tenant-ID`、`user_id` 或 `agent_id`。某个 Agent 的 token 被轮换、撤销或停用后，只有该 Agent 的后续 MCP 请求返回 `401`，同一用户下的其他 Agent 不受影响。

`security.admin_api_key` 与 `OFFICIAL_ACCOUNT_ADMIN_API_KEY` 只为旧静态 token / stdio 兼容保留，不要交给远程 Agent。

## 授权二维码链接

MCP 提供 `official_account_get_authorization_entry`。工具会使用当前用户 token 在服务端生成一次性 state，并返回：

- `authorization_entry_url`：可直接打开的微信官方授权页。
- `qr_code_payload_url`：二维码内容，与 `authorization_entry_url` 相同。

agent 可以把 `qr_code_payload_url` 交给自己的 UI 生成二维码，或直接打开 `authorization_entry_url`。链接不接受 `tenant_id`，同一公众号也不能被另一个用户重复认领。

## 暴露的 MCP Tools

每个工具的线上 `description` 都包含“调用前、输入、返回、后续、出错时”五部分。Agent 调用前应先用用户当前语言说明操作目的；缺少 ID、确认值或正文时先询问，不得猜测。

| Tool | 功能 | 关键输入 | 返回与下一步 |
|---|---|---|---|
| `official_account_get_identity` | 确认当前 MCP 用户与 Agent 身份 | 无；身份只来自当前 Token | 返回用户名、用户 ID、角色、Agent ID、名称和用途；不返回 Token、提示或摘要 |
| `official_account_list_accounts` | 列出当前用户已授权公众号 | 无 | 使用 `items[].id` 作为后续 `authorizer_id`；为空时生成授权入口 |
| `official_account_get_authorization_entry` | 生成绑定当前用户的一次性授权入口 | 可选 `component_appid` | 把完整 `authorization_entry_url` 或二维码交给公众号管理员扫码 |
| `official_account_generate_authorization_url` | 生成可限定授权类型/预选 AppID 的高级授权入口 | 可选 `auth_type`、`biz_appid`；通常不要传 `redirect_uri` | 原样打开首方 `authorization_url`，不得提取内部微信直链 |
| `official_account_list_drafts` | 列出本地 `draft` / `failed` 草稿 | 无 | 使用草稿 `id` 读取、编辑、发布或删除 |
| `official_account_get_draft` | 读取一个可编辑本地草稿的完整内容 | `draft_id` | 返回正文、封面 asset id 和状态 |
| `official_account_create_draft` | 创建本地草稿，不写微信草稿箱 | `authorizer_id`、`title` | 保存 `draft.id`，再上传图片和编辑 |
| `official_account_update_draft` | 完整替换本地草稿字段 | `draft_id`、标题、作者、摘要、HTML、封面 asset id | 返回保存后的草稿；未传字段不会自动保留 |
| `official_account_delete_draft` | 删除未发布本地草稿及本地素材 | `draft_id`、`confirm_delete="DELETE"` | 删除后用草稿列表复查；拒绝发布中/已发布文章 |
| `official_account_publish_draft` | 将本地草稿真实公开发布到微信 | `draft_id`、`confirm_publish=true` | 返回发布记录，继续同步状态 |
| `official_account_list_articles` | 列出全部本地文章状态 | 无 | 使用本地 article id 调用兼容工具；实时微信列表用下一个工具 |
| `official_account_create_article` | 兼容旧 Agent 的本地草稿创建工具 | `authorizer_id`、`title` | 新 Agent 优先使用 `create_draft` |
| `official_account_update_article` | 兼容旧 Agent 的完整文章更新工具 | `article_id`、标题、作者、摘要、HTML、封面 asset id | 返回保存后的本地文章 |
| `official_account_upload_image` | 上传正文图片或封面 | `authorizer_id`、`article_id`、`usage`，且三个图片来源只能选一个 | 正文使用 `asset.wechat_url`；封面绑定使用 `asset.id`，不是 `media_id` |
| `official_account_publish_article` | 兼容旧 Agent 的真实发布工具 | `article_id`、`confirm_publish=true` | 返回发布记录，继续同步状态 |
| `official_account_delete_article` | 删除微信已发布副本、本地文章和本地素材 | `article_id`、`confirm_delete="DELETE"` | 发布记录仍保留审计；用本地和微信列表复查 |
| `official_account_list_publish_records` | 列出本地发布审计记录 | 无 | `publishing` 继续同步；`published` 可删除微信副本 |
| `official_account_sync_publish_status` | 向微信同步一次发布任务状态 | `publish_record_id` | 返回 `publishing` / `published` / `failed` / `deleted` |
| `official_account_delete_published_record` | 只删除某条发布记录对应的微信副本，保留本地文章 | `publish_record_id`、`confirm_delete="DELETE"` | 成功后记录状态为 `deleted`，可编辑本地文章后重发 |
| `official_account_list_published_articles` | 实时读取微信已发布列表 | `authorizer_id`；可选分页和正文/删除项开关 | 返回 `msgid` 供评论查询，也包含非本系统发布的历史文章 |
| `official_account_get_article_metrics` | 查询某发表日期的阅读、分享、点赞、评论、收藏和完成率 | `authorizer_id`、`date=YYYY-MM-DD`，最晚昨天 | 返回该日文章累计指标；微信仅保留发表后 30 天数据 |
| `official_account_list_article_comments` | 实时读取一篇微信文章的评论 | `authorizer_id`、发布列表/统计返回的 `msgid` | 按 `next_begin` 分页，不返回评论者 OpenID |
| `official_account_list_permanent_materials` | 实时读取微信永久图片素材库 | `authorizer_id`；可选分页 | 返回名称、URL 和 `media_id`，删除前必须让用户核对 |
| `official_account_delete_permanent_material` | 不可恢复地删除微信永久素材 | `authorizer_id`、`media_id`、`confirm_delete="DELETE"` | 删除后复查素材库和引用它的草稿/文章 |

## MCP 参数错误与执行错误

调用参数会在进入业务逻辑前统一校验：

- 一次列出所有缺失字段，而不是只报第一个。
- 拒绝未知字段；`tenant_id` / `user_id` 不能由调用方覆盖。
- 检查 ID 正整数、分页范围、枚举、布尔类型、日期和确认值。
- `official_account_upload_image` 会检查三个图片来源必须且只能提供一个。
- 正文、Base64、文件路径、图片 URL 和回调 URL 不会原样回显到错误中。

缺参或错参时，`isError=true`，文本内容可以直接向用户解释，`structuredContent.error` 便于 Agent 自动修正：

```json
{
  "error": {
    "code": "invalid_tool_arguments",
    "tool": "official_account_create_draft",
    "issues": [
      {
        "code": "missing_argument",
        "field": "authorizer_id",
        "received": "未提供",
        "expected": "大于 0 的整数",
        "fix": "先调用 official_account_list_accounts，让用户确认公众号后使用返回的 id。"
      }
    ],
    "expected_inputs": "必填 authorizer_id、title；author、digest、content_html 可选。",
    "next_step": "如需图片先调用 upload_image，再用 update_draft 写入正文和封面。",
    "retryable": true
  }
}
```

业务执行错误同样返回可恢复信息：

| code | 含义 | Agent 应如何处理 |
|---|---|---|
| `resource_not_found` | 资源不存在、已删除或不属于当前用户 | 重新调用对应列表工具获取 ID，不要猜测 |
| `invalid_request` | 参数组合或资源状态不允许当前操作 | 按错误中的 `check` 检查状态、正文、封面或确认值 |
| `unauthorized` | MCP Token 无效、撤销或用户停用 | 请用户更新 Token，禁止输出旧 Token |
| `forbidden` | 用户或公众号缺少权限 | 核对公众号授权权限，必要时重新授权 |
| `conflict` | 资源被并发修改或公众号已归属其他用户 | 重新读取最新状态，不要覆盖别人的数据 |
| `configuration_required` | 服务配置或微信权限未完成 | 完成对应配置/授权后重试 |
| `timeout` | 微信或服务端超时，结果尚未确认 | 先查询列表/同步状态，避免盲目重复写操作 |
| `service_error` | 服务端内部错误 | 向用户说明稍后重试；内部细节只写安全日志 |

Agent 收到错误后必须逐项告诉用户：哪个 `field` 有问题、当前 `received` 内容、正确 `expected` 格式和 `fix`。不得只回复“参数错误”，也不得静默补造 ID、`media_id`、`msgid`、确认值或用户内容。

安全约束：

- `official_account_publish_draft` 必须传 `confirm_publish=true`。
- `official_account_delete_draft` 必须传 `confirm_delete="DELETE"`，并且只接受本地 `draft` / `failed` 状态的草稿。
- `official_account_publish_article` 必须传 `confirm_publish=true`。
- `official_account_delete_article` 必须传 `confirm_delete="DELETE"`；后端会先清理微信侧已发布副本，再删除本地文章和素材。
- `official_account_delete_published_record` 必须传 `confirm_delete="DELETE"`。
- `official_account_delete_permanent_material` 必须传 `confirm_delete="DELETE"`，并且只删除用户明确选择的 `media_id`。
- 所有工具都复用现有后台 API，只能访问 token 所属用户的数据，不返回 token、secret、refresh token。

草稿管理工具全部操作本服务数据库，不直接把内容写入微信草稿箱：`official_account_create_draft`、`official_account_update_draft`、`official_account_delete_draft`、`official_account_list_drafts` 和 `official_account_get_draft` 管理本地草稿；只有 `official_account_publish_draft` 会在用户确认后调用现有发布链路，临时创建微信草稿并提交发布。`official_account_list_drafts` 返回 `draft` 和可重试的 `failed` 状态，不返回 `publishing` / `published` 文章。

原有 `official_account_*_article` 工具继续保留以兼容已有 Agent。`official_account_list_articles` 读取全部本地草稿和文章；`official_account_list_published_articles` 每次直接读取微信侧已发布列表，也会包含不经过本服务发布的历史文章。后者按微信消息使用 `offset` / `count` 分页，单条多图文消息可能展开成多个文章条目。

`official_account_get_article_metrics` 按文章发表日期查询阅读、分享、爱心赞、拇指赞、评论数、收藏、赞赏、阅读后关注、完成率和来源明细。`date` 必须为 `YYYY-MM-DD`，一次只能查一天，最晚为昨天；微信只保留每篇文章发表后 30 天内的统计。

`official_account_list_article_comments` 使用实时文章列表或统计结果中的 `msgid` 查询具体评论，支持当天文章、分页和普通/精选筛选。返回评论内容、精选状态和公众号回复，但不会向 Agent 暴露评论者 OpenID。

`official_account_list_permanent_materials` 按 `authorizer_id` 实时读取该公众号的永久图片素材库，使用 `offset` / `count` 分页，单次最多 20 条。结果包含微信 `media_id`、素材名、更新时间和 URL；它与本地文章素材记录是两套数据，不会返回任何 access token。

`official_account_delete_permanent_material` 直接删除微信侧永久素材且不可恢复。调用前应先用列表工具确认素材名称和 `media_id`，再传 `confirm_delete="DELETE"`；删除仍被草稿或文章引用的素材可能导致图片失效。

## 线上 Agent 图片上传

`official_account_upload_image` 的图片来源必须且只能提供一个：

- `file_path`：MCP 服务所在机器可读取的本地文件。
- `content_base64`：Base64 图片内容。
- `image_url`：用户在 Agent 上传附件后，由 Agent 平台提供的临时公网 HTTPS 下载地址。

线上 Agent 推荐使用 `image_url`：

```json
{
  "authorizer_id": 1,
  "article_id": 12,
  "usage": "cover",
  "image_url": "https://agent-files.example.com/uploads/cover.png?signature=temporary"
}
```

服务端只下载公网 HTTPS 图片，拒绝回环、私网、链路本地、保留地址及指向这些地址的重定向。响应最多 8 MiB，仅接受 JPEG、PNG、GIF、WebP，并以实际文件内容判断类型。临时 URL 必须在工具调用期间有效，且无需额外 Cookie 或请求 Header 即可下载。

## 推荐 Agent 工作流

1. 调 `official_account_list_accounts` 查看是否已有授权公众号。
2. 如果没有账号，调 `official_account_get_authorization_entry`，把返回链接或二维码给用户扫码授权。
3. 生成文章内容，整理为微信兼容 HTML。
4. 调 `official_account_create_draft` 创建本地草稿。
5. 如有正文图片，调 `official_account_upload_image` 上传 `inline_image`；线上附件使用 `image_url`，并把返回的 `wechat_url` 写回正文 HTML。
6. 上传封面图：调 `official_account_upload_image`，`usage=cover`；线上附件同样使用 `image_url`。
7. 调 `official_account_update_draft` 写入最终标题、摘要、正文 HTML、`cover_media_asset_id`。
8. 发布前向用户确认标题、公众号、封面、摘要。
9. 调 `official_account_publish_draft` 发布，传 `confirm_publish=true`。
10. 调 `official_account_sync_publish_status` 同步结果。
11. 调 `official_account_list_publish_records` 核对状态。
