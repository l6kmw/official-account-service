# Official Account Service

Official Account Service 是一个微信公众号第三方平台微服务，用来完成公众号授权、账号管理、文章草稿、素材上传、发布、发布状态同步，以及给外部 Agent 使用的 MCP 工具接入。

它由几部分组成：

- 后端 HTTP API：`cmd/server`，默认监听 `:8080`。
- 管理后台前端：`web/admin`，React + Vite，构建后作为静态文件部署。
- MCP Server：`cmd/mcp-server`，支持本地 `stdio` 和线上 `streamable-http`。
- PostgreSQL：生产环境持久化文章、账号、发布记录、微信回调审计数据。
- Redis：生产环境用于 token 刷新锁、跨实例 token 缓存和 asynq 异步队列。

核心文档：

- `doc/official-account-service-product-design-v1.0.md`
- `doc/02-AI协作开发约束规范.md`
- `doc/03-development-gap-and-roadmap.md`
- `doc/04-frontend-development-guide.md`
- `doc/05-frontend-ui-design-spec.md`
- `doc/06-agent-integration-mcp.md`

## 快速本地运行

本地开发可以不接 PostgreSQL、Redis 和真实微信配置。空 `database.dsn` 会使用内存存储，空 `redis.addr` 会关闭异步队列和分布式 token 缓存。

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

前端本地开发：

```bash
cd web/admin
npm install
npm run dev
```

Vite 开发服务默认运行在 `http://localhost:5173`，并把 `/api` 和 `/wechat` 代理到本地 Go 服务 `http://localhost:8080`。

前端构建检查：

```bash
cd web/admin
npm run build
```

## 部署前准备

推荐生产部署使用一个公网 HTTPS 域名承载管理后台、后端 API、微信授权入口、微信回调和 MCP，例如：

```text
PUBLIC_BASE_URL=https://mp.example.com
PUBLIC_DOMAIN=mp.example.com
```

如果管理后台和授权入口拆成不同域名，必须确保 `wechat-authorize.html` 所在域名、授权回调页所在域名、微信第三方平台里填写的授权发起页域名完全一致。为了减少微信授权域名不一致的问题，第一版部署建议使用单域名。

部署前需要准备：

- 一台 Linux 服务器。
- Docker 和 Docker Compose，或 Go/Node 运行环境。
- 一个已配置 HTTPS 的公网域名。
- 微信开放平台第三方平台的 `Component AppID`、`AppSecret`、`Verify Token`、`EncodingAESKey`。
- PostgreSQL 14+。
- Redis 6+。
- 管理员账号密码。
- 一个给 MCP 远程 Agent 使用的长随机 token。

生成推荐密钥：

```bash
openssl rand -base64 32
openssl rand -base64 48
```

生成管理员密码 bcrypt hash：

```bash
htpasswd -bnBC 12 "" "your-admin-password" | tr -d ':\n'
```

`htpasswd` 通常来自 `apache2-utils` 或 `httpd-tools`。

## 配置文件

应用配置统一使用 YAML。真实环境不要使用 `.env` 作为主要配置来源。

本地开发：

```bash
cp config.yaml.example config.yaml
```

Docker Compose：

```bash
cp config.docker.yaml.example config.docker.yaml
```

生产配置模板：

```yaml
app:
  env: prod

http:
  addr: ":8080"

log:
  level: info

database:
  dsn: "postgres://official_account:<DB_PASSWORD>@postgres:5432/official_account?sslmode=disable"

redis:
  addr: "redis:6379"

security:
  admin_api_key: "<LONG_RANDOM_ADMIN_API_KEY_FOR_SERVER_SIDE_CALLS>"
  admin_username: "admin"
  admin_password_hash: "<BCRYPT_HASH>"
  admin_session_secret: "<LONG_RANDOM_SESSION_SECRET>"

wechat:
  component_app_id: "<WX_COMPONENT_APPID>"
  component_app_secret: "<WX_COMPONENT_APPSECRET>"
  api_base_url: ""
  component_verify_token: "<WX_VERIFY_TOKEN>"
  component_encoding_aes_key: "<WX_ENCODING_AES_KEY>"
  refresh_token_encryption_key: "<BASE64_32_BYTE_KEY>"

mcp:
  token: "<LONG_RANDOM_MCP_TOKEN>"
  path: "/mcp"
```

