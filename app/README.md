# 安巡云巡检（app/）

物业管理平台移动巡检端，**经典 uni-app（Vue3 + Vite）** 单代码库多端编译：Android / iOS / 鸿蒙 NEXT / 微信小程序。

- 设计依据：`docs/移动端四端技术方案.md`、`docs/移动端四端UI-UX设计方案.md`（文档以 uni-app x 编写，工程已按方向调整回经典 uni-app，功能与令牌不变）
- 后端接口：`docs/接口文档.md`（APP 三端 `/api/app`，小程序 `/api/mp`，信封 `{code, message, data}`）
- 当前进度：**M1 骨架**（登录/注册/今日任务/原生 tabBar + midButton 中央扫码/扫码占位/我的）

## 开发工作流（约定）

- **编译运行一律在 HBuilderX GUI 手动进行**（运行 → 运行到手机或模拟器 → Android App 基座），不用 CLI 驱动；
- HBuilderX 直接打开**本 `app/` 目录**（中文路径可正常工作）；
- 真机连后端：开发/测试默认使用 `https://pi.hbuer.com`，无需随开发机内网 IP 变化修改。

## 环境要求

- HBuilderX 5.07+（经典 uni-app 项目，打开即用，无需 uts 扩展）
- 状态管理：**pinia**（经典 uni-app Vue3 内置支持，`import { defineStore } from 'pinia'` 直接可用，无需额外安装）
- 后端：开发/测试接口默认使用 `https://pi.hbuer.com`；本地后端仅在专项联调时临时覆盖。

## 如何运行（Android 真机）

1. HBuilderX → 文件 → 打开目录 → 选择本 `app/` 目录（识别为 uni-app 项目）。
2. 手机开 USB 调试连电脑；菜单 运行 → 运行到手机或模拟器 → 运行到 Android App 基座（首次需下载标准基座或制作自定义基座）。
3. 真机默认访问 `https://pi.hbuer.com`，不需要配置电脑局域网 IP；如需临时联调本机后端，再手动把 `BASE_URL_DEV` 改为电脑内网地址。
4. 其他端：运行 → 运行到小程序模拟器（微信开发者工具）/ iOS / 鸿蒙。

## 环境切换（baseURL）

`services/request.ts` 顶部：

- `ACTIVE_ENV = 'dev' | 'prod'`：dev/prod 当前均指向 `https://pi.hbuer.com`，保留环境开关便于后续拆分；
- 按端条件编译：`#ifndef MP-WEIXIN` → `/api/app`，`#ifdef MP-WEIXIN` → `/api/mp`。

## 待申请/待配置（manifest.json 不允许注释，统一在此说明）

| 项 | 位置 | 状态 |
|---|---|---|
| DCloud appid | `manifest.json` → `appid` | 待申请（HBuilderX 内一键生成） |
| 微信小程序 AppID | `manifest.json` → `mp-weixin.appid` | 待申请（小程序主体认证） |
| 微信开放平台「移动应用」AppID | APP 端微信登录用，与小程序 AppID 不同 | 待申请，登录页按钮为占位 |
| API 域名 | `services/request.ts` → `BASE_URL_DEV` / `BASE_URL_PROD` | 已配置 `https://pi.hbuer.com` |
| Android keystore / iOS 证书 / 鸿蒙签名 | 打包时配置 | 发布前清单 |

## 目录约定

```
app/
├── manifest.json            # 应用配置（appid 等待申请，见上表）
├── pages.json               # 路由表 + 导航样式 + tabBar(custom: true)
├── main.ts / App.vue        # pinia 初始化 / 登录守卫 + 恢复用户缓存（无 token 跳登录页）
├── pages/                   # 页面（.vue，Vue3 options API + TS）
├── components/AppTabBar.vue # 自绘底部导航（角色装配 + 中央凸起扫码按钮 + 未读角标）
├── services/                # request 封装（40102 静默刷新单飞锁）+ api（类型对齐后端 DTO）
├── stores/                  # pinia：auth（登录态持久化）、message（未读角标）
├── utils/                   # theme.ts（设计令牌唯一来源）、storage.ts（存储 key）
├── static/                  # 图标/插画（占位图标的正式资源落地于此）
└── unpackage/               # 编译产物（gitignore）
```

## 编码约定

- **tabBar**：`pages.json` 中 `custom: true` 对小程序生效（custom-tab-bar）；APP 端由各 tab 页内引入 `components/AppTabBar.vue` 自绘渲染，切换用 `uni.reLaunch`。
- **颜色/尺寸不写死在页面**：语义色值经 `:style` 绑定 `utils/theme.ts` 常量；`App.vue` 全局样式（page 底色/CSS 变量）与 `pages.json` 中的色值镜像 theme.ts，改动需同步。
- **接口类型**：`services/api.ts` 用接口类型 + 类型断言解析信封 data，函数签名与返回结构与后端已联调，勿擅自变更。
- **条件编译**集中在 utils/组件层：`// #ifdef APP-PLUS / MP-WEIXIN`（APP-ANDROID/APP-IOS/APP-HARMONY 细分平台按需）。
- 图标暂为文字符号占位（标有 `TODO(图标)`），M5 前替换为正式 iconfont/SVG 资源（禁 emoji）。

## 已实现链路

- 启动登录守卫 → 登录页（账号密码登录、密码明文切换、内联错误、注册开关 + 注册弹层含手机号校验、微信登录占位）
- token 自动携带；40102 → refresh_token 静默刷新重放（单飞锁），失败清登录态回登录页
- 今日任务（骨架屏/空态/进度条/下拉刷新）；自绘 tabBar（消息角标、中央扫码按钮）
- 扫码页：`uni.scanCode` + 手输编号降级 → `GET /points/by-code/:code` Toast 点位名（M2 接检查项页）
- 我的页：用户信息 + 退出登录（二次确认 → 调登出接口 → 清 token 回登录页）
