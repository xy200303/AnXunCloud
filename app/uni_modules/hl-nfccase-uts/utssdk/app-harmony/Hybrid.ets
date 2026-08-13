/**
 * NFC UTS 插件 - Harmony 原生实现
 * 使用 @ohos.nfc.tag 和 @ohos.nfc.controller 实现 NFC 标签读取
 */
import tag from '@ohos.nfc.tag'
import controller from '@ohos.nfc.controller'
import util from '@ohos.util'
import bundleManager from '@ohos.bundle.bundleManager'
import { BusinessError } from '@ohos.base'

// 回调类型定义（避免循环引用）
export type HlNfcTagCallback = (data: object) => void

// ======================== 接口定义 ========================

/** API 返回结果 */
interface ApiResult {
  code: number
  data: ApiResultData
}

/** API 返回数据 */
interface ApiResultData {
  error?: string
  message?: string
  list?: object[]
  sector?: number
  block?: number
  blocks?: string[]
  dataHex?: string
  startPage?: number
  pages?: string[]
  page?: number
  responseHex?: string
  ndef?: NdefData
  ndefText?: string | null
  ndefUri?: string | null
  readbackConnectionLostAfterWrite?: boolean
}

/** NFC-A 额外信息 */
interface NfcAExtras {
  atqa: string
  sak: number
}

/** NFC-B 额外信息 */
interface NfcBExtras {
  appData: string
  protocolInfo: string
}

/** NFC-V 额外信息 */
interface NfcVExtras {
  responseFlags: number
  dsfId: number
}

/** IsoDep 额外信息 */
interface IsoDepExtras {
  historicalBytes: string
  hiLayerResponse: string
}

/** MiFare Classic 额外信息 */
interface MifareClassicExtras {
  type: string
  size: number
  sectorCount: number
}

/** MiFare Ultralight 额外信息 */
interface MifareUltralightExtras {
  type: string
}

/** NDEF 记录 */
interface NdefRecordData {
  tnf: number
  typeHex: string
  idHex: string
  payloadHex: string
}

/** NDEF 数据 */
interface NdefData {
  records: NdefRecordData[]
  text: string | null
  uri: string | null
}

/** 标签额外信息 */
interface TagExtras {
  nfcA?: NfcAExtras
  nfcB?: NfcBExtras
  nfcV?: NfcVExtras
  isoDep?: IsoDepExtras
  mifareClassic?: MifareClassicExtras
  mifareUltralight?: MifareUltralightExtras
}

/** 标签数据 */
interface TagData {
  timestamp: number
  action: string
  uidHex: string
  techs: string[]
  atqa?: string
  sak?: string
  tagType?: string
  mifareType?: string
  mifareUltralightType?: string
  ndef?: NdefData
  ndefText?: string | null
  ndefUri?: string | null
  extras: TagExtras
}

/** 配置选项 */
interface ConfigOptions {
  dedupeMs: number
  pendingMax: number
  activeTagTtlMs: number
}

/** 写入记录输入（从 index.uts 传入） */
export interface WriteRecordInput {
  type: string
  data: string
}

// ======================== 工具函数 ========================

/**
 * 将 number[] 转换为十六进制字符串
 */
function bytesToHex(bytes: number[] | null): string {
  if (bytes == null || bytes.length === 0) {
    return ''
  }
  let hex = ''
  for (let i = 0; i < bytes.length; i++) {
    const byte = bytes[i]
    hex += ('0' + (byte & 0xFF).toString(16)).slice(-2).toUpperCase()
  }
  return hex
}

/**
 * 将十六进制字符串转换为 number[]
 */
function hexToBytes(hex: string): number[] {
  const bytes: number[] = []
  for (let i = 0; i < hex.length; i += 2) {
    bytes.push(parseInt(hex.substring(i, i + 2), 16))
  }
  return bytes
}

/**
 * 将字符串转换为字节数组（UTF-8）
 */
function stringToBytes(str: string): number[] {
  const bytes: number[] = []
  for (let i = 0; i < str.length; i++) {
    const charCode = str.charCodeAt(i)
    if (charCode < 0x80) {
      bytes.push(charCode)
    } else if (charCode < 0x800) {
      bytes.push(0xC0 | (charCode >> 6))
      bytes.push(0x80 | (charCode & 0x3F))
    } else if (charCode < 0x10000) {
      bytes.push(0xE0 | (charCode >> 12))
      bytes.push(0x80 | ((charCode >> 6) & 0x3F))
      bytes.push(0x80 | (charCode & 0x3F))
    }
  }
  return bytes
}

/**
 * 获取当前时间戳（毫秒）
 */
function currentTimeMillis(): number {
  return Date.now()
}

/**
 * 获取当前活动标签，超出活动时间视为失效
 */
function getActiveTagInfo(): tag.TagInfo | null {
  if (currentTagInfo == null) {
    return null
  }

  const ttl = configOptions.activeTagTtlMs
  if (ttl > 0 && (currentTimeMillis() - lastTagAt) > ttl) {
    return null
  }

  return currentTagInfo
}

/**
 * 创建成功结果
 */
function createSuccessResult(data: ApiResultData): ApiResult {
  const result: ApiResult = {
    code: 0,
    data: data
  }
  return result
}

/**
 * 创建错误结果
 */
function createErrorResult(error: string): ApiResult {
  const data: ApiResultData = { error: error }
  const result: ApiResult = {
    code: -1,
    data: data
  }
  return result
}

