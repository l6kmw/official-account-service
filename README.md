# Official Account Service

这是一个微信公众号第三方平台服务，用来做：

- 公众号扫码授权
- 管理多个公众号账号
- 创建、编辑、删除文章
- 上传正文图片和封面
- 发布文章、同步发布状态、删除已发布内容
- 给外部 Agent 提供 MCP 工具

服务由三部分组成：

- 后端 API：Go 服务，默认 `:8080`
- 管理后台：`web/admin` 静态前端
- MCP 服务：单独进程，默认 `127.0.0.1:8091/mcp`

## 1. 本地开发

后端：

```bash
cp config.yaml.example config.yaml
make test
make run
```

前端：

```bash
cd web/admin
npm install
npm run dev
```

本地前端默认运行在：

```text
http://localhost:5173
```

本地后端默认运行在：

```text
http://localhost:8080
```

本地开发时，`database.dsn` 和 `redis.addr` 可以为空。为空时后端会使用内存存储，不适合生产。

## 2. 生产部署要准备什么

你需要准备：

- 一台 Linux 服务器
- 一个公网 HTTPS 域名，例如 `https://mp.example.com`
- Docker 和 Docker Compose
- PostgreSQL
- Redis
- 微信开放平台第三方平台配置
- 管理员账号密码
- MCP token

建议先统一一个公网地址：

```text
PUBLIC_DOMAIN=mp.example.com
PUBLIC_BASE_URL=https://mp.example.com
```

第一版部署建议管理后台、后端 API、微信授权入口、微信回调、MCP 都走同一个域名。这样最不容易踩微信授权域名不一致的问题。

## 3. 配置 YAML

本地真实配置文件不会提交到 Git。

这里有两个文件名，内容结构一样：

- `config.yaml`：本地运行、二进制部署、MCP 默认会读它。
- `config.docker.yaml`：仓库里的 `docker-compose.yml` 默认会把它挂载到容器内 `/app/config.yaml`。

如果你用 Docker Compose 部署后端，先复制：

```bash
cp config.docker.yaml.example config.docker.yaml
```

生产环境最少需要填这些：

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
  admin_api_key: "<LONG_RANDOM_ADMIN_API_KEY>"
  admin_username: "admin"
  admin_password_hash: "<BCRYPT_PASSWORD_HASH>"
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

生成随机密钥：

```bash
openssl rand -base64 32
openssl rand -base64 48
```

生成管理员密码 hash：

```bash
htpasswd -bnBC 12 "" "your-admin-password" | tr -d ':\n'
```

重点区分两个 token：

- `security.admin_api_key`：给服务端脚本和 MCP 进程调用后端 API 用。
- `mcp.token`：给外部 Agent 连接 MCP 用。

不要把这两个 token 写进前端 `admin-config.js`。

## 4. 启动后端、PostgreSQL、Redis

仓库里的 `docker-compose.yml` 会启动：

- `app`：后端 API
- `postgres`：数据库
- `redis`：队列、锁和 token 缓存

启动：

```bash
docker compose up -d --build
curl http://127.0.0.1:8080/healthz
```

看到下面结果就说明后端活着：

```json
{"status":"ok"}
```

停止：

```bash
docker compose down
```

如果你用外部 PostgreSQL，不用 compose 内置数据库，需要手动执行迁移：

```bash
export DATABASE_DSN='postgres://official_account:<DB_PASSWORD>@127.0.0.1:5432/official_account?sslmode=disable'

for f in migrations/*.sql; do
  psql "$DATABASE_DSN" -f "$f"
done
```

注意：当前后端启动时不会自动建表。

如果使用 Docker Compose，并且 `postgres_data` 已经存在，PostgreSQL 不会自动重新执行新加入的 migration。每次更新代码后先备份数据库，再执行：

```bash
for f in migrations/*.sql; do
  docker compose exec -T postgres psql -U official_account -d official_account < "$f"
done
```

本次安全更新至少需要应用 `008_authorization_state.sql` 和 `009_publish_in_progress_unique.sql`，再重启后端。

## 5. 构建并部署管理后台

构建前端：

```bash
cd web/admin
npm ci
npm run build
```

把构建产物放到 Nginx 静态目录：