字段说明：

- `app.env`：生产环境设为 `prod`。`prod` 下必须配置后台鉴权。
- `http.addr`：后端 HTTP API 监听地址。
- `database.dsn`：PostgreSQL DSN。为空时仅适合本地内存模式。
- `redis.addr`：Redis 地址。为空时关闭 Redis 锁、缓存和异步队列。
- `security.admin_api_key`：服务端脚本或 MCP 调用管理 API 时使用，不要放进前端。
- `security.admin_username`、`security.admin_password_hash`、`security.admin_session_secret`：管理后台登录配置，三个字段需要一起配置。
- `wechat.component_app_id`：微信开放平台第三方平台 Component AppID。
- `wechat.component_app_secret`：微信开放平台第三方平台 AppSecret。
- `wechat.component_verify_token`：微信回调校验 Token。
- `wechat.component_encoding_aes_key`：微信消息加解密 EncodingAESKey。
- `wechat.refresh_token_encryption_key`：base64 编码的 32 字节密钥，用于加密保存 authorizer refresh token。
- `mcp.token`：远程 MCP 的访问 token。管理台 MCP 配置页会以掩码方式展示它，但不要写入前端静态配置。
- `mcp.path`：Streamable HTTP MCP 路径，默认 `/mcp`。

## 数据库初始化

如果使用仓库里的 `docker-compose.yml` 启动内置 PostgreSQL，首次创建数据库卷时会自动执行 `migrations/*.sql`。

如果使用外部或已有 PostgreSQL，需要手动按文件名顺序执行迁移：

```bash
export DATABASE_DSN='postgres://official_account:<DB_PASSWORD>@127.0.0.1:5432/official_account?sslmode=disable'

for f in migrations/*.sql; do
  psql "$DATABASE_DSN" -f "$f"
done
```

当前项目没有单独的迁移命令，后端启动时只连接并检查数据库，不会自动建表。

## Docker 部署后端

仓库内置的 `docker-compose.yml` 会启动后端、PostgreSQL 和 Redis。

```bash
cp config.docker.yaml.example config.docker.yaml
```

编辑 `config.docker.yaml`，至少填入：

- `app.env: prod`
- `security.admin_api_key`
- 管理员登录三项配置
- 微信第三方平台配置
- `wechat.refresh_token_encryption_key`
- `mcp.token`

启动：

```bash
docker compose up -d --build
curl http://127.0.0.1:8080/healthz
```

停止：

```bash
docker compose down
```

生产环境注意：

- 默认 compose 把 PostgreSQL `5432` 和 Redis `6379` 映射到宿主机，正式上线建议用防火墙限制访问，或移除端口映射只保留 Docker 内网访问。
- 当前 compose 只包含后端 HTTP API、PostgreSQL 和 Redis，不包含管理后台静态文件服务，也不包含远程 MCP 进程。
- Redis 配置当前只支持地址，不支持密码字段。生产环境应让 Redis 只在可信内网或 Docker 网络内可访问。

## 二进制部署后端

不用 Docker 部署后端时，可以直接构建二进制：

```bash
go build -trimpath -ldflags="-s -w" -o /opt/official-account-service/bin/official-account-service ./cmd/server
```

systemd 示例：

```ini
[Unit]
Description=Official Account Service API
After=network.target

[Service]
WorkingDirectory=/opt/official-account-service
ExecStart=/opt/official-account-service/bin/official-account-service -config /opt/official-account-service/config.yaml
Restart=always
RestartSec=3
User=official-account

[Install]
WantedBy=multi-user.target
```

启动后检查：

```bash
curl http://127.0.0.1:8080/healthz
```

## 管理后台部署

管理后台是静态前端，需要单独构建和部署。

```bash
cd web/admin
npm ci
npm run build
```

把 `web/admin/dist` 发布到 Nginx 静态目录，例如：

