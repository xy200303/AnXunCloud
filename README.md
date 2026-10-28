# AI云管家 · 物业巡检 SaaS

AI云管家（仓库名 AnXunCloud）面向物业公司的设备巡检、工单闭环、承接查验、设备台账和报告归档。系统包含 PC 管理后台、uni-app 移动端和 Go 后端，支持多租户、角色与任职数据范围、AI 辅助识别以及基于甲方母版的 DOCX/PDF 输出。

本期不包含社区运营和设备接入。

## 主要功能

- **巡检管理**：检查项模板、计划与轮次、任务生成、点位打卡、现场照片、云端草稿、AI 项级识别、人工复核。
- **工单闭环**：异常审核通过后自动入池，支持抢单、分派、整改和主管审核；审核通过即完成闭环，不设重复复查流程。
- **设备与维保**：设备台账、寿命与换粉周期、批量维保、逐项抽查、维保审核和到期提醒。
- **承接查验**：七类甲方母版、批次与清单快照、分户档案、问题接收、冻结、签署、归档和撤回重开。
- **报告中心**：使用甲方母版填充真实业务数据、照片和签字，输出 DOCX/PDF；报告文件按册别、阶段和修订保留。
- **权限与配置**：多租户隔离、角色＋任职模型、审批候选人、防止自审、平台/租户/小区配置覆盖、文件业务授权。
- **AI 与统计**：供应商和模型配置、能力绑定、配额、助手、Agent、MCP、River 异步任务和可重建统计投影。

## 技术栈

| 层 | 技术 | 目录 |
|---|---|---|
| 后端 | Go 1.26、Gin、GORM、PostgreSQL 15、Redis 7、Goose、River | `backend/` |
| PC 管理端 | Vue 3、Vite、TypeScript、Element Plus、Pinia、ECharts | `frontend/` |
| 移动端 | uni-app、Vue 3、uni-ui，支持 Android/H5 | `app/` |
| 报告渲染 | OOXML 母版、独立 LibreOffice worker、Poppler 校验 | `backend/cmd/report-worker/` |
| 部署 | Docker Compose 开发与生产配置 | 根目录 |

```text
uni-app / PC 管理端
        │
        ▼
   Go API（Gin）
   ├── PostgreSQL：业务事实、Goose 迁移、River 任务
   ├── Redis：会话、黑名单和缓存
   ├── AI 能力服务：识别、助手与配额
   └── 共享报告任务目录
            │
            ▼
   LibreOffice report-renderer
   DOCX → PDF → pdfinfo 结构校验
```

跨领域状态联动使用同事务函数调用和业务唯一键，不引入消息中间件。AI、通知和报告转换等外部副作用通过 River 执行。

## 目录结构

```text
物业管理平台/
├── backend/
│   ├── cmd/                     # server、seed-demo、report-worker
│   ├── internal/module/         # auth/community/equipment/file/inspection/mp/report/stats/system
│   │                            # 以及 acceptance/ai/workorder
│   ├── internal/pkg/database/   # Goose 迁移与系统 seed
│   ├── internal/pkg/ooxml/      # 甲方母版与 OOXML 渲染
│   └── internal/pkg/libreoffice/# PDF 转换与 worker 客户端
├── frontend/                    # PC 管理后台
├── app/                         # uni-app 移动端
├── docs/AI云管家/设计方案/      # 业务与架构设计（01～14）
├── docs/AI云管家/APP原型/       # App 原型与交互说明
├── docs/AI云管家/后台系统原型/  # PC HTML 原型与预览
├── deploy/                      # 其他部署组件
├── Dockerfile                   # backend-dev / frontend-dev / prod / report-renderer
└── docker-compose.*.yml         # 开发、生产编排
```

## 环境要求

推荐 Docker Desktop。本地直跑需要：

- Go 1.26+
- Node.js 20+
- PostgreSQL 15+
- Redis 7+
- 如需在本机直接转换 PDF，还需 LibreOffice；使用 Compose 时无需单独安装

## 配置

复制配置模板并填写开发环境变量：

```bash
cp .env.example .env.dev
```

配置优先级为：**真实环境变量 > `.env` 文件 > 代码默认值**。后端按 `APP_ENV` 加载根目录对应文件，也可通过 `ENV_FILE` 显式指定。

| 变量 | 用途 |
|---|---|
| `APP_ENV` / `ENV_FILE` | 运行环境或显式配置文件路径 |
| `POSTGRES_*` | PostgreSQL 连接 |
| `REDIS_*` | Redis 连接 |
| `JWT_SECRET` | JWT 签名密钥，生产环境使用至少 32 位随机值 |
| `ADMIN_USERNAME` / `ADMIN_PASSWORD` / `ADMIN_NAME` | 首次初始化超管账号 |
| `UPLOAD_MODE` / `UPLOAD_LOCAL_DIR` | 本地或云文件存储 |
| `OSS_*` / `COS_*` | 对应云存储配置 |
| `LIBREOFFICE_*` | PDF 转换方式、共享目录、超时和大小限制 |
| `WECHAT_*` / `UNIPUSH_*` | 移动通道配置，生产环境关闭 mock |
| `SPA_DIST_PATH` | 生产环境 PC 前端托管目录 |

