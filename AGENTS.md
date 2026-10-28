# AGENTS.md — AI云管家（物业巡检 SaaS）开发指南

> 面向后续开发者（人/AI 通用）。读完本文再动代码。
> 设计方案全文在 `docs/AI云管家/设计方案/`（01~14），**本文是开发约定，方案是业务设计，两者都读**。

## 1. 项目是什么

AI云管家（原"安巡云"演进）是给大型物业公司做的**设备巡检 + 工单闭环 + 台账/报告输出**系统，后面要卖成 SaaS。

- 巡检：点位打卡（二维码/NFC/围栏）→ 表单式检查 → AI 识别 → 审核链 → 报告
- 工单：巡检异常审核通过 → 自动形成待接单工单 → 工单池（抢单/分派）→ 整改 → 主管审核通过闭环
- 台账：设备全生命周期（生产日期/换粉周期/到期提醒/抽查）
- 报告：按甲方版式出 5 本巡检月报 PDF、承接查验六个类别（母版共六份，**册号待甲方确认，不硬编码 1~6**，见 05 §5）、换粉台账 xlsx
- 承接查验：物业交接时的查验记录与报告（期2 并行）

## 2. 技术栈与目录

| 层 | 技术 | 目录 |
|---|---|---|
| 后端 | Go 1.26 + Gin + GORM + PostgreSQL（按月分区）+ Goose；Redis 仅会话/黑名单。**跨域联动走同事务函数调用，不建事件总线**；通知/AI/渲染异步任务使用 `riverqueue/river` + 同一 PG（01 §4） | `backend/` |
| PC 管理端 | Vue 3 + Vite 8 + TypeScript + Element Plus + Pinia | `frontend/` |
| App | uni-app + Vue 3 + uni-ui（DCloud 官方，HBuilderX 打包 Android） | `app/` |
| 文档 | 设计方案 `docs/AI云管家/设计方案/`、App 原型 `docs/AI云管家/APP原型/`、PC 原型 `docs/AI云管家/后台系统原型/`、甲方模板 `docs/模板/` | `docs/` |
| 部署 | Docker Compose（dev/prod），生产 `https://pi.hbuer.com` | 根目录 `Dockerfile`、`docker-compose.*.yml` |

关键后端包：

```
backend/internal/
  module/            # 业务模块：auth/community/equipment/file/inspection/mp/report/stats/system
  pkg/               # 通用：ai(视觉识别)/database(goose+seed)/excel/file/pdf(待删)/types
  middleware/        # 认证、租户隔离（TenantScopeOrDefault / CheckTenantRow）
```

## 3. 构建、运行、测试

### 后端（`backend/`）

```bash
# 开发热重载（Windows 下 fsnotify 不可靠，.air.toml 已启用轮询）
air

# 构建
go build -o ./tmp/server ./cmd/server

# 测试（有测试的包，提交前至少跑过）
go test ./...

# 种子演示数据（新库）
go run ./cmd/seed-demo
```

数据库：Goose 迁移文件在 `internal/pkg/database/migrations/`（embed，**启动即迁移**），命名 `000NN_说明.sql`。Goose 记录版本，现有入口使用 PG 会话锁，迁移失败中止启动；一次性转换脚本需可重入，破坏性发布仍须按 06 停写、备份和恢复门禁执行。

### PC 前端（`frontend/`）

```bash
npm run dev          # Vite 开发
npm run build        # 生产构建
npm run type-check   # vue-tsc --noEmit（提交前必须过）
```

### App（`app/`）

```bash
npm run dev:h5       # H5 开发（内置预构建 sync-version）
npm run build:h5     # H5 构建
# Android：HBuilderX 云打包；版本号在 manifest.json，打包版本 ≥ 服务端推送阈值
```

uni-ui 组件在 `app/main.ts` 手工注册，新增 uni-ui 组件要在那里注册（数量以该文件当前注册表为准）。**不要用 wot-ui**（迁移分支已归档不启用）。

### 环境

- `.env.dev` / `.env.prod`（不入库的密钥管理：DB/Redis/AI key 只走 env）
- 生产：`https://pi.hbuer.com`。测试小区「戎安华府」、测试巡检员/管理员账号按受控账号清单发放；不引用或提交含密码文件，**不要提交新密码到代码**。