```bash
mkdir -p /var/www/official-account-admin
cp -R web/admin/dist/. /var/www/official-account-admin/
```

前端运行时配置在部署目录里的 `admin-config.js`。换服务器或换域名时，优先改这个文件，不需要重新构建前端。

```js
window.__OFFICIAL_ACCOUNT_ADMIN_CONFIG__ = {
  tenantID: 'tenant-1',
  publicBaseURL: 'https://<PUBLIC_DOMAIN>',
  componentAppID: '<WX_COMPONENT_APPID>',
  adminAPIKey: ''
}
```

字段说明：

- `tenantID`：当前管理后台默认租户 ID。
- `publicBaseURL`：公网 HTTPS 地址，用来生成授权入口页和微信回调地址。
- `componentAppID`：微信第三方平台 Component AppID。
- `adminAPIKey`：兼容旧部署的可选字段。公开站点不要使用它作为主要鉴权方式，生产环境使用管理员登录会话。

不要把管理员密码、AppSecret、Verify Token、EncodingAESKey、refresh token、`security.admin_api_key`、`mcp.token` 或任何数据库密码写入 `admin-config.js`。

## 远程 MCP 部署

远程 Agent 接入使用 `cmd/mcp-server` 的 `streamable-http` 模式。它是独立进程，不是 `cmd/server` 里的普通 HTTP 路由。

构建：

```bash
go build -trimpath -ldflags="-s -w" -o /opt/official-account-service/bin/official-account-mcp ./cmd/mcp-server
```

启动所需环境变量：

```bash
export OFFICIAL_ACCOUNT_MCP_TRANSPORT="streamable-http"
export OFFICIAL_ACCOUNT_MCP_ADDR="127.0.0.1:8091"
export OFFICIAL_ACCOUNT_MCP_PATH="/mcp"
export OFFICIAL_ACCOUNT_BASE_URL="http://127.0.0.1:8080"
export OFFICIAL_ACCOUNT_PUBLIC_BASE_URL="https://<PUBLIC_DOMAIN>"
export OFFICIAL_ACCOUNT_COMPONENT_APP_ID="<WX_COMPONENT_APPID>"
export OFFICIAL_ACCOUNT_TENANT_ID="tenant-1"
export OFFICIAL_ACCOUNT_ADMIN_API_KEY="<LONG_RANDOM_ADMIN_API_KEY_FOR_SERVER_SIDE_CALLS>"
export OFFICIAL_ACCOUNT_MCP_ALLOWED_ROOT="/opt/official-account-service/uploads"
```

启动：

```bash
/opt/official-account-service/bin/official-account-mcp -config /opt/official-account-service/config.yaml
```

`mcp.token` 建议保存在 `config.yaml`，不要分散到环境变量里。`OFFICIAL_ACCOUNT_MCP_TOKEN` 仍作为兼容环境变量可用，但线上建议以 YAML 为准。

systemd 示例：

```ini
[Unit]
Description=Official Account Service MCP
After=network.target

[Service]
WorkingDirectory=/opt/official-account-service
Environment=OFFICIAL_ACCOUNT_MCP_TRANSPORT=streamable-http
Environment=OFFICIAL_ACCOUNT_MCP_ADDR=127.0.0.1:8091
Environment=OFFICIAL_ACCOUNT_MCP_PATH=/mcp
Environment=OFFICIAL_ACCOUNT_BASE_URL=http://127.0.0.1:8080
Environment=OFFICIAL_ACCOUNT_PUBLIC_BASE_URL=https://<PUBLIC_DOMAIN>
Environment=OFFICIAL_ACCOUNT_COMPONENT_APP_ID=<WX_COMPONENT_APPID>
Environment=OFFICIAL_ACCOUNT_TENANT_ID=tenant-1
Environment=OFFICIAL_ACCOUNT_ADMIN_API_KEY=<LONG_RANDOM_ADMIN_API_KEY_FOR_SERVER_SIDE_CALLS>
Environment=OFFICIAL_ACCOUNT_MCP_ALLOWED_ROOT=/opt/official-account-service/uploads
ExecStart=/opt/official-account-service/bin/official-account-mcp -config /opt/official-account-service/config.yaml
Restart=always
RestartSec=3
User=official-account

[Install]
WantedBy=multi-user.target
```