/**
 * 转换 NFC 错误为友好提示
 */
function formatNfcError(errorMessage: string): string {
  if (errorMessage.includes('tag running state is abnormal')) {
    return '标签连接已断开，请保持标签在设备上并重新扫描'
  }
  if (errorMessage.includes('not connected')) {
    return '标签未连接，请重新扫描标签'
  }
  if (errorMessage.includes('authentication failed') || errorMessage.includes('auth')) {
    return '标签认证失败，请检查密钥是否正确'
  }
  if (errorMessage.includes('timeout')) {
    return '操作超时，请重试'
  }
  if (errorMessage.includes('not support')) {
    return '标签不支持此操作'
  }
  // 鸿蒙端高级读写功能受限的通用提示
  return '鸿蒙端高级读写功能受限: ' + errorMessage
}

// ======================== 全局状态 ========================

// NFC 标签回调
let tagCallback: HlNfcTagCallback | null = null

// 待处理的 NFC 标签队列
let pendingTags: TagData[] = []

// 去重相关
let lastSavedAt: number = 0
let lastSavedUidHex: string = ''

// 配置项
let configOptions: ConfigOptions = {
  dedupeMs: 600,
  pendingMax: 10,
  activeTagTtlMs: 5000
}

// 当前连接的标签
let currentTagInfo: tag.TagInfo | null = null
let lastTagAt: number = 0

// 前台调度状态
let foregroundDispatchRegistered: boolean = false

// ======================== NFC 前台调度 ========================

/**
 * NFC 标签发现回调函数（AsyncCallback 类型）
 */
function onNfcTagDiscovered(err: BusinessError, tagInfo: tag.TagInfo): void {
  if (err) {
    console.error('[HlNfc-Harmony] onNfcTagDiscovered error:', err.message)
    return
  }
  console.log('[HlNfc-Harmony] onNfcTagDiscovered callback triggered')
  processTagInfo(tagInfo)
}

/**
 * 注册 NFC 前台调度
 */
async function registerForegroundDispatch(): Promise<void> {
  if (foregroundDispatchRegistered) {
    console.log('[HlNfc-Harmony] Foreground dispatch already registered')
    return
  }

  try {
    // 检查 NFC 状态
    const nfcState = controller.getNfcState()
    if (nfcState !== controller.NfcState.STATE_ON) {
      console.log('[HlNfc-Harmony] NFC is not enabled, state:', nfcState)
      return
    }

    // 获取应用 bundleName
    const bundleInfo = await bundleManager.getBundleInfoForSelf(
      bundleManager.BundleFlag.GET_BUNDLE_INFO_WITH_APPLICATION
    )
    const bundleName = bundleInfo.name
    console.log('[HlNfc-Harmony] Bundle name:', bundleName)

    // 定义要监听的技术类型
    const techList: number[] = [
      tag.NFC_A,
      tag.NFC_B,
      tag.NFC_V,
      tag.ISO_DEP,
      tag.NDEF,
      tag.MIFARE_CLASSIC,
      tag.MIFARE_ULTRALIGHT
    ]

    // 构造 ElementName
    const elementName: bundleManager.ElementName = {
      bundleName: bundleName,
      abilityName: 'EntryAbility'
    }

    // 注册前台调度（使用函数式回调）
    tag.registerForegroundDispatch(elementName, techList, onNfcTagDiscovered)
    foregroundDispatchRegistered = true
    console.log('[HlNfc-Harmony] Foreground dispatch registered successfully')
  } catch (e) {
    const error = e as Error
    console.error('[HlNfc-Harmony] Failed to register foreground dispatch:', error.message)
  }
}

/**
 * 注销 NFC 前台调度
 */
function unregisterForegroundDispatch(): void {
  if (!foregroundDispatchRegistered) {
    console.log('[HlNfc-Harmony] Foreground dispatch not registered')
    return
  }

  try {
    // 获取应用 bundleName（同步方式）
    bundleManager.getBundleInfoForSelf(
      bundleManager.BundleFlag.GET_BUNDLE_INFO_WITH_APPLICATION
    ).then((bundleInfo: bundleManager.BundleInfo) => {
      const elementName: bundleManager.ElementName = {
        bundleName: bundleInfo.name,
        abilityName: 'EntryAbility'
      }
      tag.unregisterForegroundDispatch(elementName)
      foregroundDispatchRegistered = false
      console.log('[HlNfc-Harmony] Foreground dispatch unregistered')
    }).catch((e: Error) => {
      console.error('[HlNfc-Harmony] Failed to get bundle info:', e.message)
    })
  } catch (e) {
    const error = e as Error
    console.error('[HlNfc-Harmony] Failed to unregister foreground dispatch:', error.message)
  }
}

// ======================== NFC 标签处理 ========================

/**
 * 处理 NFC 标签发现事件（供外部调用）
 */
