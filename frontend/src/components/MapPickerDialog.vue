<template>
  <el-dialog
    :model-value="visible"
    title="地图选点"
    width="720px"
    :close-on-click-modal="false"
    destroy-on-close
    @update:model-value="(v: boolean) => emit('update:visible', v)"
    @opened="handleOpened"
    @closed="destroyMap"
  >
    <div v-loading="loading" :element-loading-text="loadingText" class="map-picker">
      <div class="map-wrap">
        <div ref="mapEl" class="map-canvas" />
      </div>
      <div class="picker-panel">
        <!-- 地点搜索：放在右侧面板，避开地图画布事件层 -->
        <el-input
          v-model="keyword"
          size="small"
          placeholder="搜索地点、小区、路名"
          clearable
          :prefix-icon="Search"
          @input="onKeywordInput"
          @keyup.enter="doSearch"
          @clear="searchResults = []"
        />
        <div v-if="searchResults.length > 0" class="search-results">
          <div
            v-for="(p, i) in searchResults"
            :key="i"
            class="search-result-item"
            @click="pickResult(p)"
          >
            <div class="result-title">{{ p.title }}</div>
            <div class="result-address">{{ p.address }}</div>
          </div>
        </div>
        <div v-else-if="searchError != ''" class="search-results">
          <div class="search-result-empty">{{ searchError }}</div>
        </div>
        <div v-else-if="searchEmpty" class="search-results">
          <div class="search-result-empty">未找到相关地点</div>
        </div>
        <div class="coord-display">
          <div class="coord-item">
            <span class="coord-label">经度</span>
            <span class="coord-value">{{ position ? position.lng.toFixed(6) : '--' }}</span>
          </div>
          <div class="coord-item">
            <span class="coord-label">纬度</span>
            <span class="coord-value">{{ position ? position.lat.toFixed(6) : '--' }}</span>
          </div>
        </div>
        <div class="panel-tip">点击地图放置标记，或直接拖拽标记微调位置</div>
        <el-button size="small" :icon="Aim" :loading="locating" @click="locate">使用当前定位</el-button>
      </div>
    </div>
    <template #footer>
      <el-button @click="emit('update:visible', false)">取消</el-button>
      <el-button type="primary" :disabled="!position" @click="handleConfirm">确定</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, shallowRef } from 'vue'
import { ElMessage } from 'element-plus'
import { Aim, Search } from '@element-plus/icons-vue'
import { searchMapPlaces, type MapPlace } from '@/api/map'

// 腾讯地图 GL 为全局变量注入，无官方类型包，统一按 any 使用
declare global {
  interface Window {
    TMap?: any
  }
}

interface LngLat {
  lng: number
  lat: number
}

const props = defineProps<{
  visible: boolean
  apiKey: string
  longitude?: number | null
  latitude?: number | null
}>()

const emit = defineEmits<{
  'update:visible': [v: boolean]
  confirm: [pos: LngLat]
}>()

// 默认中心：杭州（GCJ-02）
const DEFAULT_CENTER: LngLat = { lng: 120.155, lat: 30.274 }

const mapEl = ref<HTMLElement>()
const loading = ref(false)
const loadingText = ref('地图加载中…')
const locating = ref(false)
const position = ref<LngLat | null>(null)

// ===== 地点搜索 =====
const keyword = ref('')
const searchResults = ref<MapPlace[]>([])
const searchEmpty = ref(false)
const searchError = ref('')
let searchTimer: ReturnType<typeof setTimeout> | null = null

function onKeywordInput() {
  if (searchTimer) clearTimeout(searchTimer)
  searchEmpty.value = false
  searchError.value = ''
  if (keyword.value.trim() == '') {
    searchResults.value = []
    return
  }
  searchTimer = setTimeout(doSearch, 400)
}

async function doSearch() {
  const kw = keyword.value.trim()
  if (kw == '') return
  searchError.value = ''
  try {
    // 传当前选中点/地图中心作为偏向坐标，附近结果优先
    const loc = position.value ? `${position.value.lat},${position.value.lng}` : undefined
    const list = await searchMapPlaces(kw, loc)
    searchResults.value = list
    searchEmpty.value = list.length == 0
  } catch (e: any) {
    // 搜索失败在结果区直接展示原因（toast 可能被弹窗遮挡注意不到）
    searchResults.value = []
    searchEmpty.value = false
    searchError.value = e?.message || '地点搜索失败'
  }
}