检查：

```bash
curl http://127.0.0.1:8091/mcp/healthz
```

## Nginx 示例

下面示例使用同一个域名承载管理后台、后端 API、微信回调和 MCP。

```nginx
server {
    listen 80;
    server_name <PUBLIC_DOMAIN>;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl http2;
    server_name <PUBLIC_DOMAIN>;

    ssl_certificate /etc/letsencrypt/live/<PUBLIC_DOMAIN>/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/<PUBLIC_DOMAIN>/privkey.pem;

    root /var/www/official-account-admin;
    index index.html;

    client_max_body_size 12m;

    location /api/ {
        proxy_pass http://127.0.0.1:8080/api/;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto https;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header Authorization $http_authorization;
    }

    location /wechat/ {
        proxy_pass http://127.0.0.1:8080/wechat/;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto https;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }

    location = /healthz {
        proxy_pass http://127.0.0.1:8080/healthz;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto https;
    }

    location = /mcp {
        proxy_pass http://127.0.0.1:8091/mcp;
        proxy_http_version 1.1;
        proxy_buffering off;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto https;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header Authorization $http_authorization;
    }

    location = /mcp/ {
        proxy_pass http://127.0.0.1:8091/mcp;
        proxy_http_version 1.1;
        proxy_buffering off;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto https;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header Authorization $http_authorization;
    }

    location = /mcp/healthz {
        proxy_pass http://127.0.0.1:8091/mcp/healthz;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto https;
    }

    location / {
        try_files $uri $uri/ /index.html;
    }
}
```

检查 Nginx：

```bash
nginx -t
systemctl reload nginx
curl https://<PUBLIC_DOMAIN>/healthz
curl https://<PUBLIC_DOMAIN>/api/v1/healthz
curl https://<PUBLIC_DOMAIN>/mcp/healthz
```

`/healthz` 如果由前端静态站点接管，可能返回前端页面；后端健康检查以 `/api/v1/healthz` 为准。

## 微信第三方平台配置

在微信开放平台第三方平台中配置以下地址。所有 `<PUBLIC_DOMAIN>` 都替换为同一个公网域名。

```text
授权发起页域名:
<PUBLIC_DOMAIN>

授权事件接收 URL:
https://<PUBLIC_DOMAIN>/wechat/component/callback

授权后公众号消息与事件接收 URL:
https://<PUBLIC_DOMAIN>/wechat/authorizer/$APPID$/callback

授权回调 URL:
https://<PUBLIC_DOMAIN>/api/v1/wechat/authorization-callback?tenant_id=tenant-1&component_appid=<WX_COMPONENT_APPID>
```

注意：

- 授权入口页是 `https://<PUBLIC_DOMAIN>/wechat-authorize.html?tenant_id=tenant-1&component_appid=<WX_COMPONENT_APPID>`。
- `wechat-authorize.html` 会在浏览器中生成真实微信授权 URL，并跳转到微信二维码页。
- 授权入口页所在域名、授权后回调页所在域名、微信第三方平台填写的授权发起页域名必须一致。
- `component_verify_token` 和 `component_encoding_aes_key` 必须和微信平台页面填写的一致。
- 后端收到 `component_verify_ticket` 后，才能生成真实 `pre_auth_code`。

授权成功后，`GET /api/v1/wechat/authorization-callback` 会保存公众号账号。管理后台账号页应能看到新授权的公众号。

## MCP 客户端连接

Streamable HTTP MCP 连接信息：

```text
URL: https://<PUBLIC_DOMAIN>/mcp
Transport: streamable-http
Header name: Authorization
Header value: Bearer <MCP_TOKEN>
```

推荐 MCP URL 使用 `/mcp`，不要主动加尾部斜杠。上面的 Nginx 示例兼容 `/mcp/`，但不同客户端对重定向和路径规范化的处理不完全一致。

部分客户端使用 API Key Header 模板，也可以这样填：

```text
Header name: X-API-Key
Header value: <MCP_TOKEN>
```