export function processTagInfo(tagInfo: tag.TagInfo): void {
  console.log('[HlNfc-Harmony] Tag discovered')
  
  // 保存当前标签
  currentTagInfo = tagInfo
  lastTagAt = currentTimeMillis()
  
  // 解析标签数据
  const tagData: TagData = parseTagInfo(tagInfo)
  
  // 去重检查
  const now = currentTimeMillis()
  const uidHex = tagData.uidHex
  if (configOptions.dedupeMs > 0 && uidHex.length > 0 && 
      lastSavedUidHex === uidHex && (now - lastSavedAt) < configOptions.dedupeMs) {
    console.log('[HlNfc-Harmony] Deduped tag:', uidHex)
    return
  }
  
  // 更新去重状态
  lastSavedUidHex = uidHex
  lastSavedAt = now
  
  // 保存到待处理队列
  saveTagToPending(tagData)
  
  // 触发回调
  if (tagCallback != null) {
    tagCallback(tagData)
  }
}

/**
 * 解析 NFC 标签信息
 */
function parseTagInfo(tagInfo: tag.TagInfo): TagData {
  const extras: TagExtras = {}
  
  // 获取支持的技术类型
  const techs: string[] = []
  const techProfiles = tagInfo.technology
  for (let i = 0; i < techProfiles.length; i++) {
    const tech = techProfiles[i]
    if (tech === tag.NFC_A) {
      techs.push('NfcA')
    } else if (tech === tag.NFC_B) {
      techs.push('NfcB')
    } else if (tech === tag.NFC_F) {
      techs.push('NfcF')
    } else if (tech === tag.NFC_V) {
      techs.push('NfcV')
    } else if (tech === tag.ISO_DEP) {
      techs.push('IsoDep')
    } else if (tech === tag.NDEF) {
      techs.push('Ndef')
    } else if (tech === tag.MIFARE_CLASSIC) {
      techs.push('MifareClassic')
    } else if (tech === tag.MIFARE_ULTRALIGHT) {
      techs.push('MifareUltralight')
    } else if (tech === tag.NDEF_FORMATABLE) {
      techs.push('NdefFormatable')
    }
  }
  
  // 初始化结果
  const result: TagData = {
    timestamp: currentTimeMillis(),
    action: 'Harmony_NFC_TAG',
    uidHex: bytesToHex(tagInfo.uid),
    techs: techs,
    extras: extras
  }
  
  // 解析各技术类型的额外信息
  
  // NFC-A 信息
  if (techProfiles.includes(tag.NFC_A)) {
    try {
      const nfcA = tag.getNfcA(tagInfo)
      if (nfcA != null) {
        const nfcAExtras: NfcAExtras = {
          atqa: bytesToHex(nfcA.getAtqa()),
          sak: nfcA.getSak()
        }
        extras.nfcA = nfcAExtras
        result.atqa = bytesToHex(nfcA.getAtqa())
        result.sak = nfcA.getSak().toString(16).toUpperCase()
      }
    } catch (e) {
      console.log('[HlNfc-Harmony] Failed to get NfcA info')
    }
  }
  
  // NFC-B 信息
  if (techProfiles.includes(tag.NFC_B)) {
    try {
      const nfcB = tag.getNfcB(tagInfo)
      if (nfcB != null) {
        const nfcBExtras: NfcBExtras = {
          appData: bytesToHex(nfcB.getRespAppData()),
          protocolInfo: bytesToHex(nfcB.getRespProtocol())
        }
        extras.nfcB = nfcBExtras
      }
    } catch (e) {
      console.log('[HlNfc-Harmony] Failed to get NfcB info')
    }
  }
  
  // NFC-V 信息
  if (techProfiles.includes(tag.NFC_V)) {
    try {
      const nfcV = tag.getNfcV(tagInfo)
      if (nfcV != null) {
        const nfcVExtras: NfcVExtras = {
          responseFlags: nfcV.getResponseFlags(),
          dsfId: nfcV.getDsfId()
        }
        extras.nfcV = nfcVExtras
      }
    } catch (e) {
      console.log('[HlNfc-Harmony] Failed to get NfcV info')
    }
  }
  
  // IsoDep 信息
  if (techProfiles.includes(tag.ISO_DEP)) {
    try {
      const isoDep = tag.getIsoDep(tagInfo)
      if (isoDep != null) {
        const isoDepExtras: IsoDepExtras = {
          historicalBytes: bytesToHex(isoDep.getHistoricalBytes()),
          hiLayerResponse: bytesToHex(isoDep.getHiLayerResponse())
        }
        extras.isoDep = isoDepExtras
      }
    } catch (e) {
      console.log('[HlNfc-Harmony] Failed to get IsoDep info')
    }
  }
  
  // NDEF 信息
  if (techProfiles.includes(tag.NDEF)) {
    try {
      const ndef = tag.getNdef(tagInfo)
      if (ndef != null) {
        const ndefMessage = ndef.getNdefMessage()
        if (ndefMessage != null) {
          const records = ndefMessage.getNdefRecords()
          const ndefRecords: NdefRecordData[] = []
          let ndefText: string | null = null
          let ndefUri: string | null = null
          
          for (let i = 0; i < records.length; i++) {
            const record = records[i]
            const recordObj: NdefRecordData = {
              tnf: record.tnf,
              typeHex: bytesToHex(record.rtdType),
              idHex: bytesToHex(record.id),
              payloadHex: bytesToHex(record.payload)
            }
            ndefRecords.push(recordObj)
            
            // 解析文本记录
            if (record.tnf === 1 && bytesToHex(record.rtdType) === '54') {
              ndefText = parseNdefText(record.payload)
            }
            
            // 解析 URI 记录
            if (record.tnf === 1 && bytesToHex(record.rtdType) === '55') {
              ndefUri = parseNdefUri(record.payload)
            }
          }
          
          const ndefData: NdefData = {
            records: ndefRecords,
            text: ndefText,
            uri: ndefUri
          }
          result.ndef = ndefData
          result.ndefText = ndefText
          result.ndefUri = ndefUri
        }
      }
    } catch (e) {
      console.log('[HlNfc-Harmony] Failed to get NDEF info')
    }
  }
  
  // MiFare Classic 信息
  if (techProfiles.includes(tag.MIFARE_CLASSIC)) {
    try {
      const mifareClassic = tag.getMifareClassic(tagInfo)
      if (mifareClassic != null) {
        const mifareType = mifareClassic.getType()
        let typeName = 'Unknown'
        if (mifareType === tag.MifareClassicType.TYPE_CLASSIC) {
          typeName = 'Classic'
        } else if (mifareType === tag.MifareClassicType.TYPE_PLUS) {
          typeName = 'Plus'
        } else if (mifareType === tag.MifareClassicType.TYPE_PRO) {
          typeName = 'Pro'
        }
        const mifareExtras: MifareClassicExtras = {
          type: typeName,
          size: mifareClassic.getTagSize(),
          sectorCount: mifareClassic.getSectorCount()
        }
        extras.mifareClassic = mifareExtras
        result.tagType = 'MifareClassic'
        result.mifareType = typeName
      }
    } catch (e) {
      console.log('[HlNfc-Harmony] Failed to get MifareClassic info')
    }
  }
  
  // MiFare Ultralight 信息
  if (techProfiles.includes(tag.MIFARE_ULTRALIGHT)) {
    try {
      const mifareUl = tag.getMifareUltralight(tagInfo)
      if (mifareUl != null) {
        const ulType = mifareUl.getType()
        let typeName = 'Unknown'
        if (ulType === tag.MifareUltralightType.TYPE_ULTRALIGHT) {
          typeName = 'Ultralight'
        } else if (ulType === tag.MifareUltralightType.TYPE_ULTRALIGHT_C) {
          typeName = 'UltralightC'
        }
        const ulExtras: MifareUltralightExtras = {
          type: typeName
        }
        extras.mifareUltralight = ulExtras
        result.tagType = 'MifareUltralight'
        result.mifareUltralightType = typeName
      }
    } catch (e) {
      console.log('[HlNfc-Harmony] Failed to get MifareUltralight info')
    }
  }
  
  return result
}

