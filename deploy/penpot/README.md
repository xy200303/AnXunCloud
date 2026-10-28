# Penpot 自托管部署说明

> 目标：在你们自己的服务器上跑一套 Penpot（原型设计），与主业务环境隔离；不动现有 `docker-compose.dev.yml` / `docker-compose.prod.yml`。
> 版本策略：首次部署用 `:latest` 验证可用后，**把四个镜像 tag 钉死**（记录具体版本号），避免后续自动升级踩坑。

## 1. 前置条件

- 服务器：Docker + Docker Compose v2；建议 ≥ 4 核 8G（小团队；与主站同机时留足余量，Penpot 导出/渲染会吃 CPU）
- 域名：建议单独子域，如 `penpot.example.com`，解析到该服务器
- 反向代理：现有 Nginx / Caddy 均可，负责 TLS 终止并转发到 `127.0.0.1:${PENPOT_HTTP_PORT}`
- 磁盘：Postgres 数据卷 + 上传素材卷，按团队文件量预留（素材走本地卷，量大再接 MinIO/S3）

## 2. 部署步骤

```bash
cd deploy/penpot
cp penpot.env.example penpot.env
chmod 600 penpot.env
```

编辑 `penpot.env`：

- `PENPOT_PUBLIC_URI`：最终 HTTPS 域名（**必须与反代域名一致**，否则导出/分享链接会错）
- `PENPOT_SECRET_KEY`：`openssl rand -hex 32` 生成
- `PENPOT_DB_PASSWORD`：24 位以上随机串
- 默认关闭自助注册（邀请制）；需要开放时把 flags 改成 `enable-registration`

启动：

```bash
docker compose -f docker-compose.yml up -d
docker compose -f docker-compose.yml ps        # 五个容器应为 running，postgres healthy
docker compose -f docker-compose.yml logs -f penpot-backend   # 首次迁移日志
```

创建首个管理员（邀请制下必须先有人）：

```bash
docker compose -f docker-compose.yml exec penpot-backend \
  ./manage.sh create-profile --email admin@example.com --fullname "Admin" --password '强密码'
```

（若要提权为超管，加 `--is-admin`；具体参数以容器内 `./manage.sh create-profile --help` 为准。）

## 3. 反向代理（Nginx 示例）

```nginx
server {
    listen 443 ssl http2;
    server_name penpot.example.com;

    ssl_certificate     /etc/letsencrypt/live/penpot.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/penpot.example.com/privkey.pem;

    # 与 compose 中 PENPOT_HTTP_SERVER_MAX_BODY_SIZE 保持一致
    client_max_body_size 367001600;

    # 实时协作 WebSocket
    location /ws/notifications {
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_pass http://127.0.0.1:9001/ws/notifications;
    }

    # MCP WebSocket（enable-mcp 必需）
    location /mcp/ws {
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_pass http://127.0.0.1:9001/mcp/ws;
    }

    location / {
        proxy_pass http://127.0.0.1:9001;
        proxy_http_version 1.1;
        proxy_set_header Host $http_host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Scheme $scheme;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_redirect off;
    }
}
```

要点：

- **两个 WebSocket 路径必须单独放行**：`/ws/notifications`（实时协作）与 `/mcp/ws`（MCP），只配通用 `location /` 会握手失败
- `client_max_body_size` 与 compose 的 `PENPOT_HTTP_SERVER_MAX_BODY_SIZE` 对齐（367MB）
- 生产环境建议限制来源 IP 或加一层 basic auth，设计文件属内部资料

## 4. MCP 接入（agent 操作 Penpot）

- **远程 MCP**：登录自建实例 → 账号概览里复制 MCP 地址，形如
  `https://penpot.example.com/mcp/stream?userToken=<MCP Key>`
  把它配到 `~/.kimi-code/mcp.json` 的 `penpot` 服务（`transport: http`）即可，与云版写法一致。
- **本地 MCP**（实例版本暂不支持远程 MCP 时）：`npx -y @penpot/mcp@<与实例匹配的版本>`，需要同时在 Penpot 文件里打开插件，三件套（MCP 服务 + 插件服务 + 浏览器插件）都在跑。
- 版本对齐：npx 包版本要和 Penpot 实例版本一致，否则工具协议不匹配。

## 5. 备份 / 升级 / 回滚

**备份（建议每日）**

```bash
# 数据库
docker compose -f docker-compose.yml exec -T penpot-postgres \
  pg_dump -U penpot penpot | gzip > penpot-db-$(date +%F).sql.gz
# 素材卷（与 pg_dump 同一时间点）
docker run --rm -v deploy_penpot_assets:/data -v "$PWD":/backup alpine \
  tar czf /backup/penpot-assets-$(date +%F).tar.gz -C /data .
```