## 4. 设计约定（改代码前必读的"系统宪法"）

### 4.1 三层模板（08 文档，以下是需改/新建目标）

| 层 | 表 | 管什么 | 关键规则 |
|---|---|---|---|
| L1 检查项模板 | `check_template` / `check_template_item` | 查什么、怎么判、要不要拍照 | `point_type` 必填、字典配置 system/level；`purpose=patrol/spot/verify`、draft/enabled、稳定 `code`；同类型允许多套，**本期**计划 `template_ids` 显式选用、任务固化集合 |
| L2 资产台账字段方案 | `equipment_type_schema` | 资产字段/列表/导入导出 | 独立于 L1，已有平台默认+租户整体覆盖；不承载查验资料清单，**不加 `table_mode`** |
| L3 输出模板 | 新建 `doc_template` / report 域输出分组 | 报告版式、章节/表头与字段映射 | `doc_kind=monthly_report\|acceptance\|maintenance_ledger`、`engine=ooxml\|xlsx_template`、块白名单、StylePack；输出分组独立于 L1 采集分组 |

不做模板历史版本树；L1/L3 均无 `version` 列。最新模板仅用于新生成/重出报告，重出=新报告；记录存 `item_code` 与事实，不复制整棵模板。查验清单 schema 归 acceptance 自有，矩阵可引用 `purpose=verify`。

**命名约定（08 §0.1，全文一致）**：`check_template.purpose` 是检查用途、`doc_template.doc_kind` 是输出文档类别，**不共用 `kind` 一个词**；被检对象用具体实体名 `equipment_id`（NULL=场所自身），不用 `target_instance_id`/`subject_key`/`occurrence_id` 这类抽象键名；跨域来源引用统一 `source_type`/`source_record_id`/`source_item_id`。`id` 指向一行、`code` 指向一个业务概念，两者分工见 08 §3.2.4。

### 4.2 两个“类型”不要混

- `point_type` = 去哪查的点位/场所类型，决定候选 L1；报告章节由 report 输出配置映射，不由 L1 分组绑死。
- `equipment_type` = 资产对象类型，决定 L2/寿命/维保规则。
- 唯一连接键 `equipment_type.attrs.point_type`；不留第二张硬编码映射。

### 4.3 数据、文件与快照

- `checkin_record_item` = 身份（`template_id`/`item_code`/`equipment_id`，另补 `tenant_id`）+ 事实（`result`/`note`/`photos`/`value_num`/`value_date`/`value_text`/`abnormal_tags`/`ai_verdict`）+ 最小展示快照（**只有 `name`/`standard`**）。不复制 `ai_hint`/`judge_*`/`photo_required`/`tags` 全集/`standard_no`/整棵模板。唯一键 `(tenant_id, record_id, template_id, item_code, equipment_id)`，`equipment_id IS NULL`=场所自身项（用两个部分唯一索引去重）。
- 判定参数只有一个来源：`number` 看 `value_min`/`value_max`、`enum` 看 `enum_options`、`ok` 看 AI verdict + 人工确认、`text`/`date` 不判定。原 `judge_type`/`judge_config` 一列承载三个正交维度，已拆为 `ai_prompt_kind`（只生成 prompt）/`ai_enabled`（是否调 AI）/`builtin_kind`（系统内置合成项），详 08 §3.2.3；合成项不再按名称前缀判断，也不另立 `synthetic` 列。
- 报告主记录一条，**新建 `report_file` 子表**记册别/阶段/格式/文件ID/摘要/批次，不选 file_ids 数组。预览→冻结审核→最终签字归档，冻结待审件和最终件新增保存、不覆盖；归档不可改，重出新报告。
- 正式记录归档锁定；过程草稿提交后删除，但新字段/code 需迁移，不承诺旧结构不变。
- 原始上传证据与清洗/水印/缩略图分开保存和摘要关联，先受限提取 EXIF/防作弊事实再生成副本，不覆盖原证据。

### 4.4 租户与权限（09 §4）