/**
 * 解析 NDEF 文本记录
 */
function parseNdefText(payload: number[]): string | null {
  if (payload == null || payload.length < 2) {
    return null
  }
  
  const status = payload[0]
  const langLen = status & 0x3F
  
  if (payload.length < 1 + langLen + 1) {
    return null
  }
  
  const textBytes = payload.slice(1 + langLen)
  try {
    const uint8Array = new Uint8Array(textBytes)
    const decoder = new util.TextDecoder('utf-8')
    return decoder.decodeWithStream(uint8Array)
  } catch (e) {
    return null
  }
}

/**
 * 解析 NDEF URI 记录
 */
function parseNdefUri(payload: number[]): string | null {
  if (payload == null || payload.length < 2) {
    return null
  }
  
  const prefixCode = payload[0]
  const uriPrefixes: string[] = [
    '', 'http://www.', 'https://www.', 'http://', 'https://',
    'tel:', 'mailto:', 'ftp://anonymous:anonymous@', 'ftp://ftp.',
    'ftps://', 'sftp://', 'smb://', 'nfs://', 'ftp://', 'dav://',
    'news:', 'telnet://', 'imap:', 'rtsp://', 'urn:', 'pop:',
    'sip:', 'sips:', 'tftp:', 'btspp://', 'btl2cap://', 'btgoep://',
    'tcpobex://', 'irdaobex://', 'file://', 'urn:epc:id:', 'urn:epc:tag:',
    'urn:epc:pat:', 'urn:epc:raw:', 'urn:epc:', 'urn:nfc:'
  ]
  
  const prefix = prefixCode < uriPrefixes.length ? uriPrefixes[prefixCode] : ''
  
  const uriBytes = payload.slice(1)
  try {
    const uint8Array = new Uint8Array(uriBytes)
    const decoder = new util.TextDecoder('utf-8')
    const suffix = decoder.decodeWithStream(uint8Array)
    return prefix + suffix
  } catch (e) {
    return null
  }
}

/**
 * 从 NDEF 消息中提取结构化数据
 */
function extractNdefData(ndefMessage: tag.NdefMessage): NdefData {
  const records = ndefMessage.getNdefRecords()
  const ndefRecords: NdefRecordData[] = []
  let ndefText: string | null = null
  let ndefUri: string | null = null

  for (let i = 0; i < records.length; i++) {
    const record = records[i]
    const recordObj: NdefRecordData = {
      tnf: record.tnf,
      typeHex: bytesToHex(record.rtdType),
      idHex: bytesToHex(record.id),
      payloadHex: bytesToHex(record.payload)
    }
    ndefRecords.push(recordObj)

    if (record.tnf === 1 && bytesToHex(record.rtdType) === '54') {
      ndefText = parseNdefText(record.payload)
    }

    if (record.tnf === 1 && bytesToHex(record.rtdType) === '55') {
      ndefUri = parseNdefUri(record.payload)
    }
  }

  const ndefData: NdefData = {
    records: ndefRecords,
    text: ndefText,
    uri: ndefUri
  }
  return ndefData
}