function pickResult(p: MapPlace) {
  searchResults.value = []
  searchEmpty.value = false
  if (!map.value) return
  const latLng = new window.TMap!.LatLng(p.lat, p.lng)
  map.value.setCenter(latLng)
  map.value.setZoom(17)
  setPosition(p.lng, p.lat)
}

// shallowRef：地图实例不可被 Vue 响应式代理包装（GL 内部 Worker postMessage 会报错）
const map = shallowRef<any>(null)
const markerLayer = shallowRef<any>(null)
let dragging = false
// document 级 mouseup 兜底（endDrag），关闭弹窗时须移除
let endDragFn: (() => void) | null = null

// ===== GL JS API 幂等加载（全局只插一次 script）=====
let loaderPromise: Promise<any> | null = null

function loadTMap(key: string): Promise<any> {
  if (window.TMap) return Promise.resolve(window.TMap)
  if (loaderPromise) return loaderPromise
  loaderPromise = new Promise((resolve, reject) => {
    const script = document.createElement('script')
    script.charset = 'utf-8'
    script.src = `https://map.qq.com/api/gljs?v=1.exp&key=${encodeURIComponent(key)}`
    script.onload = () => (window.TMap ? resolve(window.TMap) : reject(new Error('TMap 未定义')))
    script.onerror = () => {
      loaderPromise = null
      reject(new Error('腾讯地图脚本加载失败'))
    }
    document.head.appendChild(script)
  })
  return loaderPromise
}

// ===== 初始中心：已有坐标 → 浏览器定位 → 杭州 =====
async function resolveCenter(): Promise<{ center: LngLat; hasCoord: boolean }> {
  if (props.longitude != null && props.latitude != null) {
    return { center: { lng: props.longitude, lat: props.latitude }, hasCoord: true }
  }
  const located = await browserLocate(false)
  if (located) return { center: located, hasCoord: false }
  return { center: DEFAULT_CENTER, hasCoord: false }
}

// 浏览器定位；silent=false 时失败弹提示（「使用当前定位」按钮用）
function browserLocate(silent = true): Promise<LngLat | null> {
  return new Promise((resolve) => {
    if (!navigator.geolocation) {
      if (!silent) ElMessage.warning('当前浏览器不支持定位')
      resolve(null)
      return
    }
    navigator.geolocation.getCurrentPosition(
      (pos) => resolve({ lng: pos.coords.longitude, lat: pos.coords.latitude }),
      () => {
        if (!silent) ElMessage.warning('定位失败，请检查浏览器定位权限后重试')
        resolve(null)
      },
      { timeout: 8000, maximumAge: 60000 }
    )
  })
}

// ===== 地图初始化 =====
async function handleOpened() {
  loading.value = true
  loadingText.value = '地图加载中…'
  try {
    const TMap = await loadTMap(props.apiKey)
    const { center, hasCoord } = await resolveCenter()
    const tmapCenter = new TMap.LatLng(center.lat, center.lng)

    map.value = new TMap.Map(mapEl.value, {
      center: tmapCenter,
      zoom: hasCoord ? 17 : 16,
      viewMode: '2D'
    })
    // GL 默认控件对选点无用，关掉保持面板干净
    map.value.removeControl(TMap.constants.DEFAULT_CONTROL_ID.ZOOM)
    map.value.removeControl(TMap.constants.DEFAULT_CONTROL_ID.SCALE)

    // 点标记图层：阻止事件冒泡，避免按住标记时触发地图 click 挪点
    markerLayer.value = new TMap.MultiMarker({
      id: 'picker-marker',
      map: map.value,
      geometries: []
    })
    markerLayer.value.setStopPropagation(true)

    // 手动拖拽：GL 的 MultiMarker 无 draggable 配置，
    // 用 mousedown/mousemove/mouseup 实现直接拖拽（官方推荐编辑器太重，不适合单点选点）
    const endDrag = () => {
      if (!dragging) return
      dragging = false
      map.value?.setDraggable(true)
    }
    endDragFn = endDrag
    markerLayer.value.on('mousedown', () => {
      dragging = true
      map.value.setDraggable(false)
    })
    // 标记上 mouseup 会被 stopPropagation 拦截，三层兜底：标记层 / 地图 / document
    markerLayer.value.on('mouseup', endDrag)
    map.value.on('mousemove', (evt: any) => {
      if (dragging && evt?.latLng) setPosition(evt.latLng.getLng(), evt.latLng.getLat())
    })
    map.value.on('mouseup', endDrag)
    document.addEventListener('mouseup', endDrag)

    // 点击地图放置/移动标记
    map.value.on('click', (evt: any) => {
      if (evt?.latLng) setPosition(evt.latLng.getLng(), evt.latLng.getLat())
    })

    // 已有坐标直接落标记
    if (hasCoord) setPosition(center.lng, center.lat)
  } catch {
    ElMessage.error('地图加载失败，请确认腾讯地图 Key 有效且已配置域名白名单')
  } finally {
    loading.value = false
  }
}

