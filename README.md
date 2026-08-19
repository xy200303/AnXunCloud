# 安巡云 AnXunCloud · 物业巡检管理系统

**安巡云（AnXunCloud）**——物业巡检闭环管理系统：巡检员用**微信小程序**完成"到点打卡（二维码/GPS 围栏）+ 强制现场拍照 + 异常上报"，管理端用 **Web 后台**掌握每个小区/点位/人员的执行情况，形成"任务 → 执行 → 异常 → 整改 → 复核"完整闭环。

## 技术栈与架构

- 后端：Go 1.26 + Gin + GORM + PostgreSQL 15 + Redis 7（JWT 双令牌 + Redis 会话 + RBAC 权限点 + 小区级数据权限）
- 管理后台：Vue 3 + Vite + Element Plus + Pinia + ECharts
- 小程序：微信原生（预留 `miniprogram/`）
- 部署：Docker Compose 双版本（dev 热更新四容器 / prod 单容器单端口）

```
┌──────────────┐   ┌──────────────┐
│ 微信小程序端  │   │ Web 管理后台  │
└──────┬───────┘   └──────┬───────┘
       │ /api/mp          │ /api/admin（prod 同端口托管 SPA）
       └────────┬─────────┘
                ▼
        ┌───────────────┐     ┌────────────┐
        │  Go API (Gin) │────▶│ PostgreSQL │ 17+ 张表，打卡/操作日志按月分区
        │  迁移+seed 自启│────▶│   Redis    │ 会话/黑名单/限流/序号/锁
        └───────────────┘     └────────────┘
```

## 目录结构

```
物业管理平台/
├── backend/                # Go 后端（cmd/server 入口；internal/{config,middleware,module,pkg,router,template}）
│   ├── .air.toml           # air 配置（Windows 挂载卷下启用轮询）
│   └── internal/template/  # 页面模板与静态资源（官网 website/、pdfjs/、点位 H5；go:embed 随二进制分发）
├── frontend/               # Vue3 管理后台（部署在 /admin 子路径）
├── miniprogram/            # 微信小程序端（预留）
├── docs/                   # 方案/需求/接口/数据库/UI 文档
├── .env.dev                # 开发环境配置（docker dev 与本地直跑共用）
├── .env.prod               # 生产环境配置（含 CHANGE 占位，上线必改）
├── .env.example            # 配置模板（全量变量说明）
├── Dockerfile              # 唯一 Dockerfile，多 target：backend-dev / frontend-dev / prod
├── docker-compose.dev.yml  # 开发：postgres + redis + backend(air) + frontend(vite)
└── docker-compose.prod.yml # 生产：postgres + redis + app（单端口 SPA，target=prod）
```

## 环境要求

- Docker Desktop（推荐方式，一键起全栈）
- 本地直跑：Go 1.26+、Node 20+、PostgreSQL 15+、Redis 7+

## 配置说明（.env 集中管理）

配置来源**只有一种**：代码内默认值（`backend/internal/config`，用 viper.SetDefault 显式注册全部键）
+ 项目根目录 .env 文件 + 真实环境变量。优先级为 **真实环境变量 > .env 文件 > 代码默认值**。
后端启动时按 `APP_ENV`（或 `ENV_FILE` 指定路径）加载根目录对应 .env 文件（dev→.env.dev，prod→.env.prod）。