/**
 * 主动连接当前标签并重新读取一次最新快照
 */
async function buildLiveTagDataAsync(tagInfo: tag.TagInfo): Promise<TagData> {
  const result = parseTagInfo(tagInfo)

  if (tagInfo.technology.includes(tag.NDEF)) {
    try {
      const ndef = tag.getNdef(tagInfo)
      if (ndef != null) {
        await ndef.connect()
        const ndefMessage = ndef.getNdefMessage()
        if (ndefMessage != null) {
          const ndefData = extractNdefData(ndefMessage)
          result.ndef = ndefData
          result.ndefText = ndefData.text
          result.ndefUri = ndefData.uri
        }
      }
    } catch (e) {
      console.log('[HlNfc-Harmony] buildLiveTagDataAsync: failed to refresh NDEF data')
    }
  }

  result.timestamp = currentTimeMillis()
  result.action = 'Harmony_NFC_ACTIVE_REFRESH'
  return result
}

/**
 * 保存标签到待处理队列
 */
function saveTagToPending(tagData: TagData): void {
  pendingTags.unshift(tagData)
  
  if (pendingTags.length > configOptions.pendingMax) {
    pendingTags = pendingTags.slice(0, configOptions.pendingMax)
  }
  
  console.log('[HlNfc-Harmony] Tag saved to pending, queue size:', pendingTags.length)
}

// ======================== 导出类 ========================

export class Hybrid {
  /**
   * 监听 NFC 标签发现事件
   * 注意：鸿蒙端 NFC 前台调度需要在 Ability 的 module.json5 中配置 skills
   * 并在 EntryAbility 的 onCreate 中处理 want 参数
   */
  static onTagDiscovered(callback: HlNfcTagCallback): void {
    console.log('[HlNfc-Harmony] onTagDiscovered registered')
    tagCallback = callback
    
    // 如果有缓存事件，立即派发
    if (pendingTags.length > 0) {
      const pending = pendingTags[0]
      callback(pending)
    }
  }
  
  /**
   * 获取待处理的 NFC 标签
   */
  static getPendingTagSync(clearRead: boolean): object {
    if (pendingTags.length === 0) {
      return createErrorResult('no pending tag')
    }
    
    const tagData = pendingTags[0]
    
    if (clearRead) {
      pendingTags.shift()
    }
    
    const result: ApiResult = {
      code: 0,
      data: tagData as ApiResultData
    }
    return result
  }
  
  /**
   * 获取多个待处理的 NFC 标签
   */
  static getPendingTagsSync(clearRead: boolean, max: number): object {
    const list = pendingTags.slice(0, max)
    
    if (clearRead) {
      pendingTags = []
    }
    
    const data: ApiResultData = { list: list }
    return createSuccessResult(data)
  }

  /**
   * 立即刷新当前活动标签快照
   */
  static refreshActiveTagSync(): object {
    const tagInfo = getActiveTagInfo()
    if (tagInfo == null) {
      return createErrorResult('没有活动的NFC标签，请先扫描标签')
    }

    try {
      const tagData = parseTagInfo(tagInfo)
      return createSuccessResult(tagData as ApiResultData)
    } catch (e) {
      const error = e as Error
      return createErrorResult(error.message)
    }
  }

  /**
   * 异步刷新当前活动标签快照
   * 适用于写入后保持贴卡，主动重新读取最新内容
   */
  static async refreshActiveTagAsync(): Promise<object> {
    const tagInfo = getActiveTagInfo()
    if (tagInfo == null) {
      return createErrorResult('没有活动的NFC标签，请先扫描标签')
    }

    try {
      const tagData = await buildLiveTagDataAsync(tagInfo)
      return createSuccessResult(tagData as ApiResultData)
    } catch (e) {
      const error = e as Error
      return createErrorResult(formatNfcError(error.message))
    }
  }
  
  /**
   * 清空待处理的 NFC 标签
   */
  static clearPendingTag(): void {
    pendingTags = []
    console.log('[HlNfc-Harmony] Pending tags cleared')
  }

  static clearActiveTag(): void {
    currentTagInfo = null
    lastTagAt = 0
    lastSavedAt = 0
    lastSavedUidHex = ''
    console.log('[HlNfc-Harmony] clearActiveTag: active tag and dedupe state cleared')
  }

  static resetNfcState(): void {
    Hybrid.clearPendingTag()
    Hybrid.clearActiveTag()
    console.log('[HlNfc-Harmony] resetNfcState: all NFC state cleared')
  }
  
  /**
   * 检查设备是否支持 NFC
   */
  static isNfcSupported(): boolean {
    try {
      return controller.isNfcAvailable()
    } catch (e) {
      return false
    }
  }
  
  /**
   * 检查 NFC 是否已开启
   */
  static isNfcEnabled(): boolean {
    try {
      const state = controller.getNfcState()
      return state === controller.NfcState.STATE_ON
    } catch (e) {
      return false
    }
  }
  
  /**
   * 配置插件参数
   */
  static configure(options: object | null): void {
    if (options == null) {
      return
    }
    
    const opts = options as Record<string, ESObject>
    
    if (opts['dedupeMs'] != null) {
      configOptions.dedupeMs = opts['dedupeMs'] as number
    }
    if (opts['pendingMax'] != null) {
      configOptions.pendingMax = opts['pendingMax'] as number
    }
    if (opts['activeTagTtlMs'] != null) {
      configOptions.activeTagTtlMs = opts['activeTagTtlMs'] as number
    }
    
    console.log('[HlNfc-Harmony] Config updated:', JSON.stringify(configOptions))
  }
  