Cherry Studio 等客户端常见写法：

```text
Authorization=Bearer <MCP_TOKEN>
```

MCP token 来自 `config.yaml` 的 `mcp.token`。管理后台登录后可以在 MCP 配置页查看连接模板，默认以掩码展示 token。

MCP 暴露的工具：

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

推荐 Agent 流程：

1. 调 `official_account_list_accounts` 查看是否已有授权公众号。
2. 没有账号时，调 `official_account_get_authorization_entry`，把返回链接或二维码给用户扫码授权。
3. 调 `official_account_create_article` 创建本地草稿。
4. 如有正文图或封面图，调 `official_account_upload_image` 上传。
5. 调 `official_account_update_article` 写入最终标题、摘要、正文 HTML 和封面素材 ID。
6. 发布前让用户确认标题、公众号、封面和摘要。
7. 调 `official_account_publish_article`，必须传 `confirm_publish=true`。
8. 调 `official_account_sync_publish_status` 和 `official_account_list_publish_records` 核对结果。

删除约束：

- `official_account_delete_article` 只删除本地 `draft` / `failed` 文章，必须传 `confirm_delete="DELETE"`。
- `publishing` / `published` 文章需要先用 `official_account_delete_published_record` 删除公众号侧内容。
- `official_account_delete_published_record` 也必须传 `confirm_delete="DELETE"`。

## 管理 API 鉴权

管理后台推荐使用管理员登录：

- `POST /api/v1/admin/session` 登录。
- 服务端签发 HttpOnly + SameSite cookie。
- 写操作需要 `X-CSRF-Token`。

服务端脚本和 MCP 进程可以使用：

```text
X-Admin-API-Key: <security.admin_api_key>
```

或：

```text
Authorization: Bearer <security.admin_api_key>
```

`security.admin_api_key` 和 `mcp.token` 是两个不同 token：

- `security.admin_api_key` 用于调用后端管理 API。
- `mcp.token` 用于远程 Agent 连接 MCP endpoint。

## API 概览

HTTP API 文档位于 `api/openapi.yaml`。

常用入口：

- `GET /api/v1/healthz`
- `POST /api/v1/admin/session`
- `GET /api/v1/accounts`
- `GET /api/v1/articles`
- `POST /api/v1/articles`
- `PUT /api/v1/articles/:id`
- `DELETE /api/v1/articles/:id`
- `POST /api/v1/materials/inline-images`
- `POST /api/v1/materials/covers`
- `POST /api/v1/articles/:id/publish`
- `GET /api/v1/publish-records`
- `POST /api/v1/publish-records/:id/sync-status`
- `POST /api/v1/publish-records/:id/delete-published`
- `GET /api/v1/admin/mcp-config`
- `GET /api/v1/wechat/authorization-url`
- `GET /api/v1/wechat/authorization-callback`
- `POST /wechat/component/callback`
- `POST /wechat/authorizer/:app_id/callback`

所有管理接口都需要 `X-Tenant-ID`。启用鉴权后，还需要管理员登录会话或后台 API key。

## 发布行为说明

- 未配置微信密钥时，真实微信能力会返回 `501 not_implemented`。
- 配置 `wechat.component_app_secret` 并收到 `component_verify_ticket` 后，服务会获取 `component_access_token` 和 `pre_auth_code`。
- 授权回调落库前会加密保存 `authorizer_refresh_token`。
- 配置 `redis.addr` 后，服务会启用 token 刷新锁、跨实例缓存和 asynq 队列。
- 配置 `wechat.component_app_id` 后，素材上传会调用微信接口。
- `POST /api/v1/articles/:id/publish` 会创建微信草稿并提交发布。
- 已发布文章再次发布会提交修订版；修订版发布成功后，会自动删除同一文章的上一版已发布内容。
- 删除文章时，如果本地保存了微信 `article_id`，后端会先调用微信 `freepublish/delete` 删除公众号侧图文，再删除本地文章。
- 正在发布中的文章会拒绝删除，避免微信异步发布结果变成孤儿内容。
- 已经删除本地文章但公众号侧仍存在的历史内容，可以通过发布记录页或 `POST /api/v1/publish-records/:id/delete-published` 删除。