| 变量 | 含义 | 默认值 | 必填 |
|---|---|---|---|
| `APP_ENV` | 运行环境（dev/prod），决定默认加载的 .env 文件 | dev | 否 |
| `ENV_FILE` | 显式指定 env 文件路径（优先于 APP_ENV 推导） | - | 否 |
| `SERVER_PORT` | 后端监听端口 | 8090 | 否 |
| `SERVER_MODE` | gin 模式 debug/release | debug | 否 |
| `LOG_LEVEL` | 日志级别 debug/info/warn/error | info | 否 |
| `APP_BASE_URL` | 对外访问地址（拼文件 URL/OSS 回调） | - | 生产必填 |
| `POSTGRES_HOST/PORT/USER/PASSWORD/DBNAME/SSLMODE` | PostgreSQL 连接 | 见 .env.example | 是 |
| `REDIS_ADDR/PASSWORD/DB` | Redis 连接 | 127.0.0.1:6379 | 是 |
| `JWT_SECRET` | JWT 签名密钥 | - | **是（生产 ≥32 位随机串）** |
| `JWT_ACCESS_TTL` / `JWT_REFRESH_TTL` | 双令牌有效期 | 2h / 168h | 否 |
| `CORS_ALLOW_ORIGINS` | 允许跨域来源 | * | 否 |
| `ADMIN_USERNAME` / `ADMIN_PASSWORD` / `ADMIN_NAME` | 初始超管账号（仅首次 seed 生效，bcrypt 入库；prod 用默认弱密码会打印醒目警告） | admin / Admin@123 / 系统管理员 | 生产必改 |
| `WECHAT_APPID` / `WECHAT_SECRET` | 微信小程序凭据；**缺失或 MOCK=true 即 mock 登录模式**（code 传 `mock:<手机号>`，仅开发联调） | 空 | 否 |
| `WECHAT_MOCK` | 强制 mock 登录开关 | true（dev） | 否 |
| `UPLOAD_MODE` | 上传模式：dev 本地存储 / oss 阿里云直传 | dev | 否 |
| `UPLOAD_LOCAL_DIR` | dev 模式存储目录（以 /uploads 静态路由提供访问） | uploads | 否 |
| `UPLOAD_MAX_FILE_SIZE` | 单文件上限（字节） | 20971520 | 否 |
| `OSS_*`（ACCESS_KEY_ID/SECRET/ROLE_ARN/BUCKET/ENDPOINT/EXPIRE_SECONDS） | 阿里云 OSS + STS 配置 | 空 | oss 模式必填 |
| `WATERMARK_FONT_PATH` | 水印中文字体路径（TTF） | 仓库自带 `backend/fonts/NotoSansSC.ttf`，容器内固定 `/app/fonts/NotoSansSC.ttf` | 否 |
| `WATERMARK_LOGO_PATH` | 二维码标牌 LOGO 路径（PNG） | 仓库自带 `backend/assets/logo.png`（安巡云品牌图形标），容器内固定 `/app/assets/logo.png`；文件缺失时自动跳过 LOGO | 否 |
| `SPA_DIST_PATH` | 生产 SPA 托管目录（管理后台挂在 `/admin` 子路径，history 路由 fallback index.html） | 空（不启用） | prod 必填 |

## 快速开始（Docker）

### 开发环境（热更新）

```bash
docker compose --env-file .env.dev -f docker-compose.dev.yml up -d --build
```

- backend：air 监听 `./backend` 源码变更自动重编译（Windows 挂载卷用轮询）；同时托管官网 `http://localhost:8091/` 与下载页 `http://localhost:8091/download`（主机端口 8091→容器 8090，避开 8090 冲突）
- frontend：vite dev server + HMR，`/api`、`/uploads` 代理到 backend 容器；后台访问 `http://localhost:5181/admin/`（base 已改为 /admin 子路径；主机端口 5181→容器 5180，避开 5180 冲突）
- 首次启动自动完成建表迁移 + seed（超管/角色/菜单/字典/参数）

### 数据库迁移（goose）