  /**
   * 启动 NFC 扫描会话
   */
  static startNFCSession(): void {
    console.log('[HlNfc-Harmony] startNFCSession called')
    registerForegroundDispatch()
  }
  
  /**
   * 停止 NFC 扫描会话
   */
  static stopNFCSession(): void {
    console.log('[HlNfc-Harmony] stopNFCSession called')
    unregisterForegroundDispatch()
  }
  
  /**
   * MiFare Classic 读扇区（同步版本 - 返回提示）
   */
  static mifareClassicReadSector(sector: number, keyType: string, keyHex: string): object {
    return createErrorResult('鸿蒙端 MiFare Classic 高级读写功能受限，请使用标签发现回调获取基本信息')
  }
  
  /**
   * MiFare Classic 读扇区（异步版本）
   */
  static async mifareClassicReadSectorAsync(sector: number, keyType: string, keyHex: string): Promise<object> {
    console.log('[HlNfc-Harmony] mifareClassicReadSectorAsync called, sector:', sector)
    
    if (currentTagInfo == null) {
      return createErrorResult('没有活动的NFC标签，请先扫描标签')
    }
    
    try {
      const mifareClassic = tag.getMifareClassic(currentTagInfo)
      if (mifareClassic == null) {
        return createErrorResult('当前标签不支持 MiFare Classic')
      }
      
      // 连接标签
      await mifareClassic.connect()
      
      // 认证扇区
      const keyBytes = hexToBytes(keyHex)
      const isKeyB = keyType.toUpperCase() === 'B'
      await mifareClassic.authenticateSector(sector, keyBytes, isKeyB)
      
      // 获取扇区块数和起始块
      const sectorCount = mifareClassic.getSectorCount()
      if (sector >= sectorCount) {
        return createErrorResult('扇区号超出范围')
      }
      
      // 计算扇区内的块数（扇区0-31每扇区4块，扇区32+每扇区16块）
      const blocksInSector = sector < 32 ? 4 : 16
      const firstBlock = sector < 32 ? sector * 4 : 128 + (sector - 32) * 16
      
      // 读取扇区内所有块
      const blocks: string[] = []
      for (let i = 0; i < blocksInSector; i++) {
        const blockIndex = firstBlock + i
        const blockData = await mifareClassic.readSingleBlock(blockIndex)
        blocks.push(bytesToHex(blockData))
      }
      
      const data: ApiResultData = {
        sector: sector,
        blocks: blocks
      }
      return createSuccessResult(data)
    } catch (e) {
      const error = e as Error
      console.error('[HlNfc-Harmony] mifareClassicReadSectorAsync error:', error.message)
      return createErrorResult(formatNfcError(error.message))
    }
  }
  
  /**
   * MiFare Classic 读块（同步版本 - 返回提示）
   */
  static mifareClassicReadBlock(block: number, keyType: string, keyHex: string): object {
    return createErrorResult('鸿蒙端 MiFare Classic 高级读写功能受限，请使用标签发现回调获取基本信息')
  }
  
  /**
   * MiFare Classic 读块（异步版本）
   */
  static async mifareClassicReadBlockAsync(block: number, keyType: string, keyHex: string): Promise<object> {
    console.log('[HlNfc-Harmony] mifareClassicReadBlockAsync called, block:', block)
    
    if (currentTagInfo == null) {
      return createErrorResult('没有活动的NFC标签，请先扫描标签')
    }
    
    try {
      const mifareClassic = tag.getMifareClassic(currentTagInfo)
      if (mifareClassic == null) {
        return createErrorResult('当前标签不支持 MiFare Classic')
      }
      
      // 连接标签
      await mifareClassic.connect()
      
      // 计算扇区号（块0-127属于扇区0-31，每扇区4块；块128+属于扇区32+，每扇区16块）
      const sector = block < 128 ? Math.floor(block / 4) : 32 + Math.floor((block - 128) / 16)
      
      // 认证扇区
      const keyBytes = hexToBytes(keyHex)
      const isKeyB = keyType.toUpperCase() === 'B'
      await mifareClassic.authenticateSector(sector, keyBytes, isKeyB)
      
      // 读取块
      const blockData = await mifareClassic.readSingleBlock(block)
      
      const data: ApiResultData = {
        block: block,
        dataHex: bytesToHex(blockData)
      }
      return createSuccessResult(data)
    } catch (e) {
      const error = e as Error
      console.error('[HlNfc-Harmony] mifareClassicReadBlockAsync error:', error.message)
      return createErrorResult(formatNfcError(error.message))
    }
  }
  
  /**
   * MiFare Ultralight 读页（同步版本 - 返回提示）
   */
  static mifareUltralightReadPages(startPage: number, pageCount: number): object {
    return createErrorResult('鸿蒙端 MiFare Ultralight 高级读写功能受限，请使用标签发现回调获取基本信息')
  }
  
