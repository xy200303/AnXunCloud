<h2>UTS环境兼容性</h2>

|  uni-app	| uni-app x	|
|  :----:	| :----:	|
| √			| √			|

# hl-nfccase-uts

跨端 **NFC 读/写** 插件，支持 **Android / iOS / HarmonyOS** 三端。

覆盖 NDEF 读写、Mifare Classic/Ultralight 读写、IsoDep/NfcV 通信、NfcA/NfcB 原始帧等能力。

## 目录

- [快速开始](#快速开始)
- [平台能力总览](#平台能力总览)
- [读功能](#一读功能)
  - [基础读卡（UID / NDEF）](#1-基础读卡-uid-ndef)
  - [Mifare Classic 读扇区读块](#2-mifare-classic-读扇区读块)
  - [Mifare Ultralight 读页](#3-mifare-ultralight-读页)
  - [IsoDep 与 NfcV 通信](#4-isodep-与-nfcv-通信)
  - [NfcA 与 NfcB Raw Transceive](#5-nfca-与-nfcb-raw-transceive)
- [写功能](#二写功能)
  - [NDEF 写入](#1-ndef-写入)
  - [Mifare Classic 写块](#2-mifare-classic-写块)
  - [Mifare Ultralight 写页](#3-mifare-ultralight-写页)
  - [NdefFormatable 格式化](#4-ndefformatable-格式化)
- [API参考](#api参考)
- [返回数据结构](#返回数据结构)
- [常见问题FAQ](#常见问题faq)
- [技术支持](#技术支持)

---

## 快速开始

### 1. 前置准备

| 平台 | 必要配置 |
|------|----------|
| **Android** | 自定义基座或重新打包（含 NFC Manifest、后台分发 Activity） |
| **iOS** | Info.plist 添加 `NFCReaderUsageDescription`；Xcode 开启 NFC Capability；Entitlements 含 NDEF + TAG |
| **HarmonyOS** | `ohos.permission.NFC_TAG` 权限 |

### 2. 导入

```js
import * as HlNfc from '@/uni_modules/hl-nfccase-uts'
```

### 3. 最小示例

> **uni-app 与 uni-app x 的核心区别**：传统 uni-app 中 `APP-HARMONY` 条件编译不被支持，`APP-PLUS` 已覆盖三端；uni-app x 可直接使用 `APP-ANDROID` / `APP-IOS` / `APP-HARMONY`。

**uni-app —— 用 `APP-PLUS` + 运行时检测：**

```js
import * as HlNfc from '@/uni_modules/hl-nfccase-uts'
import { onShow, onHide } from '@dcloudio/uni-app'

// 运行时平台检测
function detectPlatform() {
  const info = uni.getSystemInfoSync()
  const plat = (info.platform || info.osName || '').toLowerCase()
  if (plat === 'harmony' || plat === 'harmonyos') return 'harmony'
  if (plat === 'ios') return 'ios'
  if (plat === 'android') return 'android'
  return 'other'
}

onShow(() => {
  HlNfc.configure({ pendingMax: 10, dedupeMs: 600 })
  HlNfc.onTagDiscovered((event) => {
    console.log('UID:', event.uidHex, event)
  })
  // iOS / Harmony 需主动启动会话（不能用 #ifdef APP-HARMONY）
  // #ifdef APP-PLUS
  const plat = detectPlatform()
  if (plat === 'ios' || plat === 'harmony') {
    HlNfc.startNFCSession()
  }
  // #endif
})

onHide(() => {
  // #ifdef APP-PLUS
  const plat = detectPlatform()
  if (plat === 'ios' || plat === 'harmony') {
    HlNfc.stopNFCSession()
  }
  // #endif
})
```

**uni-app x —— 可直接用条件编译：**

```js
import * as HlNfc from '@/uni_modules/hl-nfccase-uts'
import { onShow, onHide } from '@dcloudio/uni-app'

onShow(() => {
  HlNfc.configure({ pendingMax: 10, dedupeMs: 600 })
  HlNfc.onTagDiscovered((event) => {
    console.log('UID:', event.uidHex, event)
  })
  // #ifdef APP-IOS || APP-HARMONY
  HlNfc.startNFCSession()
  // #endif
})

onHide(() => {
  // #ifdef APP-IOS || APP-HARMONY
  HlNfc.stopNFCSession()
  // #endif
})
```

判断能力：`HlNfc.isNfcSupported()` / `HlNfc.isNfcEnabled()`。

---

## 平台能力总览

| 特性 | Android | iOS | HarmonyOS |
|------|:-------:|:---:|:---------:|
| **启动方式** | 自动监听 | 需 `startNFCSession()` | 需 `startNFCSession()` |
| **系统弹窗** | 无 | 有 | 无 |
| **NDEF 读取** | 是 | 是 | 是 |
| **NDEF 写入** | 是 | 是（需 `keepSessionAlive`） | 是 |
| **Mifare Classic 读块** | 是 | 否 | 是 |
| **Mifare Classic 写块** | 是 | 否 | 是 |
| **Mifare Ultralight 读页** | 是 | 否 | 是 |
| **Mifare Ultralight 写页** | 是 | 否 | 是 |
| **IsoDep / NfcV 通信** | 是 | 否 | 是 |
| **NfcA / NfcB Raw** | 是 | 否 | 否 |
| **NdefFormatable 格式化** | 是 | 否 | 否 |
| **FeliCa** | 是 | 否 | 是 |
| **最低系统** | Android 5.0+ | iOS 13.0+ | HarmonyOS 3.0+ |

### 条件编译标识符差异

| 标识符 | uni-app | uni-app x |
|--------|:-----------------:|:---------:|
| `APP-PLUS` | 是，覆盖三端 | 兼容但推荐用具体平台 |
| `APP-ANDROID` | 是 | 是 |
| `APP-IOS` | 是 | 是 |
| `APP-HARMONY` | 否 **不支持** | 是 |

> **关键注意**：传统 uni-app 中 `APP-PLUS` 已覆盖 iOS + Android + HarmonyOS 三端，`APP-HARMONY` 是 uni-app x 专属标识符，uniapp中使用会导致鸿蒙端代码被编译器排除。

---

## 一读功能

### 1. 基础读卡（UID 与 NDEF）

获取标签 UID、技术列表、NDEF 文本/URI，全平台支持。

#### Android

前台自动监听，注册 `onTagDiscovered` 即可收到回调。

```js
HlNfc.onTagDiscovered((event) => {
  console.log('UID:', event.uidHex)
  console.log('NDEF文本:', event.ndefText)
  console.log('NDEF URI:', event.ndefUri)
  console.log('技术列表:', event.techs)
})
// 无需 startNFCSession，Android 自动监听
```

#### iOS

必须调用 `startNFCSession()` 唤起系统 NFC 弹窗，用户贴卡后回调。

```js
HlNfc.onTagDiscovered((event) => {
  console.log('UID:', event.uidHex)
})
HlNfc.startNFCSession('请将卡片靠近设备顶部')
```

**iOS 特殊处理：**
- 不支持后台/冷启动唤醒，每次扫描都需主动调 `startNFCSession()`
- 可通过 `onSessionInvalidated` 监听会话失效（用户取消 / 超时等）
- 可通过 `configure` 自定义系统弹框文案

```js
// iOS 自定义弹框文案
HlNfc.configure({
  keepSessionAlive: false,
  readSuccessAlertMessage: '读取成功！',
})
HlNfc.onSessionInvalidated((e) => {
  console.log('会话失效:', e.reason) // userCanceled / sessionTimeout / systemBusy
})
```

#### HarmonyOS

需调用 `startNFCSession()`，能力与 Android 接近。

```js
HlNfc.onTagDiscovered((event) => {
  console.log('UID:', event.uidHex)
})
HlNfc.startNFCSession()
```

**HarmonyOS 特殊处理：**
- 部分 API 存在 `*Async` 变体（返回 Promise），需运行时检测后 await 调用
- `startNFCSession` 无系统弹窗（与 iOS 不同）

---

### 2. Mifare Classic 读扇区读块

> **iOS 不支持**（CoreNFC 不提供 Mifare Classic 读块能力）

#### Android

在 `onTagDiscovered` 回调的同一贴卡时间窗内调用（默认约 5s，可 `configure({ activeTagTtlMs })` 调整）。

```js
HlNfc.onTagDiscovered(() => {
  // 读扇区（返回该扇区所有块数据）
  const sectorRes = HlNfc.mifareClassicReadSector(1, 'A', 'FFFFFFFFFFFF')
  console.log(sectorRes)

  // 读单个块
  const blockRes = HlNfc.mifareClassicReadBlock(4, 'A', 'FFFFFFFFFFFF')
  console.log(blockRes)
})
```

#### HarmonyOS

API 相同，但可能返回 Promise。

**uni-app —— 运行时检测 async/sync：**

```js
HlNfc.onTagDiscovered(async () => {
  // 优先尝试 Async 变体（鸿蒙端存在），否则 fallback 同步版本
  const asyncFn = HlNfc.mifareClassicReadBlockAsync
  const syncFn = HlNfc.mifareClassicReadBlock
  let res
  if (typeof asyncFn === 'function') {
    res = await asyncFn(4, 'A', 'FFFFFFFFFFFF')
  } else if (typeof syncFn === 'function') {
    res = syncFn(4, 'A', 'FFFFFFFFFFFF')
  }
  console.log(res)
})
```

**uni-app x —— 直接条件编译：**

```js
// #ifdef APP-ANDROID
const res = HlNfc.mifareClassicReadBlock(4, 'A', 'FFFFFFFFFFFF')
// #endif
// #ifdef APP-HARMONY
const res = await HlNfc.mifareClassicReadBlockAsync(4, 'A', 'FFFFFFFFFFFF')
// #endif
```

---

### 3. Mifare Ultralight 读页

> **iOS 不支持**

#### Android / HarmonyOS

```js
HlNfc.onTagDiscovered(() => {
  // 读连续页（每页 4 字节），通常不需要密钥
  const res = HlNfc.mifareUltralightReadPages(4, 4) // 从第 4 页读 4 页
  console.log(res)
})
```

> HarmonyOS 端同样可能返回 Promise，处理方式同 [Mifare Classic 读扇区读块](#2-mifare-classic-读扇区读块)。

---

### 4. IsoDep 与 NfcV 通信

> **iOS 不支持**

发送 APDU（IsoDep）或原始命令（NfcV），返回 hex 响应。

```js
// IsoDep：发送 APDU
const res = HlNfc.isoDepTransceive('00A4040008A00000000310100100')
console.log(res.data.responseHex)

// NfcV：发送原始命令
const res2 = HlNfc.nfcVTransceive('0200210030')
```

---

### 5. NfcA 与 NfcB Raw Transceive

> **仅 Android 支持**，iOS 和 HarmonyOS 均不支持

```js
// NfcA 原始帧
HlNfc.nfcATransceive('3004') // Type2 读示例

// NfcB 原始帧
HlNfc.nfcBTransceive('05...')
```

> 勿随意写 Ultralight 低页（0-3 页为 OTP/UID 区域），有 NDEF 时优先用 `ndefWrite`。

---

## 二写功能

### 1. NDEF 写入

全平台支持。写入 NDEF 文本、URI、MIME、External Type 等记录。

#### Android

在 `onTagDiscovered` 回调内直接调用，同步返回。

```js
HlNfc.onTagDiscovered(() => {
  const res = HlNfc.ndefWrite([
    { type: 'text', data: 'Hello NFC', lang: 'en' },
    { type: 'uri', data: 'https://example.com' }
  ])
  if (res.code !== 0) console.error(res.data.error)
})
```

#### iOS

**必须** 先 `configure({ keepSessionAlive: true })` 保持会话，再 `startNFCSession()`，在回调内写入。

```js
// iOS 写入三步走
HlNfc.configure({
  keepSessionAlive: true,
  activeTagTtlMs: 10000,
  readSuccessAlertMessage: '识别成功',
  readSuccessKeepAliveAlertMessage: '识别成功，请保持贴卡以继续写入'
})
HlNfc.startNFCSession('请将手机顶部靠近门锁')
HlNfc.onTagDiscovered(() => {
  const res = HlNfc.ndefWrite([
    { type: 'text', data: 'Hello', lang: 'en' }
  ])
  if (res.code !== 0) console.error(res.data.error)
})
// 写入完成后可手动 stopNFCSession 或让会话自然超时
```

> **iOS 关键点**：不设 `keepSessionAlive: true` 会导致读卡后会话自动关闭，写入必失败。

#### HarmonyOS

API 与 Android 相同，但 **NDEF 写入可能返回 Promise**，建议统一 `await`。

**uni-app ：**

```js
HlNfc.onTagDiscovered(async () => {
  // await 兼容三端：iOS/Android 返回同步对象（await 立即解包），Harmony 返回 Promise
  const res = await HlNfc.ndefWrite([
    { type: 'text', data: 'Hello', lang: 'en' }
  ])
  if (res.code !== 0) console.error(res.data.error)
})
```

**uni-app x：**

```js
HlNfc.onTagDiscovered(async () => {
  // #ifdef APP-ANDROID || APP-IOS
  const res = HlNfc.ndefWrite([{ type: 'text', data: 'Hello', lang: 'en' }])
  // #endif
  // #ifdef APP-HARMONY
  const res = await HlNfc.ndefWrite([{ type: 'text', data: 'Hello', lang: 'en' }])
  // #endif
  if (res.code !== 0) console.error(res.data.error)
})
```

#### NDEF 记录格式说明

```js
// 文本记录
{ type: 'text', data: 'Hello NFC', lang: 'en' }

// URI 记录
{ type: 'uri', data: 'https://example.com' }

// MIME 记录
{ type: 'mime', data: '内容文本', mimeType: 'text/plain' }

// External Type（如写 Android 包名）
{ type: 'external', domain: 'android.com', extType: 'pkg', data: 'com.example.app' }

// 原始 NDEF 记录（data 按十六进制解析）
{ type: 'raw', data: '00112233', tnf: 0, typeHex: '', idHex: '' }
```

> 只有 `type: 'raw'` 的 `data` 会按十六进制解析，其余类型 `data` 按普通字符串写入。

---

### 2. Mifare Classic 写块

> **iOS 不支持**

在 `onTagDiscovered` 回调的同一贴卡时间窗内调用。

```js
HlNfc.onTagDiscovered(() => {
  // 参数：块号、密钥类型('A'/'B')、密钥Hex(12位)、数据Hex(32位=16字节)
  const res = HlNfc.mifareClassicWriteBlock(
    4, 'A', 'FFFFFFFFFFFF', '00112233445566778899AABBCCDDEEFF'
  )
  if (res.code === 0) console.log('写入成功', res.data.uidHex)
})
```

> HarmonyOS 端 async/sync 处理方式同 [读功能](#2-mifare-classic-读扇区读块)。

---

### 3. Mifare Ultralight 写页

> **iOS 不支持**

```js
HlNfc.onTagDiscovered(() => {
  // 参数：页号、数据Hex(8位=4字节)
  const res = HlNfc.mifareUltralightWritePage(4, '00112233')
  if (res.code === 0) console.log('写入成功')
})
```

> 勿随意写低页（0-3），可能损坏卡片。有 NDEF 时优先用 `ndefWrite`。

---

### 4. NdefFormatable 格式化

> **仅 Android 支持**

将空白卡格式化为 NDEF 格式并写入初始消息。

```js
HlNfc.onTagDiscovered(() => {
  // 格式化并写入内容
  const res = HlNfc.ndefFormatAndWrite([
    { type: 'text', data: 'Hello', lang: 'en' }
  ])
  // 仅格式化不写内容：HlNfc.ndefFormatAndWrite([])
})
```

> 如果卡片已经是 NDEF 格式，请直接使用 `ndefWrite`。

---

### 写后重读（全平台推荐）

写入后不移开手机，立即重读标签最新内容：

```js
HlNfc.onTagDiscovered(async () => {
  const w = await HlNfc.ndefWrite([{ type: 'text', data: 'Hi', lang: 'en' }])
  if (w.code === 0) {
    const r = HlNfc.refreshActiveTagSync()
    if (r.code === 0) console.log('重读:', r.data.ndefText)
  }
})
```

> 建议调大 `activeTagTtlMs`（默认 5000ms）以延长可操作时间窗。

---

## API参考

### 基础能力

| 方法 | 平台 | 说明 |
|------|------|------|
| `onTagDiscovered(callback)` | 全平台 | 注册碰卡回调 |
| `onSessionInvalidated(cb)` | 仅 iOS | 监听 NFC 会话失效（用户取消/超时等） |
| `startNFCSession(alertMessage?)` | iOS / HarmonyOS | 启动前台扫描；`alertMessage` 仅 iOS 生效 |
| `stopNFCSession()` | iOS / HarmonyOS | 停止前台扫描 |
| `getPendingTagSync(clearRead)` | 全平台 | 读一条缓存（后台/冷启动场景） |
| `getPendingTagsSync(clearRead, max)` | 全平台 | 读多条缓存 |
| `refreshActiveTagSync()` | 全平台 | 写后不离卡，重读当前标签快照 |
| `clearPendingTag()` | 全平台 | 清缓存队列 |
| `clearActiveTag()` | 全平台 | 清内存活动标签 + 去重状态 |
| `resetNfcState()` | 全平台 | 清缓存 + 清活动标签 + 重置去重 |
| `isNfcSupported()` / `isNfcEnabled()` | 全平台 | 设备能力与开关 |
| `configure(options)` | 全平台 | 配置插件行为 |
| `clearReadCache()` | 全平台 | 清除读取指令缓存 |
| `checkAndSaveCurrentIntentNfc()` | Android | 兜底：从当前 Intent 取 NFC |
| `checkNfcDispatchCalled()` | Android | 调试：分发 Activity 是否被调过 |
| `getNFCDebugInfo()` | 全平台 | 获取调试信息（iOS session 状态等） |

### 读 API

| 方法 | Android | iOS | HarmonyOS |
|------|:-------:|:---:|:---------:|
| `mifareClassicReadSector(sector, keyType, keyHex)` | 是 | 否 | 是 |
| `mifareClassicReadBlock(block, keyType, keyHex)` | 是 | 否 | 是 |
| `mifareUltralightReadPages(startPage, pageCount)` | 是 | 否 | 是 |
| `isoDepTransceive(apduHex)` | 是 | 否 | 是 |
| `nfcVTransceive(cmdHex)` | 是 | 否 | 是 |
| `nfcATransceive(cmdHex)` | 是 | 否 | 否 |
| `nfcBTransceive(cmdHex)` | 是 | 否 | 否 |

### 写 API

| 方法 | Android | iOS | HarmonyOS |
|------|:-------:|:---:|:---------:|
| `ndefWrite(records)` | 是 | 是 | 是 |
| `mifareClassicWriteBlock(block, keyType, keyHex, dataHex)` | 是 | 否 | 是 |
| `mifareUltralightWritePage(page, dataHex)` | 是 | 否 | 是 |
| `ndefFormatAndWrite(records)` | 是 | 否 | 否 |

### `configure` 配置项

**通用字段（主要 Android）：**

| 字段 | 默认值 | 说明 |
|------|--------|------|
| `useReaderMode` | - | 前台使用 ReaderMode（更丝滑） |
| `activeTagTtlMs` | 5000 | 活动标签可操作时间窗（ms） |
| `dedupeMs` | - | 同 UID 去重窗口（ms） |
| `pendingMax` | 10 | 后台缓存队列最大条数 |
| `skipNdefCheck` | false | ReaderMode 下跳过 NDEF 检查 |
| `readCacheTtlMs` | 0 | 读指令缓存存活期（ms，0 不启用） |

**iOS 额外字段（系统弹框文案，均可选）：**

| 字段 | 默认值 | 说明 |
|------|--------|------|
| `keepSessionAlive` | false | 读卡后保持会话，便于同一会话内 `ndefWrite` |
| `readSuccessAlertMessage` | `读取成功！` | 读卡完成且即将关闭会话时的弹框文案 |
| `readSuccessKeepAliveAlertMessage` | `读取成功，请保持标签靠近设备以继续操作` | `keepSessionAlive: true` 时读卡完成文案 |

> 扫描开始文案通过 `HlNfc.startNFCSession(alertMessage?)` 传入，默认 `请将 NFC 标签靠近设备`。

---

## 返回数据结构

所有高级读写 API 统一返回 `{ code, data }`，`code: 0` 成功，非 0 失败。

### `onTagDiscovered` 事件结构

#### Android

```json
{
  "timestamp": 1769741815417,
  "action": "TECH_DISCOVERED",
  "uidHex": "04A1B2C3D4E5F6",
  "techs": ["android.nfc.tech.NfcA", "android.nfc.tech.Ndef"],
  "ndefText": "文本内容",
  "ndefUri": "https://example.com",
  "extras": {}
}
```

#### iOS

```json
{
  "timestamp": 1769741815417,
  "action": "iOS_CoreNFC_MiFare",
  "uidHex": "53318EF7210001",
  "techs": ["MiFare", "MiFareUnknown"],
  "ndefUri": "https://example.com"
}
```

#### HarmonyOS

```json
{
  "timestamp": 1769741815417,
  "action": "Harmony_NFC_TAG",
  "uidHex": "04A1B2C3D4E5F6",
  "techs": ["NfcA", "MifareClassic", "Ndef"],
  "ndefText": "文本内容",
  "extras": {}
}
```

### 失败返回

```json
{ "code": -1, "data": { "error": "错误描述", "reason": "NO_TAG_IN_MEMORY" } }
```

常见 `reason` 值：

| reason | 说明 |
|--------|------|
| `NO_TAG_IN_MEMORY` | 内存中无活动标签 |
| `TAG_TTL_EXPIRED` | 标签超时（附带 `elapsedMs` / `ttlMs`） |

### 清理状态对比

| 方法 | 缓存队列 | 活动标签 | 去重 |
|------|:--------:|:--------:|:----:|
| `clearPendingTag` | 是 | 否 | 否 |
| `clearActiveTag` | 否 | 是 | 是 |
| `resetNfcState` | 是 | 是 | 是 |

---

## 平台注意事项

### Android

- **必须自定义基座或重打包**（含 NFC Intent、后台 `NfcDispatchActivity`）
- 后台/冷启动碰卡会先缓存，前端在 `onShow` 等时机用 `getPendingTagSync(true)` 消费并跳转
- 同一张卡在不同机型上 `techs` 可能不同；仅有 `NfcA` 而无 `MifareUltralight` 时，用 `nfcATransceive` 或 `ndefWrite`，不要强行 `mifareUltralightReadPages`
- `mifareUltralight*` 要求 techList 里真有 `android.nfc.tech.MifareUltralight`
- 高级读写依赖内存里的 `Tag`；仅读过磁盘缓存不算「有 Tag」。超时见 `TAG_TTL_EXPIRED`，可调大 `activeTagTtlMs` 或重新贴卡
- 插件已处理 Intent 消费防重复、ReaderMode 去重、读写后延长去重，一般无需手动管；若异常可 `resetNfcState()` 再试

### iOS

- 需在 Info.plist 配置 `NFCReaderUsageDescription`，Xcode 开启 NFC Capability，Entitlements 含 NDEF + TAG
- **无后台唤醒**；每次扫描通常需 `startNFCSession()`
- 每次新会话会丢弃上一轮未送达的旧回调；可选 `clearPendingTag()` 再注册
- 会话内续读：`configure({ keepSessionAlive: true, activeTagTtlMs: 10000 })` + `refreshActiveTagSync()`
- **写入 NDEF 必须设 `keepSessionAlive: true`**，否则读卡后会话自动关闭，写入必失败
- 不支持 Mifare Classic/Ultralight 读写块、NfcA/B raw transceive、NdefFormatable

### HarmonyOS

- 需 `ohos.permission.NFC_TAG` 权限
- 需 `startNFCSession()`，但无系统弹窗（与 iOS 不同）
- **部分 API 存在 `*Async` 变体**（返回 Promise），需运行时检测后 await 调用
- 在传统 uni-app 中**不能用** `#ifdef APP-HARMONY`，需用运行时检测替代

---

## 常见问题FAQ

### Q1：`#ifdef APP-HARMONY` 在 uni-app 中不生效？

**A：** 传统 uni-app 不支持 `APP-HARMONY` 条件编译标识符（它是 uni-app x 专属）。传统 uni-app 中 `APP-PLUS` 已覆盖 iOS + Android + HarmonyOS 三端。

**uni-app 正确做法：**
```js
// #ifdef APP-PLUS
import * as HlNfc from '@/uni_modules/hl-nfccase-uts'
// 运行时检测平台
const plat = uni.getSystemInfoSync().platform
// #endif
```

**uni-app x 可直接用：**
```js
// #ifdef APP-ANDROID || APP-IOS || APP-HARMONY
import * as HlNfc from '@/uni_modules/hl-nfccase-uts'
// #endif
```

### Q2：HarmonyOS 端调用 API 返回 undefined 或报错？

**A：** HarmonyOS 端 UTS 编译器会为部分 API 生成 `*Async` 变体（返回 Promise），直接调用同步版本可能不存在。需运行时检测：

```js
// 检测 Async 变体是否存在，存在则 await，否则调用同步版本
const asyncFn = HlNfc.mifareClassicReadBlockAsync
const syncFn = HlNfc.mifareClassicReadBlock
let res
if (typeof asyncFn === 'function') {
  res = await asyncFn(block, keyType, keyHex)
} else if (typeof syncFn === 'function') {
  res = syncFn(block, keyType, keyHex)
}
```

### Q3：iOS 写 NDEF 失败？

**A：** iOS 写入 NDEF 必须满足以下条件：
1. 先 `configure({ keepSessionAlive: true })` 保持会话
2. 再 `startNFCSession()` 启动扫描
3. 在 `onTagDiscovered` 回调内**立即**调用 `ndefWrite()`

离开会话或插件读完后自动关闭会话，都会导致写入失败。

### Q4：有 UID 但 `ndefWrite` / 读块报没有活动标签？

**A：** 可能只有磁盘缓存、没有本次前台扫描到的 `Tag`。高级读写 API 依赖进程内最近一次扫描的活动标签。

**解决：** 重新贴卡触发 `onTagDiscovered`，或检查 `activeTagTtlMs` 是否过短，或先 `resetNfcState()` 再重新扫描。

### Q5：Android 写一次后又触发一次 `onTagDiscovered` 回调？

**A：** 射频断开再连会再发现同一张卡。插件已做去重与写后延长去重。仍异常可试 `configure({ dedupeMs })` 或 `resetNfcState()`。

### Q6：iOS 系统 NFC 弹窗不出现？

**A：** 请检查：
1. Info.plist 中 `NFCReaderUsageDescription` 是否已添加
2. Xcode Capabilities 中 NFC Tag Reading 是否开启
3. Entitlements 文件是否包含 `com.apple.developer.nfc.readersession.formats`（NDEF + TAG）
4. 调用 `getNFCDebugInfo()` 查看当前 session 状态

### Q7：如何处理后台/冷启动碰卡？

**A：** Android 端后台碰卡会先缓存，App 启动后在 `onShow` 等时机用 `getPendingTagSync(true)` 消费：

```js
onShow(() => {
  const pending = HlNfc.getPendingTagSync(true)
  if (pending) {
    // 处理后台碰到的卡，如跳转页面
    console.log('后台碰卡:', pending.uidHex)
  }
})
```

### Q8：同一张卡在不同手机上 `techs` 不同？

**A：** 正常现象，不同机型 NFC 芯片和驱动实现不同。请根据实际 `techs` 字段判断能力，不要硬编码。仅有 `NfcA` 而无 `MifareUltralight` 时，用 `nfcATransceive` 或 `ndefWrite`，不要强行调用 `mifareUltralightReadPages`。

### Q9：iOS 关闭 NFC 后 `isNfcEnabled()` 仍然返回 `true`？

**A：** 这是 iOS CoreNFC 的已知限制。iOS 没有提供公开 API 来检查用户是否在系统设置中关闭了 NFC。

- `isNfcSupported()` 和 `isNfcEnabled()` 在 iOS 端返回相同值，都只检查「硬件支持 + entitlement 配置」
- iOS 的 NFC 设置开关（设置 > 通用 > NFC 标签读取）仅影响**后台碰卡检测**，前台 `NFCTagReaderSession` 不受其影响
- Android 端通过 `NfcAdapter.isEnabled` 可正确检测系统开关状态；HarmonyOS 通过 `controller.getNfcState()` 可检测

**iOS 正确做法**：不要依赖 `isNfcEnabled()` 判断用户是否关闭了 NFC，而是在 `startNFCSession` 后通过 `onSessionInvalidated` 回调判断会话是否异常中断：

```js
HlNfc.onSessionInvalidated((e) => {
  console.log('会话失效:', e.reason)
  // reason: 'userCanceled' / 'sessionTimeout' / 'systemBusy' / 'error'
})
```

---

## 技术支持

### 技术交流群

遇到文档未覆盖的问题？欢迎加入im技术交流群获取支持：

---

> 文档版本随插件迭代更新。