- 表分租户业务/受控子表/平台默认覆盖/平台控制/任务事件投影，不宣称“全表已有 tenant_id”；业务租户来自可信上下文，缺失 fail closed，平台范围显式授权审计。
- HTTP 可用现有 TenantScopeOrDefault/CheckTenantRow，但须核验缺失身份与 NULL 语义；后台 job/公开契约传显式 TenantScope+actor/data scope，禁止缺 Gin identity 时隐式放行/默认租户。
- 文件/API/STS/回调/报告产物统一**先租户及业务数据范围，再 scene/用途权限**；report:download/签章权限不能绕过租户，未知 scene 拒绝，云文件私有短期授权。
- 分层配置统一使用 `config(scope, scope_id)` 与 `approval_flow(scope, scope_id)`：platform=零 UUID、tenant=租户 ID、project=小区 ID，不再使用 `tenant_config` 或审批链 NULL 层级。覆盖是否允许与是否密钥由平台行 `allow_tenant_override/is_secret` 决定；密钥只存 `credential_ref`，不落配置表。
- **授权模型以 14 为准：五个概念塌缩为两个——角色 + 任职。** **角色挂在 `sys_user` 上、全局唯一（一人一个角色，结构约束不是约定）**，自带权限点 + 可抢工单类型 + 主管标志；`scope_level` 决定**范围怎么算**（platform=平台须指定租户 / tenant=本租户全部小区 / project=他的任职行覆盖的小区，无任职=无范围 fail closed），**项目级即甲方口径的"岗位"，UI 显示为岗位**。`project_staff` **一行一个小区**（不是用户表上的数组），只管范围与小区级属性：`building_ids`（**责任楼栋=分派筛选，不参与鉴权**；精度到楼栋，楼层是点位属性 `inspection_point.floor`）、生效期、代理人。需不同岗位时另建账号，须登记自然人标识、审核候选人按自然人去重（防自审）。`sys_role.data_scope` 去范围语义。**审批链节点直接绑角色**，候选人 = 持该角色且在业务对象范围内有任职的在职人员（跨小区报告据此解析出多地签字人，不加片区层）；节点**按职责命名不按角色命名**（月报审批/打卡复核，不叫"经理确认"）。**审批授权不设独立权限点**：能否审某记录 = 是否为该记录当前节点候选人（唯一判定），候选即赋予该记录读权限（"能审必能看"），菜单可见性由"角色是否被任一节点绑定"推导。合并后 `post_dict`、`duty_binding`、`Slot` 枚举、`post_dict.role_id`、`project_staff.posts`、`middleware/roleunion.go` 运行时并集、casbin `SyncAll` 第二份展开、报告链 `fallbackInspector` 特例**一并删除**，不改写；`SlotAuthorized` 的超管/租管短路放行去掉；`project_admin`/`field_staff` 两个桶拆成 7 个工种角色。空名单 = 审批链断链，挂起+告警+修复入口，不得跳过/自动通过（现 `flow_service.go:139` `WalkFlowSkipEmpty` 必改，**与模型无关、须单独改**）；临时人员走抢单即授权。`approval_flow` 只有项目/租户两级，平台默认链由预置包实例化成租户行。当前 `project_staff.project_id` 即 communityID，project=community 一对一，不设片区、不虚构两层。业主不进任职表，独立身份表 + 独立登录入口分流。超管也指定目标范围。高危操作审计含模板、签字归档、权限、证据下载、死信重放；已有 sys_operation_log，audit_log 待收口/补齐。

### 4.5 交互与生效点

- App 表单式一页卡片；AI 项级并行，pending/running 阻止提交，失败必须显式人工确认/审核，不能伪装合格。
- App 不显示分组标题；L1 采集分组不等于 report 输出两级表头。
- place 点位 0~N 设备，先场所项后逐台项。
- 巡检异常**审核通过后**以 `inspection.checkin.abnormal_approved` 入池；提交/AI初判/驳回不入池。整改提交后进入待审核，主管一次审核通过即闭环，不设复查任务；`workorder.item.review_passed` 不改原始巡检事实，维保有效日期在同一审核生效点按设备规则回写。
- 维保换粉是流水不是模板清单；审核/确认通过在 equipment 事务生效后发 `equipment.maintenance.confirmed`，待审/驳回不生效，重复确认幂等；逐项检查用 `purpose=spot`。
- acceptance 批次/记录/清单独立，不复用打卡或设备 L2 schema；承接查验不集中打卡，问题只进本域报告/跟踪。
- AI 统一走能力服务（09 §2.2）：业务域只提交任务类型/对象/证据/期望结构/审核策略，不直接接 OpenAI/Gemini/Claude；资料扫描、表计读数、标签日期、照片检查都是同一契约的不同任务类型。AI 只给建议，生效规则仍归业务域。