  /**
   * MiFare Ultralight 读页（异步版本）
   */
  static async mifareUltralightReadPagesAsync(startPage: number, pageCount: number): Promise<object> {
    console.log('[HlNfc-Harmony] mifareUltralightReadPagesAsync called, startPage:', startPage)
    
    if (currentTagInfo == null) {
      return createErrorResult('没有活动的NFC标签，请先扫描标签')
    }
    
    try {
      const mifareUl = tag.getMifareUltralight(currentTagInfo)
      if (mifareUl == null) {
        return createErrorResult('当前标签不支持 MiFare Ultralight')
      }
      
      // 连接标签
      await mifareUl.connect()
      
      // 读取页（每次读取4页）
      const pages: string[] = []
      for (let i = 0; i < pageCount; i++) {
        const pageData = await mifareUl.readMultiplePages(startPage + i)
        // readMultiplePages 返回从指定页开始的4页数据（16字节），我们只取第一页（4字节）
        const singlePageData = pageData.slice(0, 4)
        pages.push(bytesToHex(singlePageData))
      }
      
      const data: ApiResultData = {
        startPage: startPage,
        pages: pages
      }
      return createSuccessResult(data)
    } catch (e) {
      const error = e as Error
      console.error('[HlNfc-Harmony] mifareUltralightReadPagesAsync error:', error.message)
      return createErrorResult(formatNfcError(error.message))
    }
  }
  
  /**
   * IsoDep 发送 APDU（同步版本 - 返回提示）
   */
  static isoDepTransceive(apduHex: string): object {
    return createErrorResult('鸿蒙端 IsoDep APDU 功能受限，请使用标签发现回调获取基本信息')
  }
  
  /**
   * IsoDep 发送 APDU（异步版本）
   */
  static async isoDepTransceiveAsync(apduHex: string): Promise<object> {
    console.log('[HlNfc-Harmony] isoDepTransceiveAsync called')
    
    if (currentTagInfo == null) {
      return createErrorResult('没有活动的NFC标签，请先扫描标签')
    }
    
    try {
      const isoDep = tag.getIsoDep(currentTagInfo)
      if (isoDep == null) {
        return createErrorResult('当前标签不支持 IsoDep')
      }
      
      // 连接标签
      await isoDep.connect()
      
      // 发送 APDU
      const response = await isoDep.transmit(hexToBytes(apduHex))
      
      const data: ApiResultData = {
        responseHex: bytesToHex(response)
      }
      return createSuccessResult(data)
    } catch (e) {
      const error = e as Error
      console.error('[HlNfc-Harmony] isoDepTransceiveAsync error:', error.message)
      return createErrorResult(formatNfcError(error.message))
    }
  }
  
  /**
   * NfcV 发送命令（同步版本 - 返回提示）
   */
  static nfcVTransceive(cmdHex: string): object {
    return createErrorResult('鸿蒙端 NfcV 功能受限')
  }
  
  /**
   * NfcV 发送命令（异步版本）
   */
  static async nfcVTransceiveAsync(cmdHex: string): Promise<object> {
    console.log('[HlNfc-Harmony] nfcVTransceiveAsync called')
    
    if (currentTagInfo == null) {
      return createErrorResult('没有活动的NFC标签，请先扫描标签')
    }
    
    try {
      const nfcV = tag.getNfcV(currentTagInfo)
      if (nfcV == null) {
        return createErrorResult('当前标签不支持 NfcV')
      }
      
      // 连接标签
      await nfcV.connect()
      
      // 发送命令（NfcV 没有 transceive，使用 NfcATag 的方式）
      // 鸿蒙 NfcV 可能没有直接的 transceive 方法，这里返回不支持
      return createErrorResult('鸿蒙端 NfcV transceive 暂不支持')
    } catch (e) {
      const error = e as Error
      console.error('[HlNfc-Harmony] nfcVTransceiveAsync error:', error.message)
      return createErrorResult(formatNfcError(error.message))
    }
  }
  
  /**
   * 写入 NDEF 消息（同步版本 - 返回提示）
   */
  static ndefWrite(records: WriteRecordInput[]): object {
    return createErrorResult('鸿蒙端 NDEF 写入功能受限，标签连接可能已断开，请保持标签在设备上并重新扫描')
  }
  
