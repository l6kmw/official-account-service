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

远程 MCP token 建议保存在服务端 `config.yaml`：

```yaml
mcp:
  token: "replace-with-a-long-random-mcp-token"
  path: "/mcp"
```

`OFFICIAL_ACCOUNT_MCP_TOKEN` 仍可作为兼容环境变量使用；线上建议以 YAML 为准，避免 token 分散在多个位置。

公网连接地址：

```text
URL: https://mp.example.com/mcp
Transport: streamable-http
Header name: Authorization
Header value: Bearer <MCP_TOKEN>
```

如果客户端只提供 API Key Header 配置，也可以使用：

```text
Header name: X-API-Key
Header value: <MCP_TOKEN>
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
- `OFFICIAL_ACCOUNT_TENANT_ID`：MCP 固定绑定的租户。多租户建议启动多个 MCP server，而不是让 agent 任意传 tenant。
- `OFFICIAL_ACCOUNT_ADMIN_API_KEY`：MCP 调用后台 API 使用的服务端密钥。不要放进前端运行时配置，也不要交给模型上下文。
- `OFFICIAL_ACCOUNT_MCP_ALLOWED_ROOT`：可选。限制 `file_path` 图片上传只能读取该目录下的文件；线上 Agent 上传的附件建议通过 `image_url` 传入。
- `OFFICIAL_ACCOUNT_MCP_TRANSPORT`：默认 `stdio`；线上远程连接使用 `streamable-http`。
- `OFFICIAL_ACCOUNT_MCP_ADDR`：Streamable HTTP 监听地址。线上建议只监听 `127.0.0.1`，再由 Nginx 提供 HTTPS。
- `OFFICIAL_ACCOUNT_MCP_PATH`：Streamable HTTP MCP 路径，默认 `/mcp`。
- `OFFICIAL_ACCOUNT_MCP_TOKEN`：可选兼容环境变量。远程 MCP 访问令牌优先建议配置在 `mcp.token`，只给受信任 agent 使用。它不是后台 `admin_api_key`。

## API Key

stdio 模式通过本机进程通信，不需要给 agent 暴露 MCP API key。

streamable-http 模式会暴露公网 HTTPS endpoint，必须配置 `mcp.token`，agent 连接时用：

```text
Authorization: Bearer <MCP_TOKEN>
```

部分 MCP 客户端把 API key 作为 `X-API-Key` 发送；服务端也兼容这种格式。不要选择 OAuth 登录流，除非后续单独实现 OAuth 授权服务器。

但 MCP 需要调用受保护的管理后台 API，所以线上服务应配置 `security.admin_api_key`，并通过 `OFFICIAL_ACCOUNT_ADMIN_API_KEY` 注入 MCP 进程。这样 agent 只能调用 MCP 工具，不能直接拿到后台 API key。

如果线上只启用了管理员登录 cookie，没有配置 `security.admin_api_key`，MCP 无法稳定调用写接口。建议给 MCP 单独配置一个长随机 `security.admin_api_key`，或由内网网关注入 `X-Admin-API-Key`。

## 授权二维码链接

MCP 提供 `official_account_get_authorization_entry`，返回：

- `authorization_entry_url`：可直接打开的公众号授权接入页。
- `qr_code_payload_url`：二维码内容，通常和 `authorization_entry_url` 相同。

agent 可以把 `qr_code_payload_url` 交给自己的 UI 生成二维码；如果 UI 暂时不能渲染二维码，就直接给用户打开 `authorization_entry_url`。

## 暴露的 MCP Tools

- `official_account_list_accounts`
- `official_account_list_articles`
- `official_account_list_published_articles`
- `official_account_get_article_metrics`
- `official_account_list_article_comments`
- `official_account_create_article`
- `official_account_update_article`
- `official_account_upload_image`
- `official_account_publish_article`
- `official_account_delete_article`
- `official_account_list_publish_records`
- `official_account_sync_publish_status`
- `official_account_delete_published_record`
- `official_account_get_authorization_entry`
- `official_account_generate_authorization_url`

安全约束：

- `official_account_publish_article` 必须传 `confirm_publish=true`。
- `official_account_delete_article` 只删除本地 `draft` / `failed` 文章，必须传 `confirm_delete="DELETE"`；`publishing` / `published` 文章会拒绝删除。
- `official_account_delete_published_record` 必须传 `confirm_delete="DELETE"`。
- 所有工具都复用现有后台 API，不返回 token、secret、refresh token。

`official_account_list_articles` 读取本地草稿和文章；`official_account_list_published_articles` 每次直接读取微信侧已发布列表，也会包含不经过本服务发布的历史文章。后者按微信消息使用 `offset` / `count` 分页，单条多图文消息可能展开成多个文章条目。

`official_account_get_article_metrics` 按文章发表日期查询阅读、分享、爱心赞、拇指赞、评论数、收藏、赞赏、阅读后关注、完成率和来源明细。`date` 必须为 `YYYY-MM-DD`，一次只能查一天，最晚为昨天；微信只保留每篇文章发表后 30 天内的统计。

`official_account_list_article_comments` 使用统计结果中的 `msgid` 查询具体评论，支持分页和普通/精选筛选。返回评论内容、精选状态和公众号回复，但不会向 Agent 暴露评论者 OpenID。

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
4. 调 `official_account_create_article` 创建草稿。
5. 如有正文图片，调 `official_account_upload_image` 上传 `inline_image`；线上附件使用 `image_url`，并把返回的 `wechat_url` 写回正文 HTML。
6. 上传封面图：调 `official_account_upload_image`，`usage=cover`；线上附件同样使用 `image_url`。
7. 调 `official_account_update_article` 写入最终标题、摘要、正文 HTML、`cover_media_asset_id`。
8. 发布前向用户确认标题、公众号、封面、摘要。
9. 调 `official_account_publish_article` 发布，传 `confirm_publish=true`。
10. 调 `official_account_sync_publish_status` 同步结果。
11. 调 `official_account_list_publish_records` 核对状态。
