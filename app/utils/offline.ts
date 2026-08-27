/**
 * 打卡离线队列（技术方案 §5.7）。
 *
 * 无网/网络异常时把打卡请求整体暂存本地（照片保留本地路径，不删文件），
 * 网络恢复后由 App.vue onShow / 今日任务页 onShow 触发 syncOfflineCheckins 逐条补传：
 * 先上传该条全部本地照片换 file_key，再 POST /checkin/offline-sync（逐条，失败即停本轮防雪崩）。
 *
 * 幂等：入队时客户端生成 UUIDv7 写入 req.id，补传/重试携带同一 ID，
 * 服务端发现该 ID 已存在则直接幂等返回（见 mp CheckinService.doCheckinLocked）。
 */

import { apiOfflineSync, apiUploadLocal, CheckinReqPayload } from '@/services/api'

/** 队列 storage key */
const KEY_QUEUE = 'offline_checkins'

/** 网络异常错误文案前缀（request.ts 网络失败 / api.ts 上传失败） */
export const NETWORK_ERR_PREFIX = '网络异常'

/** 队列条目本地照片引用：item=检查项名（照片唯一归属逐项） */
export type OfflinePhoto = {
  item: string
  local_path: string
}

/** 队列条目 */
export type OfflineEntry = {
  /** 打卡完整请求体（photos/check_items[].photos 留空，补传时回填 file_key） */
  req: CheckinReqPayload
  photos_local: OfflinePhoto[]
  saved_at: string
}

/** 生成 UUIDv7（毫秒时间序 + 随机位；无安全随机源时退化为 Math.random） */
export function uuidv7(): string {
  const ts = Date.now()
  const rnd = (n: number): string => {
    let s = ''
    for (let i = 0; i < n; i++) {
      s += Math.floor(Math.random() * 16).toString(16)
    }
    return s
  }
  // 48bit 时间戳 + version 7 + 12bit 随机 + variant 10xx + 62bit 随机
  const tsHex = ('000000000000' + ts.toString(16)).slice(-12)
  return (
    tsHex.slice(0, 8) + '-' + tsHex.slice(8, 12) + '-7' + rnd(3) +
    '-' + (8 + Math.floor(Math.random() * 4)).toString(16) + rnd(3) +
    '-' + rnd(12)
  )
}

/** 读队列（JSON 容错，损坏时重置为空） */
export function listOfflineCheckins(): OfflineEntry[] {
  const raw = uni.getStorageSync(KEY_QUEUE) as string
  if (raw == '') return []
  try {
    const arr = JSON.parse(raw) as OfflineEntry[]
    return Array.isArray(arr) ? arr : []
  } catch (e) {
    return []
  }
}

/** 待补传条数 */
export function offlineCount(): number {
  return listOfflineCheckins().length
}

/** 入队（req.id 为空时自动补 UUIDv7 作为幂等键） */
export function enqueueOfflineCheckin(req: CheckinReqPayload, photosLocal: OfflinePhoto[]): void {
  if (req.id == null || req.id == '') {
    req.id = uuidv7()
  }
  const list = listOfflineCheckins()
  list.push({
    req: req,
    photos_local: photosLocal,
    saved_at: new Date().toISOString()
  })
  uni.setStorageSync(KEY_QUEUE, JSON.stringify(list))
}

/** 出队头部一条（补传成功后调用） */
function dequeueFirst(): void {
  const list = listOfflineCheckins()
  list.shift()
  uni.setStorageSync(KEY_QUEUE, JSON.stringify(list))
}

/** 单飞锁：防止 onShow 并发触发多轮补传 */
let syncing = false

/**
 * 补传一轮（有网才跑；单飞）。
 * 逐条：上传本地照片 → 回填 file_key → 单条 offline-sync；
 * 任一条失败（网络/业务）保留该条及后续并停止本轮，避免弱网雪崩。
 * @returns {done, left} 本轮成功条数与剩余条数
 */
export function syncOfflineCheckins(): Promise<{ done: number; left: number }> {
  return new Promise<{ done: number; left: number }>((resolve) => {
    if (syncing) {
      resolve({ done: 0, left: offlineCount() })
      return
    }
    uni.getNetworkType({
      success: (net) => {
        if (net.networkType == 'none') {
          resolve({ done: 0, left: offlineCount() })
          return
        }
        syncing = true
        runQueue()
          .then((done) => {
            syncing = false
            resolve({ done: done, left: offlineCount() })
          })
          .catch(() => {
            syncing = false
            resolve({ done: 0, left: offlineCount() })
          })
      },
      fail: () => {
        resolve({ done: 0, left: offlineCount() })
      }
    })
  })
}

/** 顺序处理队列：返回本轮成功条数 */
async function runQueue(): Promise<number> {
  let done = 0
  while (true) {
    const list = listOfflineCheckins()
    if (list.length == 0) break
    const entry = list[0]
    try {
      await syncOne(entry)
      dequeueFirst()
      done++
    } catch (e) {
      // 失败保留并停止本轮
      break
    }
  }
  return done
}

/** 补传单条：上传照片换 file_key 回填 req，再调 offline-sync；服务端业务失败也视为失败（保留重试） */
async function syncOne(entry: OfflineEntry): Promise<void> {
  const req = entry.req
  // 上传本地照片，按检查项名归组 file_key
  const keysByItem: Record<string, string[]> = {}
  for (let i = 0; i < entry.photos_local.length; i++) {
    const ph = entry.photos_local[i]
    const r = await apiUploadLocal(ph.local_path, 'checkin')
    if (keysByItem[ph.item] == null) keysByItem[ph.item] = []
    keysByItem[ph.item].push(r.file_key)
  }
  // 回填逐项 photos（照片唯一归属逐项，无记录级照片）
  req.check_items.forEach((ci) => {
    ci.photos = keysByItem[ci.name] ?? []
  })
  const res = await apiOfflineSync([req])
  // 服务端逐条处理：出现在 failed 列表视为本条失败（保留在队列，下轮重试；幂等键保证不产生重复）
  if (res.failed.length > 0) {
    throw new Error(res.failed[0].message || '补传失败')
  }
}
