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
- `OFFICIAL_ACCOUNT_MCP_ALLOWED_ROOT`：可选。限制 `file_path` 图片上传只能读取该目录下的文件；目录外图片可以用 `content_base64` 传入。
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

## 推荐 Agent 工作流

1. 调 `official_account_list_accounts` 查看是否已有授权公众号。
2. 如果没有账号，调 `official_account_get_authorization_entry`，把返回链接或二维码给用户扫码授权。
3. 生成文章内容，整理为微信兼容 HTML。
4. 调 `official_account_create_article` 创建草稿。
5. 如有正文图片，调 `official_account_upload_image` 上传 `inline_image`，把返回的 `wechat_url` 写回正文 HTML。
6. 上传封面图：调 `official_account_upload_image`，`usage=cover`。
7. 调 `official_account_update_article` 写入最终标题、摘要、正文 HTML、`cover_media_asset_id`。
8. 发布前向用户确认标题、公众号、封面、摘要。
9. 调 `official_account_publish_article` 发布，传 `confirm_publish=true`。
10. 调 `official_account_sync_publish_status` 同步结果。
11. 调 `official_account_list_publish_records` 核对状态。