**升级**：先备份 → 修改镜像 tag 到目标版本 → `docker compose up -d` → 观察 backend 迁移日志；Penpot 只支持逐版本小步升级，**不要跨大版本跳**。

**回滚**：镜像 tag 改回旧版本 + 用备份恢复 Postgres 与素材卷（两者必须同一时间点，否则图纸引用会断）。

## 6. 从官方源码构建镜像（不用 Docker Hub 预构建镜像）

适用场景：要打补丁、改代码、或强制从 release tag 构建。构建链条较长，官方 CI 也是这套；**只部署的话建议直接用官方镜像**（第 2 节），源码构建留给需要改代码时。

### 6.1 工具链（构建机，非运行时）

- Docker + buildx、Git
- JDK 21+、Clojure CLI、Babashka（`bb`）
- Node 20+、pnpm
- Rust（`render-wasm` 需要 wasm 目标）
- Python 3（`manage.py` 与模板预取）

### 6.2 取源码并切到发布 tag

```bash
git clone https://github.com/penpot/penpot.git
cd penpot
git checkout <release-tag>        # 建议用 release tag，不要直接用 main
```

### 6.3 分别构建前后端 bundle

官方 Dockerfile 要求工作目录 `docker/images/` 下存在 `bundle-frontend/` 与 `bundle-backend/`，这两个目录来自仓库自身的构建脚本：

```bash
# 前端（在 frontend/ 目录执行）
cd frontend
./scripts/build <version>          # 产物在 frontend/target/dist/

# 后端（在 backend/ 目录执行）
cd ../backend
./scripts/build <version>          # 产物在 backend/target/dist/
```

### 6.4 放到 Dockerfile 期望的位置并构建镜像

```bash
cd ../docker/images
rm -rf bundle-frontend bundle-backend
cp -r ../../frontend/target/dist bundle-frontend
cp -r ../../backend/target/dist  bundle-backend

# 逐个构建（--local 用本机 docker 引擎，不打到 registry）
./build.sh frontend --local
./build.sh backend  --local
./build.sh exporter --local
```

`build.sh` 默认走 `PENPOT_DOCKER_NAMESPACE`（默认 `penpotapp`）与 `PENPOT_BUILD_VERSION`（默认 `latest`），建议显式指定，避免和官方镜像同名混淆：

```bash
PENPOT_DOCKER_NAMESPACE=ourorg PENPOT_BUILD_VERSION=<release-tag> ./build.sh frontend --local
```

### 6.5 compose 指向自建镜像

把 `docker-compose.yml` 里三个 `image:` 改成 `ourorg/frontend:<tag>`、`ourorg/backend:<tag>`、`ourorg/exporter:<tag>`（postgres/redis 仍用官方镜像），然后照第 2 节启动。

### 6.6 注意

- **必须用 release tag 构建**：`main` 分支与发布版行为可能不同，且官方镜像的握手/迁移脚本以 release 为准
- 构建产物不入库：`bundle-*`、`target/` 加进 `.gitignore`
- 自建镜像同样要跟版本小步升级；跨大版本时先看官方升级说明（如 2.18 新增 admin-console 服务）
- MCP 依赖 `enable-mcp` 标志；若构建后 `/mcp/ws` 握手失败，先检查该标志与反代的两条 WebSocket 路径


## 7. 与主站的关系

- 独立 compose 文件、独立网络与数据卷，不与业务库共用 Postgres
- 端口只绑 `127.0.0.1`（compose 默认），由反代对外
- 评估项：若与生产同机，先看内存与 CPU 余量；压力大时把 Penpot 单独放到设计协作机器


## 8. 邮件（SMTP）配置要点

- 变量名是 **`PENPOT_SMTP_DEFAULT_FROM`** / **`PENPOT_SMTP_DEFAULT_REPLY_TO`**，不存在 `PENPOT_SMTP_FROM`；写错会退回默认 `no-reply@example.com`
- 必须同时在 `PENPOT_FLAGS` 里加 **`enable-smtp`**，否则只配 env 不生效
- 腾讯企业邮（`smtp.exmail.qq.com:465`，SSL）要求**信封发件人 = SMTP 认证账号**，否则报 `501 mail from address must be same as authorization user`
- 建议用公共邮箱/别名做系统发信身份（如 `system@` 或 `noreply@`），密码用邮箱的"客户端专用密码"
- 验证：`docker compose exec penpot-backend python3 -c "import smtplib,ssl,os; ..."` 先测认证，再在 Penpot 里发一次团队邀请看是否收到