function setPosition(lng: number, lat: number) {
  position.value = { lng, lat }
  markerLayer.value?.setGeometries([
    { id: 'picker', position: new window.TMap!.LatLng(lat, lng) }
  ])
}

// 「使用当前定位」：定位成功则移图落点，失败 toast
async function locate() {
  locating.value = true
  try {
    const pos = await browserLocate(false)
    if (!pos || !map.value) return
    const latLng = new window.TMap!.LatLng(pos.lat, pos.lng)
    map.value.setCenter(latLng)
    map.value.setZoom(17)
    setPosition(pos.lng, pos.lat)
  } finally {
    locating.value = false
  }
}

function handleConfirm() {
  if (!position.value) return
  // 统一 6 位小数回填
  emit('confirm', {
    lng: Number(position.value.lng.toFixed(6)),
    lat: Number(position.value.lat.toFixed(6))
  })
  emit('update:visible', false)
}

function destroyMap() {
  dragging = false
  if (endDragFn) {
    document.removeEventListener('mouseup', endDragFn)
    endDragFn = null
  }
  if (searchTimer) {
    clearTimeout(searchTimer)
    searchTimer = null
  }
  keyword.value = ''
  searchResults.value = []
  searchEmpty.value = false
  searchError.value = ''
  markerLayer.value = null
  if (map.value) {
    map.value.destroy()
    map.value = null
  }
  position.value = null
}
</script>

<style scoped lang="scss">
.map-picker {
  display: flex;
  gap: $spacing-md;
  min-height: 420px;
}

.map-wrap {
  position: relative;
  flex: 1;
  min-width: 0;
}

.map-canvas {
  width: 100%;
  height: 420px;
  border-radius: $radius-card;
  overflow: hidden;
  background: $color-bg-page;
}

.search-results {
  background: #fff;
  border: 1px solid $color-border;
  border-radius: $radius-card;
  max-height: 200px;
  overflow-y: auto;
}

.search-result-item {
  padding: 8px 12px;
  cursor: pointer;

  &:hover {
    background: $color-bg-page;
  }

  .result-title {
    font-size: $font-size-body;
    color: $color-text-primary;
  }

  .result-address {
    font-size: $font-size-aux;
    color: $color-text-secondary;
    margin-top: 2px;
  }
}

.search-result-empty {
  padding: 12px;
  font-size: $font-size-aux;
  color: $color-text-secondary;
  text-align: center;
}

.picker-panel {
  width: 200px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: $spacing-md;
}

.coord-display {
  display: flex;
  flex-direction: column;
  gap: $spacing-sm;
}

.coord-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: $spacing-sm $spacing-md;
  background: $color-bg-page;
  border-radius: $radius-card;

  .coord-label {
    font-size: $font-size-aux;
    color: $color-text-secondary;
  }

  .coord-value {
    font-family: monospace;
    font-size: $font-size-body;
    color: $color-text-primary;
  }
}

.panel-tip {
  font-size: $font-size-aux;
  color: $color-text-secondary;
  line-height: 1.6;
}
</style>