```bash
mkdir -p /var/www/official-account-admin
cp -R web/admin/dist/. /var/www/official-account-admin/
```

修改线上静态目录里的 `admin-config.js`：

```js
window.__OFFICIAL_ACCOUNT_ADMIN_CONFIG__ = {
  tenantID: 'tenant-1',
  publicBaseURL: 'https://<PUBLIC_DOMAIN>',
  componentAppID: '<WX_COMPONENT_APPID>',
  adminAPIKey: ''
}
```

这里不要填真实密钥。它只放浏览器可以看到的公开配置。

## 6. 启动 MCP 服务

远程 MCP 是单独进程，不是后端 API 里自带的路由。

先构建：

```bash
go build -trimpath -ldflags="-s -w" -o /opt/official-account-service/bin/official-account-mcp ./cmd/mcp-server
```

启动前设置环境变量：

```bash
export OFFICIAL_ACCOUNT_MCP_TRANSPORT="streamable-http"
export OFFICIAL_ACCOUNT_MCP_ADDR="127.0.0.1:8091"
export OFFICIAL_ACCOUNT_MCP_PATH="/mcp"
export OFFICIAL_ACCOUNT_BASE_URL="http://127.0.0.1:8080"
export OFFICIAL_ACCOUNT_PUBLIC_BASE_URL="https://<PUBLIC_DOMAIN>"
export OFFICIAL_ACCOUNT_COMPONENT_APP_ID="<WX_COMPONENT_APPID>"
export OFFICIAL_ACCOUNT_TENANT_ID="tenant-1"
export OFFICIAL_ACCOUNT_ADMIN_API_KEY="<LONG_RANDOM_ADMIN_API_KEY>"
```

启动：

```bash
/opt/official-account-service/bin/official-account-mcp -config /opt/official-account-service/config.docker.yaml
```

检查：

```bash
curl http://127.0.0.1:8091/mcp/healthz
```

看到：

```json
{"status":"ok"}
```

就说明 MCP 进程活着。

## 7. Nginx 配置

