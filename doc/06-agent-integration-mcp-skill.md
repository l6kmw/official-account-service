# 公众号能力接入 Agent：MCP + Skill 方案

## 结论

公众号的真实执行能力优先用 MCP 接入；Skill 用来约束 agent 的工作流。

- MCP：让 agent 调用真实工具，例如创建文章、上传封面、发布、同步发布状态。
- Skill：告诉 agent 应该怎样写公众号文章、怎样用 gzh 排版、发布前检查什么、什么时候必须让用户确认。

不要只做 Skill。Skill 只能描述流程，不能给 agent 一个稳定、可审计的工具边界。也不要让 agent 直接拿微信 AppSecret、refresh token 或数据库密码；MCP 只调用本服务已有 HTTP API。

## MCP Server

本仓库提供 stdio MCP server：

```bash
go run ./cmd/mcp-server
```

生产建议先构建二进制：

```bash
go build -trimpath -ldflags="-s -w" -o official-account-mcp ./cmd/mcp-server
```

### 环境变量

```bash
OFFICIAL_ACCOUNT_BASE_URL="https://mp.example.com"
OFFICIAL_ACCOUNT_TENANT_ID="tenant-1"
OFFICIAL_ACCOUNT_ADMIN_API_KEY="replace-with-a-long-server-side-api-key"
OFFICIAL_ACCOUNT_MCP_ALLOWED_ROOT="/path/to/agent/workspace"
```

说明：

- `OFFICIAL_ACCOUNT_BASE_URL`：公众号服务地址。
- `OFFICIAL_ACCOUNT_TENANT_ID`：MCP 固定绑定的租户。多租户建议启动多个 MCP server，而不是让 agent 任意传 tenant。
- `OFFICIAL_ACCOUNT_ADMIN_API_KEY`：MCP 调用后台 API 使用的服务端密钥。不要放进前端运行时配置。
- `OFFICIAL_ACCOUNT_MCP_ALLOWED_ROOT`：可选。限制 `file_path` 图片上传只能读取该目录下的文件；目录外图片可以用 `content_base64` 传入。

如果线上只启用了管理员登录 cookie，没有配置 `security.admin_api_key`，MCP 无法稳定调用写接口。建议给 MCP 单独配置一个长随机 `security.admin_api_key`，或由内网网关注入 `X-Admin-API-Key`。

## 暴露的 MCP Tools

- `official_account_list_accounts`
- `official_account_list_articles`
- `official_account_create_article`
- `official_account_update_article`
- `official_account_upload_image`
- `official_account_publish_article`
- `official_account_list_publish_records`
- `official_account_sync_publish_status`
- `official_account_delete_published_record`
- `official_account_generate_authorization_url`

安全约束：

- `official_account_publish_article` 必须传 `confirm_publish=true`。
- `official_account_delete_published_record` 必须传 `confirm_delete="DELETE"`。
- 所有工具都复用现有后台 API，不返回 token、secret、refresh token。

## 推荐 Agent 工作流

1. `official_account_list_accounts` 选择公众号。
2. 生成文章内容，使用 gzh 排版得到微信兼容 HTML。
3. `official_account_create_article` 创建草稿。
4. 如有正文图片，用 `official_account_upload_image` 上传 `inline_image`，把返回的 `wechat_url` 写回正文 HTML。
5. 上传封面图：`official_account_upload_image`，`usage=cover`。
6. `official_account_update_article` 写入最终标题、摘要、正文 HTML、`cover_media_asset_id`。
7. 发布前向用户确认标题、公众号、封面、摘要。
8. `official_account_publish_article` 发布，传 `confirm_publish=true`。
9. `official_account_sync_publish_status` 同步结果。
10. `official_account_list_publish_records` 核对状态。

## Skill 层建议

Skill 不负责保存密钥，不直接调微信 API，只负责规范 agent 行为：

```markdown
# Official Account Publisher Skill

Use this skill when the user wants to write, import, edit, or publish a WeChat official-account article.

Rules:
- Use gzh-compatible HTML for article body.
- Never ask for or expose AppSecret, access_token, refresh_token, admin password, or database password.
- Use MCP tools whose names start with official_account_ for execution.
- Before publishing, summarize account, title, digest, cover status, and ask for explicit confirmation.
- Call official_account_publish_article only after the user confirms.
- After publishing, sync status and report the publish record.
```

这个 Skill 可以放到你的 agent 技能目录里；MCP server 则作为真实工具后端。