  /**
   * 写入 NDEF 消息（异步版本）
   */
  static async ndefWriteAsync(records: WriteRecordInput[]): Promise<object> {
    console.log('[HlNfc-Harmony] ndefWriteAsync called')
    
    if (currentTagInfo == null) {
      return createErrorResult('没有活动的NFC标签，请先扫描标签')
    }
    
    try {
      const ndef = tag.getNdef(currentTagInfo)
      if (ndef == null) {
        return createErrorResult('当前标签不支持 NDEF')
      }
      
      // 连接标签
      await ndef.connect()
      
      // 构建 NDEF 记录
      const ndefRecords: tag.NdefRecord[] = []
      for (let i = 0; i < records.length; i++) {
        const input = records[i]
        let ndefRecord: tag.NdefRecord
        
        if (input.type === 'text') {
          // 文本记录
          const textBytes = stringToBytes(input.data)
          const langCode = stringToBytes('en')
          const payload: number[] = [langCode.length]
          payload.push(...langCode)
          payload.push(...textBytes)
          
          ndefRecord = {
            tnf: 1, // TNF_WELL_KNOWN
            rtdType: [0x54], // 'T' for Text
            id: [],
            payload: payload
          }
        } else if (input.type === 'uri') {
          // URI 记录
          let prefixCode = 0
          let uri = input.data
          
          // 检查常见前缀
          if (uri.startsWith('https://www.')) {
            prefixCode = 2
            uri = uri.substring(12)
          } else if (uri.startsWith('http://www.')) {
            prefixCode = 1
            uri = uri.substring(11)
          } else if (uri.startsWith('https://')) {
            prefixCode = 4
            uri = uri.substring(8)
          } else if (uri.startsWith('http://')) {
            prefixCode = 3
            uri = uri.substring(7)
          }
          
          const uriBytes = stringToBytes(uri)
          const payload: number[] = [prefixCode]
          payload.push(...uriBytes)
          
          ndefRecord = {
            tnf: 1, // TNF_WELL_KNOWN
            rtdType: [0x55], // 'U' for URI
            id: [],
            payload: payload
          }
        } else {
          continue
        }
        
        ndefRecords.push(ndefRecord)
      }
      
      if (ndefRecords.length === 0) {
        return createErrorResult('没有有效的 NDEF 记录')
      }
      
      // 创建 NDEF 消息并写入
      const ndefMessage = tag.ndef.createNdefMessage(ndefRecords)
      await ndef.writeNdef(ndefMessage)
      
      const data: ApiResultData = {
        message: '写入成功'
      }
      
      // 提取写入的数据并组装，与 Android/iOS 对齐，跳过不可靠的轮询读回
      const writtenNdefData = extractNdefData(ndefMessage)
      data.ndef = writtenNdefData
      data.ndefText = writtenNdefData.text
      data.ndefUri = writtenNdefData.uri
      data.readbackConnectionLostAfterWrite = true
      
      return createSuccessResult(data)
    } catch (e) {
      const error = e as Error
      console.error('[HlNfc-Harmony] ndefWriteAsync error:', error.message)
      return createErrorResult(formatNfcError(error.message))
    }
  }
  
  /**
   * MiFare Classic 写块（同步版本 - 返回提示）
   */
  static mifareClassicWriteBlock(block: number, keyType: string, keyHex: string, dataHex: string): object {
    return createErrorResult('鸿蒙端 MiFare Classic 高级读写功能受限')
  }
  
  /**
   * MiFare Classic 写块（异步版本）
   */
  static async mifareClassicWriteBlockAsync(block: number, keyType: string, keyHex: string, dataHex: string): Promise<object> {
    console.log('[HlNfc-Harmony] mifareClassicWriteBlockAsync called, block:', block)
    
    if (currentTagInfo == null) {
      return createErrorResult('没有活动的NFC标签，请先扫描标签')
    }
    
    try {
      const mifareClassic = tag.getMifareClassic(currentTagInfo)
      if (mifareClassic == null) {
        return createErrorResult('当前标签不支持 MiFare Classic')
      }
      
      // 连接标签
      await mifareClassic.connect()
      
      // 计算扇区号
      const sector = block < 128 ? Math.floor(block / 4) : 32 + Math.floor((block - 128) / 16)
      
      // 认证扇区
      const keyBytes = hexToBytes(keyHex)
      const isKeyB = keyType.toUpperCase() === 'B'
      await mifareClassic.authenticateSector(sector, keyBytes, isKeyB)
      
      // 写入块
      const dataBytes = hexToBytes(dataHex)
      await mifareClassic.writeSingleBlock(block, dataBytes)
      
      const data: ApiResultData = {
        message: '写入成功',
        block: block
      }
      return createSuccessResult(data)
    } catch (e) {
      const error = e as Error
      console.error('[HlNfc-Harmony] mifareClassicWriteBlockAsync error:', error.message)
      return createErrorResult(formatNfcError(error.message))
    }
  }
  
  /**
   * MiFare Ultralight 写页（同步版本 - 返回提示）
   */
  static mifareUltralightWritePage(page: number, dataHex: string): object {
    return createErrorResult('鸿蒙端 MiFare Ultralight 高级读写功能受限')
  }
  
  /**
   * MiFare Ultralight 写页（异步版本）
   */
  static async mifareUltralightWritePageAsync(page: number, dataHex: string): Promise<object> {
    console.log('[HlNfc-Harmony] mifareUltralightWritePageAsync called, page:', page)
    
    if (currentTagInfo == null) {
      return createErrorResult('没有活动的NFC标签，请先扫描标签')
    }
    
    try {
      const mifareUl = tag.getMifareUltralight(currentTagInfo)
      if (mifareUl == null) {
        return createErrorResult('当前标签不支持 MiFare Ultralight')
      }
      
      // 连接标签
      await mifareUl.connect()
      
      // 写入页
      const dataBytes = hexToBytes(dataHex)
      await mifareUl.writeSinglePage(page, dataBytes)
      
      const data: ApiResultData = {
        message: '写入成功',
        page: page
      }
      return createSuccessResult(data)
    } catch (e) {
      const error = e as Error
      console.error('[HlNfc-Harmony] mifareUltralightWritePageAsync error:', error.message)
      return createErrorResult(formatNfcError(error.message))
    }
  }
  
  /**
   * 检查并保存当前 Intent 中的 NFC 数据（兼容 Android 接口）
   */
  static checkAndSaveCurrentIntentNfc(): boolean {
    return false
  }
  
  /**
   * 检查 NfcDispatchActivity 是否被调用过（兼容 Android 接口）
   */
  static checkNfcDispatchCalled(): number {
    return 0
  }
}