### 4.6 工程纪律与切换（06）

- **破坏性更新，不双写、不建兼容层**；逐类保留/转换/批准清理，不清正式事实、账号、台账主数据、审核签字与归档证据。只有明确演示数据/无引用临时文件可按清单清理。
- 旧 App 服务端强更停写（含离线补传），停止调度/worker，排空或审计取消在途任务；停写最终备份 DB+文件+必要队列状态，恢复演练后一次迁移。歧义映射/缺文件/门禁失败即保持停写中止，协调恢复不是兼容；开放新写入后不能直接回滚覆盖新增事实。
- 幂等 UUIDv7/业务唯一键；批量按07落库重查/筛选/事务/幂等。类型/合成项/通用模板配置化，不按名称前缀判业务。
- 保留现有 Redis 会话/黑名单；AI 队列期1 迁入 River。**不引入消息中间件**（Watermill/Kafka/NATS/RabbitMQ 一概不引）；新增依赖只有 `riverqueue/river`，跑在同一 PG 实例、不新增进程或基础设施组件。模块显式注册，不做代码行数保证；不引规则框架/微服务。
- **跨域联动按 01 §4.1 的生效点，用同事务 + 业务唯一键实现 —— 不建 outbox、不做消息投递、不存在进程间调用。** 8 个生效点名称是事务边界标记，不是消息名。原持久投递协议（outbox/delivery/ACK/租约/栅栏/死信重放）已降级为 01 §4.4「拆分时的目标协议」，本期不实施。真异步的只有外部副作用（通知/AI/渲染），走 River：`InsertTx` 与业务同事务入队、`UniqueOpts` 幂等、`MaxAttempts` + discarded 死信。
- 提交/推送等用户明确要求，不自动 commit。

### 4.7 领域边界（01 §1.1，存量需改目标）

- 一表一 owner，迁移注释 `-- owner: <域>`；mp 仅通道适配，业务规则迁回 inspection/equipment。
- 跨域公开 `service/api.go`/contract 提供批量/分页 DTO、显式 scope、错误与一致性语义，不暴露 GORM/model/map；禁止跨域读表/Join/Preload/model import。
- 跨域仅 ID 不加新跨域 FK，入口校验归属，删除/作废有 owner 契约；展示投影标来源/水位/延迟并可重建。stats 自有读模型，report 装配经已审核事实接口并校验冻结读取边界。
- 同步校验/查询可调公开接口；跨域状态联动在同一事务内直调 owner 域契约，生效点以 01 §4.1 为准；入参自包含业务事实，文件仍走授权。
- 用 import/SQL/owner 违例检查、契约测试、租户负例和投影重建核验边界；不用拷包编译错误计数。report 主记录/任务/文件/签字有状态，未来仅渲染 worker 可评估独立部署，不承诺拆服务零改动。

## 5. 渲染与报告（期2 待建目标）

已审核事实读接口 + report 母版槽位映射 → 原始 DOCX 母版保留全部版式并动态填槽 → docx → 隔离 soffice → PDF/PNG；文件登记到 report_file。禁止用 Sections+Rows 自绘业务版式，缺母版/槽位必须失败，不回退自绘。

- 后端镜像不装 LO；dev/prod Compose 默认启动独立 Debian `report-renderer`，共享目录原子任务、非特权无网络、独立临时 profile、资源/超时/并发限制及 PDF 校验；真实多册签字归档与黄金样本未通过前不得宣称 PDF 验收。
- 模板包拒绝 ZIP 炸弹/路径穿越、XML 外部实体、外链/宏/ActiveX/OLE/DDE；layout 白名单与 xlsx 文本防公式注入，详09 §4。
- 旧 `internal/pkg/pdf` 与 gofpdf 已删除，不保留自绘兼容链路；无 LO 只出明确标注 docx，要求 PDF 的部署不能算通过。
- 每册保留黄金样本基线；候选图像相似度 SSIM ≥0.98 仅作建议阈值，最终通过线以甲方确认基线为准，另验文字/分页/签字位置/空册。性能容量数值按09 §5待压测预算，不是已达 SLA。