下面是最小可用示例。把 `<PUBLIC_DOMAIN>` 替换成你的域名。

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

    location = /healthz {
        proxy_pass http://127.0.0.1:8080/healthz;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto https;
    }

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

    location = /mcp {
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
curl https://<PUBLIC_DOMAIN>/api/v1/healthz
curl https://<PUBLIC_DOMAIN>/mcp/healthz
```

## 8. 微信第三方平台要填什么

在微信开放平台第三方平台里填这些。

```text
授权发起页域名:
<PUBLIC_DOMAIN>

授权事件接收 URL:
https://<PUBLIC_DOMAIN>/wechat/component/callback

授权后公众号消息与事件接收 URL:
https://<PUBLIC_DOMAIN>/wechat/authorizer/$APPID$/callback

授权回调 URL:
https://<PUBLIC_DOMAIN>/api/v1/wechat/authorization-callback
```

授权入口页是：

```text
https://<PUBLIC_DOMAIN>/wechat-authorize.html?tenant_id=tenant-1&component_appid=<WX_COMPONENT_APPID>
```

最重要的一点：

```text
wechat-authorize.html 所在域名
= 授权回调 URL 所在域名
= 微信平台填写的授权发起页域名
```

这三个必须一致，否则微信会报授权入口域名错误。

授权发起时，服务端会自动生成一次性 `state` 并写入回调 URL。不要手工添加 `tenant_id` 或 `component_appid` 回调参数。

## 9. Agent 如何连接 MCP

Streamable HTTP 配置：

```text
URL: https://<PUBLIC_DOMAIN>/mcp
Transport: streamable-http
Header: Authorization=Bearer <MCP_TOKEN>
```

如果客户端是 API Key Header 模板，也可以填：

```text
Header name: X-API-Key
Header value: <MCP_TOKEN>
```

常用工具：

- `official_account_list_accounts`
- `official_account_get_authorization_entry`
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

删除时注意区分：

- `official_account_delete_article`：完整删除文章。后端会先删除所有已发布的微信内容，再删除本地文章和素材；发布记录会保留为 `deleted` 供审计。
- `official_account_delete_published_record`：只下线指定发布记录对应的微信内容，本地文章继续保留。

查看公众号当前全部已发布文章时，使用 `official_account_list_published_articles`。它每次直接查询微信，不依赖本地文章或发布记录，因此也能看到在微信后台或其他工具发布的历史文章。`offset` / `count` 按微信消息分页，`count` 最大为 20；默认不返回正文 HTML 和已删除条目。

查看阅读、分享、点赞、评论数、收藏等数据时，使用 `official_account_get_article_metrics`，传文章发表日期 `date=YYYY-MM-DD`。微信一次只允许查询一天，最晚只能查昨天，并且每篇文章只统计发表后 30 天内的数据。

查看具体评论时，使用 `official_account_list_article_comments`，把统计结果中的 `msgid` 原样传入。`type=0` 返回全部评论，`1` 只返回普通评论，`2` 只返回精选评论；单次 `count` 最大为 49。工具不会返回评论者 OpenID。

上传图片时，`file_path`、`content_base64`、`image_url` 必须三选一。线上 Agent 应把用户附件的临时公网 HTTPS 下载地址传给 `image_url`；服务端最多下载 8 MiB，并拒绝私网地址、危险重定向和非图片内容。

典型流程：

1. `official_account_list_accounts` 看有没有公众号。
2. 没有就用 `official_account_get_authorization_entry` 生成授权链接。
3. 创建文章草稿。
4. 上传封面和正文图片；线上附件通过 `image_url` 传入。
5. 更新文章内容。
6. 发布前让用户确认。
7. 调 `official_account_publish_article` 发布，必须传 `confirm_publish=true`。
8. 同步发布状态。

## 10. 上线后怎么检查

按顺序检查：

```bash
curl https://<PUBLIC_DOMAIN>/api/v1/healthz
curl https://<PUBLIC_DOMAIN>/mcp/healthz
```

然后检查：

- 能打开管理后台。
- 能登录管理员账号。
- 账号页能生成公众号授权入口。
- 扫码授权后，账号页能看到公众号。
- MCP 客户端能连接并列出工具。
- 发布测试文章后，发布记录能同步状态。

## 常见问题

### MCP 能用，但 config.yaml 里没有 mcp 配置

可能是 MCP token 来自环境变量：

```text
OFFICIAL_ACCOUNT_MCP_TOKEN
```

也可能是旧进程还没重启。长期部署建议把它写回 YAML：

```yaml
mcp:
  token: "<LONG_RANDOM_MCP_TOKEN>"
  path: "/mcp"
```

### MCP 401

检查请求头是不是：

```text
Authorization: Bearer <MCP_TOKEN>
```

注意这里是 `mcp.token`，不是管理员密码，也不是 `security.admin_api_key`。

### MCP 工具能连，但创建文章或发布失败

检查 MCP 进程环境变量：

```text
OFFICIAL_ACCOUNT_ADMIN_API_KEY
```

它必须等于 YAML 里的：

```text
security.admin_api_key
```

### 授权入口没有跳到微信二维码

检查：

- `wechat.component_app_secret` 是否配置。
- 微信是否已经推送 `component_verify_ticket`。
- `wechat.component_verify_token` 和 `wechat.component_encoding_aes_key` 是否和微信平台一致。
- 授权入口页域名和回调域名是否一致。

### 授权成功后页面只显示 JSON

这是当前正常行为。授权成功后去管理后台账号页看公众号是否出现。

### 修改已发布文章后公众号没有自动变化

公众号已发布内容不会因为本地文章修改而自动刷新。需要再次发布修订版。修订版发布成功后，系统会自动删除上一版已发布内容。

### 本地删了文章，公众号里还在

已经发布到公众号的内容，需要通过发布记录删除公众号侧内容：

```text
POST /api/v1/publish-records/:id/delete-published
```

## 安全提醒

- 不要提交 `config.yaml`、`config.docker.yaml`。
- 不要把 AppSecret、EncodingAESKey、数据库密码、`security.admin_api_key`、`mcp.token` 放进前端。
- 不要把 PostgreSQL 和 Redis 暴露到公网。
- MCP 必须走 HTTPS，并且必须配置长随机 token。
- 生产环境必须开启管理员登录或后台 API key。