结构迁移使用 [goose v3](https://github.com/pressly/goose)（库模式 + embed 内嵌），启动时自动执行，无需单独跑 CLI：

- 迁移文件：`backend/internal/pkg/database/migrations/` 下编号 SQL 文件（`-- +goose Up/Down` 注解）。**2026-08-18 起已把早期 25 个迁移 squash 为单一基线 `00001_init.sql`**（全量 schema，pg_dump 导出后清理）；基线只管结构，预置数据（角色/菜单/字典/默认租户/岗位模板/槽位默认/超管）全部由 seed 写入；开发期约定不兼容老数据，老环境重置数据库后由基线 + seed 重建
- 多副本同时启动由 PG advisory lock 串行化，失败即终止启动，不会带错误结构对外服务
- 新增迁移：新建下一个编号文件（如 `00002_xxx.sql`），`-- +goose Up` 写变更、`-- +goose Down` 写回滚，重启即生效
- `checkin_record` / `sys_operation_log` 是按月分区表，月份分区由 `EnsurePartitions` 在启动时和每日调度中滚动创建（不属于 goose 迁移）
- 排查版本：容器内 `psql -c "SELECT * FROM goose_db_version"`，或用 goose CLI `goose -dir backend/internal/pkg/database/migrations postgres "<dsn>" status`

### 生产环境（单端口 SPA）

```bash
# 先修改 .env.prod 中所有 CHANGE 占位（PG 密码、JWT 密钥、超管密码）
docker compose --env-file .env.prod -f docker-compose.prod.yml up -d --build
```

app 容器由根目录唯一 `Dockerfile` 的 `prod` target 多阶段构建：node 构建 `frontend/dist` → go 构建后端二进制 → alpine 运行镜像（含二进制 + dist + 中文字体）。
手动构建示例：`docker build --target prod -t anxuncloud-app .`（dev 镜像对应 `--target backend-dev` / `--target frontend-dev`）。
后端单端口托管全部页面：

- `/` **品牌官网**（产品介绍，模板源码在 `backend/internal/template/website/`，go:embed 随二进制分发，改完重启即生效）
- `/download` **App 下载页**（Android APK / HarmonyOS HAP / iOS IPA / 微信小程序码四个渠道；发布物在后台「系统管理 → 品牌官网 → 下载渠道」上传，官网自动开放对应渠道，无需改文件重启）
- `/admin/` **管理后台**（前端 history 路由刷新不 404）
- `/api`、`/uploads`、`/p/{code}`（点位短链接）正常走后端

官网内容与发布物均可在后台「系统管理 → 品牌官网」配置：标语、主题色、公司名称、电话/邮箱/微信/地址、ICP 备案号、页脚文案，以及各平台安装包/小程序码的上传与删除（文件走统一文件层，scene=app，上限 512MB）。官网为服务端渲染（Go 模板注入配置），带完整 SEO/GEO（meta/OG/JSON-LD/robots.txt/sitemap.xml），首页不暴露管理后台地址。

**构建加速**：Dockerfile 已内置国内镜像源（Go 模块 `goproxy.cn`、npm `npmmirror.com`、apk 阿里云镜像），可用 build-arg 覆盖：`GOPROXY` / `NPM_REGISTRY` / `ALPINE_MIRROR`（如 `--build-arg GOPROXY=direct`）。基础镜像（golang/node/alpine）拉取慢则在服务器 `/etc/docker/daemon.json` 配 `registry-mirrors`（如 `https://docker.m.daocloud.io`）后 `systemctl restart docker`。

## 本地不用 Docker 的开发方式

```bash
# 1. 数据库与缓存（任意方式：本地实例或临时容器）
docker run -d --name axc-local-pg -e POSTGRES_PASSWORD=dev_pwd_123 -e POSTGRES_DB=anxuncloud -p 25433:5432 postgres:15-alpine
docker run -d --name pi-local-redis -p 26380:6379 redis:7-alpine

# 2. 后端（自动加载 ../.env.dev；也可 ENV_FILE=../.env.dev 显式指定）
cd backend
go run ./cmd/server            # 或 air 热更新：go install github.com/air-verse/air@latest && air

# 3. 前端（主机 5181→容器 5180，/api 代理到 http://localhost:8091）
cd frontend
npm install && npm run dev
```

仅执行迁移+seed 不启动服务：`go run ./cmd/server -migrate-only`

## 演示数据（seed-demo）

演示数据是**独立命令**（`cmd/seed-demo`，与 server 主流程和系统预置 seed 完全解耦，不随服务启动执行），幂等（演示租户已存在则跳过）：

```bash
# dev
docker compose --env-file .env.dev -f docker-compose.dev.yml exec backend go run ./cmd/seed-demo
# prod（镜像内置独立 /app/seed-demo 二进制；执行后重启 app 使演示账号的 casbin 策略生效）
docker compose --env-file .env.prod -f docker-compose.prod.yml run --rm app /app/seed-demo
docker compose --env-file .env.prod -f docker-compose.prod.yml restart app
```

内容：两家演示物业公司（华安物业 `huaan` / 金源物业 `jinyuan`），各含租户管理员、小区、楼栋、项目编制（项目经理/主管/巡检员/维修工/楼管员/前台）、检查项模板、点位（二维码/NFC/围栏凭证）、巡检计划、昨日已完成+逾期与今日待巡任务、带真实照片的打卡记录（内置 `demoassets/`，离线可用）、六态全覆盖工单、月度报告（华安已三级签字归档、金源待巡检员确认）与通知公告；金源小区开启抢单模式作对照。全部演示账号密码统一为 `Demo@12345`（如 `huaan_admin`、`jinyuan_admin`、`ha_xj01`、`jy_xj01`）。

## 初始账号

- 超管：`.env` 中 `ADMIN_USERNAME` / `ADMIN_PASSWORD`（dev 默认 `admin` / `Admin@123`），首次登录请改密
- 小程序 mock 登录：`POST /api/mp/login`，body `{"code":"mock:<已开通手机号>"}`（需先在后台用户管理开通巡检员/维修工账号）
- 预置角色：super_admin（全部权限）/ tenant_admin（租户管理员）/ project_admin（项目级后台）/ field_staff（一线移动端）；业务身份由岗位编制 + 职责槽位表达

## 端口约定

| 用途 | 开发 | 生产 | 备注 |
|---|---|---|---|
| 后端 API | 8091→8090 | 18080→8080 | prod 唯一对外端口（含 SPA） |
| 前端 dev server | 5181→5180 | -（由后端托管） | 容器内与 frontend/vite.config.ts 的 server.port 保持一致 |
| PostgreSQL | 25433→5432 | 不对外 | 避开本机 pi-postgres(25432) |
| Redis | 26380→6379 | 不对外 | 避开本机 pi-redis(26379) |

## 常用命令

```bash
# 查看日志
docker compose -f docker-compose.dev.yml logs -f backend
docker compose --env-file .env.prod -f docker-compose.prod.yml logs -f app

# 重建（改依赖或 Dockerfile 后）
docker compose --env-file .env.dev -f docker-compose.dev.yml up -d --build backend

# 重新 seed（迁移幂等；seed 仅空库执行，重置需清卷）
docker compose -f docker-compose.dev.yml exec backend go run ./cmd/server -migrate-only
docker compose -f docker-compose.dev.yml down -v   # 清数据卷（慎用）

# 进入数据库
docker compose -f docker-compose.dev.yml exec postgres psql -U postgres -d anxuncloud

# 停止 / 清理
docker compose -f docker-compose.dev.yml down
docker compose --env-file .env.prod -f docker-compose.prod.yml down
```


## 权限体系（Casbin）

接口鉴权使用 **Casbin v3 + gorm-adapter v3.41（RBAC with domains）**，模型见 `backend/internal/pkg/authz/rbac_model.conf`：

- **请求/策略**：三元组 `r = sub, dom, obj` / `p = sub, dom, obj`，匹配 `g(r.sub,p.sub,r.dom) && r.dom==p.dom && (p.obj=="*" || keyMatch2(r.obj,p.obj))`；
- **domain（多租户预留）**：`dom` 当前统一为 `default`；未来多租户（物业公司级隔离）扩展时，按租户生成子域（如 `tenant:<id>`），策略与 g 规则挂在对应域下即可，业务代码无需改动。小区级数据权限不属于 casbin 域，仍由 `ApplyCommunityFilter`/`CheckCommunity` 在查询层过滤；
- **资源标识**：obj 直接为完整权限点字符串（如 `system:user:list`，不做路径拆分）；通配支持两种形式——`*` 全量（超管策略）与 `system:user:*` 模块级前缀（keyMatch2，为"某角色开放整个模块"预留）；
- **策略规则**：`p` 规则为 `role:<code> | default | <权限点>`（super_admin 角色为通配策略 `*`，天然覆盖后续新增权限点）；`g` 规则为 `user:<uuid> → role:<code>`（默认域）；策略持久化在 PG `casbin_rule` 表（基线迁移 00001 建表）；
- **策略同步时机**：`sys_role`/`sys_menu`/`sys_role_menu` 仍是后台管理的数据源（界面与 API 不变）。在以下时机由 `authz.SyncAll` 全量重建策略（ClearPolicy + SavePolicy，无脏策略）：角色分配菜单（2.4.6）、角色增删改、用户创建/改角色/停用/删除、服务启动 seed 后；
- **中间件**：路由上的 `RequirePerm("system:user:list")` 用法不变，内部为 `enforcer.Enforce(user:<uuid>, default, "system:user:list")`；登录态（JWT 双令牌 + Redis 会话）不受影响。

## 生产部署 Checklist

- [ ] `.env.prod`：`POSTGRES_PASSWORD`、`JWT_SECRET`（≥32 位随机）、`ADMIN_PASSWORD` 全部替换为强随机值
- [ ] `APP_BASE_URL` 改为正式域名；前置 Nginx/网关终止 HTTPS（全站强制 HTTPS）
- [ ] `WECHAT_APPID/SECRET` 填真实值并置 `WECHAT_MOCK=false`
- [ ] 照片存储切 OSS：`UPLOAD_MODE=oss` + 完整 `OSS_*`，桶开启版本控制、私有读
- [ ] `CORS_ALLOW_ORIGINS` 收敛为前端域名
- [ ] 超管首次登录后立即改密；按需创建角色与账号
- [ ] PostgreSQL 每日 `pg_dump` 备份到对象存储，保留 30 天
- [ ] 月度分区滚动已内建于服务（每日幂等检查），确认服务常驻即可

## 故障排查 FAQ

- **compose 报 "project name must not be empty"**：项目目录为中文名，compose 文件已内置项目名（dev 为 `anxuncloud-dev`，prod 为 `anxuncloud-prod`，两者相互独立可同时运行）；旧版本 compose 请自行加 `-p` 参数。
- **后端起不来，连不上数据库**：dev 容器内必须用服务名 `postgres`/`redis`（compose 已自动覆盖）；本地直跑确认用映射端口 25433/26380。
- **air 不触发热编译**：Windows 挂载卷已启用轮询（`.air.toml` poll=800ms）；仍无效时重启 backend 容器。
- **前端 5181 打不开/接口 404**：确认 backend 容器健康（`docker compose ps`）；代理目标由 `VITE_PROXY_TARGET` 注入，改了需重建 frontend 容器。
- **prod 访问 / 返回 404**：确认 `SPA_DIST_PATH=/app/dist` 且镜像构建日志中前端 build 成功。
- **水印不生成**：检查 `WATERMARK_FONT_PATH` 指向的 TTF 存在且支持中文；字体重量缺失时仅跳过水印不影响打卡。
- **登录报 40105**：确认库已 seed（首次启动日志有"数据库迁移与初始化完成"）；改过 `ADMIN_PASSWORD` 但库已初始化时不会覆盖已有账号，用重置密码接口或清卷重 seed。
