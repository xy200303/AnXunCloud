## 1.4.7（2026-07-28）
iOS app store 更新优化 
## 1.4.6（2026-07-06）
ios session 优化
## 1.4.5（2026-07-06）
iOS 支付宝兼容卡优化
## 1.4.4（2026-07-03）
iOS nfc 调优
## 1.4.3（2026-07-03）
支付宝碰碰卡
## 1.4.2（2026-07-03）
兼容卡
## 1.4.1（2026-07-03）
ISO 14443-4 兼容卡
## 1.4.0（2026-07-03）
iOS 支持 上海复旦微电子的 ISO 14443-4 兼容卡
## 1.3.9（2026-06-26）
文档优化了一下
## 1.3.8（2026-06-01）
打包冲突问题处理
## 1.3.7（2026-06-01）
iOS 自定义文案
## 1.3.6（2026-05-27）
iOS 自定义识别结果文案
## 1.3.5（2026-05-27）
- iOS：`configure` 新增可选字段 `readSuccessAlertMessage`、`readSuccessKeepAliveAlertMessage`，支持自定义读卡完成时的系统弹框文案（保留默认值，非必传）

## 1.3.4（2026-05-25）
iOS 定义文案
## 1.3.3（2026-05-06）
一、优化优化
## 1.3.2（2026-05-05）
一、三端的写入优化
## 1.3.1（2026-05-03）
一、iOS 写入优化
## 1.3.0（2026-04-29）
一、iOS异常日志收紧
## 1.2.9（2026-04-29）
一、iOS相关
## 1.2.8（2026-04-28）
一、iOS 标签写入相关问题
## 1.2.7（2026-04-27）
一、ios写入问题优化
二、文档更新
# 更新日志

- 版本按 **时间倒序**（最新在上）。
- 每版先有 **一句话摘要**，再分 **新增 / 修复 / 文档** 等，便于扫读。
- 完整 API 以 `utssdk/interface.uts` 与 `readme.md` 为准。

---

## 1.2.6（2026-04-15）

**摘要**：修复 Android 上 Intent / ReaderMode 导致的重复事件；高级读写错误信息更明确；新增状态清理 API；文档改版。

### 新增（API · 全平台）

- `clearActiveTag()`：清除内存中的活动标签并重置去重，**不**清磁盘缓存队列。
- `resetNfcState()`：等价于 `clearPendingTag()` + `clearActiveTag()`，一键清空队列与内存状态。

### 修复（Android）

- 处理完 NFC Intent 后清空 Activity Intent，避免 **resume 时重复处理**同一张卡。
- `NfcDispatchActivity` 处理完后清除 Intent，避免 **重复写入缓存**。
- **ReaderMode** 在去重窗口内对同 UID 统一拦截，减少重复 **回调与缓存**。
- 写入 / transceive / 读块等操作结束后 **延长去重时间**，减轻 `close()` 后射频重连带来的 **二次触发**。
- 高级读写失败时返回 `data.reason`：`NO_TAG_IN_MEMORY`、`TAG_TTL_EXPIRED`（含 `elapsedMs` / `ttlMs` 等），便于排查。

### 文档

- readme 增加阅读指引、快速开始；Android 注意事项拆分为必读 / 进阶；补充精简 FAQ。

---

## 1.2.5（2026-04-03）

**摘要**：iOS 侧专项优化（会话与回调策略等，详见该版本发布说明）。

---

## 1.2.4（2026-04-02）

**摘要**：新增「刷新活动标签」能力；iOS 增加会话保持配置；Android NDEF 读逻辑优化。

### 新增

- `refreshActiveTagSync()`  
  - **Android**：写入后可立即基于当前活动标签重读快照。  
  - **iOS**：需先开启会话保持（见下）后方可使用。  
  - **HarmonyOS**：同步返回当前活动标签快照。
- `refreshActiveTagAsync()`：**仅 HarmonyOS**，异步刷新活动标签。

### 配置（iOS）

- `keepSessionAlive`：控制单次读卡后是否立即结束 CoreNFC 会话；开启后才适合在同一会话内连续调用 `refreshActiveTagSync()`。

### 优化（Android）

- NDEF 优先读 **实时 `ndefMessage`**，再回退 `cachedNdefMessage`，降低写后仍读到旧内容的概率。

---

## 1.2.3（2026-03-25）

**摘要**：Android 端问题修复（详见该版本说明）。

---

## 1.2.2（2026-03-13）

**摘要**：写入类能力补齐。

### 新增

- `ndefWrite`：NDEF 写入  
- `mifareClassicWriteBlock`：Mifare Classic 写块  
- `mifareUltralightWritePage`：Mifare Ultralight 写页  

---

## 1.2.1（2026-03-03）

**摘要**：`skipNdefCheck` 等行为调整（详见该版本说明）。

---

## 1.2.0（2026-01-30）

**摘要**：新增 **HarmonyOS** 平台实现（与 Android 能力对齐度以 `interface.uts` 为准）。

---

## 1.0.0（2026-01-30）

**摘要**：首个公开版本，**Android** 为主。

### 能力概览

- `onTagDiscovered`：前台碰卡持续回调  
- `getPendingTagSync`：读取后台缓存事件（可读后清空）  
- `clearPendingTag`：清空待处理缓存  
- `isNfcSupported` / `isNfcEnabled`：能力与开关  

### 支持的技术类型（识别）

NDEF、Mifare Classic / Ultralight、IsoDep、NfcA / B / F / V 等（以实际机型与标签为准）。
