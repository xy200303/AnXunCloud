// 巡查类型字典（patrol_type）共享加载：attrs.category 分大类
// daily_patrol=日常巡逻 / special=专项检查；下拉按大类分组展示
import { computed, ref } from 'vue'
import { listDictOptions } from '@/api/dict'

export interface PatrolTypeOption {
  value: string
  label: string
  category: string
}

export interface PatrolTypeGroup {
  label: string
  options: PatrolTypeOption[]
}

// 字典读取失败时回落到存量 4 类，保持旧版行为（dict-options 免权限，正常不会走到）
const FALLBACK: PatrolTypeOption[] = [
  { value: 'safety', label: '安全巡查', category: 'daily_patrol' },
  { value: 'equipment', label: '设备设施专项', category: 'special' },
  { value: 'environment', label: '环境巡查', category: 'special' },
  { value: 'building', label: '楼栋巡查', category: 'special' }
]

// 模块级缓存：字典项极少变动，整个会话加载一次即可
const options = ref<PatrolTypeOption[]>([])
let loadingPromise: Promise<void> | null = null

function ensureLoaded() {
  if (options.value.length) return
  if (!loadingPromise) {
    // silent + 失败回落：兜底保持旧版 4 类行为
    loadingPromise = listDictOptions('patrol_type', true)
      .then((list) => {
        options.value = (list || []).map((x) => ({ value: x.value, label: x.label, category: x.attrs?.category || '' }))
        if (!options.value.length) options.value = FALLBACK
      })
      .catch(() => {
        options.value = FALLBACK
      })
  }
}

export function usePatrolTypes() {
  ensureLoaded()

  // 按大类分组：日常巡逻 / 专项检查 / 其他（未标记 category 的兜底）
  const patrolTypeGroups = computed<PatrolTypeGroup[]>(() => {
    const groups: PatrolTypeGroup[] = []
    const daily = options.value.filter((o) => o.category === 'daily_patrol')
    const special = options.value.filter((o) => o.category === 'special')
    const other = options.value.filter((o) => o.category !== 'daily_patrol' && o.category !== 'special')
    if (daily.length) groups.push({ label: '日常巡逻', options: daily })
    if (special.length) groups.push({ label: '专项检查', options: special })
    if (other.length) groups.push({ label: '其他', options: other })
    return groups
  })

  const patrolTypeLabel = (v: string) => options.value.find((o) => o.value === v)?.label || v || '--'

  return { patrolTypes: options, patrolTypeGroups, patrolTypeLabel }
}
