/**
 * 坐标工具：WGS84 → GCJ-02（火星坐标）本地转换 + 统一取定位封装。
 *
 * 背景：App 端 manifest 只勾选了系统定位（未配高德/百度 SDK key），
 * 系统定位只返回 WGS84 原始坐标（uni.getLocation type:'gcj02' 会报 not support）；
 * 后端围栏校验与 PC 腾讯地图均为 GCJ-02，故 App 端取 WGS84 后本地纠偏。
 * 转换算法为公开的 eviltransform 实现，与高德/腾讯坐标系一致。
 */

const PI = 3.1415926535897932384626
const AXIS = 6378245.0 // 克拉索夫斯基椭球长半轴
const EE = 0.00669342162296594323 // 偏心率平方

/** 是否超出中国范围（境外不做纠偏，直接返回原坐标） */
function outOfChina(lng: number, lat: number): boolean {
  return lng < 72.004 || lng > 137.8347 || lat < 0.8293 || lat > 55.8271
}

function transformLat(x: number, y: number): number {
  let ret = -100.0 + 2.0 * x + 3.0 * y + 0.2 * y * y + 0.1 * x * y + 0.2 * Math.sqrt(Math.abs(x))
  ret += ((20.0 * Math.sin(6.0 * x * PI) + 20.0 * Math.sin(2.0 * x * PI)) * 2.0) / 3.0
  ret += ((20.0 * Math.sin(y * PI) + 40.0 * Math.sin((y / 3.0) * PI)) * 2.0) / 3.0
  ret += ((160.0 * Math.sin((y / 12.0) * PI) + 320 * Math.sin((y * PI) / 30.0)) * 2.0) / 3.0
  return ret
}

function transformLng(x: number, y: number): number {
  let ret = 300.0 + x + 2.0 * y + 0.1 * x * x + 0.1 * x * y + 0.1 * Math.sqrt(Math.abs(x))
  ret += ((20.0 * Math.sin(6.0 * x * PI) + 20.0 * Math.sin(2.0 * x * PI)) * 2.0) / 3.0
  ret += ((20.0 * Math.sin(x * PI) + 40.0 * Math.sin((x / 3.0) * PI)) * 2.0) / 3.0
  ret += ((150.0 * Math.sin((x / 12.0) * PI) + 300.0 * Math.sin((x / 30.0) * PI)) * 2.0) / 3.0
  return ret
}

/** WGS84 → GCJ-02；返回 [lng, lat] */
export function wgs84ToGcj02(lng: number, lat: number): [number, number] {
  if (outOfChina(lng, lat)) return [lng, lat]
  let dLat = transformLat(lng - 105.0, lat - 35.0)
  let dLng = transformLng(lng - 105.0, lat - 35.0)
  const radLat = (lat / 180.0) * PI
  let magic = Math.sin(radLat)
  magic = 1 - EE * magic * magic
  const sqrtMagic = Math.sqrt(magic)
  dLat = (dLat * 180.0) / (((AXIS * (1 - EE)) / (magic * sqrtMagic)) * PI)
  dLng = (dLng * 180.0) / ((AXIS / sqrtMagic) * Math.cos(radLat) * PI)
  return [lng + dLng, lat + dLat]
}

export type Gcj02Location = {
  longitude: number
  latitude: number
  /** 精度（米），可能为 0（部分系统不返回） */
  accuracy: number
}

/**
 * 统一取定位（GCJ-02）：优先直接要 gcj02（配了高德 SDK / 小程序端可用）；
 * 系统定位不支持 gcj02 时自动降级 wgs84 + 本地纠偏。
 */
export function getLocationGcj02(
  success: (loc: Gcj02Location) => void,
  fail?: (errMsg: string) => void
) {
  uni.getLocation({
    type: 'gcj02',
    success: (res) => {
      success({ longitude: res.longitude, latitude: res.latitude, accuracy: res.accuracy || 0 })
    },
    fail: (err) => {
      const msg = err != null && err.errMsg != null ? String(err.errMsg) : ''
      if (msg.indexOf('not support') < 0) {
        if (fail != null) fail(msg)
        return
      }
      // 系统定位（未配 SDK key）：只支持 wgs84，本地转 GCJ-02
      uni.getLocation({
        type: 'wgs84',
        success: (res) => {
          const r = wgs84ToGcj02(res.longitude, res.latitude)
          success({ longitude: r[0], latitude: r[1], accuracy: res.accuracy || 0 })
        },
        fail: (e2) => {
          if (fail != null) fail(e2 != null && e2.errMsg != null ? String(e2.errMsg) : '')
        }
      })
    }
  })
}