## 6. 工作流（新增功能怎么落地）

1. **先方案**：改 `docs/AI云管家/设计方案/` 对应文档（或新增），与既有 01~14 对齐（期次、术语按 08 §0.2）
2. **原型**：UI 变更先出原型（**App 规范、逐页清单与最新 PNG 在 `docs/AI云管家/APP原型/`**；PC 后台为 HTML 静态原型，文档、可编辑片段、完整 HTML 与 1440×900 预览在 `docs/AI云管家/后台系统原型/`），评审通过再实现。**画后台原型前必须先读对应的 `frontend/src/views` 现有实现**——现有 PC 后台已成型且设计可用，原型只标注"哪里要改"，不另画一套；审批链复用 `components/ReviewFlowEditor.vue`，权限配置是抽屉 + el-tree 权限树，列表与详情不堆在同一屏（详见后台原型 README「画后台原型前必须先读现有实现」）。**App 列表页一律按 `docs/AI云管家/APP原型/03-页面模板.md §1`「列表模板」画**（多选卡片 + 全选行 + 底部 `已选 N 条` 操作条 + 退出多选两条路径 + 整卡可点进详情）；打卡审核列表不放单个通过/驳回按钮，批量动作在底部，单条进详情处理；审核/确认/取消类动作用居中弹窗，选项类用底部上滑 sheet，不写规则说明；新建/改页前先读 `01-设计原则.md` 十二条（每条带检查方法）与 `04-逐页交互清单.md` 该页六段
3. **迁移**：加 Goose 迁移（幂等），破坏性改动附一次性迁移脚本，说明清哪些数据
4. **实现**：后端模块按 `internal/module/<name>/{model,service,controller,dto}` 四层；消费方只读模板/字典，不写死文案
5. **测试**：有测试的模块补单测；至少 `go test ./...` 与 `npm run type-check` 通过
6. **文档同步**：改了设计约定就更新本文和对应方案文档

## 7. 常用导航

| 找什么 | 去哪 |
|---|---|
| 模板三层与巡检模板（**20 套 L1 + 1 张维保输出表**，21 是输出表数不是模板数） | `docs/AI云管家/设计方案/08-模板系统设计.md` |
| 甲方 12 份 Word/Excel 的逐表解析 | `backend/.tmp/template_dump.md`（4389 行） |
| 巡检打卡链路与表单式 | `backend/internal/module/mp/service/checkin_service.go`、`docs/AI云管家/设计方案/03-表单式巡检设计.md` |
| 工单池 | `docs/AI云管家/设计方案/04-工单系统设计.md` |
| 承接查验 | `docs/AI云管家/设计方案/05-承接查验设计.md` + `08 §2.2` |
| 期次与排期 | `docs/AI云管家/设计方案/06-分期实施路线.md` |
| 批量与大数据量 | `docs/AI云管家/设计方案/07-批量作业与大数据量设计.md` |
| IoT/AI/SaaS/安全/性能 | `docs/AI云管家/设计方案/09-扩展性与平台化设计.md` |
| PC 后台菜单、App/PC 职责划分、后台配置架构 | `docs/AI云管家/设计方案/13-后台管理系统设计.md` |
| 权限与岗位模型（角色+任职、防自审、抢单授权） | `docs/AI云管家/设计方案/14-权限与岗位模型.md` |
| 原型与施工规范 | **App 原型文档集 `docs/AI云管家/APP原型/`（规范/69 页交互/清单/审计/预览 v2）**、**PC 后台原型文档集 `docs/AI云管家/后台系统原型/`（规范/43 条交互/清单/审计/预览索引）** |
| 待确认事项（唯一登记处）与验收追踪 | `docs/AI云管家/设计方案/10-验收与需求追踪.md` |