## 上线检查清单

上线前确认：

- `config.yaml` 或 `config.docker.yaml` 不在 Git 暂存区。
- `app.env` 已设为 `prod`。
- 管理员登录已配置，且密码 hash 不是明文密码。
- `security.admin_api_key` 是长随机值，并且只给服务端脚本或 MCP 进程使用。
- `mcp.token` 是长随机值，并且只给受信任 Agent 使用。
- PostgreSQL 迁移已执行。
- Redis 只允许可信网络访问。
- Nginx 已启用 HTTPS。
- `/api/v1/healthz` 返回 `{"status":"ok"}`。
- `/mcp/healthz` 返回 `{"status":"ok"}`。
- `admin-config.js` 只包含可公开配置。
- 微信第三方平台的授权发起页域名与 `wechat-authorize.html` 的域名一致。
- 微信第三方平台的 Token、EncodingAESKey 和 YAML 配置一致。
- 管理后台能登录，账号页能生成授权入口。
- MCP 客户端能 `tools/list`。

## 常见问题

### 微信提示授权入口域名不一致

检查三处是否完全一致：

- 微信第三方平台填写的授权发起页域名。
- 浏览器打开的 `wechat-authorize.html` 所在域名。
- 授权回调地址 `redirect_uri` 所在域名。

推荐统一为：

```text
https://<PUBLIC_DOMAIN>/wechat-authorize.html
https://<PUBLIC_DOMAIN>/api/v1/wechat/authorization-callback
```

### MCP 返回的授权入口不是预期域名

检查 MCP 进程的：

```text
OFFICIAL_ACCOUNT_PUBLIC_BASE_URL
OFFICIAL_ACCOUNT_COMPONENT_APP_ID
OFFICIAL_ACCOUNT_TENANT_ID
```

`official_account_get_authorization_entry` 使用 `OFFICIAL_ACCOUNT_PUBLIC_BASE_URL` 生成 `wechat-authorize.html` 链接。

### MCP 连接 401

检查客户端请求头：

```text
Authorization: Bearer <MCP_TOKEN>
```

或：

```text
X-API-Key: <MCP_TOKEN>
```

这里的 `<MCP_TOKEN>` 是 `mcp.token`，不是管理员登录密码，也不是 `security.admin_api_key`。

### MCP 工具能连上但写接口失败

MCP 进程调用后端管理 API 时需要 `OFFICIAL_ACCOUNT_ADMIN_API_KEY`，它应等于 YAML 里的 `security.admin_api_key`。

### 管理后台连接异常

检查：

- `/api/v1/healthz` 是否可访问。
- Nginx `/api/` 是否代理到 `127.0.0.1:8080`。
- 浏览器是否已经登录管理员账号。
- 写操作是否携带服务端返回的 CSRF token。

### 授权成功后页面只显示 JSON

这是当前授权回调接口的正常返回。确认管理后台账号页是否出现新公众号；如果没有，检查后端日志、数据库连接和 `authorizer_refresh_token` 加密密钥。

### 修改已发布文章后公众号不会自动刷新

微信公众号已发布内容不会因为本地草稿修改自动刷新。需要再次发布修订版；修订版发布成功后，系统会自动删除上一版已发布内容。

### 删除本地文章后公众号里还存在

本地删除不一定代表公众号侧已删除。对于已经发布的内容，应在发布记录页执行删除公众号侧内容，或调用：

```text
POST /api/v1/publish-records/:id/delete-published
```

## 安全注意事项

- 不要提交 `config.yaml`、`config.docker.yaml` 或任何真实密钥。
- 不要把 `mcp.token`、`security.admin_api_key`、AppSecret、EncodingAESKey、数据库密码写入前端静态文件。
- 不要让 PostgreSQL 和 Redis 暴露到公网。
- 远程 MCP 必须放在 HTTPS 后面，并配置长随机 token。
- 管理后台建议只对可信人员开放；如果部署在公网，至少启用管理员登录和 HTTPS。
- 日志中不要主动打印 token、secret、refresh token 或微信回调原文。