真实密码和密钥只放在未入库的 `.env.dev`、`.env.prod` 或部署环境变量中，不要提交。

## Docker 开发环境

```bash
docker compose --env-file .env.dev -f docker-compose.dev.yml up -d --build
```

开发编排包含 PostgreSQL、Redis、Go 后端、Vite 前端和独立报告渲染器。首次启动自动执行数据库迁移和系统 seed。

- PC 管理端：`http://localhost:5181/admin/`
- 后端与品牌官网：`http://localhost:8091/`
- App 下载页：`http://localhost:8091/download`

Vite 的 `/api` 默认代理到 `https://pi.hbuer.com`。联调本地后端时，将 `docker-compose.dev.yml` 中的 `VITE_PROXY_TARGET` 改为 `http://backend:8090`，再重建 frontend 容器。

后端使用 air 热重载；Windows 挂载目录已启用轮询。

## 本地开发

```bash
# 后端，自动加载 ../.env.dev
cd backend
go run ./cmd/server

# 或使用 air
air
```

```bash
# PC 管理端
cd frontend
npm ci
npm run dev
```

```bash
# App H5
cd app
npm ci
npm run dev:h5
```

仅执行迁移和系统初始化：

```bash
cd backend
go run ./cmd/server -migrate-only
```

生成独立演示数据：

```bash
# Docker 开发环境
docker compose --env-file .env.dev -f docker-compose.dev.yml exec backend go run ./cmd/seed-demo

# 本地环境
cd backend
go run ./cmd/seed-demo
```

## 数据库迁移

迁移文件位于 `backend/internal/pkg/database/migrations/`，使用 Goose embed 随二进制分发，服务启动时自动执行。PostgreSQL 会话锁负责串行化多实例启动，迁移失败会中止启动。

新增迁移时使用下一个编号，不写跨领域兼容层。破坏性升级必须先停写、备份数据库和文件，再按设计文档执行转换与恢复验证。

`checkin_record`、`sys_operation_log` 等按月分区由服务启动和每日调度幂等维护。

## 报告与 PDF

报告以甲方 DOCX 母版为版式来源，不再使用代码自绘 PDF。后端生成冻结 DOCX 后，通过共享目录交给 `report-renderer` 转换，并校验 PDF 结构。

渲染容器默认：

- 无网络、只读文件系统；
- 非特权用户运行，移除全部 Linux capabilities；
- 独立临时目录、字体和 LibreOffice profile；
- 限制 CPU、内存、进程数、输入输出大小和转换超时。

单独构建镜像：

```bash
docker build --target report-renderer -t anxuncloud-report-renderer:local .
```

Dockerfile 的 `GOPROXY`、`NPM_REGISTRY`、`ALPINE_MIRROR`、`DEBIAN_MIRROR` 可用于覆盖构建镜像源。

## 测试与构建

```bash
# 后端测试与构建
cd backend
go test ./...
go build -o ./tmp/server ./cmd/server
```

```bash
# PC 类型检查与生产构建
cd frontend
npm run type-check
npm run build
```

```bash
# App H5 构建
cd app
npm run build:h5
```

Android 安装包通过 HBuilderX 云打包，`app/manifest.json` 中的版本号不得低于服务端强更阈值。

## 生产部署

生产地址按当前项目约定为 `https://pi.hbuer.com`。编排包含 PostgreSQL、Redis、应用和报告渲染器，应用单端口托管 API、PC 管理端、官网和下载页。

```bash
cp .env.example .env.prod
# 填写数据库、JWT、初始管理员等生产配置后：
docker compose --env-file .env.prod -f docker-compose.prod.yml up -d --build
```

主要入口：

- `/`：品牌官网
- `/download`：App 下载页
- `/admin/`：PC 管理后台
- `/api`：后端 API
- `/p/{code}`：点位短链接

上线前至少确认：

- 替换所有弱密码和占位密钥，收敛 CORS，关闭移动通道 mock。
- 使用 HTTPS 反向代理，配置文件持久化、私有云文件授权及数据库/文件备份。
- 核对 `render_prod` 共享卷和渲染容器资源限制，实际生成并下载一份 PDF。
- 按发布门禁完成空库/存量迁移、真实 API 全流程和甲方母版验收。

## 端口约定

| 用途 | 开发 | 生产 |
|---|---|---|
| 后端/API/官网 | `8091 → 8090` | `18080 → 8080` |
| PC Vite | `5181 → 5180` | 由后端托管 |
| PostgreSQL | `25433 → 5432` | 不对外 |
| Redis | `26380 → 6379` | 不对外 |
| report-renderer | 仅共享卷，无网络端口 | 仅共享卷，无网络端口 |

## 开发文档

- 开发约定与领域边界：`AGENTS.md`
- 完整业务设计：`docs/AI云管家/设计方案/`
- 验收与待确认事项：`docs/AI云管家/设计方案/10-验收与需求追踪.md`
- App 原型：`docs/AI云管家/APP原型/`
- PC 后台原型：`docs/AI云管家/后台系统原型/`
